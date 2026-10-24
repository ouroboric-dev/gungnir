package schema

import "github.com/ouroboric-dev/gungnir"

// Plan is the plan stage's answer (ygg.plan/v1): the plan and its tasks as fields. Odin renders plan.md and
// tasks.md, and builds features.json from it. With Blocked set, the content fields may be empty.
type Plan struct {
	Title     string           `json:"title,omitempty"`
	Slug      string           `json:"slug,omitempty"`
	Goal      string           `json:"goal,omitempty"`
	Decisions []Decided        `json:"decisions,omitzero"`
	Risks     []string         `json:"risks,omitzero"`
	Tasks     []PlanTask       `json:"tasks,omitzero"`
	Blocked   *gungnir.Blocked `json:"blocked,omitempty"`
	Notes     []string         `json:"notes,omitzero"`
}

// PlanTask is one task of a plan. Text is the planner's words for the implementer (behaviour, routes, fields,
// limits, errors) and Contract the OpenAPI text the task adds or calls, verbatim; both reach the task's
// prompt whole. Size is S, M or L: the schema lets L through so that readiness can tell the planner to split it.
type PlanTask struct {
	ID        string   `json:"id"` // <plan slug>-T001, ...
	Title     string   `json:"title"`
	Criteria  []string `json:"criteria"` // 2 to 5, each one a thing a test can check
	NamedTest string   `json:"named_test"`
	Layer     string   `json:"layer"`
	Size      string   `json:"size"`
	DependsOn []string `json:"depends_on"`
	Text      string   `json:"text"`
	Contract  string   `json:"contract,omitempty"`
}

// FeaturesFromPlan turns the plan's tasks into features, in the plan's order. The line is 0: no file holds them.
func FeaturesFromPlan(p Plan) []Feature {
	out := make([]Feature, 0, len(p.Tasks))
	for _, t := range p.Tasks {
		out = append(out, Feature{
			ID: t.ID, Title: t.Title, Criteria: t.Criteria, Test: t.NamedTest, Layer: t.Layer, Size: t.Size,
			DependsOn: t.DependsOn, Text: t.Text, Contract: t.Contract,
		})
	}
	return out
}
