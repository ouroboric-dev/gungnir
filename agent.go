// Package gungnir is the contract every Yggdrasil component implements (spec §2.2).
package gungnir

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Agent runs one agent session. The adapters live in Gleipnir.
type Agent interface {
	Name() string
	Run(ctx context.Context, t Task) (Result, error)
}

// Task is one agent session's input. Every session starts clean (spec §4.4).
type Task struct {
	Role          string        // the factory.yaml role, such as "planner"
	Node          string        // the graph node that runs the session
	Attempt       int           // 1 the first time the node runs; one more on every re-run
	Prompt        string        // the full prompt
	Workdir       string        // the only directory the session may change
	Model         string        // the role's model
	ReadOnly      bool          // read-only roles get read and search tools only
	MaxTurns      int           // 0: the adapter's own limit
	Timeout       time.Duration // 0: no limit beyond the context
	EnvAllowlist  []string      // the host variables the session may see
	TranscriptDir string        // where the adapter writes the session's transcript
}

// Status is how a session ended.
type Status string

const (
	StatusOK     Status = "ok"
	StatusFailed Status = "failed"
)

// Usage is what a session consumed. Tokens and turns are the primary numbers (spec §5).
type Usage struct {
	InTok         int64   `json:"in_tok"`
	OutTok        int64   `json:"out_tok"`
	CacheTok      int64   `json:"cache_tok"`
	Turns         int64   `json:"turns"`
	CostUSD       float64 `json:"cost_usd"`
	CostEstimated bool    `json:"cost_estimated"`
}

// Add returns the sum of u and v. The sum is estimated if either part is.
func (u Usage) Add(v Usage) Usage {
	return Usage{
		InTok:         u.InTok + v.InTok,
		OutTok:        u.OutTok + v.OutTok,
		CacheTok:      u.CacheTok + v.CacheTok,
		Turns:         u.Turns + v.Turns,
		CostUSD:       u.CostUSD + v.CostUSD,
		CostEstimated: u.CostEstimated || v.CostEstimated,
	}
}

// Result is how a session ended and what it cost.
type Result struct {
	Status         Status
	Summary        string
	Usage          Usage
	TranscriptPath string
}

// Validate refuses a result that could be misread as a pass. Missing or malformed data is never
// defaulted (spec §4.6).
func (r Result) Validate() error {
	if r.Status != StatusOK && r.Status != StatusFailed {
		return fmt.Errorf("result status %q is neither %q nor %q", r.Status, StatusOK, StatusFailed)
	}
	u := r.Usage
	if u.InTok < 0 || u.OutTok < 0 || u.CacheTok < 0 || u.Turns < 0 {
		return fmt.Errorf("result usage has a negative count: %+v", u)
	}
	if u.CostUSD < 0 || math.IsNaN(u.CostUSD) || math.IsInf(u.CostUSD, 0) {
		return fmt.Errorf("result cost %v is not a finite, non-negative number", u.CostUSD)
	}
	return nil
}
