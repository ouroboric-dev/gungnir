package gungnir

import (
	"errors"
	"fmt"
	"regexp"
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
)

// Finding is what a check or a reviewer reports (spec §4.6).
type Finding struct {
	ID       string        `json:"id"`       // stable across fix attempts
	Source   string        `json:"source"`   // the rung or reviewer that raised it
	Severity Severity      `json:"severity"` // blocker, major or minor
	Location string        `json:"location"` // file:line or file:line-line
	Claim    string        `json:"claim"`    // what is wrong
	Status   FindingStatus `json:"status"`   // open, fixed or disputed
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
	case FindingFixed:
		if f.Proof == "" {
			errs = append(errs, errors.New("a fixed finding needs proof"))
		}
	default:
		errs = append(errs, fmt.Errorf("status %q is not open, fixed or disputed", f.Status))
	}
	return errors.Join(errs...)
}
