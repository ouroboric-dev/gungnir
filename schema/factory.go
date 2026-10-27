package schema

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
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
	Realms  *Realms                 `yaml:"realms"` // where the realm leaves are, so a leaf's pinned realm can be found
	Graphs  *Graphs                 `yaml:"graphs"` // where the graph packs are, so --graph can name one
	Models  *Models                 `yaml:"models"` // the model list of the local models; required when a role names local/<name>
	Grader  *Grader                 `yaml:"grader"` // where the benchmark tasks and the hidden suites are, and the limits of a grading run (spec §11)
}

// Models says where the operator's list of local models is (local-models spec §3.1).
type Models struct {
	File string `yaml:"file"` // an absolute path
}

// Grader says where grading finds its inputs and how much it may use (spec §11, change benchmark-split).
// Nothing is defaulted: the operator sets every limit.
type Grader struct {
	TasksDir string   `yaml:"tasks_dir"` // a checkout of the task repo (mimir): <dir>/<task id>/{task.toml,instruction.md,environment/}
	SuiteDir string   `yaml:"suite_dir"` // the store of hidden suites: <dir>/<task id>/<suite>/tests/. It is never mounted into an agent's sandbox
	Timeout  Duration `yaml:"timeout"`   // the most one grading run's container may run
	Memory   string   `yaml:"memory"`    // the container's memory limit, such as 4g; "none" is no limit (an engine that cannot apply one)
	CPUs     int      `yaml:"cpus"`      // the container's CPUs; 0 is no limit
}

// Realms says where the operator keeps the realm leaves: a folder with one checkout of each, such as
// the yggdrasil umbrella (<dir>/midgard-react/pack.yaml). A project pins the version it needs, and a
// run uses the checkout only if it is at that version (change realm-leaf-naming).
type Realms struct {
	Dir string `yaml:"dir"`
}

// Graphs says where the operator keeps the graph packs: a folder with one pack per graph, such as the
// voluspa leaf (<dir>/software/pack.yaml). A run finds its graph there by name (change voluspa).
type Graphs struct {
	Dir string `yaml:"dir"`
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
	Agent     string `yaml:"agent"`
	Model     string `yaml:"model"`
	Script    string `yaml:"script"`    // scripted adapter only: the script, relative to factory.yaml
	Price     *Price `yaml:"price"`     // optional: dollars per million tokens, for a tool that reports no cost of its own (spec §5)
	Reasoning string `yaml:"reasoning"` // optional: low, medium, high or off; "" means unset, and the adapter applies high (spec §5)
}

// Price is a model's price in dollars per million tokens.
type Price struct {
	Input      float64 `yaml:"input"`
	Output     float64 `yaml:"output"`
	CacheRead  float64 `yaml:"cache_read"`
	CacheWrite float64 `yaml:"cache_write"`
}

// check refuses a price that is not a finite number: NaN makes every cap comparison false, and infinity never trips one.
// Every price is not negative; with paid, input and output are also above 0, as a local model's price is.
func (p Price) check(paid bool) error {
	for _, c := range []struct {
		field string
		v     float64
		above bool
	}{{"input", p.Input, paid}, {"output", p.Output, paid}, {"cache_read", p.CacheRead, false}, {"cache_write", p.CacheWrite, false}} {
		switch {
		case math.IsNaN(c.v) || math.IsInf(c.v, 0):
			return fmt.Errorf("price.%s must be a finite number", c.field)
		case c.v < 0:
			return fmt.Errorf("price.%s must not be negative", c.field)
		case c.above && c.v == 0:
			return fmt.Errorf("price.%s is required and above 0", c.field)
		}
	}
	return nil
}

// Budget holds the caps. Both are required, and the factory ships no default values.
type Budget struct {
	PerRun  Cap    `yaml:"per_run"`
	PerDay  Cap    `yaml:"per_day"`
	Journal string `yaml:"journal"` // an absolute folder for the spend journal the per-day cap sums; "" means the operator's state folder
}

