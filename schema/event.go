package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ouroboric-dev/gungnir"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateEvent checks one ledger line, without its newline, against the ledger schema.
func ValidateEvent(line []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(line))
	if err != nil {
		return fmt.Errorf("ledger line: %w", err)
	}
	return validate("ledger line", "ledger-event.schema.json", doc)
}

// ParseFinding parses one finding as JSON. A finding that fails the schema or Validate is an error,
// never a pass (spec §4.6).
func ParseFinding(data []byte) (gungnir.Finding, error) {
	var f gungnir.Finding
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return f, fmt.Errorf("finding: %w", err)
	}
	if err := validate("finding", "finding.schema.json", doc); err != nil {
		return f, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return f, fmt.Errorf("finding: %w", err)
	}
	return f, f.Validate()
}

// ParseReport parses a reviewer's answer: prose may come first, then one JSON document with a findings
// list, bare or in a single code fence, as the last thing in the answer. The prose is ignored, JSON quoted in
// it included; only an object with a findings key there counts as a second document. Text after
// the report, a second report, an unclosed fence, an answer with no document, a finding
// without a location, a fixed finding without proof and an id used twice are errors, never an empty
// report: an answer that can't be read doesn't count as a pass (spec §4.6).
func ParseReport(answer []byte) (gungnir.Report, error) {
	var r gungnir.Report
	data, err := lastDocument("report", "findings", string(answer))
	if err != nil {
		return r, err
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(data))
	if err != nil {
		return r, fmt.Errorf("report: %w", err)
	}
	if err := validate("report", "report.schema.json", doc); err != nil {
		return r, err
	}
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("report: %w", err)
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		if err := f.Validate(); err != nil {
			return gungnir.Report{}, fmt.Errorf("report: finding %q: %w", f.ID, err)
		}
		if seen[f.ID] {
			return gungnir.Report{}, fmt.Errorf("report: finding id %q is used twice", f.ID)
		}
		seen[f.ID] = true
	}
	return r, nil
}

// lastDocument returns the document at the end of an answer, named what in errors: a bare JSON document or
// one in the last code fence, with any prose before it. The prose is ignored: JSON quoted in it, inline or
// in earlier fences, is prose. Only a document whose top level has key (findings for a report, disputes for
// disputes) counts as a second document, and it is refused. Text after the document, a code fence left
// unclosed and an answer with no document are errors.
func lastDocument(what, key, answer string) (string, error) {
	s := strings.TrimSpace(answer)
	var prose, data string
	switch n := strings.Count(s, "```"); {
	case n == 0:
		start := -1
		for i := 0; i < len(s); i++ {
			if s[i] == '{' && json.Valid([]byte(s[i:])) {
				start = i
				break
			}
		}
		if start < 0 {
			return "", fmt.Errorf("%s: the answer does not end with a JSON document", what)
		}
		// The report is the first tail that parses; a report-shaped document before it is a second document.
		prose, data = s[:start], s[start:]
	case n%2 == 1:
		return "", fmt.Errorf("%s: a code fence is not closed", what)
	default:
		if !strings.HasSuffix(s, "```") {
			return "", fmt.Errorf("%s: text after the report", what)
		}
		// The last fence is the document; earlier fenced blocks are prose.
		open := strings.LastIndex(strings.TrimSuffix(s, "```"), "```")
		prose = s[:open]
		_, body, ok := strings.Cut(s[open:], "\n")
		if !ok {
			return "", fmt.Errorf("%s: a code fence is not closed", what)
		}
		data = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "```"))
	}
	if containsDocument(prose, key) {
		return "", fmt.Errorf("%s: more than one document", what)
	}
	return data, nil
}

// containsDocument reports whether text holds a complete JSON object whose top level has key. Other JSON
// quoted in the text, and braces that don't parse, are prose and don't count.
func containsDocument(text, key string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		var v map[string]json.RawMessage
		if json.NewDecoder(strings.NewReader(text[i:])).Decode(&v) == nil {
			if _, ok := v[key]; ok {
				return true
			}
		}
	}
	return false
}

// unfence returns the JSON of an answer that is a bare document, or one document in one code fence.
func unfence(answer string) (string, error) {
	s := strings.TrimSpace(answer)
	if !strings.HasPrefix(s, "```") {
		return s, nil
	}
	_, body, ok := strings.Cut(s, "\n")
	body = strings.TrimSpace(body)
	if !ok || !strings.HasSuffix(body, "```") {
		return "", errors.New("report: a code fence is not closed")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "```"))
	if strings.Contains(body, "```") {
		return "", errors.New("report: more than one code fence")
	}
	return body, nil
}

// ParseConformance parses the design-conformance reviewer's verdict: one JSON document, bare or in one
// code fence. A verdict that can't be read, a deviation without an amendment and an amendment without a
// deviation are errors, never a pass: the spec gate would let a deviation through (spec §4.6).
func ParseConformance(answer []byte) (gungnir.Conformance, error) {
	var c gungnir.Conformance
	data, err := unfence(string(answer))
	if err != nil {
		return c, err
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(data))
	if err != nil {
		return c, fmt.Errorf("conformance: %w", err)
	}
	if err := validate("conformance", "conformance.schema.json", doc); err != nil {
		return c, err
	}
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return gungnir.Conformance{}, fmt.Errorf("conformance: %w", err)
	}
	if dec.More() {
		return gungnir.Conformance{}, fmt.Errorf("conformance: more than one document")
	}
	return c, nil
}

// ParseDisputes parses an implementer's dispute document, framed exactly like a report: prose may come
// first, then one JSON document with a disputes list, bare or in a single code fence, as the last thing in
// the answer. Every dispute needs a finding id, a check, a test and evidence, none empty or over its limit.
// A field left out, an unknown field, a second document, text after it and an id used twice are errors:
// a dispute that can't be read changes no finding (change disputed-findings).
func ParseDisputes(answer []byte) (gungnir.Disputes, error) {
	var d gungnir.Disputes
	data, err := lastDocument("disputes", "disputes", string(answer))
	if err != nil {
		return d, err
	}
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(data))
	if err != nil {
		return d, fmt.Errorf("disputes: %w", err)
	}
	if err := validate("disputes", "disputes.schema.json", doc); err != nil {
		return d, err
	}
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return gungnir.Disputes{}, fmt.Errorf("disputes: %w", err)
	}
	seen := map[string]bool{}
	for _, x := range d.Disputes {
		if err := x.Validate(); err != nil {
			return gungnir.Disputes{}, fmt.Errorf("disputes: dispute %q: %w", x.ID, err)
		}
		if seen[x.ID] {
			return gungnir.Disputes{}, fmt.Errorf("disputes: finding id %q is disputed twice", x.ID)
		}
		seen[x.ID] = true
	}
	return d, nil
}
