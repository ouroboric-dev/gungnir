package schema

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// packWith writes a graph pack's prompt files under a temporary folder and returns it.
func packWith(t *testing.T, prompts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range prompts {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const promptNodes = "" +
	"  - { id: design, kind: agent, role: planner, prompt: prompts/design.md, scratch: true, input: [brief], output: ygg.design/v1 }\n" +
	"  - { id: spec, kind: agent, role: planner, prompt: prompts/spec.md, scratch: true, input: [brief, design], output: ygg.spec/v1 }\n"

// planNodes is the plan, its readiness check and the head of a foreach over the tasks; a body follows.
const planNodes = "" +
	"  - { id: plan, kind: agent, role: planner, prompt: prompts/plan.md, scratch: true, input: [spec], output: ygg.plan/v1 }\n" +
	"  - { id: ready, kind: tasks, from: plan, leaves: docs/leaves.yaml }\n" +
	"  - id: each\n    kind: foreach\n    over: tasks\n    nodes:\n"

func TestCheckPromptsRefusesAnUndeclaredBuiltinSource(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{
		"prompts/design.md": "Work from the `brief`.",
		"prompts/spec.md":   "Read the `brief` and the `design`, then `decisions`.",
	})
	err = g.CheckPrompts(dir)
	want := "graph.yaml: node spec: prompts/spec.md names the input `decisions`, which the session does not declare"
	if err == nil || err.Error() != want {
		t.Errorf("CheckPrompts() = %v, want %q", err, want)
	}
}

func TestCheckPromptsRefusesAnUndeclaredNodeId(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{
		"prompts/design.md": "Work from the `brief` and the `spec`.",
		"prompts/spec.md":   "`brief` `design`",
	})
	err = g.CheckPrompts(dir)
	if err == nil || !strings.Contains(err.Error(), "node design: prompts/design.md names the input `spec`") {
		t.Errorf("CheckPrompts() = %v, want the input `spec` refused for design", err)
	}
}

func TestCheckPromptsAcceptsDeclaredInputsAndIgnoresOtherWords(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{
		"prompts/design.md": "Work from the `brief`. Write `out/design.json` and run `go test`. Say nothing of brief without ticks, or the spec.",
		"prompts/spec.md":   "Your inputs: `brief` and `design`. The word `reviewer` is neither a source nor a node.",
	})
	if err := g.CheckPrompts(dir); err != nil {
		t.Errorf("CheckPrompts() = %v, want nil", err)
	}
}

func TestCheckPromptsReadsEveryReviewer(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes+planNodes+
		"      - { id: note, kind: agent, role: planner, prompt: prompts/note.md, input: [task], output: ygg.spec/v1 }\n"+
		"      - id: implement\n        kind: loop\n        realm: leaf\n        role: implementer\n        input: [task]\n        output: ygg.implement/v1\n"+
		"        reviewers:\n"+
		"          - { id: spec, role: reviewer, prompt: prompts/review-spec.md, input: [task, change], output: ygg.review/v1 }\n"))
	if err != nil {
		t.Fatal(err)
	}
	prompts := map[string]string{
		"prompts/design.md":      "`brief`",
		"prompts/spec.md":        "`brief` `design`",
		"prompts/plan.md":        "`spec`",
		"prompts/note.md":        "`task`",
		"prompts/review-spec.md": "Read the `task` and the `change`, and the `brief`.",
	}
	err = g.CheckPrompts(packWith(t, prompts))
	want := "graph.yaml: node implement, reviewer spec: prompts/review-spec.md names the input `brief`, which the session does not declare"
	if err == nil || err.Error() != want {
		t.Errorf("CheckPrompts() = %v, want %q", err, want)
	}
	// A foreach body's own agent prompt is read too.
	prompts["prompts/review-spec.md"] = "`task` `change`"
	prompts["prompts/note.md"] = "`task` and the `plan`"
	err = g.CheckPrompts(packWith(t, prompts))
	if err == nil || !strings.Contains(err.Error(), "node note: prompts/note.md names the input `plan`") {
		t.Errorf("CheckPrompts() = %v, want the body agent's prompt refused for `plan`", err)
	}
	prompts["prompts/note.md"] = "`task`"
	if err := g.CheckPrompts(packWith(t, prompts)); err != nil {
		t.Errorf("CheckPrompts() = %v, want nil", err)
	}
}

