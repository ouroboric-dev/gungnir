package schema

import (
	"strings"
	"testing"
)

func TestFactoryNamesTheGraphStore(t *testing.T) {
	f, err := ParseFactory("factory.yaml", []byte(factoryHead+"graphs:\n  dir: /home/op/voluspa\n"))
	if err != nil {
		t.Fatalf("ParseFactory() = %v", err)
	}
	if f.Graphs == nil || f.Graphs.Dir != "/home/op/voluspa" {
		t.Errorf("Graphs = %+v", f.Graphs)
	}
	if f, err := ParseFactory("factory.yaml", []byte(factoryHead)); err != nil || f.Graphs != nil {
		t.Errorf("a config with no graphs = %+v, %v", f, err)
	}
	for name, y := range map[string]string{
		"a relative folder":         "graphs:\n  dir: voluspa\n",
		"a folder that isn't clean": "graphs:\n  dir: /home/op/../op/voluspa\n",
		"the root":                  "graphs:\n  dir: /\n",
		"no folder":                 "graphs: {}\n",
		"an unknown field":          "graphs:\n  dir: /home/op/voluspa\n  url: https://example.com\n",
	} {
		_, err := ParseFactory("factory.yaml", []byte(factoryHead+y))
		if err == nil {
			t.Errorf("%s: ParseFactory() = nil error", name)
		} else if strings.Contains(err.Error(), "no complete budget caps") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
