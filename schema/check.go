package schema

// CheckResult is what one check of the ladder wrote (ygg.check/v1). It is not an agent's output: factory code
// writes it from the tool's machine output. Passed is true only when Failures is empty. Passing tests are not
// listed; a test missing from Failures passed.
type CheckResult struct {
	Check      string         `json:"check"`
	Passed     bool           `json:"passed"`
	ExitCode   int            `json:"exit_code"`
	TimedOut   bool           `json:"timed_out"`
	DurationMS int64          `json:"duration_ms"`
	Counts     *Counts        `json:"counts,omitempty"` // where the tool has them
	Failures   []CheckFailure `json:"failures,omitzero"`
	Notes      []string       `json:"notes,omitzero"`
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
