package gungnir

import (
	"strings"
	"testing"
)

func TestCapTokensCountsInputAndOutputNotCacheReads(t *testing.T) {
	u := Usage{InTok: 100, OutTok: 20, CacheTok: 5000}
	if got := u.CapTokens(); got != 120 {
		t.Errorf("CapTokens() = %d, want 120: cache reads are reported beside the count, not in it", got)
	}
}

func TestACapErrorSaysWhatWasReached(t *testing.T) {
	e := &CapError{Scope: CapDay, Dimension: CapTokens, Used: 41, Limit: 40}
	if got := e.Error(); !strings.Contains(got, "per-day") || !strings.Contains(got, "tokens") || !strings.Contains(got, "41") || !strings.Contains(got, "40") {
		t.Errorf("Error() = %q", got)
	}
	usd := &CapError{Scope: CapRun, Dimension: CapUSD, Used: 1.5, Limit: 1}
	if got := usd.Error(); !strings.Contains(got, "$1.5000") || !strings.Contains(got, "per-run") {
		t.Errorf("Error() = %q", got)
	}
}
