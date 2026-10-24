package schema

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

// otherReaders are the embedded schemas that no contract names: each has its own parser.
var otherReaders = map[string]bool{
	"factory": true, "graph": true, "pack": true, "leaves": true, "umbrella": true, "ledger-event": true,
	"finding": true, "suite": true, "score": true,
}

func TestEveryContractHasItsSchema(t *testing.T) {
	named := map[string]bool{}
	for _, c := range Contracts() {
		name, ok := c.SchemaFile()
		if !ok {
			t.Errorf("%s has no embedded schema", c)
			continue
		}
		named[name] = true
		raw, err := fs.ReadFile(SchemaFS(), name)
		if err != nil {
			t.Errorf("%s: schema file %s is not embedded: %v", c, name, err)
			continue
		}
		var doc struct {
			ID string `json:"$id"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("%s: %v", name, err)
		} else if doc.ID != base+name {
			t.Errorf("%s: $id is %q, want %q", name, doc.ID, base+name)
		}
	}
	if len(named) == 0 {
		t.Error("no contract has a schema file")
	}
	entries, err := fs.ReadDir(SchemaFS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !named[e.Name()] && !otherReaders[strings.TrimSuffix(e.Name(), ".schema.json")] {
			t.Errorf("%s belongs to no contract and to no other reader", e.Name())
		}
	}
	if c, ok := ContractPlan.SchemaFile(); ok && c != "plan.schema.json" {
		t.Errorf("ContractPlan.SchemaFile() = %q, want plan.schema.json", c)
	}
}

// sharedKeywords are the only keywords a contract schema may use (the global constraints, and ruling R1).
var sharedKeywords = map[string]bool{
	"$schema": true, "$id": true, "$ref": true, "$defs": true, "title": true, "description": true, "type": true,
	"properties": true, "required": true, "additionalProperties": true, "items": true, "enum": true, "const": true,
	"minLength": true, "maxLength": true, "minItems": true, "maxItems": true, "uniqueItems": true, "pattern": true,
	"minimum": true, "maximum": true, "oneOf": true, "if": true, "then": true, "else": true, "not": true,
	"minProperties": true, "maxProperties": true, "propertyNames": true,
}

func TestSchemasUseOnlyTheSharedKeywords(t *testing.T) {
	seen := map[string]bool{}
	var walkFile func(name string)
	var walk func(file, ptr string, node any)
	walk = func(file, ptr string, node any) {
		m, ok := node.(map[string]any)
		if !ok {
			return // true and false are schemas too
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !sharedKeywords[k] {
				t.Errorf("%s%s: the keyword %q is not in the shared list", file, ptr, k)
			}
		}
		if m["type"] == "object" {
			ap, has := m["additionalProperties"]
			if _, hasProps := m["properties"]; hasProps {
				if has && ap != false || !has {
					t.Errorf("%s%s: an object with properties must set additionalProperties to false", file, ptr)
				}
			} else if _, isSchema := ap.(map[string]any); !isSchema {
				t.Errorf("%s%s: a map object must set additionalProperties to a schema", file, ptr)
			}
		}
		if ref, ok := m["$ref"].(string); ok {
			if file, _, _ := strings.Cut(ref, "#"); file != "" {
				walkFile(path.Base(file))
			}
		}
		for _, k := range []string{"properties", "$defs"} {
			sub, _ := m[k].(map[string]any)
			for name, s := range sub {
				walk(file, ptr+"/"+k+"/"+name, s)
			}
		}
		for _, k := range []string{"items", "additionalProperties", "propertyNames", "if", "then", "else", "not"} {
			if s, ok := m[k]; ok {
				walk(file, ptr+"/"+k, s)
			}
		}
		if list, ok := m["oneOf"].([]any); ok {
			for i, s := range list {
				walk(file, ptr+"/oneOf/"+strconv.Itoa(i), s)
			}
		}
	}
	walkFile = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		raw, err := fs.ReadFile(SchemaFS(), name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		walk(name, "", doc)
	}
	for _, c := range Contracts() {
		if name, ok := c.SchemaFile(); ok {
			walkFile(name)
		}
	}
	if !seen["conformance.schema.json"] {
		t.Error("the conformance schema was not walked")
	}
}

func TestDecodeIsStrict(t *testing.T) {
	got, err := Decode[gungnir.Conformance](ContractConformance, []byte(`{"deviation": false}`))
	if err != nil || got.Deviation {
		t.Fatalf("Decode() = %+v, %v; want a verdict with no deviation", got, err)
	}
	bad := map[string]string{
		"prose":             `The spec conforms.`,
		"two documents":     `{"deviation": false} {"deviation": false}`,
		"an unknown field":  `{"deviation": false, "ok": true}`,
		"a fenced document": "```json\n{\"deviation\": false}\n```",
		"an empty input":    ``,
	}
	for name, data := range bad {
		if v, err := Decode[gungnir.Conformance](ContractConformance, []byte(data)); err == nil {
			t.Errorf("%s: Decode() = %+v, nil; want an error", name, v)
		}
	}
	err = Validate(ContractConformance, []byte(`{"deviation": "no"}`))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate() = %v; want a *ValidationError", err)
	}
	if ve.Contract != ContractConformance || len(ve.Errors) == 0 || !strings.HasPrefix(ve.Errors[0], "/deviation") {
		t.Errorf("ValidationError = %+v; want the first error to start with /deviation", ve)
	}
	if err := Validate("ygg.nothing/v1", []byte(`{}`)); err == nil {
		t.Error("Validate() of an unknown contract = nil; want an error")
	}
}

func TestValidationErrorsAreSortedAndBounded(t *testing.T) {
	// Eighty bad keys: the unknown fields are refused, and the list stops at fifty.
	var b strings.Builder
	b.WriteString(`{"deviation": false`)
	for i := range 80 {
		b.WriteString(`, "x` + strings.Repeat("a", i) + `": 1`)
	}
	b.WriteString(`}`)
	err := Validate(ContractConformance, []byte(b.String()))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate() = %v; want a *ValidationError", err)
	}
	if len(ve.Errors) == 0 || len(ve.Errors) > 50 || !sort.StringsAreSorted(ve.Errors) {
		t.Errorf("got %d errors, sorted %v; want 1 to 50, sorted", len(ve.Errors), sort.StringsAreSorted(ve.Errors))
	}
}

