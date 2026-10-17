package schema

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ouroboric-dev/gungnir"
)

// Pack is a realm's pack.yaml (spec §9.4).
type Pack struct {
	Name    string              `yaml:"name"`
	Version string              `yaml:"version"`
	Graph   string              `yaml:"graph"`   // relative to the pack; empty for a realm that never leads a run
	Roles   map[string]PackRole `yaml:"roles"`   // default roles; factory.yaml's roles are used from Phase 1
	Checks  map[string][]string `yaml:"checks"`  // check commands: merge and pointer nodes run them from Phase 2
	Leaves  map[string]PackLeaf `yaml:"leaves"`  // the default leaves of the lead realm's graph, by path relative to the project
	Sandbox *PackSandbox        `yaml:"sandbox"` // the realm's stack layer and the hosts its commands reach (spec §6.1)

	// Leaf is what a build realm offers every leaf it provides, under the same names in every realm
	// (change realm-leaf-naming): a template, an implementer prompt and the leaf's checks. The graph
	// names these roles, and Odin looks each one up in the realm of the task's leaf.
	Leaf *PackRecipe `yaml:"leaf"`
	// Services are the containers a sandbox of a leaf of this realm starts beside the work container,
	// and the sandbox of each leaf that consumes it (spec §6.1, §9.2).
	Services map[string]PackService `yaml:"services"`
}

// PackRecipe is the contract of a build realm: how to make a leaf, what to tell its implementer, and
// the checks that decide a task in it is done.
type PackRecipe struct {
	Description string   `yaml:"description"`  // the new repo's description
	Template    string   `yaml:"template"`     // the leaf's template directory, relative to the pack
	FirstCommit []string `yaml:"first_commit"` // files of the template that make the repo's first commit, such as its licence
	Implementer string   `yaml:"implementer"`  // the implementer's prompt file, relative to the pack
	Checks      []string `yaml:"checks"`       // the check ladder, by name: each is defined in the pack's checks
}

// PackService is a container a sandbox starts beside the work container.
type PackService struct {
	Image string            `yaml:"image"` // pinned by digest
	Env   map[string]string `yaml:"env"`   // made-up development values
	Ports []int             `yaml:"ports"` // the TCP ports the work container may reach
}

// ServiceList returns the pack's services for a sandbox spec, sorted by name.
func (p *Pack) ServiceList() []gungnir.Service {
	var out []gungnir.Service
	for _, name := range sortedKeys(p.Services) {
		v := p.Services[name]
		out = append(out, gungnir.Service{Name: name, Image: v.Image, Env: v.Env, Ports: v.Ports})
	}
	return out
}

// PackSandbox is what a realm adds to its leaves' sandboxes: its stack layer, in one of two forms (a
// digest-pinned image, or a build context in the realm), and the hosts its commands reach.
type PackSandbox struct {
	Stack  string   `yaml:"stack"`  // the stack layer's image, pinned by digest
	Build  string   `yaml:"build"`  // or the folder of the realm that holds the stack's Containerfile and the check scripts it bakes in
	Target string   `yaml:"target"` // with build: the stage of the Containerfile this stack is; empty is its last stage
	Egress []string `yaml:"egress"` // host names, such as package registries, opened on port 443 beside the model APIs
	// Acceptance is the stack of the sandbox that holds a snapshot of the whole project, where a pointer
	// move's acceptance checks run: a lead realm's, in either form. It is empty for a realm that has none.
	Acceptance *PackStack `yaml:"acceptance"`
}

// PackLeaf is a leaf the realm creates when a project lacks it (spec §4.1).
type PackLeaf struct {
	Description string   `yaml:"description"`  // the new repo's description
	Template    string   `yaml:"template"`     // the leaf's template directory, relative to the pack
	FirstCommit []string `yaml:"first_commit"` // files of the template that make the repo's first commit, such as its licence
}

// PackRole is a realm's default agent and model for a role.
type PackRole struct {
	Agent string `yaml:"agent"`
	Model string `yaml:"model"`
}

