package gungnir

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// Sandbox runs commands in an isolated environment, one per leaf (spec §2.2, §6). The devcontainer
// driver lives in Gleipnir.
type Sandbox interface {
	// Up starts the sandbox for one leaf, and refuses a configuration that breaks the hardening rules.
	Up(ctx context.Context, spec SandboxSpec) (Env, error)
	// Exec runs a command in it. A command that exits non-zero is a result, not an error. When ctx
	// ends, the command and everything it started are killed.
	Exec(ctx context.Context, env Env, cmd Cmd) (ExecResult, error)
	// Down stops everything Up started. It is safe to call again.
	Down(ctx context.Context, env Env) error
}

// SandboxSpec says what a sandbox holds. Everything in it is a host path or a name; the driver
// decides how the engine sees them.
type SandboxSpec struct {
	Run    string   // the run's id; it labels the sandbox, so doctor can find leftovers
	Name   string   // the leaf's path from the project, such as "repos/app"
	Parent string   // the leaf's parent umbrella: the sandbox mirrors its layout
	Leaf   string   // the leaf's working copy: mounted read-write
	GitDir string   // the leaf's git dir, inside Parent/.git/modules: mounted read-only; empty for a scratch folder
	Egress []string // host names the firewall opens on port 443, besides DNS
	// Stack is the image of the stack layer of the realm that provides this leaf: pinned by digest, or the
	// local tag of an image built from the realm's build context (ValidStack). Empty means the driver's own images. A run with several realms starts leaves with different
	// stacks.
	Stack string
	// Services are the containers the leaf's sandbox starts beside the work container, such as a
	// database: the services of the leaf's realm, and of each leaf it consumes (spec §6.1).
	Services []Service
	// Scratch makes the sandbox hold one plain folder, not a leaf: a planner's workspace (spec §3.3).
	// It has no git dir.
	Scratch bool
	// Models is the shared model server the sandbox reaches over an internal network, or nil for none
	// (local-models spec §3.6).
	Models *ModelServer
}

// ModelServer is the shared model server a sandbox reaches: by Name, on Port, over Network.
type ModelServer struct {
	Network string
	Name    string
	Port    int
}

// Service is a container that runs beside the work container.
type Service struct {
	Name  string            // the compose service name, and the host name the work container reaches it by
	Image string            // pinned by digest
	Env   map[string]string // the service's environment: made-up development values, never a secret
	Ports []int             // the TCP ports the work container may reach; the firewall opens these and no others
	// Command replaces the image's own command, as an argument vector (no shell), for an image that does
	// nothing without one, such as an object store that must be told to serve. Empty keeps the image's.
	Command []string
}

// MaxServices is how many services one sandbox may start, and MaxServiceCommand and MaxServiceArg bound
// the command of one.
const (
	MaxServices       = 8
	MaxServiceCommand = 16
	MaxServiceArg     = 256
)

var (
	image       = regexp.MustCompile(`^[a-z0-9][a-z0-9./_-]*(:[A-Za-z0-9._-]+)?@sha256:[0-9a-f]{64}$`)
	serviceName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	modelName   = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,62}$`)
	secretName  = regexp.MustCompile(`(?i)(API_?KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)`)
)

// ValidImage reports whether ref is an image reference pinned by digest: a pull can only ever give the
// image that was reviewed.
func ValidImage(ref string) bool { return image.MatchString(ref) }

var (
	runID    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	hostName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]([a-z0-9-]*[a-z0-9])?$`)
)

// ValidHost reports whether h is a plain host name: lower case, with a dot, and no scheme, port,
// wildcard or address. The firewall opens names, never ranges.
func ValidHost(h string) bool { return hostName.MatchString(h) }

