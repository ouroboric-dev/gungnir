package schema

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

const company = `
root: true
owner: ouroboric-dev
visibility: private
children:
  projects/gallery: { kind: umbrella, owner: ouroboric-dev, visibility: private }
  projects/site: { kind: umbrella, owner: ouroboric-dev, visibility: public }
`

func TestParseUmbrella(t *testing.T) {
	u, err := ParseUmbrella("umbrella.yaml", []byte(company))
	if err != nil {
		t.Fatalf("ParseUmbrella() = %v", err)
	}
	if !u.Root || u.Owner != "ouroboric-dev" || u.Visibility != gungnir.VisibilityPrivate {
		t.Errorf("got %+v", u)
	}
	if got := u.Paths(); !slices.Equal(got, []string{"projects/gallery", "projects/site"}) {
		t.Errorf("Paths() = %q", got)
	}
	if c := u.Children["projects/gallery"]; c.Kind != ChildUmbrella || c.Visibility != gungnir.VisibilityPrivate {
		t.Errorf("projects/gallery = %+v", c)
	}
	if empty, err := ParseUmbrella("umbrella.yaml", []byte("owner: o\nvisibility: private\n")); err != nil || empty.Children == nil {
		t.Errorf("an umbrella with no children = %+v, %v; want an empty, non-nil Children", empty, err)
	}
	if len(u.Leaks()) != 0 {
		t.Errorf("a private umbrella leaks nothing, but Leaks() = %q", u.Leaks())
	}
}

func TestUmbrellaLeaksPrivateChildrenOfAPublicUmbrella(t *testing.T) {
	u, err := ParseUmbrella("umbrella.yaml", []byte(strings.Replace(company, "visibility: private\nchildren", "visibility: public\nchildren", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Leaks(); !slices.Equal(got, []string{"projects/gallery"}) {
		t.Errorf("Leaks() = %q, want the private child", got)
	}
}

func TestParseUmbrellaRefuses(t *testing.T) {
	head := "owner: ouroboric-dev\nvisibility: private\nchildren:\n"
	cases := map[string]string{
		"no owner":                              "visibility: private\n",
		"an unknown visibility":                 "owner: ouroboric-dev\nvisibility: internal\n",
		"an unknown field":                      head + "  a: { kind: leaf, owner: o, visibility: private }\nsecrets: x\n",
		"an unknown kind":                       head + "  a: { kind: folder, owner: o, visibility: private }\n",
		"a child without owner":                 head + "  a: { kind: leaf, visibility: private }\n",
		"a child that climbs":                   head + "  ../a: { kind: leaf, owner: o, visibility: private }\n",
		"an absolute child":                     head + "  /srv/a: { kind: leaf, owner: o, visibility: private }\n",
		"a child inside a child":                head + "  a: { kind: umbrella, owner: o, visibility: private }\n  a/b: { kind: leaf, owner: o, visibility: private }\n",
		"a child inside a child past a sibling": head + "  x: { kind: umbrella, owner: o, visibility: private }\n  x-y: { kind: leaf, owner: o, visibility: private }\n  x/z: { kind: leaf, owner: o, visibility: private }\n",
		"a child in .git":                       head + "  .git/x: { kind: leaf, owner: o, visibility: private }\n",
		"a child in .factory":                   head + "  .factory/x: { kind: leaf, owner: o, visibility: private }\n",
		"the umbrella as a child":               head + "  .: { kind: leaf, owner: o, visibility: private }\n",
	}
	for name, y := range cases {
		if _, err := ParseUmbrella("umbrella.yaml", []byte(y)); err == nil {
			t.Errorf("%s: ParseUmbrella() = nil error", name)
		}
	}
}

func TestUmbrellaRoundTrips(t *testing.T) {
	dir := t.TempDir()
	u, err := ParseUmbrella("umbrella.yaml", []byte(company))
	if err != nil {
		t.Fatal(err)
	}
	u.Children["projects/blog"] = Child{Kind: ChildUmbrella, Owner: "ouroboric-dev", Visibility: gungnir.VisibilityPrivate}
	if err := u.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := LoadUmbrella(dir)
	if err != nil {
		t.Fatalf("LoadUmbrella() after Save = %v", err)
	}
	if !slices.Equal(got.Paths(), []string{"projects/blog", "projects/gallery", "projects/site"}) || !got.Root {
		t.Errorf("round trip = %+v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "umbrella.yaml")); !strings.HasPrefix(string(b), "# The instance tree") {
		t.Errorf("umbrella.yaml lacks its header:\n%s", b)
	}
}
