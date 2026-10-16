package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ouroboric-dev/gungnir"
)

// Factory is factory.yaml.
type Factory struct {
	Mode    gungnir.Mode            `yaml:"mode"`
	Gates   map[gungnir.Gate]string `yaml:"gates"`
	Roles   map[string]Role         `yaml:"roles"`
	Budget  Budget                  `yaml:"budget"`
	Privacy string                  `yaml:"privacy"`
	Sandbox *Sandbox                `yaml:"sandbox"` // required when a role runs a real agent (spec §6)
	Skills  Skills                  `yaml:"skills"`
}

// Sandbox says where the container engine is and how it sees the host's files (spec §6; sandbox
// spike, decided 2026-10-14). The factory sets it here and never reads the shell's CONTAINER_HOST.
type Sandbox struct {
	Engine string  `yaml:"engine"` // the engine's socket, such as unix:///mnt/wsl/podman-sockets/.../podman-user.sock
	Mounts []Mount `yaml:"mounts"` // host path prefixes the engine sees under another path
}

// Mount says that the engine sees the host path Host as Engine, such as a bind mount into /mnt/wsl.
type Mount struct {
	Host   string `yaml:"host"`
	Engine string `yaml:"engine"`
}

// Skills configures skill loading (spec §5.1).
type Skills struct {
	PrivateDir string `yaml:"private_dir"` // an operator's private skills, which run.json records
}

// Role maps a role to an agent adapter and a model (spec §5).
type Role struct {
	Agent  string `yaml:"agent"`
	Model  string `yaml:"model"`
	Script string `yaml:"script"` // scripted adapter only: the script, relative to factory.yaml
	Price  *Price `yaml:"price"`  // optional: dollars per million tokens, for a tool that reports no cost of its own (spec §5)
}

// Price is a model's price in dollars per million tokens.
type Price struct {
	Input      float64 `yaml:"input"`
	Output     float64 `yaml:"output"`
	CacheRead  float64 `yaml:"cache_read"`
	CacheWrite float64 `yaml:"cache_write"`
}

// Budget holds the caps. Both are required, and the factory ships no default values.
type Budget struct {
	PerRun Cap `yaml:"per_run"`
	PerDay Cap `yaml:"per_day"`
}

// Cap limits tokens and turns, and dollars when a role bills a paid API.
type Cap struct {
	Tokens int64   `yaml:"tokens"`
	Turns  int64   `yaml:"turns"`
	USD    float64 `yaml:"usd"`
}

// agents are the adapters factory.schema.json accepts.
var agents = []string{"claude-code", "opencode", "scripted"}

// KnownAgent reports whether agent is an adapter this schema accepts.
func KnownAgent(agent string) bool { return slices.Contains(agents, agent) }

// PaidAPI reports whether an adapter bills a paid API. Only adapters known to run on a
// subscription or at no cost return false, so a new adapter needs a spend limit until it is listed.
func PaidAPI(agent string) bool { return agent != "claude-code" && agent != "scripted" }

// CapsError lists the cap fields that factory.yaml lacks.
type CapsError struct {
	File    string
	Missing []string
	Paid    bool // a role bills a paid API, so the caps need usd too
}

func (e *CapsError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s has no complete budget caps, so nothing runs. Missing:\n", e.File)
	for _, m := range e.Missing {
		fmt.Fprintf(&b, "  - %s\n", m)
	}
	usd := ""
	if e.Paid {
		usd = ", usd: <dollars>"
	}
	b.WriteString("Set your own limits in it; the factory ships no defaults:\n  budget:\n")
	fmt.Fprintf(&b, "    per_run: { tokens: <tokens>, turns: <turns>%s }\n", usd)
	fmt.Fprintf(&b, "    per_day: { tokens: <tokens>, turns: <turns>%s }\n", usd)
	return b.String()
}

// LoadFactory reads and parses a factory.yaml file.
func LoadFactory(path string) (*Factory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFactory(path, data)
}

