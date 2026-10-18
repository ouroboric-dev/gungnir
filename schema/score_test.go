package schema

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ouroboric-dev/gungnir"
)

const goodManifest = `{"schema":"ygg.suite/v1","suite":"v1","scenarios":[
 {"id":"photographer creates a gallery","category":"photographer","kind":"fail-to-pass"},
 {"id":"login page loads","category":"client","kind":"pass-to-pass"}]}`

func TestParseSuiteReadsAManifestExactly(t *testing.T) {
	s, err := ParseSuite([]byte(goodManifest))
	if err != nil || s.Suite != "v1" || len(s.Scenarios) != 2 || s.Scenarios[1].Kind != gungnir.PassToPass {
		t.Fatalf("ParseSuite() = %+v, %v", s, err)
	}
	bad := map[string]string{
		"nothing":          ``,
		"prose":            `the suite`,
		"another schema":   strings.Replace(goodManifest, "ygg.suite/v1", "ygg.suite/v2", 1),
		"an unknown kind":  strings.Replace(goodManifest, `"pass-to-pass"`, `"flaky"`, 1),
		"an unknown field": strings.Replace(goodManifest, `"suite":"v1",`, `"suite":"v1","extra":1,`, 1),
		"an id used twice": strings.Replace(goodManifest, "login page loads", "photographer creates a gallery", 1),
		"no scenarios":     `{"schema":"ygg.suite/v1","suite":"v1","scenarios":[]}`,
		"two documents":    goodManifest + goodManifest,
	}
	for name, doc := range bad {
		if _, err := ParseSuite([]byte(doc)); err == nil {
			t.Errorf("%s: ParseSuite() = nil", name)
		}
	}
}

func TestParseScoreReadsWhatMarshalWrote(t *testing.T) {
	t0 := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	want := gungnir.Score{
		Schema: gungnir.ScoreSchema, Run: "20261018-090000-a1b2c3", Task: "client-gallery-lite", TaskVersion: "4.0.0",
		BriefSHA256: strings.Repeat("ab", 32), Suite: "v1", Started: t0, Finished: t0.Add(time.Minute),
		Scenarios:  []gungnir.ScoredScenario{{ID: "a", Category: "photographer", Kind: gungnir.FailToPass, Status: gungnir.ScenarioPassed}},
		FailToPass: gungnir.Tally{Passed: 1, Total: 1},
		Categories: map[string]gungnir.CategoryScore{"photographer": {Passed: 1, Total: 1, Weight: 1}},
		Reward:     1,
	}
	raw, _ := json.Marshal(want)
	got, err := ParseScore(raw)
	if err != nil {
		t.Fatalf("ParseScore() = %v\n%s", err, raw)
	}
	if again, _ := json.Marshal(got); string(again) != string(raw) {
		t.Errorf("round trip changed the score:\n%s\n%s", raw, again)
	}
	bad := map[string]string{
		"an unknown field":   strings.Replace(string(raw), `"reward":1`, `"reward":1,"bonus":2`, 1),
		"a reward that lies": strings.Replace(string(raw), `"reward":1`, `"reward":0.5`, 1),
		"a reward over 1":    strings.Replace(string(raw), `"reward":1`, `"reward":2`, 1),
		"no categories":      strings.Replace(string(raw), `"categories":{"photographer":{"passed":1,"total":1,"weight":1}}`, `"categories":{}`, 1),
		"a missing field":    strings.Replace(string(raw), `"suite":"v1",`, ``, 1),
	}
	for name, doc := range bad {
		if _, err := ParseScore([]byte(doc)); err == nil {
			t.Errorf("%s: ParseScore() = nil", name)
		}
	}
}
