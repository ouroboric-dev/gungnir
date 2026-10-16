// Package gungnir is the contract every Yggdrasil component implements (spec §2.2).
package gungnir

import (
	"context"
	"errors"
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
	Sandbox       Runner        // runs the session's commands in the leaf's sandbox; nil when the adapter works on the host
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

// FailureKind says how a session failed, which decides what happens next (spec §12).
type FailureKind string

const (
	// FailureAgent is the default: a non-zero exit, a timeout, a turn limit or output that can't be
	// read. It counts as a loop attempt.
	FailureAgent FailureKind = "agent"
	// FailureTransient is an HTTP 429 or 5xx, a network error or a CLI that crashed at start. It is
	// retried with backoff, and is not a loop attempt.
	FailureTransient FailureKind = "transient"
	// FailureQuota is a provider quota or credit that is used up. The run pauses until it is back.
	FailureQuota FailureKind = "quota"
	// FailureInfra is the sandbox or the engine failing under a session. The run stops, with no retry loop.
	FailureInfra FailureKind = "infra"
)

// MaxOutput is the most bytes of a session's final answer a Result holds. A longer answer is an
// agent failure, never a truncated pass.
const MaxOutput = 256 << 10

// Result is how a session ended and what it cost.
type Result struct {
	Status         Status
	Summary        string
	Usage          Usage
	TranscriptPath string
	Provider       string      // who served the model, such as "anthropic" or "openrouter"; may be empty
	CLIVersion     string      // the agent tool's version; may be empty
	Failure        FailureKind // a failed result: how it failed; empty means FailureAgent
	Output         string      // the session's final answer, whole, up to MaxOutput bytes; a reviewer's findings come in it
}

// SessionError is a session that ended without a usable Result. Usage is what could be read of the
// session before it ended: a lower bound, so the caps count at least that much. A plain error from an
// adapter is an agent failure.
type SessionError struct {
	Kind  FailureKind
	Err   error
	Usage Usage
}

func (e *SessionError) Error() string { return e.Err.Error() }
func (e *SessionError) Unwrap() error { return e.Err }

// FailureOf returns how err failed a session: the kind of the first *SessionError in its chain, or
// FailureAgent for any other error.
func FailureOf(err error) FailureKind {
	var se *SessionError
	if errors.As(err, &se) && se.Kind != "" {
		return se.Kind
	}
	return FailureAgent
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
	switch r.Failure {
	case "":
	case FailureAgent, FailureTransient, FailureQuota:
		if r.Status != StatusFailed {
			return fmt.Errorf("result status %q has a failure kind %q", r.Status, r.Failure)
		}
	default:
		return fmt.Errorf("result failure kind %q is not agent, transient or quota", r.Failure)
	}
	if len(r.Output) > MaxOutput {
		return fmt.Errorf("result output is %d bytes, over the limit of %d", len(r.Output), MaxOutput)
	}
	return nil
}
