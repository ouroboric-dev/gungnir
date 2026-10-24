package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

// validateOnly stands in for a decoder when a contract has no Go type yet (the run record's comes with the code
// that builds it).
func validateOnly(t *testing.T, c Contract, data []byte) error {
	t.Helper()
	return Validate(c, data)
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTheOperatorsFilesReadTheirFixtures(t *testing.T) {
	runFixtures(t, map[string]fixtureKind{
		"decisions":  {ContractDecisions, roundTrip[Decisions]},
		"judgement":  {ContractJudgement, roundTrip[Judgement]},
		"retro":      {ContractRetro, roundTrip[Retro]},
		"run-record": {ContractRecord, validateOnly},
	})
}

func TestTheProofRunsDecisionsAreFiveWithTheirIDs(t *testing.T) {
	data := readFixture(t, "decisions-ok-proof-run.json")
	d, err := Decode[Decisions](ContractDecisions, data)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, x := range d.Decisions {
		ids = append(ids, x.ID)
	}
	if got, want := strings.Join(ids, ","), "password-reset,deletion,email-notice,storage-cap,logout-on-change"; got != want {
		t.Errorf("ids = %s, want %s", got, want)
	}
	if !strings.Contains(d.Decisions[1].Text, "\n") {
		t.Error("a decision's text lost its line break")
	}
}

func TestADecisionsFileSaysSomething(t *testing.T) {
	for name, doc := range map[string]string{
		"nothing at all":       `{}`,
		"an empty list":        `{"decisions":[]}`,
		"an empty list, a gap": `{"decisions":[],"answers":[]}`,
	} {
		if _, err := Decode[Decisions](ContractDecisions, []byte(doc)); err == nil {
			t.Errorf("%s is read", name)
		}
	}
	for name, doc := range map[string]string{
		"a reason alone":  `{"reason":"too wide"}`,
		"answers alone":   `{"answers":[{"question":"q1","text":"yes"}]}`,
		"decisions alone": `{"decisions":[{"id":"a","text":"b"}]}`,
	} {
		if _, err := Decode[Decisions](ContractDecisions, []byte(doc)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestADecisionTextTakesFourKiBAndNoMore(t *testing.T) {
	doc := func(n int) []byte {
		return []byte(`{"decisions":[{"id":"a","text":"` + strings.Repeat("é", n) + `"}]}`)
	}
	if _, err := Decode[Decisions](ContractDecisions, doc(4096)); err != nil {
		t.Errorf("4096 characters: %v", err)
	}
	if _, err := Decode[Decisions](ContractDecisions, doc(4097)); err == nil {
		t.Error("4097 characters are read")
	}
}

func TestAJudgementScoreRunsFromZeroToTen(t *testing.T) {
	for score, ok := range map[string]bool{"0": true, "10": true, "-1": false, "11": false} {
		doc := `{"run":"r","score":` + score + `,"well":"w","wrong":"x","time":"2026-10-24T18:30:00Z"}`
		if _, err := Decode[Judgement](ContractJudgement, []byte(doc)); (err == nil) != ok {
			t.Errorf("score %s: err = %v, want ok = %v", score, err, ok)
		}
	}
}

func TestARetroItemWithCategoryOtherTakesTwoEvidenceEntries(t *testing.T) {
	r, err := Decode[Retro](ContractRetro, readFixture(t, "retro-ok-full.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.WentWrong[0]; got.Category != "other" || len(got.Evidence) != 2 {
		t.Errorf("went_wrong[0] = %+v", got)
	}
}

func TestARetroNeedsItsListsUnlessBlocked(t *testing.T) {
	if _, err := Decode[Retro](ContractRetro, []byte(`{"blocked":{"reason":"r"},"went_well":[{"category":"other","severity":"low","text":"t","block":"b","evidence":["task:t1"]}]}`)); err != nil {
		t.Errorf("blocked with content: %v", err)
	}
	if _, err := Decode[Retro](ContractRetro, []byte(`{"went_well":[],"went_wrong":[]}`)); err == nil {
		t.Error("an unblocked retro without suggestions is read")
	}
}

func TestARetroItemTakesEvidenceAtTheLimitsAndNotOver(t *testing.T) {
	doc := func(evidence []string, text string, block string) []byte {
		ev, _ := json.Marshal(evidence)
		return []byte(`{"went_well":[{"category":"other","severity":"low","text":"` + text + `","block":"` + block +
			`","evidence":` + string(ev) + `}],"went_wrong":[],"suggestions":[]}`)
	}
	r := strings.Repeat
	many := func(n int) []string {
		var l []string
		for range n {
			l = append(l, "task:t1")
		}
		return l
	}
	if _, err := Decode[Retro](ContractRetro, doc(many(50), r("t", 16384), r("b", 200))); err != nil {
		t.Errorf("an item at every limit: %v", err)
	}
	if _, err := Decode[Retro](ContractRetro, doc([]string{"task:" + r("x", 399)}, "t", "b")); err != nil {
		t.Errorf("evidence of 404 characters: %v", err)
	}
	for name, data := range map[string][]byte{
		"51 evidence entries":   doc(many(51), "t", "b"),
		"evidence too long":     doc([]string{"task:" + r("x", 400)}, "t", "b"),
		"evidence with a space": doc([]string{"task: t1"}, "t", "b"),
		"unknown evidence kind": doc([]string{"hunch:t1"}, "t", "b"),
		"a text too long":       doc(many(1), r("t", 16385), "b"),
		"a block too long":      doc(many(1), "t", r("b", 201)),
	} {
		if _, err := Decode[Retro](ContractRetro, data); err == nil {
			t.Errorf("%s is read", name)
		}
	}
}

func TestTheRecordTakesTheMetricsAsTheyAre(t *testing.T) {
	// The fixture is the JSON of odin's metrics.Run, field for field; the record adds only optional fields.
	for _, name := range []string{"run-record-ok-metrics.json", "run-record-ok-empty-run.json"} {
		if err := Validate(ContractRecord, readFixture(t, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestTheLedgerTakesTheNewEvents(t *testing.T) {
	events := map[gungnir.EventName]string{
		gungnir.EventOutputRefused:  "output_refused",
		gungnir.EventOutputRepaired: "output_repaired",
		gungnir.EventJudged:         "judged",
		gungnir.EventRetroWritten:   "retro_written",
	}
	listed := map[gungnir.EventName]bool{}
	for _, n := range gungnir.Events() {
		listed[n] = true
	}
	for n, want := range events {
		if string(n) != want {
			t.Errorf("%q, want %q", n, want)
		}
		if !listed[n] {
			t.Errorf("%s is not in Events()", n)
		}
		e := event(n)
		e.Node, e.Attempt, e.Note = "design", 2, "ygg.design/v1: /summary is required"
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateEvent(line); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
}
