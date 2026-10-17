package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestParseConformanceReadsAVerdictExactly(t *testing.T) {
	ok := map[string]gungnir.Conformance{
		`{"deviation": false}`:                                                {},
		"```json\n{\"deviation\": false}\n```":                                {},
		`{"deviation": true, "amendment": "Add offline mode to the design."}`: {Deviation: true, Amendment: "Add offline mode to the design."},
	}
	for answer, want := range ok {
		got, err := ParseConformance([]byte(answer))
		if err != nil || got != want {
			t.Errorf("ParseConformance(%q) = %+v, %v; want %+v", answer, got, err, want)
		}
	}
	bad := map[string]string{
		"nothing":                            ``,
		"prose":                              `The spec conforms.`,
		"no verdict":                         `{}`,
		"a deviation with no amendment":      `{"deviation": true}`,
		"a deviation with a blank amendment": `{"deviation": true, "amendment": "  "}`,
		"an amendment with no deviation":     `{"deviation": false, "amendment": "x"}`,
		"a verdict that is not a bool":       `{"deviation": "no"}`,
		"an unknown field":                   `{"deviation": false, "ok": true}`,
		"two documents":                      `{"deviation": false} {"deviation": true, "amendment": "x"}`,
		"an amendment too long":              `{"deviation": true, "amendment": "` + strings.Repeat("x", gungnir.MaxAmendment+1) + `"}`,
	}
	for name, answer := range bad {
		if got, err := ParseConformance([]byte(answer)); err == nil {
			t.Errorf("%s: ParseConformance() = %+v, nil; want an error (a verdict nobody can read is not a pass)", name, got)
		}
	}
}

func TestTheLedgerRecordsAConformanceCheck(t *testing.T) {
	e := event(gungnir.EventConformanceChecked)
	e.Node, e.Result = "conformance", "deviation: the spec adds offline mode"
	line, _ := json.Marshal(e)
	if err := ValidateEvent(line); err != nil {
		t.Errorf("ValidateEvent() = %v", err)
	}
}
