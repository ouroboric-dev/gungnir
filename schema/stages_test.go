package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// roundTrip decodes a fixture into T, re-encodes it, and reports whether the two documents hold the same JSON
// values (ruling R3: key order and whitespace are not part of the contract).
func roundTrip[T any](t *testing.T, c Contract, data []byte) error {
	t.Helper()
	v, err := Decode[T](c, data)
	if err != nil {
		return err
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("the round trip changed the document:\n in: %s\nout: %s", data, out)
	}
	return nil
}

// fixtureKinds are the file-name prefixes of testdata/contracts, one per contract that has fixtures.
var fixtureKinds = []string{"design", "spec", "conformance", "reuse", "plan", "review", "implement", "check", "decisions", "judgement", "retro", "run-record"}

// fixtureKind is one prefix of the fixtures: its contract, and the function that reads a fixture as the contract's type.
type fixtureKind struct {
	contract Contract
	decode   func(*testing.T, Contract, []byte) error
}

func TestTheStageContractsReadTheirFixtures(t *testing.T) {
	runFixtures(t, map[string]fixtureKind{
		"design": {ContractDesign, roundTrip[Design]},
		"spec":   {ContractSpec, roundTrip[Spec]},
		"reuse":  {ContractReuseReview, roundTrip[ReuseReview]},
		"plan":   {ContractPlan, roundTrip[Plan]},
	})
}

// runFixtures reads the fixtures of the given prefixes and checks each against its name: -ok- is read, -bad- is
// refused at the pointer the name holds. A fixture of another known prefix is left to the test of that prefix.
func runFixtures(t *testing.T, kinds map[string]fixtureKind) {
	t.Helper()
	names, err := filepath.Glob("testdata/contracts/*.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	seen := map[string]int{}
	for _, name := range names {
		base := strings.TrimSuffix(filepath.Base(name), ".json")
		kind, rest, _ := strings.Cut(base, "-")
		if r, ok := strings.CutPrefix(base, "run-record-"); ok {
			kind, rest = "run-record", r // the one prefix with a dash in it
		}
		k, ok := kinds[kind]
		if !ok {
			if !slices.Contains(fixtureKinds, kind) {
				t.Errorf("%s: no contract is named %q", name, kind)
			}
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(base, func(t *testing.T) {
			err := k.decode(t, k.contract, data)
			switch verdict, where, _ := strings.Cut(rest, "-"); verdict {
			case "ok":
				seen[kind+"-ok"]++
				if err != nil {
					t.Fatalf("a valid fixture is refused: %v", err)
				}
			case "bad":
				seen[kind+"-bad"]++
				if err == nil {
					t.Fatal("an invalid fixture is read")
				}
				// The file name holds the JSON pointer of the failure, its slashes as dashes. A name that starts
				// with "check-" fails in the type's Check method, which has no pointer, so the error must name
				// the value (the duplicate question id, the outer leaf, the test), and say "used twice" when the rule is a
				// unique id (a spec and a check say it otherwise). A name that starts with "root-" misses a required property of the document itself:
				// the error is at "(document)" and names the property.
				wants := []string{"/" + strings.ReplaceAll(where, "-", "/")}
				if id, isCheck := strings.CutPrefix(where, "check-"); isCheck {
					wants = []string{id}
					if kind != "spec" && kind != "check" {
						wants = append(wants, "used twice")
					}
				} else if prop, isRoot := strings.CutPrefix(where, "root-"); isRoot {
					wants = []string{"(document)", prop}
				}
				for _, want := range wants {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the error does not hold %q: %v", want, err)
					}
				}
			default:
				t.Fatalf("the fixture name must hold -ok- or -bad-")
			}
		})
	}
	for kind := range kinds {
		if seen[kind+"-ok"] == 0 || seen[kind+"-bad"] == 0 {
			t.Errorf("%s has %d valid and %d invalid fixtures; it needs both", kind, seen[kind+"-ok"], seen[kind+"-bad"])
		}
	}
}