// checkRecipe checks the leaf recipe and the services: every path stays inside the realm and is there,
// every check the recipe names is defined, and the services are what a sandbox can start.
func (p *Pack) checkRecipe(file, dir string) error {
	if r := p.Leaf; r != nil {
		for what, f := range map[string]string{"template": r.Template, "implementer": r.Implementer} {
			if !filepath.IsLocal(f) {
				return fmt.Errorf("%s: leaf.%s %q must be a path inside the realm", file, what, f)
			}
		}
		if st, err := os.Stat(filepath.Join(dir, r.Template)); err != nil || !st.IsDir() {
			return fmt.Errorf("%s: leaf.template %q must be a directory of the realm", file, r.Template)
		}
		if st, err := os.Lstat(filepath.Join(dir, r.Implementer)); err != nil || !st.Mode().IsRegular() {
			return fmt.Errorf("%s: leaf.implementer %q must be a regular file of the realm", file, r.Implementer)
		}
		for _, f := range r.FirstCommit {
			if !filepath.IsLocal(f) {
				return fmt.Errorf("%s: leaf.first_commit file %q must be a path inside the template", file, f)
			}
			if st, err := os.Lstat(filepath.Join(dir, r.Template, f)); err != nil || !st.Mode().IsRegular() {
				return fmt.Errorf("%s: leaf.first_commit file %s/%s must be a regular file", file, r.Template, f)
			}
		}
		seen := map[string]bool{}
		for _, c := range r.Checks {
			if _, ok := p.Checks[c]; !ok {
				return fmt.Errorf("%s: leaf.checks names %q, which the pack doesn't define under checks", file, c)
			}
			if seen[c] {
				return fmt.Errorf("%s: leaf.checks names %q twice", file, c)
			}
			seen[c] = true
		}
	}
	if err := gungnir.ValidateServices(p.ServiceList()); err != nil {
		return fmt.Errorf("%s: services: %w", file, err)
	}
	return nil
}

// LoadPack reads dir/pack.yaml. The graph and every template must stay inside dir, and every
// first-commit file must be a regular file inside its template.
func LoadPack(dir string) (*Pack, error) {
	file := filepath.Join(dir, "pack.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var p Pack
	if err := parseYAML(file, "pack.schema.json", data, &p); err != nil {
		return nil, err
	}
	if p.Graph != "" && !filepath.IsLocal(p.Graph) {
		return nil, fmt.Errorf("%s: graph %q must be a path inside the realm", file, p.Graph)
	}
	if err := p.checkRecipe(file, dir); err != nil {
		return nil, err
	}
	if p.Sandbox != nil {
		for _, st := range []PackStack{{Stack: p.Sandbox.Stack, Build: p.Sandbox.Build, Target: p.Sandbox.Target}, ptrOr(p.Sandbox.Acceptance)} {
			if st.Build != "" && !filepath.IsLocal(st.Build) {
				return nil, fmt.Errorf("%s: sandbox build %q must be a folder inside the realm", file, st.Build)
			}
			if st.Target != "" && st.Build == "" {
				return nil, fmt.Errorf("%s: sandbox target %q needs a build context; an image has no stages", file, st.Target)
			}
		}
		if _, err := p.StackRef(dir); err != nil {
			return nil, fmt.Errorf("%s: sandbox: %w", file, err)
		}
		if _, err := p.AcceptanceRef(dir); err != nil {
			return nil, fmt.Errorf("%s: sandbox acceptance: %w", file, err)
		}
		for _, h := range p.Sandbox.Egress {
			if !gungnir.ValidHost(h) {
				return nil, fmt.Errorf("%s: sandbox.egress %q is not a plain host name (no scheme, port, wildcard or address)", file, h)
			}
		}
	}
	for _, path := range sortedKeys(p.Leaves) {
		l := p.Leaves[path]
		if !filepath.IsLocal(l.Template) {
			return nil, fmt.Errorf("%s: leaf %s: template %q must be a path inside the realm", file, path, l.Template)
		}
		for _, f := range l.FirstCommit {
			if !filepath.IsLocal(f) {
				return nil, fmt.Errorf("%s: leaf %s: first-commit file %q must be a path inside the template", file, path, f)
			}
			st, err := os.Lstat(filepath.Join(dir, l.Template, f))
			if err != nil || !st.Mode().IsRegular() {
				return nil, fmt.Errorf("%s: leaf %s: first-commit file %s/%s must be a regular file", file, path, l.Template, f)
			}
		}
	}
	return &p, nil
}

func ptrOr(s *PackStack) PackStack {
	if s == nil {
		return PackStack{}
	}
	return *s
}
