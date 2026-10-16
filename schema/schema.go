// Package schema holds the JSON schemas of Yggdrasil's files, and parses the files against them.
// Every parser refuses what it can't read exactly; nothing missing or malformed is defaulted.
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

//go:embed *.schema.json
var files embed.FS

const base = "https://github.com/ouroboric-dev/gungnir/schema/"

// compile compiles one embedded schema. Its $id must be base + name. Every embedded schema is a
// resource of the compiler, so a schema may refer to another by its file name.
func compile(name string) (*jsonschema.Schema, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	for _, e := range entries {
		raw, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if err := c.AddResource(base+e.Name(), doc); err != nil {
			return nil, err
		}
	}
	return c.Compile(base + name)
}

// validate checks a decoded document against the named schema.
func validate(file, schemaName string, doc any) error {
	s, err := compile(schemaName)
	if err != nil {
		return err
	}
	if err := s.Validate(doc); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

// decodeYAML reads exactly one YAML document and returns it as JSON values, ready for validate.
// Duplicate keys are an error in the YAML decoder itself.
func decodeYAML(file string, data []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc any
	if err := dec.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: holds more than one YAML document", file)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: every key must be a string: %w", file, err)
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// decodeStrict decodes one YAML document into v and refuses unknown fields.
func decodeStrict(file string, data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

// parseYAML validates data against the named schema, then decodes it into v.
func parseYAML(file, schemaName string, data []byte, v any) error {
	doc, err := decodeYAML(file, data)
	if err != nil {
		return err
	}
	if err := validate(file, schemaName, doc); err != nil {
		return err
	}
	return decodeStrict(file, data, v)
}