func TestCheckPromptsRefusesAMissingFile(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{"prompts/design.md": "`brief`"})
	err = g.CheckPrompts(dir)
	if err == nil || !strings.Contains(err.Error(), "node spec: prompts/spec.md") || !strings.Contains(err.Error(), "cannot be read") {
		t.Errorf("CheckPrompts() = %v, want the missing prompts/spec.md named", err)
	}
}

func TestCheckPromptsSkipsASessionWithoutAPrompt(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes+planNodes+
		"      - id: implement\n        kind: loop\n        realm: leaf\n        role: implementer\n        input: [task]\n        output: ygg.implement/v1\n"+
		"        reviewers:\n"+
		"          - { id: spec, role: reviewer, prompt: prompts/review-spec.md, input: [task, change], output: ygg.review/v1 }\n"))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{"prompts/design.md": "`brief`", "prompts/spec.md": "`design`", "prompts/plan.md": "`spec`", "prompts/review-spec.md": "`task`"})
	if err := g.CheckPrompts(dir); err != nil {
		t.Errorf("CheckPrompts() = %v, want nil: the realm's loop has no prompt here", err)
	}
}

func TestCheckPromptsRefusesAPathOutsideThePack(t *testing.T) {
	g := &Graph{Nodes: []Node{{ID: "design", Kind: KindAgent, Prompt: "../outside.md", Output: string(ContractDesign)}}}
	err := g.CheckPrompts(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "must be a path inside the pack") {
		t.Errorf("CheckPrompts() = %v, want the path refused", err)
	}
}

func TestPromptInputs(t *testing.T) {
	text := "Use `brief`, `design` and `brief` again. Not `out/x.json`, `Brief`.\n```\nfenced `task`\n```\n"
	got := PromptInputs(text, []string{"design", "spec"})
	if want := []string{"brief", "design", "task"}; !slices.Equal(got, want) {
		t.Errorf("PromptInputs() = %v, want %v", got, want)
	}
}

func TestCheckPromptsCountsANameInAFencedBlock(t *testing.T) {
	g, err := ParseGraph("graph.yaml", typed(promptNodes))
	if err != nil {
		t.Fatal(err)
	}
	dir := packWith(t, map[string]string{
		"prompts/design.md": "`brief`",
		"prompts/spec.md":   "`brief` `design`\n```\nsee `decisions`\n```\n",
	})
	err = g.CheckPrompts(dir)
	if err == nil || !strings.Contains(err.Error(), "names the input `decisions`") {
		t.Errorf("CheckPrompts() = %v, want the fenced `decisions` refused", err)
	}
}

func TestCheckPromptsWalksANestedForeach(t *testing.T) {
	g := &Graph{Nodes: []Node{
		{ID: "plan", Kind: KindAgent, Output: string(ContractPlan)},
		{ID: "outer", Kind: KindForeach, Body: []Node{
			{ID: "inner", Kind: KindForeach, Body: []Node{
				{ID: "deep", Kind: KindAgent, Prompt: "prompts/deep.md", Input: []string{SourceTask}, Output: string(ContractSpec)},
			}},
		}},
	}}
	dir := packWith(t, map[string]string{"prompts/deep.md": "`task` and `plan`"})
	err := g.CheckPrompts(dir)
	want := "node deep: prompts/deep.md names the input `plan`, which the session does not declare"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("CheckPrompts() = %v, want an error containing %q", err, want)
	}
}
