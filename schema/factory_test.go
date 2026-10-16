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
			"budget:\n  per_run: { tokens: 1000, turns: 10, usd: 5 }\n  per_day: { tokens: 2000, turns: 20, usd: 10 }\n"+sandboxBlock)); err != nil {
			t.Errorf("paid roles with spend limits: %v", err)
		}
	})
}

func TestFactoryRefusesWhatItCannotReadExactly(t *testing.T) {
	cases := map[string]string{
		"two YAML documents":   scriptedRoles + fullCaps + "---\nmode: unattended\n",
		"a duplicate key":      scriptedRoles + fullCaps + "budget:\n  per_run: { tokens: 1, turns: 1 }\n",
		"a misspelled key":     scriptedRoles + fullCaps + "privcy: strict\n",
		"zero tokens":          scriptedRoles + "budget:\n  per_run: { tokens: 0, turns: 10 }\n  per_day: { tokens: 2000, turns: 20 }\n",
		"tokens as a word":     scriptedRoles + "budget:\n  per_run: { tokens: many, turns: 10 }\n  per_day: { tokens: 2000, turns: 20 }\n",
		"tokens as a fraction": scriptedRoles + "budget:\n  per_run: { tokens: 1.5, turns: 10 }\n  per_day: { tokens: 2000, turns: 20 }\n",
		"an unknown mode":      "mode: fast\n" + scriptedRoles + fullCaps,
		"an unknown agent":     "roles:\n  planner: { agent: codex, model: x }\n" + "budget:\n  per_run: { tokens: 1, turns: 1, usd: 1 }\n  per_day: { tokens: 1, turns: 1, usd: 1 }\n",
		"scripted, no script":  "roles:\n  planner: { agent: scripted, model: none }\n" + fullCaps,
		"a script on claude":   "roles:\n  planner: { agent: claude-code, model: opus, script: x.yaml }\n" + fullCaps,
		"a non-string key":     scriptedRoles + fullCaps + "1: x\n",
	}
	for name, y := range cases {
		if _, err := ParseFactory("factory.yaml", []byte(y)); err == nil {
			t.Errorf("%s: ParseFactory() = nil error", name)
		}
	}
}

func TestFactoryGateOverrides(t *testing.T) {
	parse := func(mode, gates string) error {
		_, err := ParseFactory("factory.yaml", []byte("mode: "+mode+"\ngates: "+gates+"\n"+scriptedRoles+fullCaps))
		return err
	}
	for _, g := range gungnir.Gates() {
		err := parse("key-points", "{ "+string(g)+": stop }")
		if g.Hard() && (err == nil || !strings.Contains(err.Error(), "hard approval point")) {
			t.Errorf("%s: stop override error = %v, want the hard approval point refusal", g, err)
		}
		if !g.Hard() && err != nil {
			t.Errorf("%s: stop override error = %v, want nil", g, err)
		}
	}
	if err := parse("key-points", "{ plan: auto }"); err == nil {
		t.Error("plan: auto parsed; an override may only tighten")
	}
	if err := parse("key-points", "{ design: auto }"); err == nil {
		t.Error("design: auto parsed in key-points mode")
	}
	if err := parse("unattended", "{ design: auto }"); err != nil {
		t.Errorf("design: auto in unattended mode: %v", err)
	}
	f, err := ParseFactory("factory.yaml", []byte("mode: unattended\ngates: { design: auto }\n"+scriptedRoles+fullCaps))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.CheckGates(gungnir.ModeKeyPoints); err == nil {
		t.Error("CheckGates(key-points) accepts design: auto; a --mode switch must re-check it")
	}
}

// An unknown agent is reported as unknown, not as a paid API that needs a spend limit: the operator
// would set usd only to learn the agent doesn't exist.
func TestFactoryNamesAnUnknownAgentBeforeAskingForASpendLimit(t *testing.T) {
	_, err := ParseFactory("factory.yaml", []byte("roles:\n  planner: { agent: codex, model: x }\n"+fullCaps))
	var ce *CapsError
	if err == nil || errors.As(err, &ce) || !strings.Contains(err.Error(), "agent") {
		t.Errorf("ParseFactory() = %v, want the schema's refusal of the agent, not a *CapsError", err)
	}
	if !KnownAgent("scripted") || KnownAgent("codex") {
		t.Error("KnownAgent is wrong")
	}
	for _, a := range agents { // the Go list and the schema's enum must agree
		caps := "budget:\n  per_run: { tokens: 1, turns: 1, usd: 1 }\n  per_day: { tokens: 1, turns: 1, usd: 1 }\n"
		script := ""
		if a == "scripted" {
			script = ", script: s.yaml"
		}
		if _, err := ParseFactory("factory.yaml", []byte("roles:\n  r: { agent: "+a+", model: m"+script+" }\n"+caps+sandboxBlock)); err != nil {
			t.Errorf("known agent %s: %v", a, err)
		}
	}
}

