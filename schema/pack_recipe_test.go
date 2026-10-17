package schema

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

const pgImage = "docker.io/library/postgres:17.2@sha256:1111111111111111111111111111111111111111111111111111111111111111"

// recipeRealm writes a realm folder with a template, an implementer prompt and the given pack.yaml.
func recipeRealm(t *testing.T, pack string) string {
	t.Helper()
	dir := t.TempDir()
	for f, body := range map[string]string{
		"templates/api/LICENSE": "licence\n", "templates/api/AGENTS.md": "rules\n", "prompts/implementer.md": "Build the task.\n", "pack.yaml": pack,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const recipe = `name: helheim-go
version: 0.1.0
checks:
  lint: [go, vet, ./...]
  unit: [go, test, ./...]
leaf:
  description: the api
  template: templates/api
  first_commit: [LICENSE]
  implementer: prompts/implementer.md
  checks: [lint, unit]
services:
  postgres:
    image: ` + pgImage + `
    env: { POSTGRES_DB: app }
    ports: [5432]
`

func TestARealmWithoutAGraphOffersARecipeAndServices(t *testing.T) {
	p, err := LoadPack(recipeRealm(t, recipe))
	if err != nil {
		t.Fatalf("LoadPack() = %v", err)
	}
	if p.Graph != "" {
		t.Errorf("Graph = %q; a realm that doesn't lead has none", p.Graph)
	}
	r := p.Leaf
	if r == nil || r.Template != "templates/api" || r.Implementer != "prompts/implementer.md" || !slices.Equal(r.Checks, []string{"lint", "unit"}) {
		t.Fatalf("Leaf = %+v", r)
	}
	want := []gungnir.Service{{Name: "postgres", Image: pgImage, Env: map[string]string{"POSTGRES_DB": "app"}, Ports: []int{5432}}}
	if got := p.ServiceList(); len(got) != 1 || got[0].Name != want[0].Name || got[0].Image != pgImage || got[0].Ports[0] != 5432 || got[0].Env["POSTGRES_DB"] != "app" {
		t.Errorf("ServiceList() = %+v", got)
	}
	if p, err := LoadPack(recipeRealm(t, "name: notes\nversion: 0.1.0\n")); err != nil || p.Leaf != nil || len(p.ServiceList()) != 0 {
		t.Errorf("a realm with no leaf recipe, no graph and no stack (a documents realm) = %+v, %v", p, err)
	}
}

func TestLoadPackRefusesABadRecipe(t *testing.T) {
	swap := func(old, new string) string { return strings.Replace(recipe, old, new, 1) }
	cases := map[string]string{
		"a template outside the realm":      swap("template: templates/api", "template: ../api"),
		"a template that is not there":      swap("template: templates/api", "template: templates/web"),
		"a first file that is not there":    swap("first_commit: [LICENSE]", "first_commit: [NOTICE]"),
		"no implementer prompt":             swap("  implementer: prompts/implementer.md\n", ""),
		"an implementer outside the realm":  swap("prompts/implementer.md", "../implementer.md"),
		"an implementer that is not there":  swap("implementer: prompts/implementer.md", "implementer: prompts/other.md"),
		"no checks":                         swap("  checks: [lint, unit]\n", ""),
		"a check the pack doesn't define":   swap("checks: [lint, unit]", "checks: [lint, e2e]"),
		"a check twice":                     swap("checks: [lint, unit]", "checks: [lint, lint]"),
		"a service image without a digest":  swap(pgImage, "docker.io/library/postgres:17.2"),
		"a service with no port":            swap("    ports: [5432]\n", ""),
		"a service named work":              swap("  postgres:", "  work:"),
		"a service env name with an equals": swap("{ POSTGRES_DB: app }", "{ A=B: app }"),
		"a recipe field nobody knows":       swap("  description: the api\n", "  description: the api\n  hooks: [x]\n"),
	}
	for name, y := range cases {
		if _, err := LoadPack(recipeRealm(t, y)); err == nil {
			t.Errorf("%s: LoadPack() = nil error", name)
		} else if strings.Contains(err.Error(), "yaml:") && !strings.Contains(err.Error(), "pack.yaml") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
