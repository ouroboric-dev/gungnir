package schema

import "testing"

func TestAGraphPackHoldsNoRecipe(t *testing.T) {
	base := "name: software\nversion: 0.1.0\ngraph: graph.yaml\n"
	routing := "routing:\n  threshold: 0.8\n"
	load := func(y string) *Pack {
		t.Helper()
		p, err := LoadPack(recipeRealm(t, y))
		if err != nil {
			t.Fatalf("LoadPack(%q) = %v", y, err)
		}
		return p
	}
	if load(base).HoldsRecipe() || load(base+routing).HoldsRecipe() {
		t.Error("a pack of a graph (and routing) holds a recipe")
	}
	for name, add := range map[string]string{
		"checks":   "checks:\n  unit: [go, test]\n",
		"leaf":     "checks:\n  unit: [go, test]\nleaf:\n  description: x\n  template: templates/api\n  first_commit: [LICENSE]\n  implementer: prompts/implementer.md\n  checks: [unit]\n",
		"leaves":   "leaves:\n  repos/api: { description: x, template: templates/api, first_commit: [LICENSE] }\n",
		"sandbox":  "sandbox:\n  stack: " + pgImage + "\n",
		"services": "services:\n  db: { image: " + pgImage + ", env: {}, ports: [5432] }\n",
	} {
		if !load(base + add).HoldsRecipe() {
			t.Errorf("a pack with %s does not hold a recipe", name)
		}
	}
}
