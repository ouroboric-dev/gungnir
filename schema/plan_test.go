package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planFixture reads the valid two-task plan, whose first task has run 1's T002 shape.
func planFixture(t *testing.T) Plan {
	t.Helper()
	data, err := os.ReadFile("testdata/contracts/plan-ok-two-tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Decode[Plan](ContractPlan, data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestThePlanCarriesEachTasksTextAndContract(t *testing.T) {
	p := planFixture(t)
	if len(p.Tasks[0].Text) != 1500 || !strings.Contains(p.Tasks[0].Contract, "/api/auth/signup") {
		t.Fatalf("the fixture lost its shape: text %d bytes, contract %q", len(p.Tasks[0].Text), p.Tasks[0].Contract)
	}
	fs := FeaturesFromPlan(p)
	if len(fs) != 2 {
		t.Fatalf("FeaturesFromPlan() holds %d features, want 2", len(fs))
	}
	for i, f := range fs {
		pt := p.Tasks[i]
		if f.ID != pt.ID || f.Title != pt.Title || f.Test != pt.NamedTest || f.Layer != pt.Layer || f.Size != pt.Size || f.Line != 0 ||
			f.Text != pt.Text || f.Contract != pt.Contract || strings.Join(f.Criteria, "|") != strings.Join(pt.Criteria, "|") ||
			strings.Join(f.DependsOn, "|") != strings.Join(pt.DependsOn, "|") {
			t.Errorf("feature %d = %+v, from %+v", i, f, pt)
		}
	}
	if err := CheckReady(fs, nil); err != nil {
		t.Errorf("the plan's features are not ready: %v", err)
	}

	path := filepath.Join(t.TempDir(), "features.json")
	file := Features{Run: "r", Source: "plan.json", SourceSHA256: strings.Repeat("a", 64)}
	for _, f := range fs {
		file.Tasks = append(file.Tasks, FeatureRecord{Feature: f, Status: TaskTodo})
	}
	if err := file.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFeatures(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range got.Tasks {
		if r.Text != p.Tasks[i].Text || r.Contract != p.Tasks[i].Contract {
			t.Errorf("features.json changed the text or contract of %s", r.ID)
		}
	}
	data, _ := os.ReadFile(path)
	var raw struct{ Tasks []map[string]any }
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw.Tasks[1]["contract"]; has {
		t.Error("a task with no contract writes an empty contract")
	}
}

func TestReadinessBoundsTheTextAndTheContract(t *testing.T) {
	openapi := "openapi: 3.0.3\npaths: {}\n"
	cases := []struct {
		name string
		edit func(*Feature)
		want string
	}{
		{"text", func(f *Feature) { f.Text = strings.Repeat("x", MaxTaskText+1) }, "its text is 65537 bytes, over 65536: split the task or cut its text"},
		{"contract", func(f *Feature) { f.Contract = openapi + strings.Repeat("#", MaxTaskContract+1-len(openapi)) },
			"its contract is 65537 bytes, over 65536: split the task or cut its contract"},
		{"no openapi", func(f *Feature) { f.Contract = "paths: {}" }, "its contract is not an OpenAPI document:"},
		{"no paths", func(f *Feature) { f.Contract = "openapi: 3.0.3" }, "its contract is not an OpenAPI document:"},
		{"not yaml", func(f *Feature) { f.Contract = "openapi: [" }, "its contract is not an OpenAPI document:"},
		{"size L", func(f *Feature) { f.Size = SizeL }, "size L: split it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := ready("p-T1", "repos/api")
			f.Text = "words"
			c.edit(&f)
			err := CheckReady([]Feature{f}, nil)
			re, ok := err.(*ReadinessError)
			if !ok || len(re.Problems) != 1 || re.Problems[0].Task != "p-T1" || !strings.Contains(re.Problems[0].Text, c.want) {
				t.Fatalf("CheckReady() = %v, want one problem for p-T1 holding %q", err, c.want)
			}
		})
	}
	f := ready("p-T1", "repos/api")
	f.Contract = openapi
	if err := CheckReady([]Feature{f}, nil); err != nil {
		t.Errorf("an OpenAPI contract is refused: %v", err)
	}
}

func TestAFeaturesFileFromBeforeStillLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "features.json")
	old := `{"schema":"ygg.features/v1","run":"r","source":"tasks.md","source_sha256":"` + strings.Repeat("a", 64) + `","tasks":[
{"id":"p-T1","title":"t","criteria":["c"],"test":"TestIt","layer":"repos/api","size":"S","depends_on":[],"line":3,"status":"todo"}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFeatures(path)
	if err != nil {
		t.Fatalf("LoadFeatures() = %v", err)
	}
	if got.Tasks[0].Text != "" || got.Tasks[0].Contract != "" {
		t.Errorf("an old task has text %q, contract %q", got.Tasks[0].Text, got.Tasks[0].Contract)
	}
}

func TestAPlanTaskWithNoTextIsRefused(t *testing.T) {
	data, _ := os.ReadFile("testdata/contracts/plan-ok-two-tasks.json")
	var doc map[string]any
	_ = json.Unmarshal(data, &doc)
	delete(doc["tasks"].([]any)[1].(map[string]any), "text")
	bad, _ := json.Marshal(doc)
	if _, err := Decode[Plan](ContractPlan, bad); err == nil || !strings.Contains(err.Error(), "/tasks/1") || !strings.Contains(err.Error(), "text") {
		t.Errorf("Decode() = %v, want a refusal at /tasks/1 naming text", err)
	}
}
