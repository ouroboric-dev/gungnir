package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FeaturesSchema names the features.json format.
const FeaturesSchema = "ygg.features/v1"

// TaskStatus is where a task stands in a run.
type TaskStatus string

const (
	TaskTodo    TaskStatus = "todo"
	TaskDoing   TaskStatus = "doing"
	TaskDone    TaskStatus = "done"
	TaskBlocked TaskStatus = "blocked" // an unattended run gave it up, or a task it depends on (spec §7.3)
)

// FeatureRecord is a task with its status.
type FeatureRecord struct {
	Feature
	Status TaskStatus `json:"status"`
	Why    string     `json:"why,omitempty"` // a blocked task: what stopped it
}

// Features is features.json: the plan's tasks and where each stands, derived from tasks.md by Odin, who is
// its only writer (spec §3.3 rule 2). An agent is told its task in its prompt; it never reads this file.
type Features struct {
	Schema       string          `json:"schema"`
	Run          string          `json:"run"`           // the run that last wrote it
	Source       string          `json:"source"`        // the tasks.md it came from, relative to the project
	SourceSHA256 string          `json:"source_sha256"` // that file as Odin read it
	Tasks        []FeatureRecord `json:"tasks"`         // in the order they are built
}

func (s TaskStatus) known() bool {
	return s == TaskTodo || s == TaskDoing || s == TaskDone || s == TaskBlocked
}

// Save writes the file atomically: a temporary file in the same folder, synced, then renamed.
func (f *Features) Save(path string) error {
	f.Schema = FeaturesSchema
	for _, t := range f.Tasks {
		if !t.Status.known() {
			return fmt.Errorf("task %s has the status %q; want todo, doing, done or blocked", t.ID, t.Status)
		}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// LoadFeatures reads features.json, and refuses unknown fields, another schema and a status nobody knows.
func LoadFeatures(path string) (*Features, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f Features
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Schema != FeaturesSchema {
		return nil, fmt.Errorf("%s: schema %q, want %q", path, f.Schema, FeaturesSchema)
	}
	for _, t := range f.Tasks {
		if !t.Status.known() {
			return nil, fmt.Errorf("%s: task %s has the status %q", path, t.ID, t.Status)
		}
	}
	return &f, nil
}
