package schema

import (
	"fmt"
	"os"
	"path/filepath"
)

// Pack is a realm's pack.yaml (spec §9.4).
type Pack struct {
	Name    string              `yaml:"name"`
	Version string              `yaml:"version"`
	Graph   string              `yaml:"graph"`  // relative to the pack
	Roles   map[string]PackRole `yaml:"roles"`  // default roles; factory.yaml's roles are used from Phase 1
	Checks  map[string][]string `yaml:"checks"` // check commands, used from Phase 4
}

// PackRole is a realm's default agent and model for a role.
type PackRole struct {
	Agent string `yaml:"agent"`
	Model string `yaml:"model"`
}

// LoadPack reads dir/pack.yaml. The graph path must stay inside dir.
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
	return &p, nil
}
