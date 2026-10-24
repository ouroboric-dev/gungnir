package schema

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// typed wraps nodes in a schema 2 graph.
func typed(nodes string) []byte {
	return []byte("name: typed\nversion: 1.0.0\nschema: 2\nnodes:\n" + nodes)
}

func TestAGraphWithAMissingInputDoesNotLoad(t *testing.T) {
	cases := map[string]struct{ nodes, want string }{
		"an input that comes after the node": {
			"  - { id: conformance, kind: agent, role: reviewer, prompt: c.md, scratch: true, input: [spec], output: ygg.conformance/v1 }\n" +
				"  - { id: spec, kind: agent, role: planner, prompt: s.md, scratch: true, output: ygg.spec/v1 }\n",
			"node conformance: input spec is a node that comes after it"},
		"a deviation read from the wrong contract": {
			"  - { id: spec, kind: agent, role: planner, prompt: s.md, scratch: true, output: ygg.spec/v1 }\n" +
				"  - { id: conformance, kind: agent, role: reviewer, prompt: c.md, scratch: true, reviews: spec, output: ygg.spec/v1 }\n" +
				"  - { id: spec-gate, kind: gate, gate: spec, reruns: spec, deviation_from: conformance }\n",
			"node spec-gate: deviation_from conformance outputs ygg.spec/v1, not ygg.conformance/v1"},
		"a reviewer without an output": {
			"  - id: implement\n    kind: loop\n    repo: repos/app\n    role: implementer\n    prompt: i.md\n    checks: [vet]\n    output: ygg.implement/v1\n" +
				"    reviewers:\n      - { id: spec, role: reviewer, prompt: r.md, output: ygg.review/v1 }\n      - { id: quality, role: reviewer, prompt: q.md }\n",
			"node implement: reviewer quality has no output: every agent session returns one contract"},
		"an input that is a path": {
			"  - { id: plan, kind: agent, role: planner, prompt: p.md, scratch: true, output: ygg.plan/v1 }\n" +
				"  - { id: reuse-review, kind: agent, role: reviewer, prompt: r.md, scratch: true, reviews: plan, input: [plan, docs/leaves.yaml], output: ygg.reuse-review/v1 }\n",
			"node reuse-review: input docs/leaves.yaml is neither a node nor a built-in source"},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", typed(c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}

func TestTheSoftwareGraphShapeLoads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "graphs", "software.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGraph("software.yaml", data)
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	if last := g.Nodes[len(g.Nodes)-1]; last.ID != "retro" || last.Output != string(ContractRetro) || g.Nodes[len(g.Nodes)-2].Kind != KindGrade {
		t.Errorf("the last nodes = %+v, want the retro after the grade", last)
	}
	conformance := g.Nodes[g.Index("conformance")]
	if !slices.Equal(conformance.Input, []string{"spec", "design"}) || conformance.Output != string(ContractConformance) {
		t.Errorf("conformance = %+v, want input [spec design] and output %s", conformance, ContractConformance)
	}
	loop := g.Nodes[g.Index("tasks")].Body[0]
	if loop.Output != string(ContractImplement) || len(loop.Reviewers) != 3 || loop.Reviewers[2].Output != string(ContractReview) {
		t.Errorf("the loop = %+v, want the implement contract and three reviewers returning the review", loop)
	}
	// Each declaration is needed: dropping one no longer loads.
	for name, c := range map[string][3]string{
		"the plan's output":         {"    output: ygg.plan/v1\n", "", "node plan has no output"},
		"a reviewer's output":       {"            output: ygg.review/v1\n", "", "reviewer spec-compliance has no output"},
		"the retro's output":        {"    output: ygg.retro/v1\n", "", "retro is not one"},
		"the tasks node's plan":     {"    from: plan\n    leaves", "    from: spec\n    leaves", "outputs ygg.spec/v1, not ygg.plan/v1"},
		"the design's rejection":    {"    input: [brief, decisions, rejection]\n", "    input: [brief, decisions]\n", "node design: design-gate re-runs it on a rejection"},
		"the spec's rejection":      {"    input: [design, decisions, brief, rejection, refusal]\n", "    input: [design, decisions, brief, refusal]\n", "node spec: spec-gate re-runs it on a rejection"},
		"the plan's refusal":        {"    input: [spec, design, decisions, rejection, refusal]\n", "    input: [spec, design, decisions, rejection]\n", "node plan: ready goes back to it with a refusal"},
		"the implementer's refusal": {"        input: [task, open_findings, check_results, rejection, refusal]\n", "        input: [task, open_findings, check_results, rejection]\n", "node implement: land goes back to it with a refusal"},
	} {
		_, err := ParseGraph("software.yaml", []byte(strings.Replace(string(data), c[0], c[1], 1)))
		if err == nil || !strings.Contains(err.Error(), c[2]) {
			t.Errorf("without %s: ParseGraph() = %v, want an error containing %q", name, err, c[2])
		}
	}
}

func TestAGraphWithInputsFoldersDoesNotLoad(t *testing.T) {
	_, err := ParseGraph("graph.yaml", typed("  - { id: spec, kind: agent, role: planner, prompt: s.md, scratch: true, inputs: [docs/1-product], output: ygg.spec/v1 }\n"))
	if err == nil || !strings.Contains(err.Error(), "inputs") {
		t.Errorf("ParseGraph() = %v, want an error naming inputs", err)
	}
}

func TestAGraphForSchemaOneDoesNotLoad(t *testing.T) {
	_, err := ParseGraph("graph.yaml", []byte("name: old\nversion: 1.0.0\nschema: 1\nnodes:\n  - { id: a, kind: agent, role: planner, prompt: p.md }\n"))
	want := "this graph was written for schema 1; declare each node's input and output (change typed-boundaries)"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("ParseGraph() = %v, want an error containing %q", err, want)
	}
}

