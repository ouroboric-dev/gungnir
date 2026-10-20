package gungnir

import (
	"slices"
	"strings"
	"testing"
)

func TestAnOverruledFindingNeedsProofLikeAFixedOne(t *testing.T) {
	f := Finding{ID: "F1", Source: "spec", Severity: SeverityBlocker, Location: "api/auth.go:42", Claim: "findIndex is not a method", Status: FindingOverruled}
	if err := f.Validate(); err == nil {
		t.Error("an overruled finding with no proof: Validate() = nil, want an error")
	}
	f.Proof = "check test passed in attempt 2 and printed TestFindIndex; the implementer showed it with curl"
	if err := f.Validate(); err != nil {
		t.Errorf("an overruled finding with proof: Validate() = %v", err)
	}
	if FindingOverruled != "overruled" || EventFindingOverruled != "finding_overruled" {
		t.Errorf("names: %q, %q", FindingOverruled, EventFindingOverruled)
	}
	if !slices.Contains(Events(), EventFindingOverruled) {
		t.Error("finding_overruled is not in Events()")
	}
}

func TestADisputeNeedsAllFourFieldsWithinTheLimits(t *testing.T) {
	good := Dispute{ID: "F1", Check: "test", Test: "TestFindIndex", Evidence: "curl shows it"}
	if err := good.Validate(); err != nil {
		t.Fatalf("a good dispute: %v", err)
	}
	for name, change := range map[string]func(*Dispute){
		"no id":             func(d *Dispute) { d.ID = "" },
		"no check":          func(d *Dispute) { d.Check = "" },
		"no test":           func(d *Dispute) { d.Test = "" },
		"no evidence":       func(d *Dispute) { d.Evidence = "" },
		"an id too long":    func(d *Dispute) { d.ID = strings.Repeat("i", MaxID+1) },
		"a check too long":  func(d *Dispute) { d.Check = strings.Repeat("c", MaxDisputeCheck+1) },
		"a test too long":   func(d *Dispute) { d.Test = strings.Repeat("t", MaxDisputeTest+1) },
		"evidence too long": func(d *Dispute) { d.Evidence = strings.Repeat("e", MaxEvidence+1) },
	} {
		d := good
		change(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}
