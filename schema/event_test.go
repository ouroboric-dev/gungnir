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
		"an unknown event": strings.Replace(mustJSON(t, event(gungnir.EventRunStarted)), `"run_started"`, `"run_paused"`, 1),
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
