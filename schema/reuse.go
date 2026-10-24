package schema

import "github.com/ouroboric-dev/gungnir"

// ReuseReview is the reuse review's answer (ygg.reuse-review/v1): a verdict and the findings. Odin renders the
// review. With Blocked set, the content fields may be empty.
type ReuseReview struct {
	Verdict string           `json:"verdict,omitempty"` // one line, free text
	Items   []ReuseItem      `json:"items,omitzero"`
	Blocked *gungnir.Blocked `json:"blocked,omitempty"`
	Notes   []string         `json:"notes,omitzero"`
}

// ReuseItem is one finding: a task of the plan, what it adds, what already does the job, and the smallest
// change to the plan.
type ReuseItem struct {
	Task           string `json:"task"`
	Adds           string `json:"adds"`
	AlreadyDoneBy  string `json:"already_done_by"`
	SmallestChange string `json:"smallest_change"`
}
