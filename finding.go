package gungnir

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// The most characters each text field of a finding may hold. A finding goes back into a prompt on every
// attempt, so a reviewer's finding can't be as long as a book.
const (
	MaxID       = 100
	MaxSource   = 100
	MaxLocation = 300
	MaxClaim    = 4000
	MaxProof    = 2000
)

// Severity is how bad a finding is.
type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityMajor   Severity = "major"
	SeverityMinor   Severity = "minor"
)

// FindingStatus is where a finding stands.
type FindingStatus string

const (
	FindingOpen     FindingStatus = "open"
	FindingFixed    FindingStatus = "fixed"
	FindingDisputed FindingStatus = "disputed"
	// FindingOverruled closes a finding because the implementer's dispute of it held: a named check passed
	// and printed the test. Like a fixed finding it needs proof, and it is not open.
	FindingOverruled FindingStatus = "overruled"
)

// Finding is what a check or a reviewer reports (spec §4.6).
type Finding struct {
	ID       string        `json:"id"`       // stable across fix attempts
	Source   string        `json:"source"`   // the rung or reviewer that raised it
	Severity Severity      `json:"severity"` // blocker, major or minor
	Location string        `json:"location"` // file:line or file:line-line
	Claim    string        `json:"claim"`    // what is wrong
	Status   FindingStatus `json:"status"`   // open, fixed, disputed or overruled
	Proof    string        `json:"proof,omitempty"`
}

var location = regexp.MustCompile(`^[^\s:]+:[0-9]+(-[0-9]+)?$`)

// Validate refuses a finding with missing or malformed fields. A malformed finding is an agent
// failure, never a pass (spec §4.6).
func (f Finding) Validate() error {
	var errs []error
	if f.ID == "" || f.Source == "" || f.Claim == "" {
		errs = append(errs, errors.New("a finding needs an id, a source and a claim"))
	}
	for _, c := range []struct {
		name string
		text string
		max  int
	}{{"id", f.ID, MaxID}, {"source", f.Source, MaxSource}, {"location", f.Location, MaxLocation}, {"claim", f.Claim, MaxClaim}, {"proof", f.Proof, MaxProof}} {
		if n := utf8.RuneCountInString(c.text); n > c.max {
			errs = append(errs, fmt.Errorf("the %s is %d characters long, over the limit of %d", c.name, n, c.max))
		}
	}
	switch f.Severity {
	case SeverityBlocker, SeverityMajor, SeverityMinor:
	default:
		errs = append(errs, fmt.Errorf("severity %q is not blocker, major or minor", f.Severity))
	}
	if !location.MatchString(f.Location) {
		errs = append(errs, fmt.Errorf("location %q is not file:line or file:line-line", f.Location))
	}
	switch f.Status {
	case FindingOpen, FindingDisputed:
	case FindingFixed, FindingOverruled:
		if f.Proof == "" {
			errs = append(errs, fmt.Errorf("a %s finding needs proof", f.Status))
		}
	default:
		errs = append(errs, fmt.Errorf("status %q is not open, fixed, disputed or overruled", f.Status))
	}
	return errors.Join(errs...)
}

// Report is a reviewer's whole answer: every finding it holds open or closes (spec §4.6).
type Report struct {
	Findings []Finding `json:"findings"`
}

// The most characters each text field of a dispute may hold, and the most disputes one answer may hold.
// The evidence goes into the overruled finding's proof, with the check run, so it is shorter than MaxProof.
const (
	MaxDisputeCheck = 100
	MaxDisputeTest  = 300
	MaxEvidence     = 1500
	MaxDisputes     = 200
)

// Dispute is an implementer's claim that an open finding is wrong, settled by a check of the loop: the
// check named must have passed in the attempt, with the test id in its output (change disputed-findings).
type Dispute struct {
	ID       string `json:"id"`       // the open finding's id
	Check    string `json:"check"`    // the name of a check the loop runs
	Test     string `json:"test"`     // a test id the check's output must name
	Evidence string `json:"evidence"` // what the test showed, and where
}

// Disputes is an implementer's whole dispute document.
type Disputes struct {
	Disputes []Dispute `json:"disputes"`
}

// Validate refuses a dispute with a missing or over-long field.
func (d Dispute) Validate() error {
	var errs []error
	for _, c := range []struct {
		name string
		text string
		max  int
	}{{"id", d.ID, MaxID}, {"check", d.Check, MaxDisputeCheck}, {"test", d.Test, MaxDisputeTest}, {"evidence", d.Evidence, MaxEvidence}} {
		if c.text == "" {
			errs = append(errs, fmt.Errorf("a dispute needs a %s", c.name))
		} else if n := utf8.RuneCountInString(c.text); n > c.max {
			errs = append(errs, fmt.Errorf("the %s is %d characters long, over the limit of %d", c.name, n, c.max))
		}
	}
	return errors.Join(errs...)
}
