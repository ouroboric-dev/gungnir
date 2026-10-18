package gungnir

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"time"
)

// The schema names of the two files grading reads and writes (spec §11, change benchmark-split).
const (
	ScoreSchema = "ygg.score/v1" // <run>/score.json, which Odin writes
	SuiteSchema = "ygg.suite/v1" // tests/manifest.json, which the hidden suite holds
)

// GradeReason is why grading was refused before anything was built (the grade_refused event).
type GradeReason string

const (
	GradeRetired      GradeReason = "retired"       // the suite version of the task is retired
	GradeMissingSuite GradeReason = "missing_suite" // the store holds no suite for the task and its suite version
	GradeBriefChanged GradeReason = "brief_changed" // instruction.md no longer hashes to metadata.brief_sha256
	GradeBadManifest  GradeReason = "bad_manifest"  // the suite's manifest and its results do not agree
	GradeBadTask      GradeReason = "bad_task"      // task.toml is missing a field or holds a wrong one
)

// ScenarioKind says what a scenario of a suite proves.
type ScenarioKind string

const (
	FailToPass ScenarioKind = "fail-to-pass" // the product must make it pass; the reward counts these only
	PassToPass ScenarioKind = "pass-to-pass" // it passes on the empty product too; reported, never in the reward
)

// ScenarioStatus is what happened to a scenario in a grading run.
type ScenarioStatus string

const (
	ScenarioPassed  ScenarioStatus = "passed"
	ScenarioFailed  ScenarioStatus = "failed"
	ScenarioSkipped ScenarioStatus = "skipped"
)

