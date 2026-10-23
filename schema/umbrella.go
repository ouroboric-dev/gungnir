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
	// Graph and GraphVersion pin the graph pack that builds the project this umbrella is, such as
	// software at 0.1.0 (change voluspa). Both are set, or neither.
	Graph        string `yaml:"graph,omitempty"`
	GraphVersion string `yaml:"graph_version,omitempty"`
	// Runtime and RuntimeVersion pin the one runtime realm that provides the project's services, such
	// as niflheim-local at 0.1.0. Both are set, or neither.
	Runtime        string           `yaml:"runtime,omitempty"`
	RuntimeVersion string           `yaml:"runtime_version,omitempty"`
	Children       map[string]Child `yaml:"children,omitempty"` // path relative to the umbrella: child
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
	// Realm and RealmVersion pin the realm leaf that provides this leaf's recipe: its stack layer,
	// templates, implementer prompt and checks. Both are set, or neither; a leaf with neither uses
	// the graph pack's recipe, if it holds one.
	Realm        string `yaml:"realm,omitempty"`
	RealmVersion string `yaml:"realm_version,omitempty"`
	// Consumes lists the sibling leaves this leaf calls, such as a web leaf's api. A leaf's sandbox gets
	// the services of the realms of the leaves it consumes (spec §6.1, §9.2).
	Consumes []string `yaml:"consumes,omitempty"`
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
	if err := refuseTopLevelRealm(file, data); err != nil {
		return nil, err
	}
	if err := parseYAML(file, "umbrella.schema.json", data, &u); err != nil {
		return nil, err
	}
	paths := u.Paths()
	for i, p := range paths {
		if !filepath.IsLocal(p) {
			return nil, fmt.Errorf("%s: child %q must be a path inside the umbrella", file, p)
		}
		// '-' and '.' sort before '/', so the path before p may be a sibling: check every earlier path.
		for _, outer := range paths[:i] {
			if strings.HasPrefix(p, outer+"/") {
				return nil, fmt.Errorf("%s: child %q is inside child %q", file, p, outer)
			}
		}
	}
	if u.Children == nil { // an umbrella with no children yet; Odin adds to the map
		u.Children = map[string]Child{}
	}
	if err := u.checkRealms(file); err != nil {
		return nil, err
	}
	return &u, nil
}

// refuseTopLevelRealm refuses an umbrella.yaml made before change voluspa, which pinned a realm for the whole project at
// its top, with the way to pin a graph. It runs before the JSON schema, so the reason is not a bare
// "additional property".
func refuseTopLevelRealm(file string, data []byte) error {
	doc, err := decodeYAML(file, data)
	if err != nil {
		return err
	}
	top, _ := doc.(map[string]any)
	_, hasRealm := top["realm"]
	_, hasVersion := top["realm_version"]
	if hasRealm || hasVersion {
		return fmt.Errorf("%s: the top-level realm and realm_version are gone (change voluspa): a project pins its graph as graph and graph_version, such as graph: software and graph_version: 0.1.0; each leaf keeps its realm and realm_version", file)
	}
	return nil
}

// checkRealms checks the realm pins and the consumers: a pin is a realm name and a version, together;
// a leaf consumes other leaves of the umbrella, each once, and never in a ring.
func (u *Umbrella) checkRealms(file string) error {
	if err := checkGraphPin(file, u.Graph, u.GraphVersion); err != nil {
		return err
	}
	if err := checkRuntimePin(file, u.Runtime, u.RuntimeVersion); err != nil {
		return err
	}
	for _, p := range u.Paths() {
		c := u.Children[p]
		if err := checkPin(file, "child "+p, c.Realm, c.RealmVersion); err != nil {
			return err
		}
		if len(c.Consumes) > 0 && c.Kind != ChildLeaf {
			return fmt.Errorf("%s: child %s is an umbrella, and only a leaf consumes other leaves", file, p)
		}
		seen := map[string]bool{}
		for _, q := range c.Consumes {
			switch other, ok := u.Children[q]; {
			case q == p:
				return fmt.Errorf("%s: child %s consumes itself", file, p)
			case !ok || other.Kind != ChildLeaf:
				return fmt.Errorf("%s: child %s consumes %q, which is not a leaf of this umbrella", file, p, q)
			case seen[q]:
				return fmt.Errorf("%s: child %s consumes %s twice", file, p, q)
			}
			seen[q] = true
		}
	}
	return u.checkRing(file)
}

