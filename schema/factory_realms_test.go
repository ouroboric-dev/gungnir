package schema

import (
	"strings"
	"testing"
)

const factoryHead = "roles:\n  implementer: { agent: scripted, model: none, script: s.yaml }\nbudget:\n  per_run: { tokens: 1000, turns: 10 }\n  per_day: { tokens: 2000, turns: 20 }\n"

func TestFactoryNamesWhereTheRealmsAre(t *testing.T) {
	f, err := ParseFactory("factory.yaml", []byte(factoryHead+"realms:\n  dir: /home/op/yggdrasil\n"))
	if err != nil {
		t.Fatalf("ParseFactory() = %v", err)
	}
	if f.Realms == nil || f.Realms.Dir != "/home/op/yggdrasil" {
		t.Errorf("Realms = %+v", f.Realms)
	}
	if f, err := ParseFactory("factory.yaml", []byte(factoryHead)); err != nil || f.Realms != nil {
		t.Errorf("a config with no realms = %+v, %v", f, err)
	}
	for name, y := range map[string]string{
		"a relative folder":         "realms:\n  dir: yggdrasil\n",
		"a folder that isn't clean": "realms:\n  dir: /home/op/../op/yggdrasil\n",
		"the root":                  "realms:\n  dir: /\n",
		"no folder":                 "realms: {}\n",
		"an unknown field":          "realms:\n  dir: /home/op/yggdrasil\n  url: https://example.com\n",
	} {
		_, err := ParseFactory("factory.yaml", []byte(factoryHead+y))
		if err == nil {
			t.Errorf("%s: ParseFactory() = nil error", name)
		} else if strings.Contains(err.Error(), "no complete budget caps") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
