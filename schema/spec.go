package schema

import "github.com/ouroboric-dev/gungnir"

// Spec is the spec stage's answer (ygg.spec/v1): the sections of the engineering spec as fields, and the
// leaves. Odin renders the spec document and leaves.yaml. With Blocked set, the content fields may be empty.
type Spec struct {
	Title              string           `json:"title,omitempty"`
	Slug               string           `json:"slug,omitempty"`
	Summary            string           `json:"summary,omitempty"`
	Architecture       string           `json:"architecture,omitempty"`
	Interfaces         string           `json:"interfaces,omitempty"`
	DataModel          string           `json:"data_model,omitempty"`
	AcceptanceCriteria string           `json:"acceptance_criteria,omitempty"`
	Security           string           `json:"security,omitempty"`
	Testing            string           `json:"testing,omitempty"`
	Decisions          []Decided        `json:"decisions,omitzero"`
	Deviations         []Deviation      `json:"deviations,omitzero"`
	Leaves             Leaves           `json:"leaves,omitzero"`
	Blocked            *gungnir.Blocked `json:"blocked,omitempty"`
	Notes              []string         `json:"notes,omitzero"`
}

// Deviation is a point where the design cannot be built as written. Cites names the owner decision ids it
// comes from.
type Deviation struct {
	What  string   `json:"what"`
	Where string   `json:"where"`
	Why   string   `json:"why"`
	Cites []string `json:"cites,omitzero"`
}
