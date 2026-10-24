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

// unfence returns the JSON of an answer that is a bare document, or one document in one code fence. A fence
// inside the document's strings is part of the document.
func unfence(answer string) (string, error) {
	s := strings.TrimSpace(answer)
	if !strings.HasPrefix(s, "```") {
		return s, nil
	}
	_, body, ok := strings.Cut(s, "\n")
	body = strings.TrimSpace(body)
	if !ok || !strings.HasSuffix(body, "```") {
		return "", errors.New("a code fence is not closed")
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "```"))
	if json.Valid([]byte(body)) {
		return body, nil // a fence inside a JSON string is the document's text, not a second fence
	}
	if strings.Contains(body, "```") {
		return "", errors.New("more than one code fence")
	}
	return body, nil
}

// ParseConformance parses the design-conformance reviewer's verdict: one JSON document, bare or in one
// code fence, as the whole answer. A verdict that can't be read is an error, never a pass, and so, unless the
// verdict is blocked, are a deviation without an amendment and an amendment without a deviation: the spec gate
// would let a deviation through (spec §4.6).
func ParseConformance(answer []byte) (gungnir.Conformance, error) {
	c, err := DecodeAnswer[gungnir.Conformance](ContractConformance, answer)
	if err != nil {
		return gungnir.Conformance{}, fmt.Errorf("conformance: %w", err)
	}
	return c, nil
}
