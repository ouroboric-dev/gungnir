package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Contract names a versioned data boundary of the factory: the id of a JSON Schema in this package and of the Go
// type it describes (change typed-boundaries). Contracts change compatibly within a version, with new optional
// fields only; anything else is a new version.
type Contract string

const (
	ContractDesign      Contract = "ygg.design/v1"
	ContractSpec        Contract = "ygg.spec/v1"
	ContractConformance Contract = "ygg.conformance/v1"
	ContractPlan        Contract = "ygg.plan/v1"
	ContractReuseReview Contract = "ygg.reuse-review/v1"
	ContractReview      Contract = "ygg.review/v1"
	ContractImplement   Contract = "ygg.implement/v1"
	ContractCheck       Contract = "ygg.check/v1"
	ContractRetro       Contract = "ygg.retro/v1"
	ContractDecisions   Contract = "ygg.decisions/v1"
	ContractJudgement   Contract = "ygg.judgement/v1"
	ContractRecord      Contract = "ygg.run-record/v1"
)

// schemaNames maps each contract to its embedded schema file.
var schemaNames = map[Contract]string{
	ContractDesign:      "design.schema.json",
	ContractSpec:        "spec.schema.json",
	ContractConformance: "conformance.schema.json",
	ContractPlan:        "plan.schema.json",
	ContractReuseReview: "reuse-review.schema.json",
	ContractReview:      "review.schema.json",
	ContractImplement:   "implement.schema.json",
	ContractCheck:       "check.schema.json",
	ContractRetro:       "retro.schema.json",
	ContractDecisions:   "decisions.schema.json",
	ContractJudgement:   "judgement.schema.json",
	ContractRecord:      "run-record.schema.json",
}

// Contracts lists every contract, in a fixed order.
func Contracts() []Contract {
	return []Contract{
		ContractDesign, ContractSpec, ContractConformance, ContractPlan, ContractReuseReview, ContractReview,
		ContractImplement, ContractCheck, ContractRetro, ContractDecisions, ContractJudgement, ContractRecord,
	}
}

// SchemaFile returns the embedded file of the contract's schema, and false when the contract has none yet.
func (c Contract) SchemaFile() (string, bool) {
	name, ok := schemaNames[c]
	if !ok {
		return "", false
	}
	if _, err := fs.Stat(files, name); err != nil {
		return "", false
	}
	return name, true
}

// SchemaFS returns the embedded schemas, for the work image that carries them to a session.
func SchemaFS() fs.FS { return files }

// maxErrors is the most validation errors a ValidationError lists.
const maxErrors = 50

// ValidationError is a document that breaks its contract. Each error is a JSON pointer to the value and a
// sentence, sorted, so the list can go back to the model that wrote the document.
type ValidationError struct {
	Contract Contract
	Errors   []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Contract, strings.Join(e.Errors, "; "))
}

// compiled holds one compiled schema per contract.
var compiled = func() map[Contract]func() (*jsonschema.Schema, error) {
	m := map[Contract]func() (*jsonschema.Schema, error){}
	for c, name := range schemaNames {
		m[c] = sync.OnceValues(func() (*jsonschema.Schema, error) { return compile(name) })
	}
	return m
}()

// Validate checks data, which must be one JSON document, against the contract's schema.
func Validate(c Contract, data []byte) error {
	_, err := validateDoc(c, data)
	return err
}

func validateDoc(c Contract, data []byte) (any, error) {
	if _, ok := c.SchemaFile(); !ok {
		return nil, fmt.Errorf("%s: no schema is embedded for this contract", c)
	}
	s, err := compiled[c]()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, &ValidationError{Contract: c, Errors: []string{"(document) is not one JSON document: " + err.Error()}}
	}
	if err := s.Validate(doc); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return nil, fmt.Errorf("%s: %w", c, err)
		}
		return nil, &ValidationError{Contract: c, Errors: pointers(ve)}
	}
	return doc, nil
}

// pointers lists the leaf errors of a validation as sorted, distinct pointer-and-sentence strings.
func pointers(ve *jsonschema.ValidationError) []string {
	p := message.NewPrinter(language.English)
	seen := map[string]bool{}
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		at := "(document)"
		if len(e.InstanceLocation) > 0 {
			parts := make([]string, len(e.InstanceLocation))
			for i, s := range e.InstanceLocation {
				parts[i] = strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
			}
			at = "/" + strings.Join(parts, "/")
		}
		seen[at+" "+e.ErrorKind.LocalizedString(p)] = true
	}
	walk(ve)
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > maxErrors {
		out = out[:maxErrors]
	}
	return out
}

// checker is a contract type that holds a rule the JSON Schema cannot say, such as a unique id. Decode calls
// Check after the strict decoding.
type checker interface{ Check() error }

// Decode reads data as one JSON document of the contract: validated, then decoded into T with unknown fields
// refused. A second document, or anything but the document, is an error. A type with a Check method is checked last.
func Decode[T any](c Contract, data []byte) (T, error) {
	var v T
	if _, err := validateDoc(c, data); err != nil {
		return v, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return *new(T), fmt.Errorf("%s: %w", c, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return *new(T), fmt.Errorf("%s: more than one JSON document", c)
	}
	if ck, ok := any(&v).(checker); ok {
		if err := ck.Check(); err != nil {
			return *new(T), fmt.Errorf("%s: %w", c, err)
		}
	}
	return v, nil
}

// DecodeAnswer reads a session's whole final answer as one document of the contract. After the space is
// trimmed, the answer is the document, bare or in exactly one code fence, and nothing else: no prose before or
// after it, no second document.
func DecodeAnswer[T any](c Contract, answer []byte) (T, error) {
	var zero T
	body, err := unfence(string(answer))
	if err != nil {
		return zero, fmt.Errorf("%s: the answer must be exactly one JSON document: %w", c, err)
	}
	if !json.Valid([]byte(body)) {
		return zero, fmt.Errorf("%s: the answer must be exactly one JSON document, bare or in one code fence, with no other text", c)
	}
	return Decode[T](c, []byte(body))
}
