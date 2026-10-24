package schema

import (
	"fmt"
	"slices"
)

// The built-in sources of input: what Odin supplies to a node, beside the outputs of earlier nodes (change
// typed-boundaries). A node's input entry is a node id, which stands for that node's output, or one of these.
const (
	SourceBrief        = "brief"         // the operator's brief
	SourceDecisions    = "decisions"     // the operator's gate decisions (ygg.decisions/v1)
	SourceTask         = "task"          // the task a foreach body works on
	SourceChange       = "change"        // the task's diff
	SourceOpenFindings = "open_findings" // the findings still open against the task
	SourceCheckResults = "check_results" // the loop's check results (ygg.check/v1)
	SourceRejection    = "rejection"     // a person's rejection at a gate: its reason and decisions file
	SourceRefusal      = "refusal"       // the factory's refusal (readiness, merge, pointer, owner-decision check)
	SourceRecord       = "record"        // the run record (ygg.run-record/v1)
	SourceDate         = "date"          // today's date, from Odin
)

// bodySources are the built-in sources that exist only for a task, inside a foreach body.
var bodySources = []string{SourceTask, SourceChange, SourceOpenFindings, SourceCheckResults}

// BuiltinSources lists the built-in sources of input, in a fixed order.
func BuiltinSources() []string {
	return []string{SourceBrief, SourceDecisions, SourceTask, SourceChange, SourceOpenFindings, SourceCheckResults,
		SourceRejection, SourceRefusal, SourceRecord, SourceDate}
}

// CheckTypes checks that the graph's joins type-check: every agent node, every loop (its implementer) and every
// reviewer declares one output, a contract of Contracts(); every input is a node before it (in its scope: the
// graph, or the nodes before a foreach and those before it in its body) that has an output, or a built-in
// source, and is listed once; a loop returns the implementation (ygg.implement/v1) and its reviewers the review
// (ygg.review/v1); task, change, open_findings and check_results are sources only inside a foreach body, and
// record only for a node after the grade; a reviews: node counts as an input; a publish node publishes a node with an
// output, a tasks node reads a plan (ygg.plan/v1), and a gate's deviation_from names a conformance
// (ygg.conformance/v1). The node a gate re-runs declares rejection, and the node a back_to names declares
// refusal, so the reason reaches the re-run. No node id is a built-in source's name. Only agents, loops and
// reviewers declare input and output; the graph schema refuses them on the other kinds. The error names the node
// and the field. ParseGraph calls it after the checks of the graph's shape, which resolve each gate's re-run.
func (g *Graph) CheckTypes() error {
	t := &typeCheck{order: map[string]int{}}
	byID := map[string]Node{}
	for i, n := range g.All() {
		if slices.Contains(BuiltinSources(), n.ID) {
			return fmt.Errorf("node %s: the id is a built-in source's name", n.ID)
		}
		t.order[n.ID] = i
		byID[n.ID] = n
	}
	if err := t.scope(g.Nodes, map[string]string{}, false); err != nil {
		return err
	}
	for _, n := range g.All() {
		if target, ok := byID[n.Reruns]; ok && n.Kind == KindGate && !slices.Contains(target.Input, SourceRejection) {
			return fmt.Errorf("node %s: %s re-runs it on a rejection; declare %s in its input", target.ID, n.ID, SourceRejection)
		}
		if target, ok := byID[n.BackTo]; ok && !slices.Contains(target.Input, SourceRefusal) {
			return fmt.Errorf("node %s: %s goes back to it with a refusal; declare %s in its input", target.ID, n.ID, SourceRefusal)
		}
	}
	return nil
}

// typeCheck holds the walk order of every node id, which tells a node that comes after from one that is out of
// scope.
type typeCheck struct {
	order map[string]int
}

// scope checks the nodes of one scope in order. visible maps the ids of the nodes before them, those of the
// enclosing scopes included, to their output ("" for a node with none); it is the scope's own copy. body says
// the scope is a foreach body.
func (t *typeCheck) scope(nodes []Node, visible map[string]string, body bool) error {
	graded := false
	for _, n := range nodes {
		if err := t.node(n, visible, place{body: body, graded: graded}); err != nil {
			return err
		}
		visible[n.ID] = n.Output
		graded = graded || n.Kind == KindGrade
		if n.Kind == KindForeach {
			if err := t.scope(n.Body, cloneScope(visible), true); err != nil {
				return err
			}
		}
	}
	return nil
}

