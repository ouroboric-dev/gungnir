package schema

import (
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestParseDisputes(t *testing.T) {
	one := `{"id":"review-1","check":"test","test":"TestFindIndex","evidence":"curl showed Array.prototype.findIndex exists; see ci/test.log:12"}`
	two := `{"id":"review-2","check":"lint","test":"no-unused","evidence":"the lint rung passed"}`
	for name, answer := range map[string]string{
		"one dispute":                   `{"disputes":[` + one + `]}`,
		"two disputes":                  "\n {\"disputes\":[" + one + "," + two + "]}\n",
		"no disputes":                   `{"disputes":[]}`,
		"in a fence":                    "```json\n{\"disputes\":[" + one + "]}\n```\n",
		"a sentence then a fence":       "I disagree with the reviewer.\n\n```json\n{\"disputes\":[" + one + "]}\n```",
		"prose with braces then a bare": "Using {x} here:\n{\"disputes\":[" + one + "]}",
		"prose quoting an object":       "The route returns `{\"status\":\"ok\"}`.\n{\"disputes\":[" + one + "]}",
		"a fenced object in the prose":  "Body:\n```json\n{\"status\":\"ok\"}\n```\n```json\n{\"disputes\":[" + one + "]}\n```",
	} {
		d, err := ParseDisputes([]byte(answer))
		if err != nil {
			t.Errorf("%s: ParseDisputes() = %v", name, err)
		} else if name == "two disputes" && (len(d.Disputes) != 2 || d.Disputes[1].Check != "lint") {
			t.Errorf("%s: disputes = %+v", name, d.Disputes)
		}
	}
	for name, answer := range map[string]string{
		"an empty answer":             "",
		"prose only":                  "I dispute it.",
		"no disputes key":             `{}`,
		"disputes null":               `{"disputes":null}`,
		"an extra key":                `{"disputes":[],"verdict":"win"}`,
		"a missing id":                `{"disputes":[{"check":"test","test":"T","evidence":"e"}]}`,
		"a missing check":             `{"disputes":[{"id":"a","test":"T","evidence":"e"}]}`,
		"a missing test":              `{"disputes":[{"id":"a","check":"test","evidence":"e"}]}`,
		"a missing evidence":          `{"disputes":[{"id":"a","check":"test","test":"T"}]}`,
		"an empty id":                 `{"disputes":[{"id":"","check":"test","test":"T","evidence":"e"}]}`,
		"an empty check":              `{"disputes":[{"id":"a","check":"","test":"T","evidence":"e"}]}`,
		"an empty test":               `{"disputes":[{"id":"a","check":"test","test":"","evidence":"e"}]}`,
		"an empty evidence":           `{"disputes":[{"id":"a","check":"test","test":"T","evidence":""}]}`,
		"an unknown field":            `{"disputes":[{"id":"a","check":"test","test":"T","evidence":"e","note":"x"}]}`,
		"an id used twice":            `{"disputes":[` + one + `,` + one + `]}`,
		"text after":                  "{\"disputes\":[]}\nDone.",
		"two bare documents":          "{\"disputes\":[]}\n{\"disputes\":[]}",
		"two fences":                  "```json\n{\"disputes\":[]}\n```\n```json\n{\"disputes\":[]}\n```",
		"an unclosed fence":           "```json\n{\"disputes\":[]}",
		"a document in prose":         "Example: {\"disputes\":[]}\n```\n{\"disputes\":[]}\n```",
		"a report, not disputes":      `{"findings":[]}`,
		"a draft disputes then final": "Draft: {\"disputes\":[]}\n{\"disputes\":[" + one + "]}",
	} {
		if d, err := ParseDisputes([]byte(answer)); err == nil {
			t.Errorf("%s: ParseDisputes() = %+v, nil; want an error", name, d)
		} else if strings.HasPrefix(err.Error(), "report:") {
			t.Errorf("%s: error %q names a report, want disputes", name, err)
		}
	}
}

func TestParseDisputesBoundsEveryField(t *testing.T) {
	doc := func(id, check, test, evidence string) string {
		return `{"disputes":[{"id":"` + id + `","check":"` + check + `","test":"` + test + `","evidence":"` + evidence + `"}]}`
	}
	r := strings.Repeat
	if _, err := ParseDisputes([]byte(doc(r("i", gungnir.MaxID), r("c", gungnir.MaxDisputeCheck), r("t", gungnir.MaxDisputeTest), r("é", gungnir.MaxEvidence)))); err != nil {
		t.Errorf("a dispute at every limit: %v", err)
	}
	for name, answer := range map[string]string{
		"an id too long":    doc(r("i", gungnir.MaxID+1), "c", "t", "e"),
		"a check too long":  doc("a", r("c", gungnir.MaxDisputeCheck+1), "t", "e"),
		"a test too long":   doc("a", "c", r("t", gungnir.MaxDisputeTest+1), "e"),
		"evidence too long": doc("a", "c", "t", r("e", gungnir.MaxEvidence+1)),
	} {
		if _, err := ParseDisputes([]byte(answer)); err == nil {
			t.Errorf("%s: ParseDisputes() = nil, want an error", name)
		}
	}
	var many []string
	for i := 0; i <= gungnir.MaxDisputes; i++ {
		many = append(many, `{"id":"d`+r("x", i)+`","check":"c","test":"t","evidence":"e"}`)
	}
	if _, err := ParseDisputes([]byte(`{"disputes":[` + strings.Join(many, ",") + `]}`)); err == nil {
		t.Error("ParseDisputes() accepted more disputes than the limit")
	}
}
