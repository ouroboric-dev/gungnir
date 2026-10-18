package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ouroboric-dev/gungnir"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ParseSuite parses a hidden suite's tests/manifest.json. A manifest that fails the schema or Validate
// is an error, never an empty suite: a scenario nobody can read must not count as a pass (spec §4.6).
func ParseSuite(data []byte) (gungnir.Suite, error) {
	var s gungnir.Suite
	if err := parseJSON("manifest", "suite.schema.json", data, &s); err != nil {
		return gungnir.Suite{}, err
	}
	if err := s.Validate(); err != nil {
		return gungnir.Suite{}, fmt.Errorf("manifest: %w", err)
	}
	return s, nil
}

// ParseScore parses a run's score.json, and refuses one whose tallies, categories, weights or reward do
// not add up.
func ParseScore(data []byte) (gungnir.Score, error) {
	var s gungnir.Score
	if err := parseJSON("score", "score.schema.json", data, &s); err != nil {
		return gungnir.Score{}, err
	}
	if err := s.Validate(); err != nil {
		return gungnir.Score{}, fmt.Errorf("score: %w", err)
	}
	return s, nil
}

// parseJSON validates one JSON document against the named schema, then decodes it into v without
// unknown fields and without a second document.
func parseJSON(what, schemaName string, data []byte, v any) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if err := validate(what, schemaName, doc); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}