func cloneScope(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// place says where a node stands, which decides the built-in sources it may read.
type place struct {
	body   bool // inside a foreach body
	graded bool // after the grade node
}

func (t *typeCheck) node(n Node, visible map[string]string, at place) error {
	switch n.Kind {
	case KindAgent, KindLoop:
		if err := checkOutput("node "+n.ID, n.Output); err != nil {
			return err
		}
		if n.Kind == KindLoop && n.Output != string(ContractImplement) {
			return fmt.Errorf("node %s: output %s must be %s: a loop returns the implementation", n.ID, n.Output, ContractImplement)
		}
		input := n.Input
		if n.Reviews != "" && !slices.Contains(input, n.Reviews) {
			input = append(slices.Clone(input), n.Reviews)
		}
		if err := t.inputs("node "+n.ID, input, n, visible, at); err != nil {
			return err
		}
		for _, r := range n.Reviewers {
			who := fmt.Sprintf("node %s: reviewer %s", n.ID, r.ID)
			if err := checkOutput(who, r.Output); err != nil {
				return err
			}
			if r.Output != string(ContractReview) {
				return fmt.Errorf("%s: output %s must be %s: a reviewer returns the review", who, r.Output, ContractReview)
			}
			if err := t.inputs(who, r.Input, n, visible, at); err != nil {
				return err
			}
		}
	case KindPublish:
		out, ok := visible[n.From]
		if !ok || out == "" {
			return fmt.Errorf("node %s: from %s is not a node before it with an output", n.ID, n.From)
		}
	case KindTasks:
		out, ok := visible[n.From]
		switch {
		case !ok:
			return fmt.Errorf("node %s: from %s is not a node before it: it names the node whose output is the plan", n.ID, n.From)
		case out != string(ContractPlan):
			return fmt.Errorf("node %s: from %s outputs %s, not %s", n.ID, n.From, outputName(out), ContractPlan)
		}
	case KindGate:
		if n.DeviationFrom == "" {
			break
		}
		if out := visible[n.DeviationFrom]; out != string(ContractConformance) {
			return fmt.Errorf("node %s: deviation_from %s outputs %s, not %s", n.ID, n.DeviationFrom, outputName(out), ContractConformance)
		}
	}
	return nil
}

func outputName(out string) string {
	if out == "" {
		return "nothing"
	}
	return out
}

// checkOutput checks that an output names a contract. who is the start of the message: the node, or the node
// and its reviewer.
func checkOutput(who, out string) error {
	if out == "" {
		return fmt.Errorf("%s has no output: every agent session returns one contract", who)
	}
	if !slices.Contains(Contracts(), Contract(out)) {
		return fmt.Errorf("%s: output %s is not a contract", who, out)
	}
	return nil
}

// inputs checks one input list. who starts the message; n is the node the list belongs to (a loop, for its
// reviewers).
func (t *typeCheck) inputs(who string, input []string, n Node, visible map[string]string, at place) error {
	seen := map[string]bool{}
	for _, in := range input {
		if seen[in] {
			return fmt.Errorf("%s: input %s is listed twice", who, in)
		}
		seen[in] = true
		switch {
		case !at.body && slices.Contains(bodySources, in):
			return fmt.Errorf("%s: input %s is a source only inside a foreach body", who, in)
		case !at.graded && in == SourceRecord:
			return fmt.Errorf("%s: input %s is a source only for a node after the grade", who, in)
		}
		if out, ok := visible[in]; ok {
			if out == "" {
				return fmt.Errorf("%s: input %s is a node with no output", who, in)
			}
			continue
		}
		if pos, ok := t.order[in]; ok {
			if pos >= t.order[n.ID] {
				return fmt.Errorf("%s: input %s is a node that comes after it", who, in)
			}
			return fmt.Errorf("%s: input %s is a node out of its scope (inside a foreach body)", who, in)
		}
		if !slices.Contains(BuiltinSources(), in) {
			return fmt.Errorf("%s: input %s is neither a node nor a built-in source", who, in)
		}
	}
	return nil
}
