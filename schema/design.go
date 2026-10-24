package schema

import (
	"fmt"

	"github.com/ouroboric-dev/gungnir"
)

// Design is the design stage's answer (ygg.design/v1): the sections of the design doc as fields. Odin renders
// the document. With Blocked set, the content fields may be empty.
type Design struct {
	Title              string           `json:"title,omitempty"`
	Slug               string           `json:"slug,omitempty"`
	Summary            string           `json:"summary,omitempty"`
	Goals              string           `json:"goals,omitempty"`
	NonGoals           string           `json:"non_goals,omitempty"`
	SuccessCriteria    string           `json:"success_criteria,omitempty"`
	Context            string           `json:"context,omitempty"`
	ProductAtUserLevel string           `json:"product_at_user_level,omitempty"`
	Decisions          []Decided        `json:"decisions,omitzero"`
	OpenQuestions      []Question       `json:"open_questions,omitzero"`
	Blocked            *gungnir.Blocked `json:"blocked,omitempty"`
	Notes              []string         `json:"notes,omitzero"`
}

// Decided is a "**Decided <date>:** <decision>. <reason>." line as fields. Cites names the owner decision ids
// the decision comes from.
type Decided struct {
	Date     string   `json:"date"`
	Decision string   `json:"decision"`
	Reason   string   `json:"reason"`
	Cites    []string `json:"cites,omitzero"`
}

// Question is an open question of the design: what a person should decide.
type Question struct {
	ID           string `json:"id"` // q1, q2, ...; unique in a design
	Question     string `json:"question"`
	WhoCanAnswer string `json:"who_can_answer"`
}

// Check refuses a design that repeats a question id, which the JSON Schema cannot say.
func (d *Design) Check() error {
	seen := map[string]bool{}
	for _, q := range d.OpenQuestions {
		if seen[q.ID] {
			return fmt.Errorf("the open question id %s is used twice", q.ID)
		}
		seen[q.ID] = true
	}
	return nil
}
