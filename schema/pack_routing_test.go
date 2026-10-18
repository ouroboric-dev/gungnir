package schema

import (
	"strings"
	"testing"
)

func TestAPackSetsTheRoutingBenchThreshold(t *testing.T) {
	p, err := LoadPack(recipeRealm(t, recipe+"routing:\n  threshold: 0.8\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Routing == nil || p.Routing.Threshold != 0.8 || p.Routing.BenchPath() != DefaultBenchPath {
		t.Fatalf("Routing = %+v, want threshold 0.8 and the default bench path", p.Routing)
	}
	p, err = LoadPack(recipeRealm(t, recipe+"routing:\n  threshold: 1\n  bench: docs/bench.yaml\n"))
	if err != nil || p.Routing.BenchPath() != "docs/bench.yaml" || p.Routing.Threshold != 1 {
		t.Fatalf("LoadPack() = %+v, %v", p, err)
	}
	if p, err := LoadPack(recipeRealm(t, recipe)); err != nil || p.Routing != nil {
		t.Errorf("a pack with no routing block has Routing %+v, %v; want nil", p.Routing, err)
	}
}

func TestAPackRefusesABadRoutingBlock(t *testing.T) {
	for name, block := range map[string]string{
		"a threshold of zero":  "routing:\n  threshold: 0\n",
		"a threshold over one": "routing:\n  threshold: 1.5\n",
		"no threshold":         "routing:\n  bench: docs/bench.yaml\n",
		"a bench outside":      "routing:\n  threshold: 0.5\n  bench: ../bench.yaml\n",
		"an absolute bench":    "routing:\n  threshold: 0.5\n  bench: /etc/bench.yaml\n",
		"an unknown field":     "routing:\n  threshold: 0.5\n  model: x\n",
	} {
		_, err := LoadPack(recipeRealm(t, recipe+block))
		if err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		} else if !strings.Contains(err.Error(), "routing") {
			t.Errorf("%s: the error does not name the block: %v", name, err)
		}
	}
}
