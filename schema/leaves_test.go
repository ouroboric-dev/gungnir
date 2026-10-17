package schema

import (
	"slices"
	"strings"
	"testing"
)

const leavesFile = `
leaves:
  repos/api:
    realm: helheim-go
    realm_version: 0.1.0
    description: The gallery's API.
  repos/web:
    realm: midgard-react
    realm_version: 0.1.0
    description: The gallery's web app.
    consumes: [repos/api]
`

func TestTheSpecStageNamesTheLeavesAndTheirRealms(t *testing.T) {
	l, err := ParseLeaves("leaves.yaml", []byte(leavesFile))
	if err != nil {
		t.Fatalf("ParseLeaves() = %v", err)
	}
	if !slices.Equal(l.Paths(), []string{"repos/api", "repos/web"}) {
		t.Errorf("Paths() = %q", l.Paths())
	}
	web := l.Leaves["repos/web"]
	if web.Realm != "midgard-react" || web.RealmVersion != "0.1.0" || !slices.Equal(web.Consumes, []string{"repos/api"}) || web.Description != "The gallery's web app." {
		t.Errorf("web = %+v", web)
	}
	pins := l.Pins()
	if len(pins) != 2 || pins[0] != (Pin{"helheim-go", "0.1.0", "repos/api"}) {
		t.Errorf("Pins() = %+v", pins)
	}
}

func TestParseLeavesRefuses(t *testing.T) {
	leaf := func(path, extra string) string {
		return "leaves:\n  " + path + ": { realm: helheim-go, realm_version: 0.1.0, description: x" + extra + " }\n"
	}
	cases := map[string]string{
		"no leaves":                           "leaves: {}\n",
		"a leaf with no realm":                "leaves:\n  repos/api: { realm_version: 0.1.0, description: x }\n",
		"a leaf with no version":              "leaves:\n  repos/api: { realm: helheim-go, description: x }\n",
		"a leaf with no description":          "leaves:\n  repos/api: { realm: helheim-go, realm_version: 0.1.0 }\n",
		"a description of two lines":          "leaves:\n  repos/api: { realm: helheim-go, realm_version: 0.1.0, description: \"a\\nb\" }\n",
		"a realm in capitals":                 "leaves:\n  repos/api: { realm: Helheim, realm_version: 0.1.0, description: x }\n",
		"a path that climbs":                  leaf("../api", ""),
		"a path in .git":                      leaf(".git/api", ""),
		"a path in .factory":                  leaf(".factory/api", ""),
		"an absolute path":                    leaf("/srv/api", ""),
		"a leaf inside a leaf":                leaf("repos/api", "") + "  repos/api/sub: { realm: helheim-go, realm_version: 0.1.0, description: x }\n",
		"a leaf inside a leaf past a sibling": leaf("x", "") + "  x-y: { realm: helheim-go, realm_version: 0.1.0, description: x }\n  x/z: { realm: helheim-go, realm_version: 0.1.0, description: x }\n",
		"an unknown field":                    leaf("repos/api", ", sandbox: off"),
		"a leaf consuming itself":             leaf("repos/api", ", consumes: [repos/api]"),
		"a leaf consuming twice":              leaf("repos/api", ", consumes: [repos/db, repos/db]"),
		"a consumer that climbs":              leaf("repos/api", ", consumes: [../db]"),
		"a ring":                              leaf("repos/a", ", consumes: [repos/b]") + "  repos/b: { realm: helheim-go, realm_version: 0.1.0, description: x, consumes: [repos/a] }\n",
	}
	cases["a description too long"] = "leaves:\n  repos/api: { realm: helheim-go, realm_version: 0.1.0, description: " + strings.Repeat("x", 201) + " }\n"
	for name, y := range cases {
		if _, err := ParseLeaves("leaves.yaml", []byte(y)); err == nil {
			t.Errorf("%s: ParseLeaves() = nil error", name)
		}
	}
	if _, err := ParseLeaves("leaves.yaml", []byte(leave21())); err == nil {
		t.Error("twenty-one leaves parsed; a product with that many leaves needs a company umbrella")
	}
}

func leave21() string {
	var b strings.Builder
	b.WriteString("leaves:\n")
	for i := 0; i < 21; i++ {
		b.WriteString("  repos/l" + string(rune('a'+i)) + ": { realm: helheim-go, realm_version: 0.1.0, description: x }\n")
	}
	return b.String()
}
