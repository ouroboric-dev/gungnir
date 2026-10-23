package schema

import (
	"strings"
	"testing"
)

const leavesBody = `
leaves:
  api:
    realm: helheim-go
    realm_version: 0.1.0
    description: the api
`

const umbrellaHead = `
root: true
owner: ouroboric-dev
visibility: private
graph: software
graph_version: 0.1.0
`

// TestRuntimePin: a project pins the one runtime realm that provides its services, in the leaves
// file and in the umbrella, beside its graph pin.
func TestRuntimePin(t *testing.T) {
	t.Run("leaves file with a runtime", func(t *testing.T) {
		l, err := ParseLeaves("leaves.yaml", []byte("runtime:\n  realm: niflheim-local\n  realm_version: 0.1.0\n"+leavesBody))
		if err != nil {
			t.Fatal(err)
		}
		if l.Runtime == nil || l.Runtime.Realm != "niflheim-local" || l.Runtime.RealmVersion != "0.1.0" {
			t.Fatalf("Runtime = %+v", l.Runtime)
		}
		if got := l.Pins(); len(got) != 1 || got[0].Realm != "helheim-go" {
			t.Fatalf("Pins() = %v, want the leaf's pin only", got)
		}
	})
	t.Run("leaves file with no runtime", func(t *testing.T) {
		l, err := ParseLeaves("leaves.yaml", []byte(leavesBody))
		if err != nil {
			t.Fatal(err)
		}
		if l.Runtime != nil {
			t.Fatalf("Runtime = %+v, want nil", l.Runtime)
		}
	})
	for name, runtime := range map[string]string{
		"no version":     "  realm: niflheim-local\n",
		"no realm":       "  realm_version: 0.1.0\n",
		"bad realm name": "  realm: Niflheim_Local\n  realm_version: 0.1.0\n",
		"bad version":    "  realm: niflheim-local\n  realm_version: latest\n",
		"unknown field":  "  realm: niflheim-local\n  realm_version: 0.1.0\n  start: up\n",
		"empty runtime":  " {}\n",
	} {
		t.Run("leaves file refused: "+name, func(t *testing.T) {
			_, err := ParseLeaves("leaves.yaml", []byte("runtime:\n"+runtime+leavesBody))
			if err == nil || !strings.Contains(err.Error(), "runtime") {
				t.Fatalf("err = %v, want one naming runtime", err)
			}
		})
	}

	t.Run("umbrella with a runtime", func(t *testing.T) {
		u, err := ParseUmbrella("umbrella.yaml", []byte(umbrellaHead+"runtime: niflheim-local\nruntime_version: 0.1.0\n"))
		if err != nil {
			t.Fatal(err)
		}
		p, ok := u.RuntimePin()
		if !ok || p != (Pin{Realm: "niflheim-local", Version: "0.1.0", Where: "runtime"}) {
			t.Fatalf("RuntimePin() = %+v, %v", p, ok)
		}
		if got := u.Pins(); len(got) != 0 {
			t.Fatalf("Pins() = %v, want the leaves' pins only", got)
		}
	})
	t.Run("umbrella with no runtime", func(t *testing.T) {
		u, err := ParseUmbrella("umbrella.yaml", []byte(umbrellaHead))
		if err != nil {
			t.Fatal(err)
		}
		if p, ok := u.RuntimePin(); ok {
			t.Fatalf("RuntimePin() = %+v, want none", p)
		}
	})
	for name, body := range map[string]string{
		"runtime without version": "runtime: niflheim-local\n",
		"version without runtime": "runtime_version: 0.1.0\n",
		"bad name":                "runtime: Niflheim\nruntime_version: 0.1.0\n",
		"bad version":             "runtime: niflheim-local\nruntime_version: latest\n",
	} {
		t.Run("umbrella refused: "+name, func(t *testing.T) {
			_, err := ParseUmbrella("umbrella.yaml", []byte(umbrellaHead+body))
			if err == nil || !strings.Contains(err.Error(), "runtime") {
				t.Fatalf("err = %v, want one naming runtime", err)
			}
		})
	}
	t.Run("top-level realm still refused", func(t *testing.T) {
		_, err := ParseUmbrella("umbrella.yaml", []byte(umbrellaHead+"realm: niflheim-local\nrealm_version: 0.1.0\n"))
		if err == nil || !strings.Contains(err.Error(), "voluspa") {
			t.Fatalf("err = %v, want the voluspa refusal", err)
		}
	})
}

// TestLeavesRuntimePin: the leaves file gives its runtime pin as the umbrella does.
func TestLeavesRuntimePin(t *testing.T) {
	l, err := ParseLeaves("leaves.yaml", []byte("runtime:\n  realm: niflheim-local\n  realm_version: 0.1.0\n"+leavesBody))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := l.RuntimePin(); !ok || got != (Pin{"niflheim-local", "0.1.0", "runtime"}) {
		t.Errorf("RuntimePin() = %+v, %v", got, ok)
	}
	l, err = ParseLeaves("leaves.yaml", []byte(leavesBody))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := l.RuntimePin(); ok {
		t.Errorf("RuntimePin() = %+v, true; want none", got)
	}
}