// Cap limits tokens and turns, and dollars when a role bills a paid API.
type Cap struct {
	Tokens int64   `yaml:"tokens"`
	Turns  int64   `yaml:"turns"`
	USD    float64 `yaml:"usd"`
}

// Reached returns the first limit of c that u has reached, or nil. A limit is reached when use equals
// it, so a cap of zero stops everything. Dollars count only when usd is true, which is when a role
// bills a paid API (see PaidModel), and then only what was billed (Usage.BilledUSD): a subscription's dollar
// figure is what its sessions would cost on the API, and nobody charges it.
func (c Cap) Reached(scope gungnir.CapScope, u gungnir.Usage, usd bool) *gungnir.CapError {
	switch {
	case u.CapTokens() >= c.Tokens:
		return &gungnir.CapError{Scope: scope, Dimension: gungnir.CapTokens, Used: float64(u.CapTokens()), Limit: float64(c.Tokens)}
	case u.Turns >= c.Turns:
		return &gungnir.CapError{Scope: scope, Dimension: gungnir.CapTurns, Used: float64(u.Turns), Limit: float64(c.Turns)}
	case usd && u.BilledUSD >= c.USD:
		return &gungnir.CapError{Scope: scope, Dimension: gungnir.CapUSD, Used: u.BilledUSD, Limit: c.USD}
	}
	return nil
}

// PaysPerToken reports whether any role of f bills a paid API, so the caps count dollars.
func (f *Factory) PaysPerToken() bool {
	for _, r := range f.Roles {
		if PaidModel(r.Agent, r.Model) {
			return true
		}
	}
	return false
}

// agents are the adapters factory.schema.json accepts.
var agents = []string{"claude-code", "opencode", "scripted"}

// KnownAgent reports whether agent is an adapter this schema accepts.
func KnownAgent(agent string) bool { return slices.Contains(agents, agent) }

// PaidAPI reports whether an adapter bills a paid API. Only adapters known to run on a
// subscription or at no cost return false, so a new adapter needs a spend limit until it is listed.
func PaidAPI(agent string) bool { return agent != "claude-code" && agent != "scripted" }

// PaidModel reports whether a role on agent and model bills a paid API: false for a local model, and
// otherwise PaidAPI(agent).
func PaidModel(agent, model string) bool {
	if _, local := LocalModel(model); local {
		return false
	}
	return PaidAPI(agent)
}

// localPrefix starts the reference of a model that a local server runs.
const localPrefix = "local/"

var localName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// LocalModel reports whether model is a local model reference, local/<name>, and returns its name.
func LocalModel(model string) (name string, ok bool) {
	name, found := strings.CutPrefix(model, localPrefix)
	if !found || !localName.MatchString(name) {
		return "", false
	}
	return name, true
}

