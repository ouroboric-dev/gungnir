package gungnir

import (
	"math"
	"slices"
	"testing"
)

func TestResultValidateRefusesWhatCouldPassForSuccess(t *testing.T) {
	bad := map[string]Result{
		"no status":       {},
		"unknown status":  {Status: "passed"},
		"negative tokens": {Status: StatusOK, Usage: Usage{InTok: -1}},
		"negative turns":  {Status: StatusOK, Usage: Usage{Turns: -1}},
		"NaN cost":        {Status: StatusOK, Usage: Usage{CostUSD: math.NaN()}},
		"infinite cost":   {Status: StatusOK, Usage: Usage{CostUSD: math.Inf(1)}},
	}
	for name, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
	for _, s := range []Status{StatusOK, StatusFailed} {
		if err := (Result{Status: s}).Validate(); err != nil {
			t.Errorf("status %s: Validate() = %v, want nil", s, err)
		}
	}
}

func TestUsageAdd(t *testing.T) {
	got := Usage{InTok: 1, OutTok: 2, CacheTok: 3, Turns: 4, CostUSD: 0.5}.
		Add(Usage{InTok: 10, OutTok: 20, CacheTok: 30, Turns: 40, CostUSD: 0.25, CostEstimated: true})
	want := Usage{InTok: 11, OutTok: 22, CacheTok: 33, Turns: 44, CostUSD: 0.75, CostEstimated: true}
	if got != want {
		t.Errorf("Add() = %+v, want %+v", got, want)
	}
}

func TestHardGatesAreTheSpecList(t *testing.T) {
	hard := []Gate{"deploy-prod", "store-submit", "real-secrets-or-user-data", "budget-cap",
		"external-input", "policy-violation", "shrink-guard", "guardrail-change", "repo-admin"}
	for _, g := range Gates() {
		if g.Hard() != slices.Contains(hard, g) {
			t.Errorf("%s.Hard() = %v, want %v", g, g.Hard(), !g.Hard())
		}
		if !g.Known() {
			t.Errorf("%s.Known() = false", g)
		}
	}
	if len(Gates()) != 16 {
		t.Errorf("len(Gates()) = %d, want 16", len(Gates()))
	}
	if Gate("nope").Known() || Gate("nope").Hard() {
		t.Error(`Gate("nope") is known or hard`)
	}
}

func TestModes(t *testing.T) {
	if !ModeUnattended.Looser(ModeKeyPoints) || !ModeKeyPoints.Looser(ModeStepByStep) {
		t.Error("want unattended looser than key-points, and key-points looser than step-by-step")
	}
	if ModeStepByStep.Looser(ModeKeyPoints) || ModeKeyPoints.Looser(ModeKeyPoints) {
		t.Error("a tighter or equal mode is not looser")
	}
	if m, err := ParseMode("key-points"); err != nil || m != ModeKeyPoints {
		t.Errorf(`ParseMode("key-points") = %q, %v`, m, err)
	}
	if _, err := ParseMode("fast"); err == nil {
		t.Error(`ParseMode("fast") = nil error`)
	}
}

func TestFindingValidate(t *testing.T) {
	good := Finding{ID: "F1", Source: "reviewer-spec", Severity: SeverityMajor,
		Location: "api/auth.go:42-48", Claim: "the PIN is not throttled", Status: FindingOpen}
	if err := good.Validate(); err != nil {
		t.Fatalf("good finding: %v", err)
	}
	bad := map[string]func(*Finding){
		"no location":           func(f *Finding) { f.Location = "" },
		"location without line": func(f *Finding) { f.Location = "api/auth.go" },
		"unknown severity":      func(f *Finding) { f.Severity = "critical" },
		"fixed with no proof":   func(f *Finding) { f.Status = FindingFixed },
		"unknown status":        func(f *Finding) { f.Status = "closed" },
		"no id":                 func(f *Finding) { f.ID = "" },
	}
	for name, change := range bad {
		f := good
		change(&f)
		if err := f.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
	fixed := good
	fixed.Status, fixed.Proof = FindingFixed, "api/auth.go:44 limits tries; check run 7 rung 4 passed"
	if err := fixed.Validate(); err != nil {
		t.Errorf("fixed with proof: %v", err)
	}
}
