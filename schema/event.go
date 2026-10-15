package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

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
