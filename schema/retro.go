package schema

import "github.com/ouroboric-dev/gungnir"

// Retro is the retrospective agent's answer (ygg.retro/v1): what went well, what went wrong and what to change,
// each item with the block it targets and the evidence for it. With Blocked set, the lists may be left out.
type Retro struct {
	WentWell    []RetroItem      `json:"went_well,omitzero"`
	WentWrong   []RetroItem      `json:"went_wrong,omitzero"`
	Suggestions []RetroItem      `json:"suggestions,omitzero"`
	Blocked     *gungnir.Blocked `json:"blocked,omitempty"`
	Notes       []string         `json:"notes,omitzero"`
}

// RetroItem is one observation. Category is one of the retro skill's six (navigation, automated-checks,
// coding-standards, agents-md, tool-economy, information-access) or other. Severity is high, medium or low. Block
// is the graph block it targets. Each evidence entry is kind:reference, the kind being task, finding, ledger,
// transcript or record, such as task:t002 or ledger:182.
type RetroItem struct {
	Category string   `json:"category"`
	Severity string   `json:"severity"`
	Text     string   `json:"text"`
	Block    string   `json:"block"`
	Evidence []string `json:"evidence"`
}
