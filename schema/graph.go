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
	KindExec  NodeKind = "exec"  // runs a command
	KindAgent NodeKind = "agent" // runs one agent session
	KindGate  NodeKind = "gate"  // an approval point
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
	Timeout  Duration     `yaml:"timeout"`   // exec and agent
	Gate     gungnir.Gate `yaml:"gate"`      // gate: the approval point
	Reruns   string       `yaml:"reruns"`    // gate: the node a rejection re-runs; set by ParseGraph
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

// ParseGraph parses graph.yaml. Ids must be unique, prompt files must stay inside the realm, and
// every gate gets the node its rejection re-runs: the one it names, which must come before it and
// not be a gate, or else the nearest non-gate node before it.
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
		if n.Kind != KindGate {
			continue
		}
		if n.Reruns == "" {
			for j := i - 1; j >= 0 && n.Reruns == ""; j-- {
				if g.Nodes[j].Kind != KindGate {
					n.Reruns = g.Nodes[j].ID
				}
			}
			if n.Reruns == "" {
				return nil, fmt.Errorf("%s: gate %s has no node before it to re-run when rejected", file, n.ID)
			}
			continue
		}
		j, ok := seen[n.Reruns]
		if !ok || j == i || g.Nodes[j].Kind == KindGate {
			return nil, fmt.Errorf("%s: gate %s re-runs %q, which must be a node before it and not a gate", file, n.ID, n.Reruns)
		}
	}
	return &g, nil
}
