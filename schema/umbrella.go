package schema

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ouroboric-dev/gungnir"
	"go.yaml.in/yaml/v3"
)

// Umbrella is an umbrella's umbrella.yaml (spec §3.1): who owns it, who can see it, and its child
// nodes. Odin is its only writer.
type Umbrella struct {
	Root       bool               `yaml:"root,omitempty"` // the top of the tree: a project, or a company umbrella holding projects
	Owner      string             `yaml:"owner"`
	Visibility gungnir.Visibility `yaml:"visibility"`
	Children   map[string]Child   `yaml:"children,omitempty"` // path relative to the umbrella: child
}

// ChildKind is what a child node is.
type ChildKind string

const (
	ChildUmbrella ChildKind = "umbrella" // has children, holds no product code
	ChildLeaf     ChildKind = "leaf"     // holds product code; the only node an agent mounts
)

// Child is one child node of an umbrella. Its working copy is a submodule at the same path.
type Child struct {
	Kind       ChildKind          `yaml:"kind"`
	Owner      string             `yaml:"owner"`
	Visibility gungnir.Visibility `yaml:"visibility"`
}

// UmbrellaFile is the name of an umbrella's metadata file.
const UmbrellaFile = "umbrella.yaml"

// LoadUmbrella reads dir/umbrella.yaml.
func LoadUmbrella(dir string) (*Umbrella, error) {
	file := filepath.Join(dir, UmbrellaFile)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseUmbrella(file, data)
}

// ParseUmbrella parses umbrella.yaml. The schema keeps child paths inside the umbrella, with no
// segment that starts with a dot (so never in .git or .factory); no child may sit inside another.
func ParseUmbrella(file string, data []byte) (*Umbrella, error) {
	var u Umbrella
	if err := parseYAML(file, "umbrella.schema.json", data, &u); err != nil {
		return nil, err
	}
	paths := u.Paths()
	for i, p := range paths {
		if !filepath.IsLocal(p) {
			return nil, fmt.Errorf("%s: child %q must be a path inside the umbrella", file, p)
		}
		if i > 0 && strings.HasPrefix(p, paths[i-1]+"/") {
			return nil, fmt.Errorf("%s: child %q is inside child %q", file, p, paths[i-1])
		}
	}
	if u.Children == nil { // an umbrella with no children yet; Odin adds to the map
		u.Children = map[string]Child{}
	}
	return &u, nil
}

// Paths returns the child paths, sorted.
func (u *Umbrella) Paths() []string { return sortedKeys(u.Children) }

// Leaks returns the private children of a public umbrella. Their names are visible to anyone who
// can read the umbrella's .gitmodules, so the operator is warned (spec §3.1).
func (u *Umbrella) Leaks() []string {
	var leaks []string
	if u.Visibility != gungnir.VisibilityPublic {
		return nil
	}
	for _, p := range u.Paths() {
		if u.Children[p].Visibility == gungnir.VisibilityPrivate {
			leaks = append(leaks, p)
		}
	}
	return leaks
}

// Save writes dir/umbrella.yaml.
func (u *Umbrella) Save(dir string) error {
	var b bytes.Buffer
	b.WriteString("# The instance tree: this umbrella and its child nodes (spec §3.1). Odin writes this file.\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(u); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if _, err := ParseUmbrella(UmbrellaFile, b.Bytes()); err != nil {
		return fmt.Errorf("refusing to write an umbrella.yaml that doesn't parse: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, UmbrellaFile), b.Bytes(), 0o644)
}
