package schema

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ouroboric-dev/gungnir"
)

const zeros = "0000000000000000000000000000000000000000000000000000000000000000"

func event(name gungnir.EventName) gungnir.Event {
	return gungnir.Event{Schema: gungnir.LedgerSchema, Seq: 1, Time: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
		Run: "20261015-090000-a1b2c3", Name: name, Prev: zeros}
}

func TestEventsUseOTelNames(t *testing.T) {
	e := event(gungnir.EventNodeFinished).WithUsage(gungnir.Usage{InTok: 1200, OutTok: 300, CacheTok: 900, Turns: 4, CostUSD: 0.02})
	e.Node, e.Attempt, e.Role, e.Model, e.Adapter = "design", 1, "planner", "none", "scripted"
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"gen_ai.usage.input_tokens":1200`, `"gen_ai.usage.output_tokens":300`,
		`"gen_ai.usage.cache_read.input_tokens":900`, `"gen_ai.request.model":"none"`, `"gen_ai.agent.name":"planner"`} {
		if !strings.Contains(string(line), key) {
			t.Errorf("line lacks %s:\n%s", key, line)
		}
	}
	if err := ValidateEvent(line); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
}

func TestLedgerEventSchema(t *testing.T) {
	for _, n := range gungnir.Events() {
		line, _ := json.Marshal(event(n))
		if err := ValidateEvent(line); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	bad := map[string]string{
		"an unknown event": strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), `"run_started"`, `"run_exploded"`, 1),
		"no prev":          strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), `,"prev":"`+zeros+`"`, "", 1),
		"a short prev":     strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), zeros, "abc", 1),
		"an unknown field": strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), `{`, `{"tokens":5,`, 1),
		"a path as run id": strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), "20261015-090000-a1b2c3", "../x", 1),
		"not JSON":         `{"schema":`,
	}
	for name, line := range bad {
		if err := ValidateEvent([]byte(line)); err == nil {
			t.Errorf("%s: ValidateEvent() = nil", name)
		}
	}
}

func TestParseFindingRefusesMalformedFindings(t *testing.T) {
	good := `{"id":"F1","source":"reviewer-spec","severity":"major","location":"api/auth.go:42","claim":"no throttle","status":"open"}`
	if _, err := ParseFinding([]byte(good)); err != nil {
		t.Fatalf("good finding: %v", err)
	}
	for name, s := range map[string]string{
		"no location":         `{"id":"F1","source":"r","severity":"major","claim":"c","status":"open"}`,
		"fixed without proof": `{"id":"F1","source":"r","severity":"major","location":"a.go:1","claim":"c","status":"fixed"}`,
		"free text":           `the code looks fine`,
		"an empty object":     `{}`,
	} {
		if _, err := ParseFinding([]byte(s)); err == nil {
			t.Errorf("%s: ParseFinding() = nil error", name)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRepoCreatedIsAnEvent(t *testing.T) {
	if !slices.Contains(gungnir.Events(), gungnir.EventRepoCreated) || gungnir.EventRepoCreated != "repo_created" {
		t.Error("repo_created isn't a ledger event")
	}
}

func TestSandboxEvents(t *testing.T) {
	for name, e := range map[string]gungnir.EventName{"sandbox_up": gungnir.EventSandboxUp, "sandbox_down": gungnir.EventSandboxDown} {
		if string(e) != name || !slices.Contains(gungnir.Events(), e) {
			t.Errorf("%s isn't a ledger event", name)
		}
		line, _ := json.Marshal(event(e))
		if err := ValidateEvent(line); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRetryPauseAndFindingEvents(t *testing.T) {
	for name, n := range map[string]gungnir.EventName{"retry": gungnir.EventRetry, "run_paused": gungnir.EventRunPaused,
		"finding_raised": gungnir.EventFindingRaised, "finding_closed": gungnir.EventFindingClosed, "finding_reopened": gungnir.EventFindingReopened} {
		if string(n) != name || !slices.Contains(gungnir.Events(), n) {
			t.Errorf("%s isn't a ledger event", name)
		}
	}
	e := event(gungnir.EventFindingClosed)
	e.Node, e.FindingID, e.FindingSource, e.FindingSeverity, e.FindingAttempts, e.FindingReopens = "build", "check-test", "test", "blocker", 3, 1
	line := mustJSON(t, e)
	for _, key := range []string{`"ygg.finding.id":"check-test"`, `"ygg.finding.source":"test"`, `"ygg.finding.severity":"blocker"`,
		`"ygg.finding.attempts":3`, `"ygg.finding.reopens":1`} {
		if !strings.Contains(line, key) {
			t.Errorf("line lacks %s:\n%s", key, line)
		}
	}
	if err := ValidateEvent([]byte(line)); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
	bad := strings.Replace(line, `"blocker"`, `"critical"`, 1)
	if err := ValidateEvent([]byte(bad)); err == nil {
		t.Error("a finding with severity critical: ValidateEvent() = nil")
	}
}

func TestParseReport(t *testing.T) {
	open := `{"id":"spec-1","source":"spec","severity":"major","location":"api/auth.go:42","claim":"no throttle","status":"open"}`
	fixed := `{"id":"spec-2","source":"spec","severity":"minor","location":"api/auth.go:50","claim":"no doc","status":"fixed","proof":"api/auth.go:51 adds it; check test passed"}`
	for name, answer := range map[string]string{
		"no findings":     `{"findings":[]}`,
		"an open finding": `{"findings":[` + open + `]}`,
		"open and fixed":  "\n  {\"findings\":[" + open + "," + fixed + "]}\n",
		"in a fence":      "```json\n{\"findings\":[" + open + "]}\n```\n",
		"in a bare fence": "```\n{\"findings\":[]}\n```",
	} {
		r, err := ParseReport([]byte(answer))
		if err != nil {
			t.Errorf("%s: ParseReport() = %v", name, err)
		} else if name == "open and fixed" && (len(r.Findings) != 2 || r.Findings[1].Proof == "") {
			t.Errorf("%s: findings = %+v", name, r.Findings)
		}
	}
	for name, answer := range map[string]string{
		"prose":                    "The code looks fine to me.",
		"prose before the report":  "Here you go:\n{\"findings\":[]}",
		"prose after a fence":      "```json\n{\"findings\":[]}\n```\nDone.",
		"two fences":               "```json\n{\"findings\":[]}\n```\n```json\n{\"findings\":[]}\n```",
		"an unclosed fence":        "```json\n{\"findings\":[]}",
		"an empty answer":          "",
		"no findings key":          `{}`,
		"findings null":            `{"findings":null}`,
		"an extra key":             `{"findings":[],"verdict":"pass"}`,
		"a finding with no place":  `{"findings":[{"id":"a","source":"s","severity":"major","claim":"c","status":"open"}]}`,
		"fixed with no proof":      `{"findings":[{"id":"a","source":"s","severity":"major","location":"a.go:1","claim":"c","status":"fixed"}]}`,
		"an unknown status":        `{"findings":[{"id":"a","source":"s","severity":"major","location":"a.go:1","claim":"c","status":"closed"}]}`,
		"an id used twice":         `{"findings":[` + open + `,` + open + `]}`,
		"a finding with extra key": `{"findings":[{"id":"a","source":"s","severity":"major","location":"a.go:1","claim":"c","status":"open","fix":"x"}]}`,
	} {
		if r, err := ParseReport([]byte(answer)); err == nil {
			t.Errorf("%s: ParseReport() = %+v, nil; want an error: unreadable output is never a pass", name, r)
		}
	}
}
