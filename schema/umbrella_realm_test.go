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
graph: software
graph_version: 0.1.0
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

func TestAProjectRecordsItsGraphAndEachLeafsRealm(t *testing.T) {
	u, err := ParseUmbrella("umbrella.yaml", []byte(gallery))
	if err != nil {
		t.Fatalf("ParseUmbrella() = %v", err)
	}
	if u.Graph != "software" || u.GraphVersion != "0.1.0" {
		t.Errorf("graph = %s %s", u.Graph, u.GraphVersion)
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
	want := []Pin{{"helheim-go", "0.1.0", "repos/api"}, {"midgard-react", "0.1.0", "repos/web"}}
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
		"a graph with no version":         "graph: software\n" + head + leaf(""),
		"a graph version with no graph":   "graph_version: 0.1.0\n" + head + leaf(""),
		"a graph name in capitals":        "graph: Software\ngraph_version: 0.1.0\n" + head + leaf(""),
		"a graph version that isn't one":  "graph: software\ngraph_version: 1\n" + head + leaf(""),
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
	if got.Graph != "software" || got.Children["repos/web"].Consumes[0] != "repos/api" || got.Children["repos/api"].Visibility != gungnir.VisibilityPrivate {
		t.Errorf("round trip lost the pins: %+v", got)
	}
}

func TestTheUmbrellaPinsItsGraphAndEachLeafItsRealm(t *testing.T) {
	head := "owner: o\nvisibility: private\n"
	leaves := "children:\n  a: { kind: leaf, owner: o, visibility: private, realm: helheim-go, realm_version: 0.1.0 }\n  b: { kind: leaf, owner: o, visibility: private, realm: midgard-react, realm_version: 0.2.0 }\n"
	u, err := ParseUmbrella("umbrella.yaml", []byte("graph: software\ngraph_version: 0.1.0\n"+head+leaves))
	if err != nil || u.Graph != "software" || u.GraphVersion != "0.1.0" {
		t.Fatalf("ParseUmbrella() = %+v, %v", u, err)
	}
	if want := []Pin{{"helheim-go", "0.1.0", "a"}, {"midgard-react", "0.2.0", "b"}}; !slices.Equal(u.Pins(), want) {
		t.Errorf("Pins() = %+v, want the two leaves %+v", u.Pins(), want)
	}
	for name, y := range map[string]string{
		"a graph alone":         "graph: software\n",
		"a graph version alone": "graph_version: 0.1.0\n",
		"a graph in capitals":   "graph: Software\ngraph_version: 0.1.0\n",
		"a bad graph version":   "graph: software\ngraph_version: 1\n",
	} {
		_, err := ParseUmbrella("umbrella.yaml", []byte(y+head+leaves))
		if err == nil {
			t.Errorf("%s: ParseUmbrella() = nil error", name)
		} else if name == "a graph alone" || name == "a graph version alone" {
			if !strings.Contains(err.Error(), "a pin is both") {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

func TestAnUmbrellaWithATopLevelRealmIsRefusedWithTheWayToPinAGraph(t *testing.T) {
	head := "owner: o\nvisibility: private\n"
	leaf := "children:\n  a: { kind: leaf, owner: o, visibility: private, realm: midgard-react, realm_version: 0.1.0 }\n"
	for name, top := range map[string]string{
		"both":            "realm: midgard-react\nrealm_version: 0.1.0\n",
		"a realm":         "realm: midgard-react\n",
		"a realm version": "realm_version: 0.1.0\n",
	} {
		_, err := ParseUmbrella("umbrella.yaml", []byte(top+head+leaf))
		if err == nil {
			t.Errorf("%s: ParseUmbrella() = nil error", name)
			continue
		}
		for _, want := range []string{"graph and graph_version", "each leaf keeps its realm"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: %v does not contain %q", name, err, want)
			}
		}
	}
	if _, err := ParseUmbrella("umbrella.yaml", []byte(head+leaf)); err != nil {
		t.Errorf("a leaf's realm = %v, want it to load", err)
	}
}
