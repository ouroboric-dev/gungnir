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
	GitDir string   // the leaf's git dir, inside Parent/.git/modules: mounted read-only
	Egress []string // host names the firewall opens on port 443, besides DNS
}

var (
	runID    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	hostName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]([a-z0-9-]*[a-z0-9])?$`)
)

// Validate refuses a spec the driver couldn't build exactly as described.
func (s SandboxSpec) Validate() error {
	var errs []error
	if !runID.MatchString(s.Run) {
		errs = append(errs, fmt.Errorf("run %q is not a run id", s.Run))
	}
	if !filepath.IsLocal(s.Name) {
		errs = append(errs, fmt.Errorf("name %q must be a path inside the project", s.Name))
	}
	for what, p := range map[string]string{"parent": s.Parent, "leaf": s.Leaf, "git dir": s.GitDir} {
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
	if rel, err := filepath.Rel(filepath.Join(s.Parent, ".git", "modules"), s.GitDir); err != nil || !filepath.IsLocal(rel) {
		errs = append(errs, fmt.Errorf("git dir %q is not inside %s", s.GitDir, filepath.Join(s.Parent, ".git", "modules")))
	}
	for _, h := range s.Egress {
		if !hostName.MatchString(h) {
			errs = append(errs, fmt.Errorf("egress %q is not a plain host name (no scheme, port, wildcard or address)", h))
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
