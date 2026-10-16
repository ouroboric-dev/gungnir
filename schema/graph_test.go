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
	agent := "  - { id: a, kind: agent, role: planner, prompt: p.md }\n"
	const bySchema = "jsonschema validation failed"
	cases := map[string]struct{ nodes, want string }{
		"a duplicate id":                   {"  - { id: a, kind: exec, run: [echo] }\n  - { id: a, kind: exec, run: [echo] }\n", "used twice"},
		"a loop node (not yet)":            {"  - { id: a, kind: loop }\n", bySchema},
		"a gate with nothing before it":    {"  - { id: g, kind: gate, gate: design }\n", "no agent node before it"},
		"a gate after only exec nodes":     {"  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: plan }\n", "no agent node before it"},
		"a gate that re-runs a later node": {agent + "  - { id: g, kind: gate, gate: plan, reruns: b }\n  - { id: b, kind: agent, role: planner, prompt: p.md }\n", "must be an agent node before it"},
		"a gate that re-runs a gate":       {agent + "  - { id: g, kind: gate, gate: plan }\n  - { id: h, kind: gate, gate: feature, reruns: g }\n", "must be an agent node before it"},
		"a gate that re-runs an exec node": {agent + "  - { id: e, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: plan, reruns: e }\n", "must be an agent node before it"},
		"an unknown approval point":        {"  - { id: a, kind: exec, run: [echo] }\n  - { id: g, kind: gate, gate: ship }\n", bySchema},
		"exec without a command":           {"  - { id: a, kind: exec }\n", bySchema},
		"agent with a command":             {"  - { id: a, kind: agent, role: planner, prompt: p.md, run: [echo] }\n", bySchema},
		"a boolean as a command":           {"  - { id: a, kind: exec, run: [true] }\n", bySchema}, // YAML reads true as a boolean; write "true"
		"a prompt outside the realm":       {"  - { id: a, kind: agent, role: planner, prompt: ../secrets.md }\n", "inside the realm"},
		"an absolute prompt path":          {"  - { id: a, kind: agent, role: planner, prompt: /etc/passwd }\n", "inside the realm"},
		"a timeout in words":               {"  - { id: a, kind: exec, run: [echo], timeout: 5 minutes }\n", bySchema},
		"no nodes":                         {"  []\n", bySchema},
		"a zero timeout":                   {"  - { id: a, kind: exec, run: [echo], timeout: 0s }\n", bySchema},
		"a repo that climbs":               {"  - { id: a, kind: exec, run: [echo], repo: ../x }\n", bySchema},
		"a merge without checks":           {"  - { id: m, kind: merge, repo: repos/app }\n", bySchema},
		"a merge without a repo":           {"  - { id: m, kind: merge, checks: [unit] }\n", bySchema},
		"a merge of the project itself":    {"  - { id: m, kind: merge, repo: ., checks: [unit] }\n", "must be a child"},
		"a pointer for the project itself": {"  - { id: p, kind: pointer, repo: . }\n", "must be a child"},
		"a generated file that climbs":     {"  - { id: p, kind: pointer, repo: repos/app, generated: [../REPOS.md] }\n", "inside the umbrella"},
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
		y := "name: toy\nversion: 1.0.0\nnodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md }\n  - { id: g, kind: gate, gate: " + string(g) + " }\n"
		if _, err := ParseGraph("graph.yaml", []byte(y)); err != nil {
			t.Errorf("gate %s: %v", g, err)
		}
	}
}

const treeGraph = `
name: toy
version: 2.0.0
nodes:
  - { id: build, kind: agent, role: implementer, prompt: p.md, repo: repos/app }
  - { id: test, kind: exec, run: [go, test, ./...], repo: repos/app }
  - { id: index, kind: exec, run: [make-index], repo: . }
  - { id: merge-gate, kind: gate, gate: merge }
  - { id: merge, kind: merge, repo: repos/app, checks: [unit] }
  - { id: pointer-gate, kind: gate, gate: pointer }
  - { id: pointer, kind: pointer, repo: repos/app, checks: [unit], acceptance: [e2e], generated: [REPOS.md] }
`

func TestParseGraphMergeAndPointerNodes(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(treeGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	if got := g.Nodes[3].Reruns; got != "build" {
		t.Errorf("merge-gate reruns %q, want build (the nearest agent node; a rejection's reason is an agent's input)", got)
	}
	m, p := g.Nodes[4], g.Nodes[6]
	if m.Kind != KindMerge || m.Repo != "repos/app" || len(m.Checks) != 1 {
		t.Errorf("merge = %+v", m)
	}
	if p.Kind != KindPointer || p.Acceptance[0] != "e2e" || p.Generated[0] != "REPOS.md" {
		t.Errorf("pointer = %+v", p)
	}
	if g.Nodes[2].Repo != "." {
		t.Errorf("index runs in %q, want . (the project umbrella)", g.Nodes[2].Repo)
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
	if err := os.MkdirAll(filepath.Join(dir, "templates", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"LICENSE", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, "templates", "app", f), []byte(f+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(leaves("templates/app", "LICENSE, README.md"))
	if p, err = LoadPack(dir); err != nil {
		t.Fatalf("LoadPack() with leaves = %v", err)
	}
	if l := p.Leaves["repos/app"]; l.Template != "templates/app" || len(l.FirstCommit) != 2 || l.Description != "the toy app" {
		t.Errorf("leaves = %+v", p.Leaves)
	}
	for name, s := range map[string]string{
		"a graph outside the realm": "name: toy\nversion: 0.1.0\ngraph: ../graph.yaml\n",
		"no version":                "name: toy\ngraph: graph.yaml\n",
		"an unknown field":          "name: toy\nversion: 0.1.0\ngraph: graph.yaml\nhooks: [x]\n",
		"a template outside":        leaves("../templates/app", "LICENSE"),
		"a missing first file":      leaves("templates/app", "NOTICE"),
		"a first file that climbs":  leaves("templates/app", "../../pack.yaml"),
		"an empty first commit":     "name: toy\nversion: 0.1.0\ngraph: graph.yaml\nleaves:\n  repos/app: { description: x, template: templates/app, first_commit: [] }\n",
		"a leaf path that climbs":   "name: toy\nversion: 0.1.0\ngraph: graph.yaml\nleaves:\n  ../app: { description: x, template: templates/app, first_commit: [LICENSE] }\n",
	} {
		write(s)
		if _, err := LoadPack(dir); err == nil {
			t.Errorf("%s: LoadPack() = nil error", name)
		}
	}
}

// leaves returns a pack.yaml with one leaf, repos/app, made from template with the given first files.
func leaves(template, first string) string {
	return "name: toy\nversion: 0.1.0\ngraph: graph.yaml\nleaves:\n  repos/app: { description: the toy app, template: " +
		template + ", first_commit: [" + first + "] }\n"
}
