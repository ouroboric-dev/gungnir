package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ouroboric-dev/gungnir"
)

const toyGraph = `
name: toy
version: 1.0.0
nodes:
  - { id: design, kind: agent, role: planner, prompt: prompts/design.md, timeout: 10m }
  - { id: design-gate, kind: gate, gate: design }
  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md }
  - { id: dor, kind: exec, run: [test, -s, plan.md] }
  - { id: plan-gate, kind: gate, gate: plan, reruns: plan }
`

func TestParseGraphResolvesWhatARejectionReruns(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(toyGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	if got := g.Nodes[1].Reruns; got != "design" {
		t.Errorf("design-gate reruns %q, want design (the node before it)", got)
	}
	if got := g.Nodes[4].Reruns; got != "plan" {
		t.Errorf("plan-gate reruns %q, want plan (named)", got)
	}
	if got := time.Duration(g.Nodes[0].Timeout); got != 10*time.Minute {
		t.Errorf("design timeout = %v, want 10m", got)
	}
	if g.Index("dor") != 3 || g.Index("nope") != -1 {
		t.Errorf("Index() is wrong")
	}
}

func TestParseGraphRefuses(t *testing.T) {
	head := "name: toy\nversion: 1.0.0\nnodes:\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"a duplicate id":                   {"  - { id: a, kind: exec, run: [echo] }\n  - { id: a, kind: exec, run: [echo] }\n", "used twice"},
		"a loop node (not yet)":            {"  - { id: a, kind: loop }\n", bySchema},
		"a gate with nothing before it":    {"  - { id: g, kind: gate, gate: design }\n", "no node before it"},
		"a gate that re-runs a later node": {"  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: plan, reruns: b }\n  - { id: b, kind: exec, run: [echo] }\n", "must be a node before it"},
		"a gate that re-runs a gate":       {"  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: plan }\n  - { id: h, kind: gate, gate: feature, reruns: g }\n", "must be a node before it"},
		"an unknown approval point":        {"  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: ship }\n", bySchema},
		"exec without a command":           {"  - { id: a, kind: exec }\n", bySchema},
		"agent with a command":             {"  - { id: a, kind: agent, role: planner, prompt: p.md, run: [echo] }\n", bySchema},
		"a boolean as a command":           {"  - { id: a, kind: exec, run: [true] }\n", bySchema}, // YAML reads true as a boolean; write "true"
		"a prompt outside the realm":       {"  - { id: a, kind: agent, role: planner, prompt: ../secrets.md }\n", "inside the realm"},
		"an absolute prompt path":          {"  - { id: a, kind: agent, role: planner, prompt: /etc/passwd }\n", "inside the realm"},
		"a timeout in words":               {"  - { id: a, kind: exec, run: [echo], timeout: 5 minutes }\n", bySchema},
		"no nodes":                         {"  []\n", bySchema},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", []byte(head+c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}

func TestGraphSchemaKnowsEveryApprovalPoint(t *testing.T) {
	for _, g := range gungnir.Gates() {
		y := "name: toy\nversion: 1.0.0\nnodes:\n  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: " + string(g) + " }\n"
		if _, err := ParseGraph("graph.yaml", []byte(y)); err != nil {
			t.Errorf("gate %s: %v", g, err)
		}
	}
}

func TestLoadPack(t *testing.T) {
	dir := t.TempDir()
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("name: toy\nversion: 0.1.0\ngraph: graph.yaml\nroles:\n  planner: { agent: scripted, model: none }\nchecks:\n  unit: [go, test, ./...]\n")
	p, err := LoadPack(dir)
	if err != nil {
		t.Fatalf("LoadPack() = %v", err)
	}
	if p.Graph != "graph.yaml" || p.Roles["planner"].Agent != "scripted" || len(p.Checks["unit"]) != 3 {
		t.Errorf("got %+v", p)
	}
	for name, s := range map[string]string{
		"a graph outside the realm": "name: toy\nversion: 0.1.0\ngraph: ../graph.yaml\n",
		"no version":                "name: toy\ngraph: graph.yaml\n",
		"an unknown field":          "name: toy\nversion: 0.1.0\ngraph: graph.yaml\nhooks: [x]\n",
	} {
		write(s)
		if _, err := LoadPack(dir); err == nil {
			t.Errorf("%s: LoadPack() = nil error", name)
		}
	}
}