// Validate refuses a spec the driver couldn't build exactly as described.
func (s SandboxSpec) Validate() error {
	var errs []error
	if !runID.MatchString(s.Run) {
		errs = append(errs, fmt.Errorf("run %q is not a run id", s.Run))
	}
	if !filepath.IsLocal(s.Name) {
		errs = append(errs, fmt.Errorf("name %q must be a path inside the project", s.Name))
	}
	paths := map[string]string{"parent": s.Parent, "leaf": s.Leaf}
	switch {
	case s.Scratch && s.GitDir != "":
		errs = append(errs, fmt.Errorf("a scratch sandbox holds no git dir, but %q is given", s.GitDir))
	case !s.Scratch:
		paths["git dir"] = s.GitDir
	}
	for what, p := range paths {
		if !filepath.IsAbs(p) {
			errs = append(errs, fmt.Errorf("%s %q must be an absolute path", what, p))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if rel, err := filepath.Rel(s.Parent, s.Leaf); err != nil || !filepath.IsLocal(rel) {
		errs = append(errs, fmt.Errorf("leaf %q is not inside the parent umbrella %q", s.Leaf, s.Parent))
	}
	if !s.Scratch {
		if rel, err := filepath.Rel(filepath.Join(s.Parent, ".git", "modules"), s.GitDir); err != nil || !filepath.IsLocal(rel) {
			errs = append(errs, fmt.Errorf("git dir %q is not inside %s", s.GitDir, filepath.Join(s.Parent, ".git", "modules")))
		}
	}
	for _, h := range s.Egress {
		if !ValidHost(h) {
			errs = append(errs, fmt.Errorf("egress %q is not a plain host name (no scheme, port, wildcard or address)", h))
		}
	}
	if s.Stack != "" && !ValidStack(s.Stack) {
		errs = append(errs, fmt.Errorf("stack %q is neither an image pinned by digest nor the tag of a built stack", s.Stack))
	}
	if m := s.Models; m != nil {
		if !modelName.MatchString(m.Network) {
			errs = append(errs, fmt.Errorf("models network %q must match %s", m.Network, modelName))
		}
		if !modelName.MatchString(m.Name) {
			errs = append(errs, fmt.Errorf("models name %q must match %s", m.Name, modelName))
		}
		if m.Port < 1 || m.Port > 65535 {
			errs = append(errs, fmt.Errorf("models port %d is outside 1 to 65535", m.Port))
		}
	}
	return errors.Join(append(errs, ValidateServices(s.Services))...)
}

// ValidateServices refuses services a driver could not start exactly as described.
func ValidateServices(services []Service) error {
	var errs []error
	if len(services) > MaxServices {
		errs = append(errs, fmt.Errorf("%d services; a sandbox starts at most %d", len(services), MaxServices))
	}
	seen := map[string]bool{}
	for _, v := range services {
		switch {
		case !serviceName.MatchString(v.Name):
			errs = append(errs, fmt.Errorf("service name %q must be lower case words joined by dashes", v.Name))
		case v.Name == "work" || v.Name == "fw":
			errs = append(errs, fmt.Errorf("service name %q is taken by the sandbox's own containers", v.Name))
		case seen[v.Name]:
			errs = append(errs, fmt.Errorf("service %s is listed twice", v.Name))
		}
		seen[v.Name] = true
		if !ValidImage(v.Image) {
			errs = append(errs, fmt.Errorf("service %s: image %q is not pinned by digest", v.Name, v.Image))
		}
		if len(v.Ports) == 0 {
			errs = append(errs, fmt.Errorf("service %s names no port, so the work container could not reach it", v.Name))
		}
		ports := map[int]bool{}
		for _, p := range v.Ports {
			if p < 1 || p > 65535 || ports[p] {
				errs = append(errs, fmt.Errorf("service %s: port %d is out of range or listed twice", v.Name, p))
			}
			ports[p] = true
		}
		if len(v.Command) > MaxServiceCommand {
			errs = append(errs, fmt.Errorf("service %s: a command of %d arguments; at most %d", v.Name, len(v.Command), MaxServiceCommand))
		}
		for i, a := range v.Command {
			switch {
			case a == "" || len(a) > MaxServiceArg:
				errs = append(errs, fmt.Errorf("service %s: command argument %d is empty or over %d bytes", v.Name, i+1, MaxServiceArg))
			case strings.ContainsAny(a, "\x00\n\r"):
				errs = append(errs, fmt.Errorf("service %s: command argument %d has a NUL or a line break", v.Name, i+1))
			case strings.Contains(a, "$"):
				errs = append(errs, fmt.Errorf("service %s: command argument %d has a '$'; compose would expand it from the operator's environment, so give the value literally", v.Name, i+1))
			}
		}
		for k, val := range v.Env {
			switch {
			case k == "" || strings.ContainsAny(k, "=\x00"):
				errs = append(errs, fmt.Errorf("service %s: env name %q is not a variable name", v.Name, k))
			case secretName.MatchString(k):
				errs = append(errs, fmt.Errorf("service %s: env %s names a credential; a service in a sandbox takes none (use the image's mode without a password)", v.Name, k))
			case strings.Contains(val, "$"):
				errs = append(errs, fmt.Errorf("service %s: env %s has a value with a '$'; compose would expand it from the operator's environment, so give the value literally", v.Name, k))
			}
		}
	}
	return errors.Join(errs...)
}

// Layout holds where the sandbox mounts the leaf and its git dir, relative to the sandbox's root:
// the parent umbrella's layout, so the leaf's .git file still finds its git dir (sandbox spike).
type Layout struct{ Leaf, GitDir string }

// Layout returns the mirrored layout of a valid spec.
func (s SandboxSpec) Layout() Layout {
	leaf, _ := filepath.Rel(s.Parent, s.Leaf)
	if s.Scratch {
		return Layout{Leaf: filepath.ToSlash(leaf)}
	}
	git, _ := filepath.Rel(s.Parent, s.GitDir)
	return Layout{Leaf: filepath.ToSlash(leaf), GitDir: filepath.ToSlash(git)}
}

// Env is a running sandbox. It is plain data, so run.json can record it for doctor.
type Env struct {
	Name      string `json:"name"`      // the leaf's path from the project
	Project   string `json:"project"`   // the compose project that holds the containers
	Container string `json:"container"` // the container commands run in
	Workdir   string `json:"workdir"`   // the leaf's path inside the container
}

// Cmd is one command to run in a sandbox, without a shell.
type Cmd struct {
	Argv   []string
	Dir    string            // relative to the leaf's top; "" is the top
	Env    map[string]string // set for this command only; values never appear on a command line
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Validate refuses a command that couldn't run.
func (c Cmd) Validate() error {
	switch {
	case len(c.Argv) == 0 || c.Argv[0] == "":
		return errors.New("a sandbox command needs a program")
	case c.Dir != "" && !filepath.IsLocal(c.Dir):
		return fmt.Errorf("dir %q must be a path inside the leaf", c.Dir)
	}
	for k := range c.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") {
			return fmt.Errorf("env name %q is not a variable name", k)
		}
	}
	return nil
}

// ExecResult is how a command ended.
type ExecResult struct{ ExitCode int }

// Runner runs commands in one sandbox. A Task carries one when its session must run in a sandbox.
type Runner interface {
	Run(ctx context.Context, c Cmd) (ExecResult, error)
}

type bound struct {
	sb  Sandbox
	env Env
}

func (b bound) Run(ctx context.Context, c Cmd) (ExecResult, error) { return b.sb.Exec(ctx, b.env, c) }

// Bind returns a Runner for the sandbox env of sb.
func Bind(sb Sandbox, env Env) Runner { return bound{sb, env} }

// ErrNoSandbox is what an adapter that must run in a sandbox returns for a task without one.
var ErrNoSandbox = errors.New("this adapter runs only in a sandbox, and the task has none")
