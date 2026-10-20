package schema

import "testing"

func TestParseFindingTakesAnOverruledFindingOnlyWithProof(t *testing.T) {
	base := `{"id":"r-1","source":"spec","severity":"blocker","location":"a.js:3","claim":"findIndex is not a method","status":"overruled"`
	if _, err := ParseFinding([]byte(base + `,"proof":"check test passed and printed TestFindIndex"}`)); err != nil {
		t.Errorf("overruled with proof: ParseFinding() = %v", err)
	}
	if _, err := ParseFinding([]byte(base + `}`)); err == nil {
		t.Error("overruled with no proof: ParseFinding() = nil, want an error")
	}
	if _, err := ParseReport([]byte(`{"findings":[` + base + `}]}`)); err == nil {
		t.Error("overruled with no proof: ParseReport() = nil, want an error")
	}
}
