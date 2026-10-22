package gungnir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The architecture page names the contract of the grader, routing and dependency work: every event it added,
// the node, the factory block, the two files, and the pack block. A new event of these kinds with no line in
// the page fails here.
func TestTheArchitecturePageNamesTheGraderContract(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, e := range []EventName{EventGradeStarted, EventGradeFinished, EventGradeRefused, EventRoutingIndexBuilt,
		EventPointerContractChecked, EventDepsChecked, EventDepsBump} {
		if !strings.Contains(page, string(e)) {
			t.Errorf("docs/architecture.md does not name the event %s", e)
		}
	}
	for _, want := range []string{"`grade` node", "`grader:`", "score.json", "ygg.score/v1", "manifest.json", "ygg.suite/v1", "`routing` block", "pointer-contract", "memory: none"} {
		if !strings.Contains(page, want) {
			t.Errorf("docs/architecture.md does not mention %q", want)
		}
	}
}

// The architecture page names the graph's schema version, the project's graph pin, the graph store and the
// test of a graph pack.
func TestTheArchitecturePageNamesTheGraphPins(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{"`schema: 1`", "`graph_version`", "`graphs:`", "`HoldsRecipe`"} {
		if !strings.Contains(page, want) {
			t.Errorf("docs/architecture.md does not mention %q", want)
		}
	}
}

// The architecture page names what `ygg metrics` reads from the ledger: a new field of that kind with no line in
// the page fails here.
func TestTheArchitecturePageNamesWhatMetricsReads(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	for _, want := range []string{string(EventDisputeIgnored), "ygg.finding.origin", "ygg.routing.bench", "ygg.routing.baseline", "ygg.routing.threshold", "ygg.factory.version"} {
		if !strings.Contains(page, want) {
			t.Errorf("docs/architecture.md does not mention %q", want)
		}
	}
}
