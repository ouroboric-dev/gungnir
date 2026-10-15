package schema

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

const scriptedRoles = `
roles:
  planner: { agent: scripted, model: none, script: toy.script.yaml }
`

const paidRoles = `
roles:
  planner: { agent: claude-code, model: opus }
  reviewer-spec: { agent: opencode, model: openrouter/qwen }
`

const fullCaps = `
budget:
  per_run: { tokens: 1000, turns: 10 }
  per_day: { tokens: 2000, turns: 20 }
`

func TestFactorySchemaRequiresCaps(t *testing.T) {
	cases := []struct {
		name, yaml string
		missing    []string
	}{
		{"no caps", scriptedRoles, []string{"budget.per_run", "budget.per_day"}},
		{"a per-run cap but no per-day cap", scriptedRoles + "budget:\n  per_run: { tokens: 1000, turns: 10 }\n",
			[]string{"budget.per_day"}},
		{"a cap without turns", scriptedRoles + "budget:\n  per_run: { tokens: 1000 }\n  per_day: { tokens: 2000, turns: 20 }\n",
			[]string{"budget.per_run.turns"}},
		{"an empty file", "", []string{"budget.per_run", "budget.per_day"}},
		{"a paid-API role and no spend limit", paidRoles + fullCaps, []string{
			"budget.per_run.usd (role reviewer-spec runs on opencode, which bills a paid API)",
			"budget.per_day.usd (role reviewer-spec runs on opencode, which bills a paid API)"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseFactory("factory.yaml", []byte(c.yaml))
			var ce *CapsError
			if !errors.As(err, &ce) {
				t.Fatalf("ParseFactory() error = %v, want a *CapsError", err)
			}
			if !slices.Equal(ce.Missing, c.missing) {
				t.Errorf("Missing = %q, want %q", ce.Missing, c.missing)
			}
			msg := err.Error()
			if !strings.Contains(msg, "per_day: { tokens: <tokens>, turns: <turns>") {
				t.Errorf("the message doesn't show how to set the caps:\n%s", msg)
			}
			if regexp.MustCompile(`(tokens|turns|usd): [0-9]`).MatchString(msg) {
				t.Errorf("the message suggests a value; the factory ships no defaults:\n%s", msg)
			}
		})
	}
	t.Run("the schema itself requires the caps", func(t *testing.T) {
		doc, err := decodeYAML("factory.yaml", []byte(scriptedRoles))
		if err != nil {
			t.Fatal(err)
		}
		if err := validate("factory.yaml", "factory.schema.json", doc); err == nil {
			t.Error("the schema accepts a file without budget")
		}
	})
	t.Run("complete caps parse", func(t *testing.T) {
		f, err := ParseFactory("factory.yaml", []byte(scriptedRoles+fullCaps))
		if err != nil {
			t.Fatalf("ParseFactory() = %v", err)
		}
		if f.Budget.PerDay.Turns != 20 || f.Mode != gungnir.ModeKeyPoints || f.Privacy != "standard" {
			t.Errorf("got %+v", f)
		}
		if _, err := ParseFactory("factory.yaml", []byte(paidRoles+
			"budget:\n  per_run: { tokens: 1000, turns: 10, usd: 5 }\n  per_day: { tokens: 2000, turns: 20, usd: 10 }\n")); err != nil {
			t.Errorf("paid roles with spend limits: %v", err)
		}
	})
}
