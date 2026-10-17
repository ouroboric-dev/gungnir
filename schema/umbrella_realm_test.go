package schema

import (
	"slices"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

const gallery = `
root: true
owner: ouroboric-dev
visibility: private
realm: midgard-react
realm_version: 0.1.0
children:
  repos/api:
    kind: leaf
    owner: ouroboric-dev
    visibility: private
    realm: helheim-go
    realm_version: 0.1.0
  repos/web:
    kind: leaf
    owner: ouroboric-dev
    visibility: private
    realm: midgard-react
    realm_version: 0.1.0
    consumes: [repos/api]
  repos/legacy: { kind: leaf, owner: ouroboric-dev, visibility: private }
`

func TestAProjectRecordsItsLeadRealmAndEachLeafsRealm(t *testing.T) {
	u, err := ParseUmbrella("umbrella.yaml", []byte(gallery))
	if err != nil {
		t.Fatalf("ParseUmbrella() = %v", err)
	}
	if u.Realm != "midgard-react" || u.RealmVersion != "0.1.0" {
		t.Errorf("lead realm = %s %s", u.Realm, u.RealmVersion)
	}
	api, web, legacy := u.Children["repos/api"], u.Children["repos/web"], u.Children["repos/legacy"]
	if api.Realm != "helheim-go" || api.RealmVersion != "0.1.0" {
		t.Errorf("api = %+v", api)
	}
	if web.Realm != "midgard-react" || !slices.Equal(web.Consumes, []string{"repos/api"}) {
		t.Errorf("web = %+v", web)
	}
	if legacy.Realm != "" || legacy.RealmVersion != "" || legacy.Consumes != nil {
		t.Errorf("a leaf with no pin keeps none: %+v", legacy)
	}
	pins := u.Pins()
	want := []Pin{{"helheim-go", "0.1.0", "repos/api"}, {"midgard-react", "0.1.0", "."}, {"midgard-react", "0.1.0", "repos/web"}}
	slices.SortFunc(pins, func(a, b Pin) int { return strings.Compare(a.Realm+a.Where, b.Realm+b.Where) })
	if !slices.Equal(pins, want) {
		t.Errorf("Pins() = %+v, want %+v", pins, want)
	}
}

func TestParseUmbrellaRefusesBadRealmPins(t *testing.T) {
	head := "owner: o\nvisibility: private\nchildren:\n"
	leaf := func(extra string) string { return "  a: { kind: leaf, owner: o, visibility: private" + extra + " }\n" }
	cases := map[string]string{
		"a realm with no version":         head + leaf(", realm: midgard-react"),
		"a version with no realm":         head + leaf(", realm_version: 0.1.0"),
		"a realm name in capitals":        head + leaf(", realm: Midgard, realm_version: 0.1.0"),
		"a version that is not semver":    head + leaf(", realm: midgard-react, realm_version: v1"),
		"a lead realm with no version":    "realm: midgard-react\n" + head + leaf(""),
		"a lead version with no realm":    "realm_version: 0.1.0\n" + head + leaf(""),
		"a leaf consuming itself":         head + leaf(", consumes: [a]"),
		"a leaf consuming a stranger":     head + leaf(", consumes: [b]"),
		"a leaf consuming twice":          head + leaf(", consumes: [b, b]") + "  b: { kind: leaf, owner: o, visibility: private }\n",
		"a leaf consuming an umbrella":    head + leaf(", consumes: [u]") + "  u: { kind: umbrella, owner: o, visibility: private }\n",
		"an umbrella that consumes":       head + "  u: { kind: umbrella, owner: o, visibility: private, consumes: [a] }\n" + "  a: { kind: leaf, owner: o, visibility: private }\n",
		"two leaves consuming each other": head + leaf(", consumes: [b]") + "  b: { kind: leaf, owner: o, visibility: private, consumes: [a] }\n",
		"a ring of three":                 head + leaf(", consumes: [b]") + "  b: { kind: leaf, owner: o, visibility: private, consumes: [c] }\n  c: { kind: leaf, owner: o, visibility: private, consumes: [a] }\n",
	}
	for name, y := range cases {
		if _, err := ParseUmbrella("umbrella.yaml", []byte(y)); err == nil {
			t.Errorf("%s: ParseUmbrella() = nil error", name)
		}
	}
}

func TestUmbrellaKeepsItsPinsWhenSaved(t *testing.T) {
	dir := t.TempDir()
	u, err := ParseUmbrella("umbrella.yaml", []byte(gallery))
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := LoadUmbrella(dir)
	if err != nil {
		t.Fatalf("LoadUmbrella() after Save = %v", err)
	}
	if got.Realm != "midgard-react" || got.Children["repos/web"].Consumes[0] != "repos/api" || got.Children["repos/api"].Visibility != gungnir.VisibilityPrivate {
		t.Errorf("round trip lost the pins: %+v", got)
	}
}