// LocalModels returns the sorted, distinct names of the local models the roles of f use.
func (f *Factory) LocalModels() []string {
	var names []string
	for _, r := range f.Roles {
		if name, ok := LocalModel(r.Model); ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// checkLocal refuses a role that names a local model wrongly: on an adapter other than opencode, with a
// bad name, with reasoning (the model list sets the effort), or without an absolute models.file.
func (f *Factory) checkLocal() error {
	needFile := ""
	for _, name := range sortedKeys(f.Roles) {
		r := f.Roles[name]
		if !strings.HasPrefix(r.Model, localPrefix) {
			continue
		}
		switch {
		case r.Agent != "opencode":
			return fmt.Errorf("role %s: model %s is local, and only opencode roles run local models, not %s", name, r.Model, r.Agent)
		case !localName.MatchString(strings.TrimPrefix(r.Model, localPrefix)):
			return fmt.Errorf("role %s: model %q is not local/<name> with a name of 1 to 64 characters from a-z, 0-9, '.', '_' and '-', starting with a letter or digit", name, r.Model)
		case r.Reasoning != "":
			return fmt.Errorf("role %s: a local model takes no reasoning; set the effort in the model list", name)
		}
		if needFile == "" {
			needFile = name
		}
	}
	if needFile != "" && f.Models == nil {
		return fmt.Errorf("role %s names a local model, so models.file must be a clean, absolute path to the model list", needFile)
	}
	if m := f.Models; m != nil && (!filepath.IsAbs(m.File) || filepath.Clean(m.File) != m.File || m.File == "/") {
		if needFile != "" {
			return fmt.Errorf("role %s names a local model, so models.file must be a clean, absolute path to the model list, not %q", needFile, m.File)
		}
		return fmt.Errorf("models.file %q must be a clean, absolute path to the model list", m.File)
	}
	return nil
}

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
	if err := f.checkLocal(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	// YAML and the schema refuse a non-finite or negative price first; this holds the line for a change there. 0 is allowed, as in the schema.
	for _, name := range sortedKeys(f.Roles) {
		if p := f.Roles[name].Price; p != nil {
			if err := p.check(false); err != nil {
				return nil, fmt.Errorf("%s: role %s: %w", file, name, err)
			}
		}
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
		agent, _ := role["agent"].(string)
		model, _ := role["model"].(string)
		// A model that starts with local/ is the Go checks' to refuse when its name is bad; it asks for no spend limit.
		if KnownAgent(agent) && !strings.HasPrefix(model, localPrefix) && PaidModel(agent, model) {
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
	if j := f.Budget.Journal; j != "" && (!filepath.IsAbs(j) || filepath.Clean(j) != j) {
		return fmt.Errorf("budget.journal %q must be a clean, absolute path", j)
	}
	if f.Skills.PrivateDir != "" && !filepath.IsAbs(f.Skills.PrivateDir) {
		return fmt.Errorf("skills.private_dir %q must be an absolute path", f.Skills.PrivateDir)
	}
	if r := f.Realms; r != nil && (!filepath.IsAbs(r.Dir) || filepath.Clean(r.Dir) != r.Dir || r.Dir == "/") {
		return fmt.Errorf("realms.dir %q must be a clean, absolute path, and not the root", r.Dir)
	}
	if g := f.Graphs; g != nil && (!filepath.IsAbs(g.Dir) || filepath.Clean(g.Dir) != g.Dir || g.Dir == "/") {
		return fmt.Errorf("graphs.dir %q must be a clean, absolute path, and not the root", g.Dir)
	}
	if g := f.Grader; g != nil {
		if err := g.check(); err != nil {
			return err
		}
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

// NoLimit is the value of grader.memory that asks for no memory limit.
const NoLimit = "none"

// Limits returns the memory and CPU limits to give the grader's container: "" and 0 mean no limit. An
// operator on an engine that cannot apply limits (rootless Podman with no cgroup delegation) writes
// `memory: none` and `cpus: 0`. Writing it is the choice; the factory never picks it for them.
func (g *Grader) Limits() (memory string, cpus int) {
	if g.Memory == NoLimit {
		return "", g.CPUs
	}
	return g.Memory, g.CPUs
}

// check refuses a grader with a folder that is not clean and absolute, one folder inside the other (a
// hidden suite inside a folder an agent may be given, or the reverse), or a limit that is not positive.
func (g *Grader) check() error {
	for _, d := range []struct{ name, path string }{{"tasks_dir", g.TasksDir}, {"suite_dir", g.SuiteDir}} {
		if !filepath.IsAbs(d.path) || filepath.Clean(d.path) != d.path || d.path == "/" {
			return fmt.Errorf("grader.%s %q must be a clean, absolute path, and not the root", d.name, d.path)
		}
	}
	if g.TasksDir == g.SuiteDir || strings.HasPrefix(g.TasksDir, g.SuiteDir+"/") || strings.HasPrefix(g.SuiteDir, g.TasksDir+"/") {
		return fmt.Errorf("grader: tasks_dir %q and suite_dir %q must be two folders, neither inside the other", g.TasksDir, g.SuiteDir)
	}
	if g.Timeout <= 0 {
		return fmt.Errorf("grader.timeout must be positive")
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
