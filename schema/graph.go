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
	KindLoop    NodeKind = "loop"    // implement, check and fix with a bounded number of attempts (spec §4.2, §4.6, §7.3)
)

// DefaultAttempts is how many times a loop runs its implementer when the node sets no attempts (spec §4.2).
const DefaultAttempts = 3

// Node is one step of a graph. Which fields apply depends on Kind.
type Node struct {
	ID       string       `yaml:"id"`
	Kind     NodeKind     `yaml:"kind"`
	Run      []string     `yaml:"run"`       // exec: the command, without a shell
	Role     string       `yaml:"role"`      // agent, loop: the factory.yaml role (a loop's implementer)
	Prompt   string       `yaml:"prompt"`    // agent, loop: the prompt file, relative to the graph
	ReadOnly bool         `yaml:"read_only"` // agent
	MaxTurns int          `yaml:"max_turns"` // agent, loop (the implementer)
	Skills   []string     `yaml:"skills"`    // agent, loop: the skills loaded into its prompt, by name (spec §5.1)
	Timeout  Duration     `yaml:"timeout"`   // exec, agent, loop (each session), merge and pointer
	Gate     gungnir.Gate `yaml:"gate"`      // gate: the approval point
	Reruns   string       `yaml:"reruns"`    // gate: the agent node a rejection re-runs; set by ParseGraph

	Repo       string     `yaml:"repo"`       // exec, agent, loop, merge, pointer: a repo of the project, relative to it; "." is the project umbrella
	Checks     []string   `yaml:"checks"`     // loop, merge, pointer: pack checks run in the leaf (a loop's, in order, after each attempt)
	Attempts   int        `yaml:"attempts"`   // loop: how many attempts before it escalates; DefaultAttempts when unset
	Reviewers  []Reviewer `yaml:"reviewers"`  // loop: read-only agents that review each attempt once its checks pass, in order
	Acceptance []string   `yaml:"acceptance"` // pointer: pack checks run in the leaf's parent umbrella
	Generated  []string   `yaml:"generated"`  // pointer: generated files of the parent umbrella, committed with the move under the shrink guard
}

// Reviewer is a read-only agent that reviews a loop's work and reports typed findings (spec §4.6, §9.5).
type Reviewer struct {
	ID       string   `yaml:"id"`
	Role     string   `yaml:"role"`
	Prompt   string   `yaml:"prompt"`
	Skills   []string `yaml:"skills"`
	MaxTurns int      `yaml:"max_turns"`
	Timeout  Duration `yaml:"timeout"`
}

// SessionKey names the sessions of reviewer r of loop n in the ledger, the transcripts and a script.
func (n Node) SessionKey(r Reviewer) string { return n.ID + "-" + r.ID }

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
// agent or loop node it names, which must come before it, or else the nearest one before it. A
// rejection's reason is input for an agent, so it never re-runs an exec node, which would drop it.
// A loop's attempts default to DefaultAttempts, its reviewers' ids are unique, and a reviewer's
// session key (the loop's id, a dash and the reviewer's id) never equals a node's id.
func ParseGraph(file string, data []byte) (*Graph, error) {
	var g Graph
	if err := parseYAML(file, "graph.schema.json", data, &g); err != nil {
		return nil, err
	}
	seen := map[string]int{}
	keys := map[string]string{} // reviewer session key: the loop that owns it
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if _, dup := seen[n.ID]; dup {
			return nil, fmt.Errorf("%s: node id %q is used twice", file, n.ID)
		}
		seen[n.ID] = i
		if (n.Kind == KindAgent || n.Kind == KindLoop) && !filepath.IsLocal(n.Prompt) {
			return nil, fmt.Errorf("%s: node %s: prompt %q must be a path inside the realm", file, n.ID, n.Prompt)
		}
		if n.Kind == KindLoop {
			if err := checkLoop(file, n, keys); err != nil {
				return nil, err
			}
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
				if k := g.Nodes[j].Kind; k == KindAgent || k == KindLoop {
					n.Reruns = g.Nodes[j].ID
				}
			}
			if n.Reruns == "" {
				return nil, fmt.Errorf("%s: gate %s has no agent or loop node before it to re-run when rejected", file, n.ID)
			}
			continue
		}
		j, ok := seen[n.Reruns]
		if !ok || j == i || (g.Nodes[j].Kind != KindAgent && g.Nodes[j].Kind != KindLoop) {
			return nil, fmt.Errorf("%s: gate %s re-runs %q, which must be an agent or loop node before it", file, n.ID, n.Reruns)
		}
	}
	for key, loop := range keys {
		if _, clash := seen[key]; clash {
			return nil, fmt.Errorf("%s: loop %s: a reviewer's session key %q is also a node id", file, loop, key)
		}
	}
	return &g, nil
}

// checkLoop checks a loop node's attempts and reviewers, and records its reviewers' session keys.
func checkLoop(file string, n *Node, keys map[string]string) error {
	if n.Attempts == 0 {
		n.Attempts = DefaultAttempts
	}
	seen := map[string]bool{}
	for _, r := range n.Reviewers {
		if seen[r.ID] {
			return fmt.Errorf("%s: loop %s: reviewer id %q is used twice", file, n.ID, r.ID)
		}
		seen[r.ID] = true
		if !filepath.IsLocal(r.Prompt) {
			return fmt.Errorf("%s: loop %s: reviewer %s: prompt %q must be a path inside the realm", file, n.ID, r.ID, r.Prompt)
		}
		key := n.SessionKey(r)
		if other, dup := keys[key]; dup {
			return fmt.Errorf("%s: loop %s: reviewer session key %q is also loop %s's", file, n.ID, key, other)
		}
		keys[key] = n.ID
	}
	return nil
}
