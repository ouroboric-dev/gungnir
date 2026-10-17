package schema

import (
	"strings"
	"testing"
)

const returnsGraph = `
name: returns
version: 1.0.0
nodes:
  - { id: plan, kind: agent, role: planner, prompt: p.md, scratch: true }
  - { id: ready, kind: tasks, from: docs/tasks.md, back_to: plan, returns: 3 }
  - id: tasks
    kind: foreach
    over: tasks
    nodes:
      - { id: implement, kind: loop, realm: leaf, role: implementer }
      - { id: land, kind: merge, realm: leaf, back_to: implement }
      - { id: move, kind: pointer, realm: leaf, back_to: implement, returns: 1 }
`

func TestARefusalCanGoBackIntoAnEarlierNodeAFewTimes(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(returnsGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	ready, body := g.Nodes[1], g.Nodes[2].Body
	if ready.BackTo != "plan" || ready.Returns != 3 {
		t.Errorf("tasks = %+v", ready)
	}
	if body[1].BackTo != "implement" || body[1].Returns != DefaultReturns || DefaultReturns != 2 {
		t.Errorf("merge = %+v; returns default to %d", body[1], DefaultReturns)
	}
	if body[2].Returns != 1 {
		t.Errorf("pointer = %+v", body[2])
	}
}

func TestParseGraphRefusesBadReturns(t *testing.T) {
	head := "name: g\nversion: 1.0.0\nnodes:\n"
	agent := "  - { id: a, kind: agent, role: planner, prompt: p.md, repo: repos/app }\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"returns without back_to":        {agent + "  - { id: m, kind: merge, repo: repos/app, checks: [unit], returns: 2 }\n", bySchema},
		"zero returns":                   {agent + "  - { id: m, kind: merge, repo: repos/app, checks: [unit], back_to: a, returns: 0 }\n", bySchema},
		"six returns":                    {agent + "  - { id: m, kind: merge, repo: repos/app, checks: [unit], back_to: a, returns: 6 }\n", bySchema},
		"back_to a node that isn't":      {agent + "  - { id: m, kind: merge, repo: repos/app, checks: [unit], back_to: nope }\n", "must be an agent or loop node before it"},
		"back_to an exec node":           {"  - { id: e, kind: exec, run: [echo] }\n  - { id: m, kind: merge, repo: repos/app, checks: [unit], back_to: e }\n", "must be an agent or loop node before it"},
		"back_to a later node":           {"  - { id: m, kind: merge, repo: repos/app, checks: [unit], back_to: a }\n" + agent, "must be an agent or loop node before it"},
		"back_to itself":                 {agent + "  - { id: p, kind: pointer, repo: repos/app, back_to: p }\n", "must be an agent or loop node before it"},
		"back_to on an agent":            {agent + "  - { id: b, kind: agent, role: planner, prompt: p.md, repo: repos/app, back_to: a }\n", bySchema},
		"back_to on init":                {agent + "  - { id: i, kind: init, from: docs/leaves.yaml, back_to: a }\n", bySchema},
		"back_to from a body to outside": {agent + "  - id: t\n    kind: foreach\n    over: tasks\n    nodes:\n      - { id: implement, kind: loop, realm: leaf, role: implementer }\n      - { id: land, kind: merge, realm: leaf, back_to: a }\n", "must be an agent or loop node before it"},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", []byte(head+c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}
