package gungnir

// Conformance is the verdict of the design-conformance reviewer on a spec (spec §4.2): whether the spec
// deviates from the approved design, and if it does, the amendment to the design it proposes. A
// deviation stops the spec approval point in every mode (spec §7.1).
type Conformance struct {
	Deviation bool   `json:"deviation"`
	Amendment string `json:"amendment,omitempty"`
}

// MaxAmendment is the most bytes of an amendment Odin reads and shows an operator.
const MaxAmendment = 8000