func TestParseGraphRefusesBadTypes(t *testing.T) {
	const spec = "  - { id: spec, kind: agent, role: planner, prompt: s.md, scratch: true, output: ygg.spec/v1 }\n"
	const plan = "  - { id: plan, kind: agent, role: planner, prompt: p.md, scratch: true, output: ygg.plan/v1 }\n"
	cases := map[string]struct{ nodes, want string }{
		"an agent without an output":                 {"  - { id: a, kind: agent, role: planner, prompt: p.md }\n", "node a has no output"},
		"a loop without an output":                   {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, checks: [vet] }\n", "node a has no output"},
		"an unknown contract":                        {"  - { id: a, kind: agent, role: planner, prompt: p.md, output: ygg.notes/v1 }\n", "node a: output ygg.notes/v1 is not a contract"},
		"an unknown contract on a review":            {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, output: ygg.x/v1 }] }\n", "node a: reviewer r: output ygg.x/v1 is not a contract"},
		"an input twice":                             {spec + "  - { id: a, kind: agent, role: planner, prompt: p.md, input: [spec, spec], output: ygg.plan/v1 }\n", "node a: input spec is listed twice"},
		"a built-in source twice":                    {"  - { id: a, kind: agent, role: planner, prompt: p.md, input: [brief, brief], output: ygg.plan/v1 }\n", "input brief is listed twice"},
		"an input of itself":                         {"  - { id: a, kind: agent, role: planner, prompt: p.md, input: [a], output: ygg.plan/v1 }\n", "node a: input a names itself"},
		"an input with no output":                    {"  - { id: e, kind: exec, run: [echo] }\n  - { id: a, kind: agent, role: planner, prompt: p.md, input: [e], output: ygg.plan/v1 }\n", "node a: input e is a node with no output"},
		"a node named like a source":                 {"  - { id: brief, kind: agent, role: planner, prompt: p.md, output: ygg.plan/v1 }\n", "node brief: the id is a built-in source's name"},
		"a reviewer input of its loop":               {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, input: [a], output: ygg.review/v1 }] }\n", "node a: reviewer r: input a names its own loop"},
		"a reviewer input after the loop":            {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, input: [later], output: ygg.review/v1 }] }\n" + "  - { id: later, kind: agent, role: planner, prompt: l.md, scratch: true, output: ygg.spec/v1 }\n", "node a: reviewer r: input later is a node that comes after it"},
		"input on an exec node":                      {"  - { id: e, kind: exec, run: [echo], input: [brief] }\n", "jsonschema validation failed"},
		"output on a gate":                           {spec + "  - { id: g, kind: gate, gate: spec, output: ygg.spec/v1 }\n", "jsonschema validation failed"},
		"a loop that outputs a plan":                 {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, checks: [vet], output: ygg.plan/v1 }\n", "node a: output ygg.plan/v1 must be ygg.implement/v1"},
		"a reviewer that outputs a spec":             {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, output: ygg.spec/v1 }] }\n", "node a: reviewer r: output ygg.spec/v1 must be ygg.review/v1"},
		"a design node reading the task":             {"  - { id: design, kind: agent, role: planner, prompt: p.md, scratch: true, input: [task], output: ygg.design/v1 }\n", "node design: input task is a source only inside a foreach body"},
		"a node before the grade reading the record": {"  - { id: design, kind: agent, role: planner, prompt: p.md, scratch: true, input: [record], output: ygg.design/v1 }\n  - { id: score, kind: grade }\n", "node design: input record is a source only for a node after the grade"},
		"a top-level agent reading the change":       {"  - { id: a, kind: agent, role: planner, prompt: p.md, scratch: true, input: [change], output: ygg.plan/v1 }\n", "node a: input change is a source only for a loop and its reviewers"},
		"a body agent reading the open findings":     {"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n      - { id: x, kind: agent, role: reviewer, prompt: x.md, read_only: true, input: [open_findings], output: ygg.review/v1 }\n", "node x: input open_findings is a source only for a loop and its reviewers"},
		"a top-level loop reading the task":          {"  - { id: a, kind: loop, repo: repos/app, role: implementer, prompt: p.md, checks: [vet], input: [task], output: ygg.implement/v1 }\n", "node a: input task is a source only inside a foreach body"},
		"a publish of a later node":                  {"  - { id: p, kind: publish, from: spec, to: docs }\n" + spec, "scratch agent node before it"},
		"tasks from a glob":                          {plan + "  - { id: ready, kind: tasks, from: docs/plans/tasks.md }\n", "jsonschema validation failed"},
		"tasks from a node that is not":              {plan + "  - { id: ready, kind: tasks, from: nope }\n", "node ready: from nope is not a node before it"},
		"tasks from the wrong contract":              {spec + "  - { id: ready, kind: tasks, from: spec }\n", "node ready: from spec outputs ygg.spec/v1, not ygg.plan/v1"},
		"a reviews that is no earlier node":          {spec + "  - { id: c, kind: agent, role: reviewer, prompt: c.md, scratch: true, reviews: nope, output: ygg.conformance/v1 }\n", "reviews"},
		"a body input out of scope":                  {"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n      - { id: implement, kind: loop, realm: leaf, role: implementer, output: ygg.implement/v1 }\n" + "  - { id: after, kind: agent, role: planner, prompt: p.md, input: [implement], output: ygg.plan/v1 }\n", "node after: input implement is a node out of its scope"},
		"a grade followed by a non-retro":            {"  - { id: score, kind: grade }\n  - { id: late, kind: agent, role: planner, prompt: p.md, scratch: true, input: [record], output: ygg.plan/v1 }\n", "last node"},
		"a grade followed by a leaf agent":           {"  - { id: score, kind: grade }\n  - { id: late, kind: agent, role: planner, prompt: p.md, repo: repos/app, input: [record], output: ygg.retro/v1 }\n", "last node"},
		"a grade followed by an exec":                {"  - { id: score, kind: grade }\n  - { id: late, kind: exec, run: [echo] }\n", "last node"}, "a gate's named node without the rejection": {spec + "  - { id: spec-gate, kind: gate, gate: spec, reruns: spec }\n", "node spec: spec-gate re-runs it on a rejection; declare rejection in its input"},
		"a gate's nearest node without the rejection": {"  - { id: design, kind: agent, role: planner, prompt: d.md, scratch: true, input: [brief, refusal], output: ygg.design/v1 }\n  - { id: design-gate, kind: gate, gate: design }\n", "node design: design-gate re-runs it on a rejection; declare rejection in its input"},
		"a gate's loop without the rejection":         {"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n      - { id: implement, kind: loop, realm: leaf, role: implementer, input: [task, refusal], output: ygg.implement/v1 }\n      - { id: merge-gate, kind: gate, gate: merge }\n", "node implement: merge-gate re-runs it on a rejection; declare rejection in its input"},
		"a back_to node without the refusal":          {"  - { id: plan, kind: agent, role: planner, prompt: p.md, scratch: true, input: [rejection], output: ygg.plan/v1 }\n  - { id: ready, kind: tasks, from: plan, back_to: plan }\n", "node plan: ready goes back to it with a refusal; declare refusal in its input"},
		"a back_to loop without the refusal":          {"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n      - { id: implement, kind: loop, realm: leaf, role: implementer, input: [task, rejection], output: ygg.implement/v1 }\n      - { id: land, kind: merge, realm: leaf, back_to: implement }\n", "node implement: land goes back to it with a refusal; declare refusal in its input"},
	}
	for name, c := range cases {
		_, err := ParseGraph("graph.yaml", typed(c.nodes))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ParseGraph() = %v, want an error containing %q", name, err, c.want)
		}
	}
}

