package schema

import "fmt"

// CheckResult is what one check of the ladder wrote (ygg.check/v1). It is not an agent's output: factory code
// writes it from the tool's machine output. Passed is true only when Failures is empty. PassedTests lists the
// tests the tool saw pass, where it reports them; a test in neither list may not have run.
type CheckResult struct {
	Check       string         `json:"check"`
	Passed      bool           `json:"passed"`
	ExitCode    int            `json:"exit_code"`
	TimedOut    bool           `json:"timed_out"`
	DurationMS  int64          `json:"duration_ms"`
	Counts      *Counts        `json:"counts,omitempty"` // where the tool has them
	Failures    []CheckFailure `json:"failures,omitzero"`
	PassedTests []string       `json:"passed_tests,omitzero"` // the ids of the tests that ran and passed, as failures[].test has them
	Notes       []string       `json:"notes,omitzero"`
}

// Verify refuses a test listed both as passed and as failed. Decode calls it as it calls Check on other types;
// the field named Check leaves no room for a method of that name.
func (r *CheckResult) Verify() error {
	passed := make(map[string]bool, len(r.PassedTests))
	for _, id := range r.PassedTests {
		passed[id] = true
	}
	for _, f := range r.Failures {
		if f.Test != "" && passed[f.Test] {
			return fmt.Errorf("test %s is listed as passed and as failed", f.Test)
		}
	}
	return nil
}

// Counts are the tool's own totals.
type Counts struct {
	Run     int `json:"run"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// CheckFailure is one thing a check found wrong. Kind is one of test, lint, type, build, scan, policy or other:
// a fact about the tool, not a judgement.
type CheckFailure struct {
	Kind     string `json:"kind"`
	Test     string `json:"test,omitempty"`     // the test's id, where there is one
	Location string `json:"location,omitempty"` // file:line or file:line:column, where the tool gives one
	Message  string `json:"message"`
}
