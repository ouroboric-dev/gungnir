package schema

import (
	"fmt"

	"github.com/ouroboric-dev/gungnir"
)

// Review is a reviewer's answer (ygg.review/v1): every finding it holds open or closes. An empty list says it
// found nothing. With Blocked set, the findings may be left out.
type Review struct {
	Findings []gungnir.Finding `json:"findings,omitzero"`
	Blocked  *gungnir.Blocked  `json:"blocked,omitempty"`
	Notes    []string          `json:"notes,omitzero"`
}

// Check runs the finding's own rules on each finding and refuses an id used twice, which the JSON Schema cannot say.
func (r *Review) Check() error {
	seen := map[string]bool{}
	for _, f := range r.Findings {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("finding %q: %w", f.ID, err)
		}
		if seen[f.ID] {
			return fmt.Errorf("the finding id %s is used twice", f.ID)
		}
		seen[f.ID] = true
	}
	return nil
}

// Implement is the implementer's answer (ygg.implement/v1): what it did, and the findings it disputes. A blocked
// answer ends the attempt. With Blocked set, the summary may be left out.
type Implement struct {
	Summary  string            `json:"summary,omitempty"` // up to 16 KiB
	Disputes []gungnir.Dispute `json:"disputes,omitzero"`
	Blocked  *gungnir.Blocked  `json:"blocked,omitempty"`
	Notes    []string          `json:"notes,omitzero"`
}

// Check runs the dispute's own rules on each dispute and refuses a finding disputed twice.
func (i *Implement) Check() error {
	seen := map[string]bool{}
	for _, d := range i.Disputes {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("dispute %q: %w", d.ID, err)
		}
		if seen[d.ID] {
			return fmt.Errorf("the finding id %s is used twice in the disputes: a finding is disputed once", d.ID)
		}
		seen[d.ID] = true
	}
	return nil
}
