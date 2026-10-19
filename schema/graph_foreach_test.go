package schema

import (
	"slices"
	"strings"
	"testing"
)

// gallery is the shape of a graph pack's graph: stages that write documents, then a task shape that
// repeats for every ready task, in the leaf the task names.
const galleryGraph = `
name: gallery
version: 1.0.0
schema: 1
nodes:
  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md, repo: repos/app }
  - { id: plan-gate, kind: gate, gate: plan }
  - id: tasks
    kind: foreach
    over: tasks
    nodes:
      - id: implement
        kind: loop
        realm: leaf
        role: implementer
        reviewers:
          - { id: spec, role: reviewer, prompt: prompts/review-spec.md }
      - { id: feature-gate, kind: gate, gate: feature }
      - { id: land, kind: merge, realm: leaf }
      - { id: merge-gate, kind: gate, gate: merge }
      - { id: move, kind: pointer, realm: leaf, acceptance: [e2e], generated: [REPOS.md] }
  - { id: report, kind: exec, run: [echo, done], repo: . }
`

func TestAGraphRepeatsATaskShapeForEveryReadyTask(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(galleryGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	fe := g.Nodes[2]
	if fe.Kind != KindForeach || fe.Over != "tasks" || len(fe.Body) != 5 {
		t.Fatalf("the foreach node = %+v", fe)
	}
	loop := fe.Body[0]
	if loop.RealmFrom != RealmLeaf || loop.Attempts != DefaultAttempts || loop.Prompt != "" || len(loop.Checks) != 0 || loop.Reviewers[0].ID != "spec" {
		t.Errorf("the body's loop = %+v", loop)
	}
	if got := fe.Body[1].Reruns; got != "implement" {
		t.Errorf("feature-gate reruns %q, want implement (the nearest loop in the body)", got)
	}
	if got := fe.Body[3].Reruns; got != "implement" {
		t.Errorf("merge-gate reruns %q, want implement", got)
	}
	if g.Nodes[1].Reruns != "plan" {
		t.Errorf("plan-gate reruns %q, want plan", g.Nodes[1].Reruns)
	}
	if !slices.Equal(g.Nodes[2].Body[4].Acceptance, []string{"e2e"}) {
		t.Errorf("pointer = %+v", g.Nodes[2].Body[4])
	}
}

func TestParseGraphRefusesABadTaskShape(t *testing.T) {
	head := "name: g\nversion: 1.0.0\nschema: 1\nnodes:\n"
	fe := func(body string) string {
		return "  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n" + body
	}
	loop := "      - { id: implement, kind: loop, realm: leaf, role: implementer }\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"over something else":                {"  - id: t\n    kind: foreach\n    over: leaves\n    nodes:\n" + loop, bySchema},
		"a foreach with no body":             {"  - { id: t, kind: foreach, over: tasks, nodes: [] }\n", bySchema},
		"a foreach in a foreach":             {fe("      - id: inner\n        kind: foreach\n        over: tasks\n        nodes:\n" + strings.ReplaceAll(loop, "      -", "          -")), bySchema},
		"a repo in the body":                 {fe("      - { id: implement, kind: loop, realm: leaf, role: implementer, repo: repos/app }\n"), bySchema},
		"a loop of the graph pack":           {fe("      - { id: implement, kind: loop, role: implementer, prompt: p.md, checks: [vet] }\n"), bySchema},
		"a leaf loop with its own prompt":    {fe("      - { id: implement, kind: loop, realm: leaf, role: implementer, prompt: p.md }\n"), bySchema},
		"a leaf loop with its own checks":    {fe("      - { id: implement, kind: loop, realm: leaf, role: implementer, checks: [vet] }\n"), bySchema},
		"a merge with its own checks":        {fe(loop + "      - { id: land, kind: merge, realm: leaf, checks: [unit] }\n"), bySchema},
		"a merge of the graph pack":          {fe(loop + "      - { id: land, kind: merge, checks: [unit] }\n"), bySchema},
		"realm: lead (the value is gone)":    {fe("      - { id: implement, kind: loop, realm: lead, role: implementer }\n"), bySchema},
		"realm: leaf outside a foreach":      {"  - { id: implement, kind: loop, realm: leaf, role: implementer, repo: repos/app }\n", bySchema},
		"an id used in the body and outside": {fe(loop) + "  - { id: implement, kind: exec, run: [echo] }\n", "used twice"},
		"a gate in the body with no loop":    {fe("      - { id: g, kind: gate, gate: feature }\n"), "no agent or loop node before it"},
		"a gate that reruns a node outside":  {"  - { id: plan, kind: agent, role: planner, prompt: p.md }\n" + fe(loop+"      - { id: g, kind: gate, gate: feature, reruns: plan }\n"), "must be an agent or loop node before it"},
		"a body reviewer that climbs":        {fe("      - { id: implement, kind: loop, realm: leaf, role: implementer, reviewers: [{ id: spec, role: reviewer, prompt: ../r.md }] }\n"), "inside the realm"},
		"an exec node outside any repo":      {fe(loop + "      - { id: e, kind: exec, run: [echo], repo: . }\n"), bySchema},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", []byte(head+c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}

func TestAllListsTheBodyAfterItsForeachNode(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(galleryGraph))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range g.All() {
		ids = append(ids, n.ID)
	}
	want := "plan plan-gate tasks implement feature-gate land merge-gate move report"
	if strings.Join(ids, " ") != want {
		t.Errorf("All() = %s, want %s", strings.Join(ids, " "), want)
	}
}
