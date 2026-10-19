package schema

import (
	"slices"
	"strings"
	"testing"
)

// stagesGraph is the front of a graph pack's graph: agents that write documents in a scratch workspace,
// Odin publishing them into the project once approved, the plan read into tasks, and the leaves created.
const stagesGraph = `
name: stages
version: 1.0.0
schema: 1
nodes:
  - { id: design, kind: agent, role: planner, prompt: prompts/design.md, scratch: true }
  - { id: design-gate, kind: gate, gate: design }
  - { id: design-docs, kind: publish, from: design, to: docs }
  - { id: spec, kind: agent, role: planner, prompt: prompts/spec.md, scratch: true, inputs: [docs/1-product, brief.md], max_turns: 30 }
  - { id: spec-gate, kind: gate, gate: spec }
  - { id: spec-docs, kind: publish, from: spec, to: docs }
  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md, scratch: true, inputs: [docs] }
  - { id: plan-docs, kind: publish, from: plan, to: docs }
  - { id: ready, kind: tasks, from: docs/2-engineering/plans/*/tasks.md, leaves: docs/leaves.yaml }
  - { id: leaves, kind: init, from: docs/leaves.yaml }
`

func TestAGraphWritesDocumentsInAScratchWorkspaceAndPublishesThemIntoTheProject(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(stagesGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	spec := g.Nodes[3]
	if !spec.Scratch || spec.Repo != "" || !slices.Equal(spec.Inputs, []string{"docs/1-product", "brief.md"}) {
		t.Errorf("spec = %+v", spec)
	}
	if pub := g.Nodes[2]; pub.Kind != KindPublish || pub.From != "design" || pub.To != "docs" {
		t.Errorf("publish = %+v", pub)
	}
	if n := g.Nodes[8]; n.Kind != KindTasks || n.From != "docs/2-engineering/plans/*/tasks.md" || n.Leaves != "docs/leaves.yaml" {
		t.Errorf("tasks = %+v", n)
	}
	if n := g.Nodes[9]; n.Kind != KindInit || n.From != "docs/leaves.yaml" {
		t.Errorf("init = %+v", n)
	}
	if g.Nodes[1].Reruns != "design" || g.Nodes[4].Reruns != "spec" {
		t.Errorf("a gate re-runs the scratch agent before it: %q, %q", g.Nodes[1].Reruns, g.Nodes[4].Reruns)
	}
}

func TestParseGraphRefusesBadStageNodes(t *testing.T) {
	head := "name: g\nversion: 1.0.0\nschema: 1\nnodes:\n"
	scratch := "  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true }\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"a scratch agent with a repo":            {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, repo: repos/app }\n", "names no repo"},
		"a read-only scratch agent":              {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, read_only: true }\n", "writes its output"},
		"inputs without a scratch workspace":     {"  - { id: s, kind: agent, role: planner, prompt: p.md, repo: repos/app, inputs: [docs] }\n", "only a scratch agent"},
		"an input that climbs":                   {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [../x] }\n", "inside the project"},
		"the project root as an input":           {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [.] }\n", "not the project itself"},
		"the project root with a slash":          {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [\"./\"] }\n", "not the project itself"},
		"the .factory folder as an input":        {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [.factory] }\n", "inside the project"},
		"the .git folder as an input":            {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [.git] }\n", "inside the project"},
		"an input in .factory":                   {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [.factory/runs] }\n", "inside the project"},
		"an input in .git":                       {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [.git/config] }\n", "inside the project"},
		"an input twice":                         {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, inputs: [docs, docs] }\n", "twice"},
		"a publish without from":                 {"  - { id: p, kind: publish, to: docs }\n", bySchema},
		"a publish without to":                   {scratch + "  - { id: p, kind: publish, from: s }\n", bySchema},
		"a publish from a node that isn't":       {"  - { id: e, kind: exec, run: [echo] }\n  - { id: p, kind: publish, from: e, to: docs }\n", "scratch agent node before it"},
		"a publish from a later node":            {"  - { id: p, kind: publish, from: s, to: docs }\n" + scratch, "scratch agent node before it"},
		"a publish into the project root":        {scratch + "  - { id: p, kind: publish, from: s, to: . }\n", "a folder of the project"},
		"a publish that climbs":                  {scratch + "  - { id: p, kind: publish, from: s, to: ../docs }\n", "a folder of the project"},
		"a publish into .git":                    {scratch + "  - { id: p, kind: publish, from: s, to: .git/hooks }\n", "a folder of the project"},
		"a publish into .factory":                {scratch + "  - { id: p, kind: publish, from: s, to: .factory/runs }\n", "a folder of the project"},
		"tasks without from":                     {"  - { id: t, kind: tasks }\n", bySchema},
		"tasks from a file that climbs":          {"  - { id: t, kind: tasks, from: ../tasks.md }\n", "inside the project"},
		"init from a file that climbs":           {"  - { id: i, kind: init, from: ../leaves.yaml }\n", "inside the project"},
		"tasks naming a leaves file that climbs": {"  - { id: t, kind: tasks, from: docs/tasks.md, leaves: ../leaves.yaml }\n", "inside the project"},
		"init with a leaves file":                {"  - { id: i, kind: init, from: docs/leaves.yaml, leaves: docs/leaves.yaml }\n", bySchema},
		"init with a repo":                       {"  - { id: i, kind: init, from: docs/leaves.yaml, repo: repos/app }\n", bySchema},
		"a scratch flag on an exec node":         {"  - { id: e, kind: exec, run: [echo], scratch: true }\n", bySchema},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", []byte(head+c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}
