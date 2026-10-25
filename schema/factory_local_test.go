package schema

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const localModels = "models:\n  file: /home/ana/models.yaml\n"

func TestLocalModelOnlyOnOpencode(t *testing.T) {
	yaml := "roles:\n  planner: { agent: claude-code, model: local/x }\n" + fullCaps + sandboxBlock + localModels
	_, err := ParseFactory("factory.yaml", []byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "planner") || !strings.Contains(err.Error(), "opencode") {
		t.Fatalf("ParseFactory() error = %v, want a refusal that names role planner and opencode", err)
	}
	for _, bad := range []string{"local/", "local/X", "local/-x", "local/a b", "local/" + strings.Repeat("a", 65)} {
		yaml := "roles:\n  reviewer: { agent: opencode, model: " + bad + " }\n" + fullCaps + sandboxBlock + localModels
		if _, err := ParseFactory("factory.yaml", []byte(yaml)); err == nil || !strings.Contains(err.Error(), "reviewer") {
			t.Errorf("model %q: error = %v, want a refusal that names role reviewer", bad, err)
		}
	}
	for model, want := range map[string]string{"local/qwen3.8-27b": "qwen3.8-27b", "local/a": "a", "local/" + strings.Repeat("a", 64): strings.Repeat("a", 64)} {
		if name, ok := LocalModel(model); !ok || name != want {
			t.Errorf("LocalModel(%q) = %q, %v; want %q, true", model, name, ok, want)
		}
	}
	for _, model := range []string{"", "openrouter/x", "local", "local/", "locale/x", "local/x/y"} {
		if _, ok := LocalModel(model); ok {
			t.Errorf("LocalModel(%q) is ok, want false", model)
		}
	}
}

func TestLocalRoleRefusesReasoning(t *testing.T) {
	yaml := "roles:\n  reviewer: { agent: opencode, model: local/q, reasoning: low }\n" + fullCaps + sandboxBlock + localModels
	_, err := ParseFactory("factory.yaml", []byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "reviewer") || !strings.Contains(err.Error(), "set the effort in the model list") {
		t.Fatalf("ParseFactory() error = %v, want a refusal that names the role and says to set the effort in the model list", err)
	}
}

func TestLocalRoleNeedsAnAbsoluteModelsFile(t *testing.T) {
	roles := "roles:\n  reviewer: { agent: opencode, model: local/q }\n" + fullCaps + sandboxBlock
	for name, extra := range map[string]string{
		"missing":  "",
		"relative": "models:\n  file: models.yaml\n",
		"unclean":  "models:\n  file: /home/ana/../models.yaml\n",
	} {
		_, err := ParseFactory("factory.yaml", []byte(roles+extra))
		if err == nil || !strings.Contains(err.Error(), "models.file") || !strings.Contains(err.Error(), "reviewer") {
			t.Errorf("%s: error = %v, want a refusal that names models.file and role reviewer", name, err)
		}
	}
	f, err := ParseFactory("factory.yaml", []byte(roles+localModels))
	if err != nil {
		t.Fatalf("ParseFactory() = %v", err)
	}
	if f.Models == nil || f.Models.File != "/home/ana/models.yaml" {
		t.Errorf("Models = %+v", f.Models)
	}
}

func TestLocalRolesNeedNoUSDCap(t *testing.T) {
	local := "roles:\n  reviewer: { agent: opencode, model: local/q }\n  checker: { agent: opencode, model: local/a }\n  other: { agent: opencode, model: local/q }\n" +
		fullCaps + sandboxBlock + localModels
	f, err := ParseFactory("factory.yaml", []byte(local))
	if err != nil {
		t.Fatalf("local roles without usd: %v", err)
	}
	if f.PaysPerToken() {
		t.Error("PaysPerToken() is true for local roles only")
	}
	if got := f.LocalModels(); !slices.Equal(got, []string{"a", "q"}) {
		t.Errorf("LocalModels() = %q, want [a q]", got)
	}
	paid := "roles:\n  reviewer: { agent: opencode, model: local/q }\n  judge: { agent: opencode, model: openrouter/x }\n" +
		fullCaps + sandboxBlock + localModels
	_, err = ParseFactory("factory.yaml", []byte(paid))
	var ce *CapsError
	if !errors.As(err, &ce) || len(ce.Missing) != 2 || !strings.Contains(ce.Missing[0], "usd") {
		t.Errorf("error = %v, want a *CapsError that asks for usd", err)
	}
}

func TestLocalModelIsNotPaid(t *testing.T) {
	for _, c := range []struct {
		agent, model string
		want         bool
	}{
		{"opencode", "local/q", false},
		{"opencode", "openrouter/x", true},
		{"opencode", "", true},
		{"claude-code", "opus", false},
		{"scripted", "none", false},
	} {
		if got := PaidModel(c.agent, c.model); got != c.want {
			t.Errorf("PaidModel(%q, %q) = %v, want %v", c.agent, c.model, got, c.want)
		}
	}
	if !PaidAPI("opencode") || PaidAPI("claude-code") || PaidAPI("scripted") {
		t.Error("PaidAPI(agent) must keep its meaning: true for opencode, false for claude-code and scripted")
	}
}

func TestModelsFileIsCheckedWheneverTheSectionIsThere(t *testing.T) {
	roles := "roles:\n  reviewer: { agent: claude-code, model: sonnet }\n" + fullCaps + sandboxBlock
	for name, extra := range map[string]string{
		"relative": "models:\n  file: models.yaml\n",
		"unclean":  "models:\n  file: /home/ana/../models.yaml\n",
		"root":     "models:\n  file: /\n",
	} {
		_, err := ParseFactory("factory.yaml", []byte(roles+extra))
		if err == nil || !strings.Contains(err.Error(), "models.file") {
			t.Errorf("%s: error = %v, want a refusal naming models.file", name, err)
		}
	}
	if _, err := ParseFactory("factory.yaml", []byte(roles+localModels)); err != nil {
		t.Errorf("a good models.file without a local role: %v", err)
	}
}
