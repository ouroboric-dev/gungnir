package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ouroboric-dev/gungnir"
	"go.yaml.in/yaml/v3"
)

// Graph is a realm's graph.yaml. Nodes run in order.
type Graph struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Nodes   []Node `yaml:"nodes"`
}

// NodeKind is what a node does.
type NodeKind string

const (
	KindExec    NodeKind = "exec"    // runs a command
	KindAgent   NodeKind = "agent"   // runs one agent session
	KindGate    NodeKind = "gate"    // an approval point
	KindMerge   NodeKind = "merge"   // lands a leaf's run branch on its main, through the merge gate
	KindPointer NodeKind = "pointer" // moves a leaf's pointer in its parent umbrella, one level
)

// Node is one step of a graph. Which fields apply depends on Kind.
type Node struct {
	ID       string       `yaml:"id"`
	Kind     NodeKind     `yaml:"kind"`
	Run      []string     `yaml:"run"`       // exec: the command, without a shell
	Role     string       `yaml:"role"`      // agent: the factory.yaml role
	Prompt   string       `yaml:"prompt"`    // agent: the prompt file, relative to the graph
	ReadOnly bool         `yaml:"read_only"` // agent
	MaxTurns int          `yaml:"max_turns"` // agent
	Skills   []string     `yaml:"skills"`    // agent: the skills loaded into its prompt, by name (spec §5.1)
	Timeout  Duration     `yaml:"timeout"`   // exec, agent, merge and pointer
	Gate     gungnir.Gate `yaml:"gate"`      // gate: the approval point
	Reruns   string       `yaml:"reruns"`    // gate: the agent node a rejection re-runs; set by ParseGraph

	Repo       string   `yaml:"repo"`       // exec, agent, merge, pointer: a repo of the project, relative to it; "." is the project umbrella
	Checks     []string `yaml:"checks"`     // merge, pointer: pack checks run in the leaf
	Acceptance []string `yaml:"acceptance"` // pointer: pack checks run in the leaf's parent umbrella
	Generated  []string `yaml:"generated"`  // pointer: generated files of the parent umbrella, committed with the move under the shrink guard
}

// Duration is a time.Duration written as "30s", "10m" or "1h".
type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// Index returns the position of the node with the given id, or -1.
func (g *Graph) Index(id string) int {
	for i, n := range g.Nodes {
		if n.ID == id {
			return i
		}
	}
	return -1
}

// LoadGraph reads and parses a graph.yaml file.
func LoadGraph(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseGraph(path, data)
}

// ParseGraph parses graph.yaml. Ids must be unique, prompt files must stay inside the realm, merge
// and pointer nodes must name a child repo, and every gate gets the node its rejection re-runs: the
// agent node it names, which must come before it, or else the nearest agent node before it. A
// rejection's reason is input for an agent, so it never re-runs an exec node, which would drop it.
func ParseGraph(file string, data []byte) (*Graph, error) {
	var g Graph
	if err := parseYAML(file, "graph.schema.json", data, &g); err != nil {
		return nil, err
	}
	seen := map[string]int{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if _, dup := seen[n.ID]; dup {
			return nil, fmt.Errorf("%s: node id %q is used twice", file, n.ID)
		}
		seen[n.ID] = i
		if n.Kind == KindAgent && !filepath.IsLocal(n.Prompt) {
			return nil, fmt.Errorf("%s: node %s: prompt %q must be a path inside the realm", file, n.ID, n.Prompt)
		}
		if (n.Kind == KindMerge || n.Kind == KindPointer) && n.Repo == "." {
			return nil, fmt.Errorf("%s: node %s: repo must be a child of the project, not the project itself", file, n.ID)
		}
		for _, f := range n.Generated {
			if !filepath.IsLocal(f) {
				return nil, fmt.Errorf("%s: node %s: generated file %q must be a path inside the umbrella", file, n.ID, f)
			}
		}
		if n.Kind != KindGate {
			continue
		}
		if n.Reruns == "" {
			for j := i - 1; j >= 0 && n.Reruns == ""; j-- {
				if g.Nodes[j].Kind == KindAgent {
					n.Reruns = g.Nodes[j].ID
				}
			}
			if n.Reruns == "" {
				return nil, fmt.Errorf("%s: gate %s has no agent node before it to re-run when rejected", file, n.ID)
			}
			continue
		}
		j, ok := seen[n.Reruns]
		if !ok || j == i || g.Nodes[j].Kind != KindAgent {
			return nil, fmt.Errorf("%s: gate %s re-runs %q, which must be an agent node before it", file, n.ID, n.Reruns)
		}
	}
	return &g, nil
}
