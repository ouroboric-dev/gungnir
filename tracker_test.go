package gungnir

import (
	"slices"
	"strings"
	"testing"
)

func TestTheBoardHasSixColumnsInReadingOrder(t *testing.T) {
	got := Columns()
	want := []Column{ColumnReady, ColumnInProgress, ColumnVerifying, ColumnBlocked, ColumnNeedsYou, ColumnDone}
	if !slices.Equal(got, want) {
		t.Fatalf("Columns() = %v, want %v", got, want)
	}
	for _, c := range got {
		if c.Title() == "" || c.Title() == string(c) {
			t.Errorf("column %q has no title for a terminal board: %q", c, c.Title())
		}
	}
}

func TestABoardIsRefusedWhenAColumnIsUnknownOrATaskRepeats(t *testing.T) {
	ok := Board{Project: "/p", Cards: []Card{{Task: "v-T1", Column: ColumnDone}, {Task: "v-T2", Column: ColumnBlocked, Why: "attempts exhausted"}}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	for name, b := range map[string]Board{
		"an unknown column":             {Cards: []Card{{Task: "v-T1", Column: "doing"}}},
		"a task without an id":          {Cards: []Card{{Column: ColumnReady}}},
		"a task twice":                  {Cards: []Card{{Task: "v-T1", Column: ColumnReady}, {Task: "v-T1", Column: ColumnDone}}},
		"a blocked card with no reason": {Cards: []Card{{Task: "v-T1", Column: ColumnBlocked}}},
	} {
		if err := b.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil", name)
		}
	}
}

func TestCountsByColumnNameEveryColumn(t *testing.T) {
	b := Board{Cards: []Card{{Task: "a", Column: ColumnDone}, {Task: "b", Column: ColumnDone}, {Task: "c", Column: ColumnReady}}}
	got := b.Counts()
	if got[ColumnDone] != 2 || got[ColumnReady] != 1 || len(got) != 6 {
		t.Errorf("Counts() = %v, want done 2, ready 1 and six keys", got)
	}
	if line := b.Summary(); !strings.Contains(line, "2 done") || !strings.Contains(line, "1 ready") {
		t.Errorf("Summary() = %q", line)
	}
}
