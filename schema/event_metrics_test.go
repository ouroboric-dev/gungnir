package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestTheLedgerRecordsWhatMetricsReads(t *testing.T) {
	bench, baseline, bar := 0.85, 0.6, 0.8
	cases := []struct {
		name  gungnir.EventName
		fill  func(*gungnir.Event)
		wants []string
	}{
		{gungnir.EventDisputeIgnored, func(e *gungnir.Event) {
			e.Node, e.Attempt, e.FindingID, e.FindingSource, e.Note = "implement", 2, "no-owner-check", "spec-compliance", "the check it names is not run by this loop"
		}, []string{`"event":"dispute_ignored"`, `"ygg.finding.id":"no-owner-check"`, `"ygg.finding.source":"spec-compliance"`}},
		{gungnir.EventFindingRaised, func(e *gungnir.Event) {
			e.FindingID, e.FindingSource, e.FindingSeverity, e.FindingOrigin = "f1", "quality", "major", gungnir.OriginReview
		}, []string{`"event":"finding_raised"`, `"ygg.finding.origin":"review"`}},
		{gungnir.EventFindingClosed, func(e *gungnir.Event) {
			e.FindingID, e.FindingSource, e.FindingSeverity, e.FindingOrigin, e.FindingAttempts = "f2", "unit", "blocker", gungnir.OriginCheck, 2
		}, []string{`"event":"finding_closed"`, `"ygg.finding.origin":"check"`}},
		{gungnir.EventRoutingIndexBuilt, func(e *gungnir.Event) {
			e.IndexKind, e.Edges, e.Extracted, e.RoutingBench, e.RoutingBaseline, e.RoutingThreshold = "impact", 9, 9, &bench, &baseline, &bar
		}, []string{`"ygg.routing.bench":0.85`, `"ygg.routing.baseline":0.6`, `"ygg.routing.threshold":0.8`}},
		{gungnir.EventRunStarted, func(e *gungnir.Event) {
			e.FactoryVersion = "odin b8c9716fa818 gungnir 32cc95d9d0ff gleipnir 6b6f83eec777"
		}, []string{`"ygg.factory.version":"odin b8c9716fa818 gungnir 32cc95d9d0ff gleipnir 6b6f83eec777"`}},
	}
	for _, c := range cases {
		e := event(c.name)
		c.fill(&e)
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range c.wants {
			if !strings.Contains(string(line), want) {
				t.Errorf("%s: line lacks %s:\n%s", c.name, want, line)
			}
		}
		if err := ValidateEvent(line); err != nil {
			t.Errorf("%s: ValidateEvent() = %v", c.name, err)
		}
	}
}

func TestAZeroBenchScoreIsStillWrittenAndABadMetricsFieldIsRefused(t *testing.T) {
	zero := 0.0
	e := event(gungnir.EventRoutingIndexBuilt)
	e.IndexKind, e.Edges, e.RoutingBench, e.RoutingBaseline = "wiring", 3, &zero, &zero
	line, _ := json.Marshal(e)
	if !strings.Contains(string(line), `"ygg.routing.bench":0`) || !strings.Contains(string(line), `"ygg.routing.baseline":0`) {
		t.Errorf("a bench score of 0 was dropped:\n%s", line)
	}
	if err := ValidateEvent(line); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
	start := func(field string) string {
		return `{"schema":"ygg.ledger/v1","seq":1,"time":"2026-10-15T09:00:00Z","ygg.run.id":"20261015-090000-a1b2c3","event":"finding_raised",` + field + `,"prev":"` + zeros + `"}`
	}
	for name, bad := range map[string]string{
		"an unknown origin":         start(`"ygg.finding.origin":"robot"`),
		"a bench score over 1":      strings.Replace(string(line), `"ygg.routing.bench":0`, `"ygg.routing.bench":1.5`, 1),
		"a negative baseline":       strings.Replace(string(line), `"ygg.routing.baseline":0`, `"ygg.routing.baseline":-0.2`, 1),
		"an empty factory version":  strings.Replace(start(`"ygg.factory.version":""`), "finding_raised", "run_started", 1),
		"a factory version of 300+": strings.Replace(start(`"ygg.factory.version":"`+strings.Repeat("x", 300)+`"`), "finding_raised", "run_started", 1),
	} {
		if err := ValidateEvent([]byte(bad)); err == nil {
			t.Errorf("%s: ValidateEvent() = nil", name)
		}
	}
}