// checkRing refuses leaves that consume each other, directly or through others.
func (u *Umbrella) checkRing(file string) error {
	state := map[string]int{} // 1: on the path being walked, 2: done
	var walk func(p string, path []string) error
	walk = func(p string, path []string) error {
		switch state[p] {
		case 1:
			return fmt.Errorf("%s: leaves consume each other in a ring: %s", file, strings.Join(append(path, p), " -> "))
		case 2:
			return nil
		}
		state[p] = 1
		for _, q := range u.Children[p].Consumes {
			if err := walk(q, append(path, p)); err != nil {
				return err
			}
		}
		state[p] = 2
		return nil
	}
	for _, p := range u.Paths() {
		if err := walk(p, nil); err != nil {
			return err
		}
	}
	return nil
}

// checkPin checks that a realm and its version are both set or both empty, and well formed.
func checkPin(file, where, realm, version string) error {
	switch {
	case realm == "" && version == "":
		return nil
	case realm == "" || version == "":
		return fmt.Errorf("%s: %s sets a realm or a realm version without the other; a pin is both", file, where)
	case !gungnir.ValidRealm(realm):
		return fmt.Errorf("%s: %s: realm %q is not a realm name (lower case words joined by dashes)", file, where, realm)
	case !gungnir.ValidVersion(version):
		return fmt.Errorf("%s: %s: realm version %q is not a version such as 1.2.3", file, where, version)
	}
	return nil
}

// checkGraphPin checks that a graph and its version are both set or both empty, and well formed.
func checkGraphPin(file, graph, version string) error {
	switch {
	case graph == "" && version == "":
		return nil
	case graph == "" || version == "":
		return fmt.Errorf("%s: the umbrella sets a graph or a graph version without the other; a pin is both", file)
	case !gungnir.ValidGraph(graph):
		return fmt.Errorf("%s: the umbrella: graph %q is not a graph name (lower case words joined by dashes)", file, graph)
	case !gungnir.ValidVersion(version):
		return fmt.Errorf("%s: the umbrella: graph version %q is not a version such as 1.2.3", file, version)
	}
	return nil
}

// checkRuntimePin checks that a runtime realm and its version are both set or both empty, and well formed.
func checkRuntimePin(file, runtime, version string) error {
	switch {
	case runtime == "" && version == "":
		return nil
	case runtime == "" || version == "":
		return fmt.Errorf("%s: the umbrella sets a runtime or a runtime version without the other; a pin is both", file)
	case !gungnir.ValidRealm(runtime):
		return fmt.Errorf("%s: the umbrella: runtime %q is not a realm name (lower case words joined by dashes)", file, runtime)
	case !gungnir.ValidVersion(version):
		return fmt.Errorf("%s: the umbrella: runtime version %q is not a version such as 1.2.3", file, version)
	}
	return nil
}

// Pin is a realm leaf at a version, and the child path of the leaf that pins it.
type Pin struct{ Realm, Version, Where string }

// Pins returns every realm pin of the umbrella: each leaf's, by path. The graph pin and the runtime
// pin are not among them.
func (u *Umbrella) Pins() []Pin {
	var pins []Pin
	for _, p := range u.Paths() {
		if c := u.Children[p]; c.Realm != "" {
			pins = append(pins, Pin{c.Realm, c.RealmVersion, p})
		}
	}
	return pins
}

// RuntimePin returns the runtime realm the umbrella pins, with Where set to "runtime", and whether it
// pins one.
func (u *Umbrella) RuntimePin() (Pin, bool) {
	if u.Runtime == "" {
		return Pin{}, false
	}
	return Pin{u.Runtime, u.RuntimeVersion, "runtime"}, true
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
