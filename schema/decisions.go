package schema

import (
	"errors"
	"fmt"
)

// Decisions is what a person gives at a gate (ygg.decisions/v1): decisions under ids they choose, answers to the
// design's open questions, and for a rejection the reason. It is data: the text may span lines, and a stage's prompt
// renders it safely. It is an operator's file, not an agent's output, so it has no blocked and no notes.
type Decisions struct {
	Decisions []Decision `json:"decisions,omitzero"`
	Answers   []Answer   `json:"answers,omitzero"`
	Reason    string     `json:"reason,omitempty"` // a rejection needs it; Odin knows the action and checks that
}

// Decision is one owner decision. The id is the operator's, stable across runs.
type Decision struct {
	ID   string `json:"id"`
	Text string `json:"text"` // up to 4 KiB
}

// Answer answers one open question of this run's design, by its id (q1, q2, ...).
type Answer struct {
	Question string `json:"question"`
	Text     string `json:"text"` // up to 4 KiB
}

// Check refuses a decision id used twice, and a file that says nothing: no decision, no answer and no reason.
// The JSON Schema cannot say either.
func (d *Decisions) Check() error {
	seen := map[string]bool{}
	for _, x := range d.Decisions {
		if seen[x.ID] {
			return fmt.Errorf("the decision id %s is used twice", x.ID)
		}
		seen[x.ID] = true
	}
	if len(d.Decisions) == 0 && len(d.Answers) == 0 && d.Reason == "" {
		return errors.New("the file holds no decision, no answer and no reason")
	}
	return nil
}

// Judgement is the operator's verdict on a run, or with Task set on one of its tasks (ygg.judgement/v1). Odin
// writes it from `ygg score`; a new one for the same run or task replaces the old.
type Judgement struct {
	Run   string `json:"run"`
	Task  string `json:"task,omitempty"`
	Score int    `json:"score"` // 0 to 10
	Well  string `json:"well"`
	Wrong string `json:"wrong"`
	Time  string `json:"time"` // RFC 3339
}
