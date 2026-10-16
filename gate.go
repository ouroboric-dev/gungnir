package gungnir

import (
	"fmt"
	"slices"
)

// Gate names an approval point (spec §7.1).
type Gate string

const (
	GateDesign          Gate = "design"
	GateSpec            Gate = "spec"
	GatePlan            Gate = "plan"
	GateFeature         Gate = "feature"
	GateMerge           Gate = "merge"
	GatePointer         Gate = "pointer"
	GatePointerContract Gate = "pointer-contract"

	// The hard approval points stop in every mode, and no config can override them.
	GateDeployProd      Gate = "deploy-prod"
	GateStoreSubmit     Gate = "store-submit"
	GateRealSecrets     Gate = "real-secrets-or-user-data"
	GateBudgetCap       Gate = "budget-cap"
	GateExternalInput   Gate = "external-input"
	GatePolicyViolation Gate = "policy-violation"
	GateShrinkGuard     Gate = "shrink-guard"
	GateGuardrailChange Gate = "guardrail-change"
	GateRepoAdmin       Gate = "repo-admin" // change remote-creates-repos: an action that needs admin rights on a hosted repo
)

var (
	softGates = []Gate{GateDesign, GateSpec, GatePlan, GateFeature, GateMerge, GatePointer, GatePointerContract}
	hardGates = []Gate{GateDeployProd, GateStoreSubmit, GateRealSecrets, GateBudgetCap, GateExternalInput,
		GatePolicyViolation, GateShrinkGuard, GateGuardrailChange, GateRepoAdmin}
)

// Hard reports whether g is a hard approval point.
func (g Gate) Hard() bool { return slices.Contains(hardGates, g) }

// Known reports whether g is an approval point of the spec.
func (g Gate) Known() bool { return g.Hard() || slices.Contains(softGates, g) }

// Gates returns every known approval point, the soft ones first.
func Gates() []Gate { return slices.Concat(softGates, hardGates) }

// Mode is an autonomy mode (spec §7.1).
type Mode string

const (
	ModeStepByStep Mode = "step-by-step"
	ModeKeyPoints  Mode = "key-points"
	ModeUnattended Mode = "unattended"
)

var modes = []Mode{ModeStepByStep, ModeKeyPoints, ModeUnattended} // tightest first

// ParseMode returns the mode named s.
func ParseMode(s string) (Mode, error) {
	if m := Mode(s); slices.Contains(modes, m) {
		return m, nil
	}
	return "", fmt.Errorf("unknown mode %q (want step-by-step, key-points or unattended)", s)
}

// Looser reports whether m stops at fewer approval points than other.
func (m Mode) Looser(other Mode) bool { return slices.Index(modes, m) > slices.Index(modes, other) }
