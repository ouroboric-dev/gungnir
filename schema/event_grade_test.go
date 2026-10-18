package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestTheLedgerRecordsGradingRoutingAndDependencyChecks(t *testing.T) {
	reward := 0.75
	sha := strings.Repeat("ab", 32)
	cases := []struct {
		name  gungnir.EventName
		fill  func(*gungnir.Event)
		wants []string
	}{
		{gungnir.EventGradeStarted, func(e *gungnir.Event) {
			e.Task, e.Suite, e.BriefSHA256, e.Image = "client-gallery-lite", "v1", sha, "localhost/ygg-grader-client-gallery-lite:0123456789ab"
		}, []string{`"event":"grade_started"`, `"ygg.task.id":"client-gallery-lite"`, `"ygg.grade.suite":"v1"`, `"ygg.grade.brief_sha256":"` + sha + `"`, `"ygg.grade.image":"localhost/ygg-grader-client-gallery-lite:0123456789ab"`}},
		{gungnir.EventGradeFinished, func(e *gungnir.Event) {
			e.Task, e.Suite, e.ScorePath, e.Reward, e.FailToPass, e.PassToPass = "client-gallery-lite", "v1", "/runs/r1/score.json", &reward, "3/4", "5/5"
		}, []string{`"event":"grade_finished"`, `"ygg.grade.score_path":"/runs/r1/score.json"`, `"ygg.grade.reward":0.75`, `"ygg.grade.fail_to_pass":"3/4"`, `"ygg.grade.pass_to_pass":"5/5"`}},
		{gungnir.EventGradeRefused, func(e *gungnir.Event) {
			e.Task, e.Suite, e.Reason = "client-gallery-lite", "v1", gungnir.GradeRetired
		}, []string{`"event":"grade_refused"`, `"ygg.grade.reason":"retired"`}},
		{gungnir.EventRoutingIndexBuilt, func(e *gungnir.Event) {
			e.IndexKind, e.Edges, e.Extracted, e.Inferred = "wiring", 12, 10, 2
		}, []string{`"event":"routing_index_built"`, `"ygg.routing.kind":"wiring"`, `"ygg.routing.edges":12`, `"ygg.routing.extracted":10`, `"ygg.routing.inferred":2`}},
		{gungnir.EventPointerContractChecked, func(e *gungnir.Event) {
			e.Move, e.Evidence, e.Verdict = "repos/api", "oasdiff", "pass"
		}, []string{`"event":"pointer_contract_checked"`, `"ygg.pointer.move":"repos/api"`, `"ygg.pointer.evidence":"oasdiff"`, `"ygg.verdict":"pass"`}},
		{gungnir.EventDepsChecked, func(e *gungnir.Event) {
			e.Realm, e.Verdict, e.Findings = "helheim-go", "fail", 2
		}, []string{`"event":"deps_checked"`, `"ygg.realm.name":"helheim-go"`, `"ygg.verdict":"fail"`, `"ygg.deps.findings":2`}},
		{gungnir.EventDepsBump, func(e *gungnir.Event) {
			e.Package, e.From, e.To, e.Stage, e.Verdict = "vite", "7.1.2", "7.1.9", "canary", "fail"
		}, []string{`"event":"deps_bump"`, `"ygg.deps.package":"vite"`, `"ygg.deps.from":"7.1.2"`, `"ygg.deps.to":"7.1.9"`, `"ygg.deps.stage":"canary"`, `"ygg.verdict":"fail"`}},
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

func TestAZeroRewardIsStillWrittenAndABadGradeFieldIsRefused(t *testing.T) {
	zero := 0.0
	e := event(gungnir.EventGradeFinished)
	e.Task, e.Reward, e.FailToPass = "client-gallery-lite", &zero, "0/4"
	line, _ := json.Marshal(e)
	if !strings.Contains(string(line), `"ygg.grade.reward":0`) {
		t.Errorf("a reward of 0 was dropped:\n%s", line)
	}
	if err := ValidateEvent(line); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
	for name, bad := range map[string]string{
		"a reward over 1":          strings.Replace(string(line), `"ygg.grade.reward":0`, `"ygg.grade.reward":1.5`, 1),
		"a negative reward":        strings.Replace(string(line), `"ygg.grade.reward":0`, `"ygg.grade.reward":-0.1`, 1),
		"a tally that is no tally": strings.Replace(string(line), `"0/4"`, `"all"`, 1),
		"an unknown reason":        strings.Replace(strings.Replace(string(line), "grade_finished", "grade_refused", 1), `"ygg.grade.fail_to_pass":"0/4"`, `"ygg.grade.reason":"tired"`, 1),
		"an unknown evidence":      `{"schema":"ygg.ledger/v1","seq":1,"time":"2026-10-15T09:00:00Z","ygg.run.id":"20261015-090000-a1b2c3","event":"pointer_contract_checked","ygg.pointer.evidence":"guess","prev":"` + zeros + `"}`,
		"an unknown stage":         `{"schema":"ygg.ledger/v1","seq":1,"time":"2026-10-15T09:00:00Z","ygg.run.id":"20261015-090000-a1b2c3","event":"deps_bump","ygg.deps.stage":"yolo","prev":"` + zeros + `"}`,
		"a verdict out of the set": `{"schema":"ygg.ledger/v1","seq":1,"time":"2026-10-15T09:00:00Z","ygg.run.id":"20261015-090000-a1b2c3","event":"deps_checked","ygg.verdict":"escalate","prev":"` + zeros + `"}`,
		"a short brief hash":       `{"schema":"ygg.ledger/v1","seq":1,"time":"2026-10-15T09:00:00Z","ygg.run.id":"20261015-090000-a1b2c3","event":"grade_started","ygg.grade.brief_sha256":"abc","prev":"` + zeros + `"}`,
	} {
		if err := ValidateEvent([]byte(bad)); err == nil {
			t.Errorf("%s: ValidateEvent() = nil", name)
		}
	}
}