func TestDecodeAnswerTakesOnlyAWholeDocument(t *testing.T) {
	ok := []string{
		`{"deviation": false}`,
		"  \n{\"deviation\": false}\n ",
		"```json\n{\"deviation\": false}\n```",
		"```\n{\"deviation\": false}\n```\n",
	}
	for _, a := range ok {
		if got, err := DecodeAnswer[gungnir.Conformance](ContractConformance, []byte(a)); err != nil || got.Deviation {
			t.Errorf("DecodeAnswer(%q) = %+v, %v; want a verdict", a, got, err)
		}
	}
	bad := map[string]string{
		"a sentence before":  "The spec conforms.\n{\"deviation\": false}",
		"a sentence after":   "{\"deviation\": false}\nThat is all.",
		"a sentence outside": "Here:\n```json\n{\"deviation\": false}\n```",
		"two fences":         "```json\n{\"deviation\": false}\n```\n```json\n{\"deviation\": false}\n```",
		"an unclosed fence":  "```json\n{\"deviation\": false}",
		"two documents":      `{"deviation": false} {"deviation": false}`,
		"an empty answer":    ``,
		"a blank answer":     " \n ",
	}
	for name, a := range bad {
		_, err := DecodeAnswer[gungnir.Conformance](ContractConformance, []byte(a))
		if err == nil || !strings.Contains(err.Error(), "the answer must be exactly one JSON document") {
			t.Errorf("%s: DecodeAnswer() error = %v; want it to say the answer must be exactly one JSON document", name, err)
		}
	}
	// A whole document that breaks the contract is a validation error, not a framing error.
	_, err := DecodeAnswer[gungnir.Conformance](ContractConformance, []byte(`{"deviation": "no"}`))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("DecodeAnswer() = %v; want a *ValidationError", err)
	}
}

func TestDecodeChecksAPointerType(t *testing.T) {
	data, err := os.ReadFile("testdata/contracts/design-bad-check-q2.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode[*Design](ContractDesign, data); err == nil || !strings.Contains(err.Error(), "used twice") {
		t.Errorf("Decode[*Design]() error = %v; want it to say the question id is used twice", err)
	}
	answer := "```json\n" + `{"summary":"s","disputes":[{"id":"a","check":"c","test":"t","evidence":"e"},` +
		`{"id":"a","check":"c","test":"t","evidence":"e"}]}` + "\n```"
	if _, err := DecodeAnswer[*Implement](ContractImplement, []byte(answer)); err == nil || !strings.Contains(err.Error(), "used twice") {
		t.Errorf("DecodeAnswer[*Implement]() error = %v; want it to say the finding id is used twice", err)
	}
}

