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
schema: 2
nodes:
  - { id: design, kind: agent, role: planner, prompt: prompts/design.md, scratch: true, input: [brief, rejection], output: ygg.design/v1 }
  - { id: design-gate, kind: gate, gate: design }
  - { id: design-docs, kind: publish, from: design, to: docs }
  - { id: spec, kind: agent, role: planner, prompt: prompts/spec.md, scratch: true, input: [design, brief, rejection], output: ygg.spec/v1, max_turns: 30 }
  - { id: spec-gate, kind: gate, gate: spec }
  - { id: spec-docs, kind: publish, from: spec, to: docs }
  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md, scratch: true, input: [spec, design], output: ygg.plan/v1 }
  - { id: plan-docs, kind: publish, from: plan, to: docs }
  - { id: ready, kind: tasks, from: plan, leaves: docs/leaves.yaml }
  - { id: leaves, kind: init, from: docs/leaves.yaml }
`

func TestAGraphWritesDocumentsInAScratchWorkspaceAndPublishesThemIntoTheProject(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(stagesGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	spec := g.Nodes[3]
	if !spec.Scratch || spec.Repo != "" || !slices.Equal(spec.Input, []string{"design", "brief", "rejection"}) || spec.Output != string(ContractSpec) {
		t.Errorf("spec = %+v", spec)
	}
	if pub := g.Nodes[2]; pub.Kind != KindPublish || pub.From != "design" || pub.To != "docs" {
		t.Errorf("publish = %+v", pub)
	}
	if n := g.Nodes[8]; n.Kind != KindTasks || n.From != "plan" || n.Leaves != "docs/leaves.yaml" {
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
	head := "name: g\nversion: 1.0.0\nschema: 2\nnodes:\n"
	scratch := "  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, output: ygg.plan/v1 }\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"a scratch agent with a repo":            {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, repo: repos/app }\n", "names no repo"},
		"a read-only scratch agent":              {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, read_only: true }\n", "writes its output"},
		"input on a node that takes none":        {"  - { id: e, kind: exec, run: [echo], input: [brief] }\n", bySchema},
		"an input that climbs":                   {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [../x], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"the project root as an input":           {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [.], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"the project root with a slash":          {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [\"./\"], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"the .factory folder as an input":        {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [.factory], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"the .git folder as an input":            {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [.git], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"an input in .factory":                   {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [.factory/runs], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"an input in .git":                       {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [.git/config], output: ygg.plan/v1 }\n", "neither a node nor a built-in source"},
		"an input twice":                         {"  - { id: s, kind: agent, role: planner, prompt: p.md, scratch: true, input: [brief, brief], output: ygg.plan/v1 }\n", "twice"},
		"a publish without from":                 {"  - { id: p, kind: publish, to: docs }\n", bySchema},
		"a publish without to":                   {scratch + "  - { id: p, kind: publish, from: s }\n", bySchema},
		"a publish from a node that isn't":       {"  - { id: e, kind: exec, run: [echo] }\n  - { id: p, kind: publish, from: e, to: docs }\n", "scratch agent node before it"},
		"a publish from a later node":            {"  - { id: p, kind: publish, from: s, to: docs }\n" + scratch, "scratch agent node before it"},
		"a publish into the project root":        {scratch + "  - { id: p, kind: publish, from: s, to: . }\n", "a folder of the project"},
		"a publish that climbs":                  {scratch + "  - { id: p, kind: publish, from: s, to: ../docs }\n", "a folder of the project"},
		"a publish into .git":                    {scratch + "  - { id: p, kind: publish, from: s, to: .git/hooks }\n", "a folder of the project"},
		"a publish into .factory":                {scratch + "  - { id: p, kind: publish, from: s, to: .factory/runs }\n", "a folder of the project"},
		"tasks without from":                     {"  - { id: t, kind: tasks }\n", bySchema},
		"tasks from a file":                      {"  - { id: t, kind: tasks, from: ../tasks.md }\n", bySchema},
		"init from a file that climbs":           {"  - { id: i, kind: init, from: ../leaves.yaml }\n", "inside the project"},
		"tasks naming a leaves file that climbs": {scratch + "  - { id: t, kind: tasks, from: s, leaves: ../leaves.yaml }\n", "inside the project"},
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
