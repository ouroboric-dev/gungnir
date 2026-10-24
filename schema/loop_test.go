package schema

import (
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestTheLoopContractsReadTheirFixtures(t *testing.T) {
	runFixtures(t, map[string]fixtureKind{
		"review":    {ContractReview, roundTrip[Review]},
		"implement": {ContractImplement, roundTrip[Implement]},
		"check":     {ContractCheck, roundTrip[CheckResult]},
	})
}

func TestAReviewTakesAClaimAtTheLimitAndNotOver(t *testing.T) {
	doc := func(n int) []byte {
		return []byte(`{"findings":[{"id":"a","source":"s","severity":"major","location":"a.go:1","claim":"` +
			strings.Repeat("c", n) + `","status":"open"}]}`)
	}
	if _, err := Decode[Review](ContractReview, doc(gungnir.MaxClaim)); err != nil {
		t.Errorf("a claim at the limit: %v", err)
	}
	if _, err := Decode[Review](ContractReview, doc(gungnir.MaxClaim+1)); err == nil {
		t.Error("a claim over the limit is read")
	}
}

func TestAnImplementTakesEveryDisputeFieldAtTheLimitAndNotOver(t *testing.T) {
	doc := func(id, check, test, evidence string) []byte {
		return []byte(`{"summary":"s","disputes":[{"id":"` + id + `","check":"` + check + `","test":"` + test +
			`","evidence":"` + evidence + `"}]}`)
	}
	r := strings.Repeat
	if _, err := Decode[Implement](ContractImplement, doc(r("i", gungnir.MaxID), r("c", gungnir.MaxDisputeCheck), r("t", gungnir.MaxDisputeTest), r("é", gungnir.MaxEvidence))); err != nil {
		t.Errorf("a dispute at every limit: %v", err)
	}
	for name, data := range map[string][]byte{
		"an id too long":    doc(r("i", gungnir.MaxID+1), "c", "t", "e"),
		"a check too long":  doc("a", r("c", gungnir.MaxDisputeCheck+1), "t", "e"),
		"a test too long":   doc("a", "c", r("t", gungnir.MaxDisputeTest+1), "e"),
		"evidence too long": doc("a", "c", "t", r("e", gungnir.MaxEvidence+1)),
	} {
		if _, err := Decode[Implement](ContractImplement, data); err == nil {
			t.Errorf("%s: the dispute is read", name)
		}
	}
	var many []string
	for i := 0; i <= gungnir.MaxDisputes; i++ {
		many = append(many, `{"id":"d`+r("x", i)+`","check":"c","test":"t","evidence":"e"}`)
	}
	if _, err := Decode[Implement](ContractImplement, []byte(`{"summary":"s","disputes":[`+strings.Join(many, ",")+`]}`)); err == nil {
		t.Error("more disputes than the limit are read")
	}
}

func TestACheckResultTakesFiveHundredFailuresAndNoMore(t *testing.T) {
	doc := func(n int) []byte {
		f := strings.Repeat(`{"kind":"test","message":"m"},`, n)
		return []byte(`{"check":"test","passed":false,"exit_code":1,"timed_out":false,"duration_ms":1,"failures":[` + strings.TrimSuffix(f, ",") + `]}`)
	}
	if _, err := Decode[CheckResult](ContractCheck, doc(500)); err != nil {
		t.Errorf("500 failures: %v", err)
	}
	if _, err := Decode[CheckResult](ContractCheck, doc(501)); err == nil {
		t.Error("501 failures are read")
	}
}