func TestDecodeAnswerTakesAFenceInsideAJSONString(t *testing.T) {
	data, err := os.ReadFile("testdata/contracts/plan-ok-two-tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "```") {
		t.Fatal("the plan fixture holds no code fence in a string")
	}
	p, err := DecodeAnswer[Plan](ContractPlan, []byte("```json\n"+string(data)+"\n```"))
	if err != nil || len(p.Tasks) != 2 {
		t.Errorf("DecodeAnswer() = %d tasks, %v; want the fenced plan read whole", len(p.Tasks), err)
	}
}

// agentOutputs are the contracts an agent session answers with: each carries an optional blocked and notes
// (design §1, ruling R14).
var agentOutputs = []Contract{
	ContractDesign, ContractSpec, ContractConformance, ContractPlan, ContractReuseReview, ContractReview,
	ContractImplement, ContractRetro,
}

func TestEveryAgentOutputHasTheSameBlockedAndNotes(t *testing.T) {
	var first map[string]json.RawMessage
	for _, c := range agentOutputs {
		name, _ := c.SchemaFile()
		raw, err := fs.ReadFile(SchemaFS(), name)
		if err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		var doc struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
			Defs       map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defs := map[string]json.RawMessage{}
		for _, k := range []string{"blocked", "notes"} {
			if !jsonEqual(doc.Properties[k], `{"$ref":"#/$defs/`+k+`"}`) {
				t.Errorf("%s: properties/%s is %s; want a $ref to $defs/%s", name, k, doc.Properties[k], k)
			}
			if slices.Contains(doc.Required, k) {
				t.Errorf("%s: %s is required; it must be optional", name, k)
			}
			defs[k] = doc.Defs[k]
		}
		if first == nil {
			first = defs
			continue
		}
		for k, d := range defs {
			if !jsonEqual(d, string(first[k])) {
				t.Errorf("%s: $defs/%s differs from the one in %s", name, k, schemaNames[agentOutputs[0]])
			}
		}
	}
}

// jsonEqual reports whether a and b hold the same JSON value.
func jsonEqual(a json.RawMessage, b string) bool {
	var x, y any
	return json.Unmarshal(a, &x) == nil && json.Unmarshal([]byte(b), &y) == nil && reflect.DeepEqual(x, y)
}

func TestSchemaPatternsMeanTheSameInGoAndJS(t *testing.T) {
	// \s, \S and . differ between Go's RE2 and JavaScript's u-flag regexps (Unicode spaces, \v, \r and U+2028), so
	// every pattern spells its classes out.
	entries, err := fs.ReadDir(SchemaFS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	var walk func(file string, node any)
	walk = func(file string, node any) {
		switch v := node.(type) {
		case map[string]any:
			if p, ok := v["pattern"].(string); ok {
				if bad := unportable(p); bad != "" {
					t.Errorf("%s: pattern %q uses %s", file, p, bad)
				}
			}
			for _, sub := range v {
				walk(file, sub)
			}
		case []any:
			for _, sub := range v {
				walk(file, sub)
			}
		}
	}
	for _, e := range entries {
		raw, err := fs.ReadFile(SchemaFS(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		walk(e.Name(), doc)
	}
	for name, doc := range map[string]string{
		"a blank amendment of a vertical tab": `{"deviation": true, "amendment": "\u000b"}`,
	} {
		if err := Validate(ContractConformance, []byte(doc)); err == nil {
			t.Errorf("%s is valid", name)
		}
	}
	check := `{"check":"lint","passed":false,"exit_code":1,"timed_out":false,"duration_ms":1,"failures":[{"kind":"lint","message":"m","location":"a\rb.go:1"}]}`
	if err := Validate(ContractCheck, []byte(check)); err == nil {
		t.Error("a check location with a carriage return is valid")
	}
}

// unportable names the first escape or wildcard of a regexp that Go and JavaScript read differently, or "".
func unportable(p string) string {
	inClass := false
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '\\' && i+1 < len(p):
			i++
			if strings.ContainsRune("sSdDwWbB", rune(p[i])) {
				return `\` + string(p[i])
			}
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '.' && !inClass:
			return "."
		}
	}
	return ""
}

func TestSchemasDefineOnlyWhatTheyUse(t *testing.T) {
	entries, err := fs.ReadDir(SchemaFS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	raws := map[string]string{}
	for _, e := range entries {
		raw, err := fs.ReadFile(SchemaFS(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		raws[e.Name()] = string(raw)
	}
	for name, raw := range raws {
		var doc struct {
			Defs map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for def := range doc.Defs {
			used := strings.Contains(raw, `"#/$defs/`+def+`"`)
			for other, r := range raws {
				used = used || other != name && strings.Contains(r, `"`+name+`#/$defs/`+def+`"`)
			}
			if !used {
				t.Errorf("%s: $defs/%s is used by nothing", name, def)
			}
		}
	}
}