var (
	categoryName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	suiteName    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	sha256Hex    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// MaxScenarioID is the most characters a scenario id (a Playwright spec title) may have.
const MaxScenarioID = 300

// Scenario is one line of a suite's manifest. Its id is the title of one Playwright spec.
type Scenario struct {
	ID       string       `json:"id"`
	Category string       `json:"category"`
	Kind     ScenarioKind `json:"kind"`
}

func (s Scenario) validate() error {
	var errs []error
	if s.ID == "" || len(s.ID) > MaxScenarioID {
		errs = append(errs, fmt.Errorf("scenario id %q must have 1 to %d characters", s.ID, MaxScenarioID))
	}
	if !categoryName.MatchString(s.Category) {
		errs = append(errs, fmt.Errorf("scenario %q: category %q must be lower case words joined by dashes", s.ID, s.Category))
	}
	if s.Kind != FailToPass && s.Kind != PassToPass {
		errs = append(errs, fmt.Errorf("scenario %q: kind %q is not fail-to-pass or pass-to-pass", s.ID, s.Kind))
	}
	return errors.Join(errs...)
}

// Suite is tests/manifest.json of a hidden suite: every scenario it holds, by id, category and kind.
type Suite struct {
	Schema    string     `json:"schema"`
	Suite     string     `json:"suite"`
	Scenarios []Scenario `json:"scenarios"`
}

// Validate refuses a manifest without scenarios, with a scenario id used twice or with a malformed
// field. A manifest that can't be read exactly is an error, never a pass (spec §4.6).
func (s Suite) Validate() error {
	var errs []error
	if s.Schema != SuiteSchema {
		errs = append(errs, fmt.Errorf("schema %q is not %s", s.Schema, SuiteSchema))
	}
	if !suiteName.MatchString(s.Suite) {
		errs = append(errs, fmt.Errorf("suite %q is not a suite version", s.Suite))
	}
	if len(s.Scenarios) == 0 {
		errs = append(errs, errors.New("a suite needs at least one scenario"))
	}
	seen := map[string]bool{}
	for _, sc := range s.Scenarios {
		if err := sc.validate(); err != nil {
			errs = append(errs, err)
		}
		if seen[sc.ID] {
			errs = append(errs, fmt.Errorf("scenario id %q is used twice", sc.ID))
		}
		seen[sc.ID] = true
	}
	return errors.Join(errs...)
}

// ScoredScenario is a scenario with its result.
type ScoredScenario struct {
	ID       string         `json:"id"`
	Category string         `json:"category"`
	Kind     ScenarioKind   `json:"kind"`
	Status   ScenarioStatus `json:"status"`
}

// Tally counts passed scenarios out of a total.
type Tally struct {
	Passed int `json:"passed"`
	Total  int `json:"total"`
}

// CategoryScore is one category of the reward: its fail-to-pass scenarios and the weight of the category.
type CategoryScore struct {
	Passed int     `json:"passed"`
	Total  int     `json:"total"`
	Weight float64 `json:"weight"`
}

// Score is score.json (spec §11): the result of grading one run. Odin writes it to the run folder and
// the ledger only: never to a leaf, and never where an agent can read it.
type Score struct {
	Schema      string                   `json:"schema"`
	Run         string                   `json:"run"`
	Task        string                   `json:"task"`
	TaskVersion string                   `json:"task_version"`
	BriefSHA256 string                   `json:"brief_sha256"`
	Suite       string                   `json:"suite"`
	Started     time.Time                `json:"started"`
	Finished    time.Time                `json:"finished"`
	Scenarios   []ScoredScenario         `json:"scenarios"`
	FailToPass  Tally                    `json:"fail_to_pass"`
	PassToPass  Tally                    `json:"pass_to_pass"`
	Categories  map[string]CategoryScore `json:"categories"`
	Reward      float64                  `json:"reward"`
}

// weightEpsilon is how far the weights may be from a sum of 1, and a reward from its recomputed value.
const weightEpsilon = 1e-9

// RewardOf returns the sum, over the categories, of weight * passed / total. A category with no
// scenarios counts 0.
func RewardOf(cats map[string]CategoryScore) float64 {
	var r float64
	for _, c := range cats {
		if c.Total > 0 {
			r += c.Weight * float64(c.Passed) / float64(c.Total)
		}
	}
	return r
}

// Validate refuses a score that does not add up: a field that is malformed, a tally or a category that
// does not match the scenarios, weights that do not sum to 1, or a reward that is not the one the
// categories give. Only fail-to-pass scenarios count in the categories and the reward; pass-to-pass is
// reported on its own.
func (s Score) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if s.Schema != ScoreSchema {
		add("schema %q is not %s", s.Schema, ScoreSchema)
	}
	if s.Run == "" || s.Task == "" || s.Suite == "" {
		add("a score needs a run, a task and a suite")
	}
	if !ValidVersion(s.TaskVersion) {
		add("task_version %q is not of the form 1.2.3", s.TaskVersion)
	}
	if !sha256Hex.MatchString(s.BriefSHA256) {
		add("brief_sha256 %q is not a SHA-256 in hex", s.BriefSHA256)
	}
	if s.Started.IsZero() || s.Finished.Before(s.Started) {
		add("finished must not be before started, and both are needed")
	}
	var f2p, p2p Tally
	cats := map[string]Tally{}
	seen := map[string]bool{}
	for _, sc := range s.Scenarios {
		if err := (Scenario{ID: sc.ID, Category: sc.Category, Kind: sc.Kind}).validate(); err != nil {
			errs = append(errs, err)
		}
		if seen[sc.ID] {
			add("scenario id %q is used twice", sc.ID)
		}
		seen[sc.ID] = true
		if !slices.Contains([]ScenarioStatus{ScenarioPassed, ScenarioFailed, ScenarioSkipped}, sc.Status) {
			add("scenario %q: status %q is not passed, failed or skipped", sc.ID, sc.Status)
		}
		t := &p2p
		if sc.Kind == FailToPass {
			t = &f2p
			c := cats[sc.Category]
			c.Total++
			if sc.Status == ScenarioPassed {
				c.Passed++
			}
			cats[sc.Category] = c
		}
		t.Total++
		if sc.Status == ScenarioPassed {
			t.Passed++
		}
	}
	if s.FailToPass != f2p {
		add("fail_to_pass is %d/%d, the scenarios give %d/%d", s.FailToPass.Passed, s.FailToPass.Total, f2p.Passed, f2p.Total)
	}
	if s.PassToPass != p2p {
		add("pass_to_pass is %d/%d, the scenarios give %d/%d", s.PassToPass.Passed, s.PassToPass.Total, p2p.Passed, p2p.Total)
	}
	var sum float64
	for name, c := range s.Categories {
		sum += c.Weight
		if c.Weight < 0 || c.Weight > 1 {
			add("category %q: weight %v is not between 0 and 1", name, c.Weight)
		}
		if got := cats[name]; (Tally{c.Passed, c.Total}) != got {
			add("category %q is %d/%d, its fail-to-pass scenarios give %d/%d", name, c.Passed, c.Total, got.Passed, got.Total)
		}
	}
	for name := range cats {
		if _, ok := s.Categories[name]; !ok {
			add("category %q has fail-to-pass scenarios and no entry in categories", name)
		}
	}
	if len(s.Categories) == 0 || math.Abs(sum-1) > weightEpsilon {
		add("the weights of the categories sum to %v, not 1", sum)
	}
	if want := RewardOf(s.Categories); math.Abs(s.Reward-want) > weightEpsilon {
		add("reward is %v, the categories give %v", s.Reward, want)
	}
	return errors.Join(errs...)
}
