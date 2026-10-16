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
	Graph   string              `yaml:"graph"`   // relative to the pack
	Roles   map[string]PackRole `yaml:"roles"`   // default roles; factory.yaml's roles are used from Phase 1
	Checks  map[string][]string `yaml:"checks"`  // check commands: merge and pointer nodes run them from Phase 2
	Leaves  map[string]PackLeaf `yaml:"leaves"`  // the leaves the graph works in, by path relative to the project
	Sandbox *PackSandbox        `yaml:"sandbox"` // the realm's stack layer and the hosts its commands reach (spec §6.1)
}

// PackSandbox is what a realm adds to its leaves' sandboxes.
type PackSandbox struct {
	Stack  string   `yaml:"stack"`  // the stack layer's image, pinned by digest
	Egress []string `yaml:"egress"` // host names, such as package registries, opened on port 443 beside the model APIs
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
	if !filepath.IsLocal(p.Graph) {
		return nil, fmt.Errorf("%s: graph %q must be a path inside the realm", file, p.Graph)
	}
	if p.Sandbox != nil {
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