// ParseFactory parses factory.yaml. Incomplete caps give a *CapsError before any other check, so
// the operator learns what to set. The gate overrides are checked against the file's own mode.
func ParseFactory(file string, data []byte) (*Factory, error) {
	doc, err := decodeYAML(file, data)
	if err != nil {
		return nil, err
	}
	if err := checkCaps(file, doc); err != nil {
		return nil, err
	}
	if err := validate(file, "factory.schema.json", doc); err != nil {
		return nil, err
	}
	var f Factory
	if err := decodeStrict(file, data, &f); err != nil {
		return nil, err
	}
	if f.Mode == "" {
		f.Mode = gungnir.ModeKeyPoints
	}
	if f.Privacy == "" {
		f.Privacy = "standard"
	}
	if err := f.CheckGates(f.Mode); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if err := f.checkSandbox(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &f, nil
}

func checkCaps(file string, doc any) error {
	root, _ := doc.(map[string]any)
	roles, _ := root["roles"].(map[string]any)
	paid := ""
	for _, name := range sortedKeys(roles) {
		role, _ := roles[name].(map[string]any)
		// An unknown agent is the schema's to refuse, by name; it never asks for a spend limit.
		if agent, ok := role["agent"].(string); ok && KnownAgent(agent) && PaidAPI(agent) {
			paid = fmt.Sprintf("role %s runs on %s, which bills a paid API", name, agent)
			break
		}
	}
	budget, _ := root["budget"].(map[string]any)
	var missing []string
	for _, c := range []string{"per_run", "per_day"} {
		cp, ok := budget[c].(map[string]any)
		if !ok {
			missing = append(missing, "budget."+c)
			continue
		}
		for _, field := range []string{"tokens", "turns"} {
			if cp[field] == nil {
				missing = append(missing, "budget."+c+"."+field)
			}
		}
		if paid != "" && cp["usd"] == nil {
			missing = append(missing, "budget."+c+".usd ("+paid+")")
		}
	}
	if len(missing) > 0 {
		return &CapsError{File: file, Missing: missing, Paid: paid != ""}
	}
	return nil
}

// CheckGates refuses overrides that mode doesn't allow (spec §7.2). No override may touch a hard
// approval point. An override may only tighten, to stop; the one exception is design: auto, and
// only in unattended mode.
func (f *Factory) CheckGates(mode gungnir.Mode) error {
	for _, g := range sortedKeys(f.Gates) {
		switch v := f.Gates[g]; {
		case g.Hard():
			return fmt.Errorf("gates.%s: %s is a hard approval point, and no config can override it", g, g)
		case v == "auto" && g != gungnir.GateDesign:
			return fmt.Errorf("gates.%s: an override can only set stop; design: auto in unattended mode is the one exception", g)
		case v == "auto" && mode != gungnir.ModeUnattended:
			return fmt.Errorf("gates.design: auto is allowed only in unattended mode, not %s", mode)
		}
	}
	return nil
}

// checkSandbox refuses a real agent without a sandbox: an adapter for a real tool never runs on the
// host (spec §6), and a sandbox that names the host's root or lists a path twice is ambiguous.
func (f *Factory) checkSandbox() error {
	if f.Skills.PrivateDir != "" && !filepath.IsAbs(f.Skills.PrivateDir) {
		return fmt.Errorf("skills.private_dir %q must be an absolute path", f.Skills.PrivateDir)
	}
	if f.Sandbox == nil {
		for _, r := range sortedKeys(f.Roles) {
			if agent := f.Roles[r].Agent; agent != "scripted" {
				return fmt.Errorf("role %s runs on %s, which runs only in a sandbox; add a sandbox section (spec §6)", r, agent)
			}
		}
		return nil
	}
	seen := map[string]bool{}
	for _, m := range f.Sandbox.Mounts {
		for _, p := range []string{m.Host, m.Engine} {
			if !filepath.IsAbs(p) || filepath.Clean(p) != p {
				return fmt.Errorf("sandbox.mounts: %q must be a clean, absolute path", p)
			}
			if p == "/" {
				return fmt.Errorf("sandbox.mounts: the root %q is too wide", p)
			}
		}
		if seen[m.Host] {
			return fmt.Errorf("sandbox.mounts: host %q is listed twice", m.Host)
		}
		seen[m.Host] = true
	}
	return nil
}

func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
