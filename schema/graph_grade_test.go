package schema

import (
	"strings"
	"testing"
)

const gradeGraph = `
name: graded
version: 1.0.0
schema: 1
nodes:
  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md, scratch: true }
  - { id: build, kind: foreach, over: tasks, nodes: [ { id: impl, kind: loop, realm: leaf, role: implementer } ] }
  - { id: score, kind: grade }
`

func TestAGradeNodeEndsTheGraph(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(gradeGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	if n := g.Nodes[2]; n.Kind != KindGrade || n.ID != "score" {
		t.Errorf("the last node = %+v, want the grade node", n)
	}
}

func TestAGradeNodeThatCouldRunBesideASandboxIsRefused(t *testing.T) {
	bad := map[string]struct{ graph, want string }{
		"a node after it": {gradeGraph + "  - { id: late, kind: agent, role: planner, prompt: prompts/late.md, scratch: true }\n", "last node"},
		"two of them":     {strings.Replace(gradeGraph, "  - { id: score, kind: grade }\n", "  - { id: score, kind: grade }\n  - { id: again, kind: grade }\n", 1), "last node"},
		"in a foreach":    {strings.Replace(gradeGraph, "{ id: impl, kind: loop, realm: leaf, role: implementer }", "{ id: g, kind: grade }", 1), "kind"},
		"with a field":    {strings.Replace(gradeGraph, "{ id: score, kind: grade }", "{ id: score, kind: grade, repo: repos/web }", 1), "repo"},
		"with a command":  {strings.Replace(gradeGraph, "{ id: score, kind: grade }", "{ id: score, kind: grade, run: [sh] }", 1), "run"},
	}
	for name, c := range bad {
		_, err := ParseGraph("graph.yaml", []byte(c.graph))
		if err == nil {
			t.Errorf("%s: ParseGraph() = nil, want an error", name)
		} else if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want it to mention %q", name, err, c.want)
		}
	}
}