func TestCapReached(t *testing.T) {
	c := Cap{Tokens: 1000, Turns: 10, USD: 2}
	cases := []struct {
		name string
		u    gungnir.Usage
		usd  bool
		want gungnir.CapDimension // "" for no cap reached
	}{
		{"room left", gungnir.Usage{InTok: 400, OutTok: 500, Turns: 9, CostUSD: 1.99}, true, ""},
		{"tokens exactly at the cap", gungnir.Usage{InTok: 600, OutTok: 400}, true, gungnir.CapTokens},
		{"cache reads don't count", gungnir.Usage{InTok: 1, CacheTok: 1 << 40}, true, ""},
		{"turns at the cap", gungnir.Usage{Turns: 10}, true, gungnir.CapTurns},
		{"dollars at the cap on a paid API", gungnir.Usage{CostUSD: 2}, true, gungnir.CapUSD},
		{"dollars of a subscription never count", gungnir.Usage{CostUSD: 500}, false, ""},
	}
	for _, tc := range cases {
		got := c.Reached(gungnir.CapRun, tc.u, tc.usd)
		switch {
		case tc.want == "" && got != nil, tc.want != "" && (got == nil || got.Dimension != tc.want):
			t.Errorf("%s: Reached() = %+v, want %q", tc.name, got, tc.want)
		}
	}
	if got := (Cap{}).Reached(gungnir.CapDay, gungnir.Usage{}, false); got == nil || got.Scope != gungnir.CapDay {
		t.Errorf("a cap of zero: Reached() = %+v, want it reached at once", got)
	}
}

func TestPaysPerToken(t *testing.T) {
	sub := &Factory{Roles: map[string]Role{"a": {Agent: "claude-code"}, "b": {Agent: "scripted"}}}
	paid := &Factory{Roles: map[string]Role{"a": {Agent: "claude-code"}, "b": {Agent: "opencode"}}}
	if sub.PaysPerToken() || !paid.PaysPerToken() {
		t.Errorf("PaysPerToken() = %v and %v, want false and true", sub.PaysPerToken(), paid.PaysPerToken())
	}
}

func TestBudgetJournal(t *testing.T) {
	caps := "budget:\n  per_run: { tokens: 1, turns: 1 }\n  per_day: { tokens: 1, turns: 1 }\n  journal: "
	f, err := ParseFactory("factory.yaml", []byte(scriptedRoles+caps+"/var/lib/ygg/spend\n"))
	if err != nil || f.Budget.Journal != "/var/lib/ygg/spend" {
		t.Fatalf("ParseFactory() = %+v, %v", f, err)
	}
	if f, err := ParseFactory("factory.yaml", []byte(scriptedRoles+strings.TrimSuffix(caps, "  journal: "))); err != nil || f.Budget.Journal != "" {
		t.Errorf("no journal: %+v, %v; want it empty", f, err)
	}
	for name, j := range map[string]string{"relative": "spend", "unclean": "/var/../spend", "empty": `""`} {
		if _, err := ParseFactory("factory.yaml", []byte(scriptedRoles+caps+j+"\n")); err == nil {
			t.Errorf("%s journal %q: ParseFactory() = nil, want an error", name, j)
		}
	}
}

func TestEscalationCanBeSetToStopButNeverToAuto(t *testing.T) {
	if _, err := ParseFactory("factory.yaml", []byte(scriptedRoles+fullCaps+"gates:\n  escalation: stop\n")); err != nil {
		t.Errorf("escalation: stop: %v", err)
	}
	_, err := ParseFactory("factory.yaml", []byte(scriptedRoles+fullCaps+"gates:\n  escalation: auto\n"))
	if err == nil || !strings.Contains(err.Error(), "gates.escalation") {
		t.Errorf("escalation: auto: ParseFactory() = %v, want an error naming gates.escalation", err)
	}
}
