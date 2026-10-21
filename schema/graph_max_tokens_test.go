package schema

import (
	"strings"
	"testing"
)

func TestParseGraphMaxTokens(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(strings.Replace(loopGraph, "    max_turns: 60\n", "    max_turns: 60\n    max_tokens: 400000\n", 1)))
	if err != nil {
		t.Fatalf("a loop with max_tokens: %v", err)
	}
	if g.Nodes[0].MaxTokens != 400000 {
		t.Errorf("the loop's max_tokens = %d, want 400000", g.Nodes[0].MaxTokens)
	}
	g, err = ParseGraph("graph.yaml", []byte(schemaHead+"schema: 1\nnodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md, max_tokens: 50000 }\n"))
	if err != nil {
		t.Fatalf("an agent with max_tokens: %v", err)
	}
	if g.Nodes[0].MaxTokens != 50000 {
		t.Errorf("the agent's max_tokens = %d, want 50000", g.Nodes[0].MaxTokens)
	}
	g, err = ParseGraph("graph.yaml", []byte(schemaHead+"schema: 1\nnodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md, max_tokens: 0 }\n"))
	if err != nil {
		t.Errorf("max_tokens: 0 is no limit: %v", err)
	}
}

func TestParseGraphRefusesBadMaxTokens(t *testing.T) {
	head := schemaHead + "schema: 1\nnodes:\n"
	cases := map[string]string{
		"a negative value on an agent": "  - { id: a, kind: agent, role: planner, prompt: p.md, max_tokens: -1 }\n",
		"a negative value on a loop":   "  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, checks: [vet], max_tokens: -5 }\n",
		"a fraction":                   "  - { id: a, kind: agent, role: planner, prompt: p.md, max_tokens: 1.5 }\n",
		"on an exec":                   "  - { id: a, kind: exec, run: [echo], max_tokens: 10 }\n",
		"on a gate":                    "  - { id: a, kind: gate, gate: feature, max_tokens: 10 }\n",
		"on a merge":                   "  - { id: a, kind: merge, realm: leaf, max_tokens: 10 }\n",
	}
	for name, nodes := range cases {
		_, err := ParseGraph("graph.yaml", []byte(head+nodes))
		if err == nil || !strings.Contains(err.Error(), "max_tokens") || !strings.Contains(err.Error(), "nodes/0") {
			t.Errorf("%s: ParseGraph() = %v, want an error naming the node and max_tokens", name, err)
		}
	}
}
