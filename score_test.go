package gungnir

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func goodScore() Score {
	t0 := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	return Score{
		Schema: ScoreSchema, Run: "20261018-090000-a1b2c3", Task: "client-gallery-lite", TaskVersion: "4.0.0",
		BriefSHA256: strings.Repeat("ab", 32), Suite: "v1", Started: t0, Finished: t0.Add(20 * time.Minute),
		Scenarios: []ScoredScenario{
			{"photographer creates a gallery", "photographer", FailToPass, ScenarioPassed},
			{"photographer uploads a photo", "photographer", FailToPass, ScenarioFailed},
			{"client opens a link", "client", FailToPass, ScenarioPassed},
			{"login page loads", "client", PassToPass, ScenarioPassed},
			{"old link still works", "client", PassToPass, ScenarioFailed},
		},
		FailToPass: Tally{2, 3},
		PassToPass: Tally{1, 2},
		Categories: map[string]CategoryScore{"photographer": {1, 2, 0.4}, "client": {1, 1, 0.6}},
		Reward:     0.4*0.5 + 0.6,
	}
}

func TestAScoreRoundTripsThroughJSON(t *testing.T) {
	s := goodScore()
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back Score
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(back)
	if string(raw) != string(again) || back.Validate() != nil {
		t.Errorf("a score changed in a round trip:\n%s\n%s", raw, again)
	}
	for _, key := range []string{`"schema":"ygg.score/v1"`, `"task_version":"4.0.0"`, `"fail_to_pass":{"passed":2,"total":3}`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("score lacks %s:\n%s", key, raw)
		}
	}
}

func TestPassToPassNeverChangesTheReward(t *testing.T) {
	s := goodScore()
	want := RewardOf(s.Categories)
	for i := range s.Scenarios {
		if s.Scenarios[i].Kind == PassToPass {
			s.Scenarios[i].Status = ScenarioFailed
		}
	}
	s.PassToPass = Tally{0, 2}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if got := RewardOf(s.Categories); got != want {
		t.Errorf("RewardOf = %v after pass-to-pass failed, want %v", got, want)
	}
	if got := RewardOf(map[string]CategoryScore{"a": {0, 0, 0.5}, "b": {3, 3, 0.5}}); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("a category with no scenarios counts 0, so RewardOf = %v, want 0.5", got)
	}
}

func TestAScoreThatDoesNotAddUpIsRefused(t *testing.T) {
	cases := map[string]func(*Score){
		"another schema":                 func(s *Score) { s.Schema = "ygg.score/v2" },
		"no run":                         func(s *Score) { s.Run = "" },
		"a bad task version":             func(s *Score) { s.TaskVersion = "4" },
		"a short brief hash":             func(s *Score) { s.BriefSHA256 = "abc" },
		"finished before started":        func(s *Score) { s.Finished = s.Started.Add(-time.Second) },
		"a scenario used twice":          func(s *Score) { s.Scenarios = append(s.Scenarios, s.Scenarios[0]) },
		"an unknown status":              func(s *Score) { s.Scenarios[0].Status = "flaky" },
		"an unknown kind":                func(s *Score) { s.Scenarios[0].Kind = "maybe" },
		"a fail-to-pass tally that lies": func(s *Score) { s.FailToPass = Tally{3, 3} },
		"a pass-to-pass tally that lies": func(s *Score) { s.PassToPass = Tally{2, 2} },
		"a category that lies":           func(s *Score) { s.Categories["client"] = CategoryScore{0, 1, 0.6} },
		"a category with no entry":       func(s *Score) { delete(s.Categories, "client") },
		"weights that do not sum to 1":   func(s *Score) { s.Categories["client"] = CategoryScore{1, 1, 0.5} },
		"a reward that is not the sum":   func(s *Score) { s.Reward = 1 },
		"a weight over 1":                func(s *Score) { s.Categories["client"] = CategoryScore{1, 1, 1.6} },
	}
	for name, mutate := range cases {
		s := goodScore()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil", name)
		}
	}
}

func TestASuiteManifestIsRefusedWhenItIsAmbiguous(t *testing.T) {
	good := Suite{Schema: SuiteSchema, Suite: "v1", Scenarios: []Scenario{
		{"photographer creates a gallery", "photographer", FailToPass}, {"login page loads", "client", PassToPass}}}
	if err := good.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	cases := map[string]func(*Suite){
		"another schema":     func(s *Suite) { s.Schema = "ygg.suite/v2" },
		"no suite version":   func(s *Suite) { s.Suite = "" },
		"no scenarios":       func(s *Suite) { s.Scenarios = nil },
		"an id used twice":   func(s *Suite) { s.Scenarios[1].ID = s.Scenarios[0].ID },
		"an empty id":        func(s *Suite) { s.Scenarios[0].ID = "" },
		"a category in caps": func(s *Suite) { s.Scenarios[0].Category = "Photographer" },
		"an unknown kind":    func(s *Suite) { s.Scenarios[0].Kind = "flaky" },
	}
	for name, mutate := range cases {
		s := good
		s.Scenarios = append([]Scenario(nil), good.Scenarios...)
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil", name)
		}
	}
}
