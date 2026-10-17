package schema

import (
	"slices"
	"strings"
	"testing"
)

func TestAPackServiceCarriesItsCommandToTheSandbox(t *testing.T) {
	pack := strings.Replace(recipe, "    ports: [5432]\n", "    ports: [5432]\n    command: [postgres, -c, fsync=off]\n", 1)
	p, err := LoadPack(recipeRealm(t, pack))
	if err != nil {
		t.Fatal(err)
	}
	got := p.ServiceList()
	if len(got) != 1 || !slices.Equal(got[0].Command, []string{"postgres", "-c", "fsync=off"}) {
		t.Fatalf("ServiceList() = %+v, want the command carried", got)
	}
	for name, cmd := range map[string]string{
		"a command with a dollar": "[postgres, \"$HOME\"]",
		"an empty command":        "[]",
		"an empty argument":       "[postgres, \"\"]",
	} {
		bad := strings.Replace(recipe, "    ports: [5432]\n", "    ports: [5432]\n    command: "+cmd+"\n", 1)
		if _, err := LoadPack(recipeRealm(t, bad)); err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		}
	}
}
