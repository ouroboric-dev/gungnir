package schema

import (
	"strings"
	"testing"
)

// conformanceGraph is the spec stage of the web realm (spec §4.2): the spec is written in a scratch
// workspace, a second scratch agent reviews it against the design, and the spec gate reads what that
// reviewer found before anything is published.
const conformanceGraph = `
name: conformance
version: 1.0.0
schema: 2
nodes:
  - { id: spec, kind: agent, role: planner, prompt: prompts/spec.md, scratch: true, input: [brief], output: ygg.spec/v1 }
  - { id: conformance, kind: agent, role: reviewer, prompt: prompts/conformance.md, scratch: true, reviews: spec, input: [spec], output: ygg.conformance/v1 }
  - { id: spec-gate, kind: gate, gate: spec, reruns: spec, deviation_from: conformance }
  - { id: spec-docs, kind: publish, from: spec, to: docs }
`

func TestAScratchReviewerReadsTheOutputOfTheAgentItReviewsAndTheSpecGateReadsItsVerdict(t *testing.T) {
	g, err := ParseGraph("graph.yaml", []byte(conformanceGraph))
	if err != nil {
		t.Fatalf("ParseGraph() = %v", err)
	}
	if c := g.Nodes[1]; c.Reviews != "spec" {
		t.Errorf("conformance.Reviews = %q, want spec", c.Reviews)
	}
	if gate := g.Nodes[2]; gate.DeviationFrom != "conformance" || gate.Reruns != "spec" {
		t.Errorf("spec-gate = %+v, want it to read conformance and re-run spec", gate)
	}
}

func TestAReviewerAndTheDeviationItReportsAreRefusedWhenTheyPointAtNothingReal(t *testing.T) {
	noScratch := strings.Replace(conformanceGraph, "id: spec, kind: agent, role: planner, prompt: prompts/spec.md, scratch: true, input: [brief], output: ygg.spec/v1", "id: spec, kind: agent, role: planner, prompt: prompts/spec.md, repo: repos/web, output: ygg.spec/v1", 1)
	noScratch = strings.Replace(noScratch, "  - { id: spec-gate, kind: gate, gate: spec, reruns: spec, deviation_from: conformance }\n  - { id: spec-docs, kind: publish, from: spec, to: docs }\n", "", 1)
	bad := map[string]struct{ graph, want string }{
		"reviews a node that is not there":      {strings.Replace(conformanceGraph, "reviews: spec", "reviews: nope", 1), "reviews"},
		"reviews itself":                        {strings.Replace(conformanceGraph, "reviews: spec", "reviews: conformance", 1), "reviews"},
		"reviews a node after it":               {strings.Replace(conformanceGraph, "reviews: spec", "reviews: spec-docs", 1), "reviews"},
		"reviews an agent that is no scratch":   {noScratch, "reviews"},
		"a reviewer that is no scratch agent":   {strings.Replace(conformanceGraph, "scratch: true, reviews: spec, input: [spec]", "reviews: spec", 1), "only a scratch agent reviews"},
		"a deviation read from no node":         {strings.Replace(conformanceGraph, "deviation_from: conformance", "deviation_from: nope", 1), "deviation_from"},
		"a deviation read from a plain scratch": {strings.Replace(conformanceGraph, "deviation_from: conformance", "deviation_from: spec", 1), "deviation_from"},
		"a deviation read by a gate not spec":   {strings.Replace(conformanceGraph, "gate: spec,", "gate: plan,", 1), "deviation_from"},
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
