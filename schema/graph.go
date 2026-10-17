package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	KindForeach NodeKind = "foreach" // repeats its body for every ready task of the plan, in the leaf the task names (spec §4.2)
	KindPublish NodeKind = "publish" // Odin copies a scratch agent's output into the project and commits it (spec §3.3 rule 1)
	KindTasks   NodeKind = "tasks"   // Odin reads the plan's tasks.md, checks every task is ready, and writes features.json (spec §4.2)
	KindInit    NodeKind = "init"    // Odin creates the leaves the spec names, each from its realm's template (spec §4.2, §4.1)
)

// DefaultReturns is how many times a refusal may send a run back into the node named by back_to when the
// node sets no returns, and MaxReturns the most it may set.
const (
	DefaultReturns = 2
	MaxReturns     = 5
)

// RealmFrom says whose files a node uses: the lead realm's (the default) or the realm of the task's leaf.
const (
	RealmLead = "lead"
	RealmLeaf = "leaf"
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

	Scratch bool     `yaml:"scratch"` // agent: runs in a scratch workspace, not a leaf, and writes its documents to out/ (spec §3.3 rule 1)
	Inputs  []string `yaml:"inputs"`  // agent (scratch): files and folders of the project, copied into the workspace; changes to the copies are ignored
	From    string   `yaml:"from"`    // publish: the scratch agent whose output is published; tasks: the tasks.md, by glob; init: the leaves file
	To      string   `yaml:"to"`      // publish: the folder of the project its files go to

	BackTo  string `yaml:"back_to"` // merge, pointer, tasks: the earlier agent or loop node a refusal goes back into, with the refusal as its input
	Returns int    `yaml:"returns"` // merge, pointer, tasks: how many times it may go back; DefaultReturns when back_to is set and this is not

	Over      string `yaml:"over"`  // foreach: what it repeats for; only "tasks"
	Body      []Node `yaml:"nodes"` // foreach: the nodes that run for each task, in the task's leaf
	RealmFrom string `yaml:"realm"` // loop, merge, pointer in a foreach body: "leaf", the realm of the task's leaf supplies the prompt and the checks

	// Set by Odin when it expands a foreach for one task; never in a graph file.
	Realm string `yaml:"-"` // the realm whose files this node's prompt and checks come from; empty is the lead realm
	Task  string `yaml:"-"` // the id of the task this node works for
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

// All returns every node of the graph, the nodes of a foreach body after the foreach node itself, in the
// order they are written. It is for what applies to every node, such as the roles and skills the graph
// names.
func (g *Graph) All() []Node {
	var out []Node
	for _, n := range g.Nodes {
		out = append(out, n)
		out = append(out, n.Body...)
	}
	return out
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

// ParseGraph parses graph.yaml. Ids must be unique across the graph, prompt files must stay inside the
// realm, merge and pointer nodes must name a child repo, and every gate gets the node its rejection
// re-runs: the agent or loop node it names, which must come before it in its own scope (the graph, or
// one foreach body), or else the nearest one before it. A rejection's reason is input for an agent, so
// it never re-runs an exec node, which would drop it. A loop's attempts default to DefaultAttempts, its
// reviewers' ids are unique, and a reviewer's session key (the loop's id, a dash and the reviewer's id)
// never equals a node's id. A foreach node's body is checked as a graph of its own, in which a node
// names no repo: the task's leaf is its repo.
func ParseGraph(file string, data []byte) (*Graph, error) {
	var g Graph
	if err := parseYAML(file, "graph.schema.json", data, &g); err != nil {
		return nil, err
	}
	c := &graphCheck{file: file, seen: map[string]bool{}, keys: map[string]string{}}
	if err := c.scope(g.Nodes, false); err != nil {
		return nil, err
	}
	for key, loop := range c.keys {
		if c.seen[key] {
			return nil, fmt.Errorf("%s: loop %s: a reviewer's session key %q is also a node id", file, loop, key)
		}
	}
	return &g, nil
}

// graphCheck holds what ParseGraph learns across scopes: every node id, and the reviewer session keys.
type graphCheck struct {
	file string
	seen map[string]bool   // node ids of the whole graph, bodies included
	keys map[string]string // reviewer session key: the loop that owns it
}

// scope checks the nodes of the graph, or of one foreach body.
func (c *graphCheck) scope(nodes []Node, body bool) error {
	file := c.file
	at := map[string]int{} // node id: its position in this scope
	for i := range nodes {
		n := &nodes[i]
		if c.seen[n.ID] {
			return fmt.Errorf("%s: node id %q is used twice", file, n.ID)
		}
		c.seen[n.ID] = true
		at[n.ID] = i
		leafLoop := n.Kind == KindLoop && n.RealmFrom == RealmLeaf
		if (n.Kind == KindAgent || n.Kind == KindLoop) && !leafLoop && !filepath.IsLocal(n.Prompt) {
			return fmt.Errorf("%s: node %s: prompt %q must be a path inside the realm", file, n.ID, n.Prompt)
		}
		if n.Kind == KindLoop {
			if err := checkLoop(file, n, c.keys); err != nil {
				return err
			}
		}
		if (n.Kind == KindMerge || n.Kind == KindPointer) && n.Repo == "." {
			return fmt.Errorf("%s: node %s: repo must be a child of the project, not the project itself", file, n.ID)
		}
		for _, f := range n.Generated {
			if !filepath.IsLocal(f) {
				return fmt.Errorf("%s: node %s: generated file %q must be a path inside the umbrella", file, n.ID, f)
			}
		}
		if err := c.stage(n, nodes[:i]); err != nil {
			return err
		}
		if n.BackTo != "" {
			if j, ok := at[n.BackTo]; !ok || j >= i || (nodes[j].Kind != KindAgent && nodes[j].Kind != KindLoop) {
				return fmt.Errorf("%s: node %s goes back to %q, which must be an agent or loop node before it", file, n.ID, n.BackTo)
			}
			if n.Returns == 0 {
				n.Returns = DefaultReturns
			}
		}
		if n.Kind == KindForeach {
			if err := c.scope(n.Body, true); err != nil {
				return err
			}
		}
		if n.Kind != KindGate {
			continue
		}
		if n.Reruns == "" {
			for j := i - 1; j >= 0 && n.Reruns == ""; j-- {
				if k := nodes[j].Kind; k == KindAgent || k == KindLoop {
					n.Reruns = nodes[j].ID
				}
			}
			if n.Reruns == "" {
				return fmt.Errorf("%s: gate %s has no agent or loop node before it to re-run when rejected", file, n.ID)
			}
			continue
		}
		j, ok := at[n.Reruns]
		if !ok || j == i || (nodes[j].Kind != KindAgent && nodes[j].Kind != KindLoop) {
			return fmt.Errorf("%s: gate %s re-runs %q, which must be an agent or loop node before it", file, n.ID, n.Reruns)
		}
	}
	return nil
}

// stage checks the nodes that work on documents and the project, not on a leaf: a scratch agent, and
// publish, tasks and init. before holds the nodes of the scope ahead of n.
func (c *graphCheck) stage(n *Node, before []Node) error {
	file := c.file
	switch n.Kind {
	case KindAgent:
		switch {
		case n.Scratch && n.Repo != "":
			return fmt.Errorf("%s: node %s: a scratch agent names no repo; it works in a workspace of its own", file, n.ID)
		case n.Scratch && n.ReadOnly:
			return fmt.Errorf("%s: node %s: a scratch agent writes its output to out/, so it can't be read-only", file, n.ID)
		case !n.Scratch && len(n.Inputs) > 0:
			return fmt.Errorf("%s: node %s: only a scratch agent takes inputs", file, n.ID)
		}
		seen := map[string]bool{}
		for _, in := range n.Inputs {
			if !projectPath(in) {
				return fmt.Errorf("%s: node %s: input %q must be a path inside the project, outside .git and .factory", file, n.ID, in)
			}
			if seen[in] {
				return fmt.Errorf("%s: node %s: input %q is listed twice", file, n.ID, in)
			}
			seen[in] = true
		}
	case KindPublish:
		if !slices.ContainsFunc(before, func(m Node) bool { return m.ID == n.From && m.Kind == KindAgent && m.Scratch }) {
			return fmt.Errorf("%s: node %s publishes the output of %q, which must be a scratch agent node before it", file, n.ID, n.From)
		}
		if n.To == "." || !projectPath(n.To) {
			return fmt.Errorf("%s: node %s: to %q must be a folder of the project (not the project itself, .git or .factory)", file, n.ID, n.To)
		}
	case KindTasks, KindInit:
		if !projectPath(n.From) {
			return fmt.Errorf("%s: node %s: from %q must be a path inside the project, outside .git and .factory", file, n.ID, n.From)
		}
	}
	return nil
}

// projectPath reports whether p is a path inside a project that a run may read or write: relative,
// without ".." and not inside .git or .factory, which are Odin's.
func projectPath(p string) bool {
	if !filepath.IsLocal(p) {
		return false
	}
	first, _, _ := strings.Cut(filepath.ToSlash(filepath.Clean(p)), "/")
	return first != ".git" && first != ".factory"
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
		if slices.Contains(n.Checks, r.ID) {
			return fmt.Errorf("%s: loop %s: reviewer id %q is also the name of a check of this loop", file, n.ID, r.ID)
		}
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
