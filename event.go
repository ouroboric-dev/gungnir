package gungnir

import "time"

// LedgerSchema names the ledger line format. A breaking change gets a new version.
const LedgerSchema = "ygg.ledger/v1"

// EventName is what a ledger line records (spec §10.1).
type EventName string

const (
	EventRunStarted      EventName = "run_started"
	EventRunResumed      EventName = "run_resumed"
	EventRunFinished     EventName = "run_finished"
	EventRunFailed       EventName = "run_failed"
	EventNodeStarted     EventName = "node_started"
	EventNodeFinished    EventName = "node_finished"
	EventNodeFailed      EventName = "node_failed"
	EventCheckResult     EventName = "check_result"
	EventGatePending     EventName = "gate_pending"
	EventGateApproved    EventName = "gate_approved"
	EventGateRejected    EventName = "gate_rejected"
	EventGateAutoPassed  EventName = "gate_auto_passed"
	EventLoopAttempt     EventName = "loop_attempt"
	EventEscalation      EventName = "escalation"
	EventRoutingLookup   EventName = "routing_lookup"
	EventBudgetWarning   EventName = "budget_warning"
	EventPolicyViolation EventName = "policy_violation"
	EventPointerMoved    EventName = "pointer_moved"
	EventModeChanged     EventName = "mode_changed"
	EventConfigChanged   EventName = "config_changed"
)

// Events returns every event name, in the order above.
func Events() []EventName {
	return []EventName{EventRunStarted, EventRunResumed, EventRunFinished, EventRunFailed,
		EventNodeStarted, EventNodeFinished, EventNodeFailed, EventCheckResult, EventGatePending,
		EventGateApproved, EventGateRejected, EventGateAutoPassed, EventLoopAttempt, EventEscalation,
		EventRoutingLookup, EventBudgetWarning, EventPolicyViolation, EventPointerMoved,
		EventModeChanged, EventConfigChanged}
}

// Event is one ledger line. Fields follow OpenTelemetry's GenAI attribute names (gen_ai.*) where
// one exists, so an OTel exporter is a mapping (spec §10.1); the rest use the ygg.* namespace.
type Event struct {
	Schema  string    `json:"schema"`
	Seq     int64     `json:"seq"` // 1 on the first line
	Time    time.Time `json:"time"`
	Run     string    `json:"ygg.run.id"`
	Node    string    `json:"ygg.node.id,omitempty"`
	Name    EventName `json:"event"`
	Attempt int       `json:"ygg.attempt,omitempty"`

	Role       string `json:"gen_ai.agent.name,omitempty"` // the factory.yaml role
	Provider   string `json:"gen_ai.provider.name,omitempty"`
	Model      string `json:"gen_ai.request.model,omitempty"`
	Adapter    string `json:"ygg.agent.adapter,omitempty"`
	CLIVersion string `json:"ygg.agent.cli_version,omitempty"`

	InputTokens     int64   `json:"gen_ai.usage.input_tokens,omitempty"`
	OutputTokens    int64   `json:"gen_ai.usage.output_tokens,omitempty"`
	CacheReadTokens int64   `json:"gen_ai.usage.cache_read.input_tokens,omitempty"`
	Turns           int64   `json:"ygg.usage.turns,omitempty"`
	CostUSD         float64 `json:"ygg.usage.cost_usd,omitempty"`
	CostEstimated   bool    `json:"ygg.usage.cost_estimated,omitempty"`

	DurationMS int64  `json:"ygg.duration_ms,omitempty"`
	Result     string `json:"ygg.result,omitempty"`
	Gate       string `json:"ygg.gate.name,omitempty"` // the gate node's id
	Note       string `json:"ygg.note,omitempty"`      // an approval note or a rejection reason
	Mode       Mode   `json:"ygg.mode,omitempty"`
	Looser     bool   `json:"ygg.mode.looser,omitempty"`
	ConfigHash string `json:"ygg.config.sha256,omitempty"`
	Commit     string `json:"ygg.ref.commit,omitempty"`
	GateFile   string `json:"ygg.ref.gate_file,omitempty"`

	Prev string `json:"prev"` // hex SHA-256 of the previous line; 64 zeros on the first line
}

// WithUsage returns e with u's numbers in the usage fields.
func (e Event) WithUsage(u Usage) Event {
	e.InputTokens, e.OutputTokens, e.CacheReadTokens = u.InTok, u.OutTok, u.CacheTok
	e.Turns, e.CostUSD, e.CostEstimated = u.Turns, u.CostUSD, u.CostEstimated
	return e
}
