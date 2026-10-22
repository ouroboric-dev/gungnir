package gungnir

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Column is where a task stands on the board (spec §10.3).
type Column string

const (
	ColumnReady      Column = "ready"       // todo: planned, not started
	ColumnInProgress Column = "in-progress" // an agent is implementing or reviewing it
	ColumnVerifying  Column = "verifying"   // built; the checks, the merge or the acceptance run are judging it
	ColumnBlocked    Column = "blocked"     // given up, or waiting on a task that was
	ColumnNeedsYou   Column = "needs-you"   // a person's approval or ruling is the only thing it waits for
	ColumnDone       Column = "done"
)

// Columns lists the board's columns in reading order.
func Columns() []Column {
	return []Column{ColumnReady, ColumnInProgress, ColumnVerifying, ColumnBlocked, ColumnNeedsYou, ColumnDone}
}

// Title is the column's heading on a terminal board.
func (c Column) Title() string {
	switch c {
	case ColumnReady:
		return "Ready"
	case ColumnInProgress:
		return "In progress"
	case ColumnVerifying:
		return "Verifying"
	case ColumnBlocked:
		return "Blocked"
	case ColumnNeedsYou:
		return "Needs you"
	case ColumnDone:
		return "Done"
	}
	return string(c)
}

// Card is one task on the board.
type Card struct {
	Task   string // the task's id in features.json
	Title  string
	Leaf   string // the leaf that holds the work, by path in the project
	Column Column
	Why    string // blocked: what stopped it; needs-you: what waits for the person
	Run    string // the run that last touched the task
}

// Board is a project's tasks by column, as a tracker projects them (spec §10.4).
type Board struct {
	Project string
	Cards   []Card
}

// Validate refuses a board with an unknown column, a task without an id or listed twice, or a blocked card
// that does not say why.
func (b Board) Validate() error {
	var errs []error
	seen := map[string]bool{}
	for _, c := range b.Cards {
		switch {
		case c.Task == "":
			errs = append(errs, errors.New("a card has no task id"))
		case seen[c.Task]:
			errs = append(errs, fmt.Errorf("task %s is on the board twice", c.Task))
		}
		seen[c.Task] = true
		if !slices.Contains(Columns(), c.Column) {
			errs = append(errs, fmt.Errorf("task %s is in the unknown column %q", c.Task, c.Column))
		}
		if c.Column == ColumnBlocked && c.Why == "" {
			errs = append(errs, fmt.Errorf("task %s is blocked and says nothing about why", c.Task))
		}
	}
	return errors.Join(errs...)
}

// Counts is how many cards each column holds; every column has a key.
func (b Board) Counts() map[Column]int {
	out := map[Column]int{}
	for _, c := range Columns() {
		out[c] = 0
	}
	for _, c := range b.Cards {
		out[c.Column]++
	}
	return out
}

// Summary is the counts on one line, such as "2 done, 1 ready".
func (b Board) Summary() string {
	n := b.Counts()
	var parts []string
	for _, c := range Columns() {
		if n[c] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n[c], strings.ReplaceAll(string(c), "-", " ")))
		}
	}
	if len(parts) == 0 {
		return "no tasks"
	}
	return strings.Join(parts, ", ")
}

// Tracker projects the factory's state one way, into a board (spec §10.4). Odin is its only writer; an agent
// never touches it. The local tracker reads features.json and the ledger; the GitHub tracker is iteration 2.
type Tracker interface {
	Board(ctx context.Context) (Board, error)
}