func TestParseGraphLoadsTypedJoins(t *testing.T) {
	const spec = "  - { id: spec, kind: agent, role: planner, prompt: s.md, scratch: true, input: [brief, decisions, date], output: ygg.spec/v1 }\n"
	cases := map[string]string{
		"reviews counts as an input, listed or not": spec +
			"  - { id: c1, kind: agent, role: reviewer, prompt: c.md, scratch: true, reviews: spec, output: ygg.conformance/v1 }\n" +
			"  - { id: c2, kind: agent, role: reviewer, prompt: c.md, scratch: true, reviews: spec, input: [spec], output: ygg.conformance/v1 }\n",
		"a foreach body reads earlier outer nodes and earlier body nodes": spec +
			"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n" +
			"      - { id: implement, kind: loop, realm: leaf, role: implementer, input: [task, spec], output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, input: [change, spec], output: ygg.review/v1 }] }\n" +
			"      - { id: extra, kind: agent, role: reviewer, prompt: e.md, read_only: true, input: [implement, spec], output: ygg.review/v1 }\n",
		"a read-only retro after the grade": spec + "  - { id: score, kind: grade }\n" +
			"  - { id: retro, kind: agent, role: planner, prompt: r.md, read_only: true, input: [record], output: ygg.retro/v1 }\n" +
			"  - { id: retro2, kind: agent, role: planner, prompt: r.md, scratch: true, input: [retro], output: ygg.retro/v1 }\n",
		"a non-scratch agent with inputs": "  - { id: a, kind: agent, role: implementer, prompt: p.md, repo: repos/app, input: [brief], output: ygg.implement/v1 }\n", "re-run targets that declare their reason": "  - { id: plan, kind: agent, role: planner, prompt: p.md, scratch: true, input: [brief, rejection, refusal], output: ygg.plan/v1 }\n" +
			"  - { id: ready, kind: tasks, from: plan, back_to: plan }\n  - { id: plan-gate, kind: gate, gate: plan }\n" +
			"  - id: tasks\n    kind: foreach\n    over: tasks\n    nodes:\n" +
			"      - { id: implement, kind: loop, realm: leaf, role: implementer, input: [task, rejection, refusal], output: ygg.implement/v1 }\n" +
			"      - { id: merge-gate, kind: gate, gate: merge }\n      - { id: land, kind: merge, realm: leaf, back_to: implement }\n",
	}
	cases["a top-level loop reads its own findings, checks and change"] =
		"  - { id: implement, kind: loop, repo: repos/app, role: implementer, prompt: i.md, checks: [vet], input: [open_findings, check_results], output: ygg.implement/v1, reviewers: [{ id: r, role: reviewer, prompt: r.md, input: [change, open_findings], output: ygg.review/v1 }] }\n"
	for name, nodes := range cases {
		if _, err := ParseGraph("graph.yaml", typed(nodes)); err != nil {
			t.Errorf("%s: ParseGraph() = %v", name, err)
		}
	}
}
