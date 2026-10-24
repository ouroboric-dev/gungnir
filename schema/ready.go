package schema

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Problem is one reason a task is not ready.
type Problem struct{ Task, Text string }

// ReadinessError lists why tasks are not ready (the Definition of Ready, spec §4.2). Its text goes back
// to the planner as input, so it names each problem and is cut short.
type ReadinessError struct{ Problems []Problem }

// MaxReadinessText bounds the error's text: it becomes part of a prompt.
const MaxReadinessText = 6000

func (e *ReadinessError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "the plan has %d problem(s) that keep its tasks from being ready:\n", len(e.Problems))
	for i, p := range e.Problems {
		line := fmt.Sprintf("- %s: %s\n", p.Task, p.Text)
		if b.Len()+len(line) > MaxReadinessText {
			fmt.Fprintf(&b, "- ... and %d more\n", len(e.Problems)-i)
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// CheckReady applies the Definition of Ready to a plan: every task has acceptance criteria, a named
// test, a layer, a size of S or M (L means split it), a text and a contract within their bounds (the contract, when
// given, an OpenAPI document), and dependencies that are tasks of the plan,
// without a ring; and no two ids are the same node id. leaves, when given, are the paths a layer may
// name. The error is a *ReadinessError that lists everything, not the first problem.
func CheckReady(fs []Feature, leaves []string) error {
	if len(fs) == 0 {
		return &ReadinessError{Problems: []Problem{{"plan", "has no task"}}}
	}
	var ps []Problem
	add := func(f Feature, format string, a ...any) { ps = append(ps, Problem{f.ID, fmt.Sprintf(format, a...)}) }
	byID := map[string]Feature{}
	slugs := map[string]string{}
	for _, f := range fs {
		byID[f.ID] = f
		if other, dup := slugs[f.Slug()]; dup {
			add(f, "id %q and %q are the same node id (%s)", f.ID, other, f.Slug())
		}
		slugs[f.Slug()] = f.ID
	}
	for _, f := range fs {
		if !slices.ContainsFunc(f.Criteria, func(c string) bool { return strings.TrimSpace(c) != "" }) {
			add(f, "no acceptance criteria (a checkbox under \"Acceptance criteria\")")
		}
		if strings.TrimSpace(f.Test) == "" {
			add(f, "no named test: name the file and the test that will prove it")
		}
		switch {
		case f.Layer == "":
			add(f, "no layer: name the leaf that holds the work")
		case leaves != nil && !slices.Contains(leaves, f.Layer):
			add(f, "layer %q is not a leaf of this project (%s)", f.Layer, strings.Join(leaves, ", "))
		}
		switch f.Size {
		case SizeS, SizeM:
		case SizeL:
			add(f, "size L: split it into tasks of size S (under 100 changed lines) or M (100 to 400)")
		case "":
			add(f, "no size: S (under 100 changed lines) or M (100 to 400)")
		default:
			add(f, "size %q is not S, M or L", f.Size)
		}
		if n := len(f.Text); n > MaxTaskText {
			add(f, "its text is %d bytes, over %d: split the task or cut its text", n, MaxTaskText)
		}
		if n := len(f.Contract); n > MaxTaskContract {
			add(f, "its contract is %d bytes, over %d: split the task or cut its contract", n, MaxTaskContract)
		} else if f.Contract != "" {
			if err := checkOpenAPI(f.Contract); err != nil {
				add(f, "its contract is not an OpenAPI document: %v", err)
			}
		}
		for _, d := range f.DependsOn {
			switch _, ok := byID[d]; {
			case d == f.ID:
				add(f, "depends on itself")
			case !ok:
				add(f, "depends on %q, which is not a task of this plan", d)
			}
		}
	}
	if len(ps) == 0 {
		if _, err := ReadyOrder(fs); err != nil {
			ps = append(ps, Problem{"plan", err.Error()})
		}
	}
	if len(ps) > 0 {
		return &ReadinessError{Problems: ps}
	}
	return nil
}

// checkOpenAPI accepts a document that parses as YAML (JSON is YAML) with a string "openapi" and a mapping "paths".
func checkOpenAPI(contract string) error {
	var doc struct {
		OpenAPI any `yaml:"openapi"`
		Paths   any `yaml:"paths"`
	}
	if err := yaml.Unmarshal([]byte(contract), &doc); err != nil {
		return fmt.Errorf("it does not parse as YAML: %w", err)
	}
	if v, ok := doc.OpenAPI.(string); !ok || v == "" {
		return errors.New("it has no \"openapi\" version string")
	}
	if _, ok := doc.Paths.(map[string]any); !ok {
		return errors.New("it has no \"paths\" mapping")
	}
	return nil
}

// ReadyOrder returns the tasks in the order they can be built: the plan's own order, with a task held
// back until everything it depends on is placed. It refuses a ring of dependencies.
func ReadyOrder(fs []Feature) ([]Feature, error) {
	done := map[string]bool{}
	var out []Feature
	left := slices.Clone(fs)
	for len(left) > 0 {
		next := slices.IndexFunc(left, func(f Feature) bool {
			return !slices.ContainsFunc(f.DependsOn, func(d string) bool { return !done[d] })
		})
		if next < 0 {
			var ring []string
			for _, f := range left {
				ring = append(ring, f.ID)
			}
			return nil, fmt.Errorf("these tasks depend on each other in a ring (or on a task that is not in the plan): %s", strings.Join(ring, ", "))
		}
		out = append(out, left[next])
		done[left[next].ID] = true
		left = slices.Delete(left, next, next+1)
	}
	return out, nil
}
