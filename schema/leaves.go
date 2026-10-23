package schema

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Leaves is a product's leaves file (leaves.yaml): the leaves the spec stage names, and the realm that
// provides each one.
type Leaves struct {
	// Runtime pins the one runtime realm that provides the project's services, such as niflheim-local.
	// It is optional; a file without one leaves the project's pin as it was.
	Runtime *RuntimePin         `yaml:"runtime,omitempty"`
	Leaves  map[string]LeafSpec `yaml:"leaves"`
}

// RuntimePin names a runtime realm and its version.
type RuntimePin struct {
	Realm        string `yaml:"realm"`
	RealmVersion string `yaml:"realm_version"`
}

// LeafSpec is one leaf of the file.
type LeafSpec struct {
	Realm        string   `yaml:"realm"`
	RealmVersion string   `yaml:"realm_version"`
	Description  string   `yaml:"description"`        // the new repo's description
	Consumes     []string `yaml:"consumes,omitempty"` // sibling leaves this one calls
}

// ParseLeaves parses leaves.yaml. A leaf's path stays inside the project and outside .git and .factory,
// no leaf is inside another, and no leaf consumes itself, a leaf twice, or another in a ring. A leaf may
// consume one the file doesn't list: an earlier run made it, and Odin checks the project has it.
func ParseLeaves(file string, data []byte) (*Leaves, error) {
	var l Leaves
	if err := parseYAML(file, "leaves.schema.json", data, &l); err != nil {
		return nil, err
	}
	paths := l.Paths()
	for i, p := range paths {
		if !projectPath(p) {
			return nil, fmt.Errorf("%s: leaf %q must be a path inside the project, outside .git and .factory", file, p)
		}
		// '-' and '.' sort before '/', so the path before p may be a sibling: check every earlier path.
		for _, outer := range paths[:i] {
			if strings.HasPrefix(p, outer+"/") {
				return nil, fmt.Errorf("%s: leaf %q is inside leaf %q", file, p, outer)
			}
		}
		seen := map[string]bool{}
		for _, q := range l.Leaves[p].Consumes {
			switch {
			case !filepath.IsLocal(q):
				return nil, fmt.Errorf("%s: leaf %s consumes %q, which must be a path inside the project", file, p, q)
			case q == p:
				return nil, fmt.Errorf("%s: leaf %s consumes itself", file, p)
			case seen[q]:
				return nil, fmt.Errorf("%s: leaf %s consumes %s twice", file, p, q)
			}
			seen[q] = true
		}
	}
	// A ring among the listed leaves is a ring of the umbrella they will be recorded in.
	u := &Umbrella{Children: map[string]Child{}}
	for _, p := range paths {
		u.Children[p] = Child{Kind: ChildLeaf, Consumes: l.Leaves[p].Consumes}
	}
	for _, p := range paths {
		var kept []string
		for _, q := range u.Children[p].Consumes {
			if _, listed := u.Children[q]; listed {
				kept = append(kept, q)
			}
		}
		c := u.Children[p]
		c.Consumes = kept
		u.Children[p] = c
	}
	if err := u.checkRing(file); err != nil {
		return nil, err
	}
	return &l, nil
}

// Paths returns the leaf paths, sorted.
func (l *Leaves) Paths() []string { return sortedKeys(l.Leaves) }

// Pins returns the realm pin of every leaf, by path.
func (l *Leaves) Pins() []Pin {
	var pins []Pin
	for _, p := range l.Paths() {
		pins = append(pins, Pin{l.Leaves[p].Realm, l.Leaves[p].RealmVersion, p})
	}
	return pins
}

// RuntimePin returns the runtime realm the leaves file pins, with Where set to RuntimeWhere, and whether it
// pins one.
func (l *Leaves) RuntimePin() (Pin, bool) {
	if l.Runtime == nil {
		return Pin{}, false
	}
	return Pin{l.Runtime.Realm, l.Runtime.RealmVersion, RuntimeWhere}, true
}
