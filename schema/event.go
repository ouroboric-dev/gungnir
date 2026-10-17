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

// ParseReport parses a reviewer's answer: one JSON document with a findings list, which may be wrapped
// in a single code fence and in nothing else. Prose, a second document, a finding without a location, a
// fixed finding without proof and an id used twice are errors, never an empty report: an answer that
// can't be read doesn't count as a pass (spec §4.6).
func ParseReport(answer []byte) (gungnir.Report, error) {
	var r gungnir.Report
	data, err := unfence(string(answer))
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
