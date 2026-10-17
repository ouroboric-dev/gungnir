package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleFeatures() *Features {
	return &Features{Run: "20261017-101500-a1b2c3", Source: "docs/2-engineering/plans/p/tasks.md", SourceSHA256: strings.Repeat("a", 64),
		Tasks: []FeatureRecord{
			{Feature: ready("p-T1", "repos/api"), Status: TaskDone},
			{Feature: ready("p-T2", "repos/web", "p-T1"), Status: TaskBlocked, Why: "its loop ran out of attempts"},
		}}
}

func TestFeaturesRoundTripThroughTheFileOdinWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "features.json")
	if err := sampleFeatures().Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFeatures(path)
	if err != nil {
		t.Fatalf("LoadFeatures() = %v", err)
	}
	if got.Schema != FeaturesSchema || len(got.Tasks) != 2 || got.Tasks[1].Status != TaskBlocked || got.Tasks[1].DependsOn[0] != "p-T1" ||
		got.Tasks[1].Why == "" || got.Tasks[0].Criteria[0] != "it works" {
		t.Errorf("round trip = %+v", got)
	}
	if left, _ := filepath.Glob(path + ".tmp*"); len(left) != 0 {
		t.Errorf("Save left temporary files: %v", left)
	}
}

func TestLoadFeaturesRefusesWhatOdinDidNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "features.json")
	if err := sampleFeatures().Save(path); err != nil {
		t.Fatal(err)
	}
	good, _ := os.ReadFile(path)
	cases := map[string]string{
		"another schema":    strings.Replace(string(good), FeaturesSchema, "ygg.features/v9", 1),
		"an unknown field":  strings.Replace(string(good), `"run"`, `"owner": "x", "run"`, 1),
		"an unknown status": strings.Replace(string(good), `"done"`, `"finished"`, 1),
		"not JSON":          "{",
	}
	for name, body := range cases {
		os.WriteFile(path, []byte(body), 0o644)
		if _, err := LoadFeatures(path); err == nil {
			t.Errorf("%s: LoadFeatures() = nil error", name)
		}
	}
}

func TestSavingFeaturesWithAnUnknownStatusIsRefused(t *testing.T) {
	f := sampleFeatures()
	f.Tasks[0].Status = "finished"
	if err := f.Save(filepath.Join(t.TempDir(), "features.json")); err == nil {
		t.Error("Save() = nil for a status nobody knows")
	}
}
