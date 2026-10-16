package gungnir

import "fmt"

// CapScope says which budget cap was reached (spec §7.4).
type CapScope string

const (
	CapRun CapScope = "run"
	CapDay CapScope = "day"
)

// CapDimension says what the cap limits.
type CapDimension string

const (
	CapTokens CapDimension = "tokens"
	CapTurns  CapDimension = "turns"
	CapUSD    CapDimension = "usd"
	// CapJournal is not a limit: the record of the day's spend can't be read, so the per-day cap
	// can't be checked, and a cap that can't be checked is treated as reached.
	CapJournal CapDimension = "journal"
)

// CapError is a budget cap that has been reached: the run stops at the hard approval point
// budget-cap, with everything resumable (spec §7.4).
type CapError struct {
	Scope     CapScope
	Dimension CapDimension
	Used      float64
	Limit     float64
	Note      string // CapJournal: why the record can't be read
}

func (e *CapError) Error() string {
	if e.Dimension == CapJournal {
		return fmt.Sprintf("the per-%s cap can't be checked, so it counts as reached: %s", e.Scope, e.Note)
	}
	if e.Dimension == CapUSD {
		return fmt.Sprintf("the per-%s cap is reached: $%.4f spent of $%.4f", e.Scope, e.Used, e.Limit)
	}
	return fmt.Sprintf("the per-%s %s cap is reached: %.0f used of %.0f", e.Scope, e.Dimension, e.Used, e.Limit)
}

// Meter says whether the budget lets another session start.
type Meter interface {
	// Check returns the cap that is reached, or nil when every cap still has room.
	Check() *CapError
}

// CapTokens is the number of tokens a cap counts: input and output tokens. Cache reads are reported
// beside them in the ledger and don't count (Phase 4, decision 6).
func (u Usage) CapTokens() int64 { return u.InTok + u.OutTok }
