package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// Feature is one task of a plan's tasks.md, as Odin keeps it in features.json (spec §4.2, §5 canonical
// artifacts). The planner writes the file; its text is data, never an instruction.
type Feature struct {
	ID        string   `json:"id"`       // stable, scoped to the plan: <plan>-T001
	Title     string   `json:"title"`    // the heading's words after the id
	Criteria  []string `json:"criteria"` // the acceptance criteria, one for each checkbox
	Test      string   `json:"test"`     // the named test: the file and the test that will prove it
	Layer     string   `json:"layer"`    // the leaf that holds the work, by path in the project, such as repos/api
	Size      string   `json:"size"`     // S (under 100 changed lines), M (100 to 400) or L (split it)
	DependsOn []string `json:"depends_on"`
	Line      int      `json:"line"`               // where the task's heading is in tasks.md; 0 for a task that comes from a plan document
	Text      string   `json:"text,omitempty"`     // the planner's words for the task, whole (a plan document's task)
	Contract  string   `json:"contract,omitempty"` // the OpenAPI text the task adds or calls, verbatim
}

// Task sizes of the Definition of Ready.
const (
	SizeS = "S"
	SizeM = "M"
	SizeL = "L"
)

// MaxTasksFile is the biggest tasks.md Odin reads, and MaxTasks the most tasks it holds: a plan is
// written by an agent, and what it writes is bounded before anything reads it.
const (
	MaxTasksFile = 1 << 20
	MaxTasks     = 200
)

// MaxTaskText and MaxTaskContract bound a task's text and contract, in bytes. Odin never cuts one: readiness
// refuses an over-size task so the planner splits it.
const (
	MaxTaskText     = 64 << 10
	MaxTaskContract = 64 << 10
)

var (
	taskHeading = regexp.MustCompile(`^#{2,3}\s+(?:Task\s+)?([A-Za-z0-9][A-Za-z0-9._-]*):\s*(.*?)\s*$`)
	anyHeading  = regexp.MustCompile(`^#{1,3}\s`)
	fieldLine   = regexp.MustCompile(`^- \*\*([A-Za-z ]+?):\*\*\s*(.*)$`)
	checkbox    = regexp.MustCompile(`^\s+- \[[ xX]\]\s+(.*)$`)
	noneWord    = regexp.MustCompile(`^(?i:nothing|none|no one|n/a|-|—)$`)
	splitIDs    = regexp.MustCompile(`[\s,;&]+`)
	notSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

// Slug is the task's id as a node id: lower case words joined by dashes, starting with a letter.
func (f Feature) Slug() string {
	s := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(f.ID), "-"), "-")
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		s = "t" + s
	}
	return s
}

// ParseTasks reads a plan's tasks.md. A task is a heading of the second or third level, "## <id>:
// <title>" or "### Task <id>: <title>", the id holding a digit, and the bullets under it: "**Acceptance criteria:**" with a
// checkbox for each criterion, "**Named test:**", "**Layer:**", "**Size:**" and "**Depends on:**". Other
// bullets and prose are ignored, and so is everything inside a code fence. A task with fields missing
// parses (the readiness check refuses it, with the reasons); an id used twice, a field given twice, a
// criterion outside a field, an unclosed fence, a file with no task and a file over MaxTasksFile are
// errors.
//
// Deprecated: reads the tasks.md of a run from before change typed-boundaries; nothing new calls it (T18 removes
// it).
func ParseTasks(file string, data []byte) ([]Feature, error) {
	if len(data) > MaxTasksFile {
		return nil, fmt.Errorf("%s: %d bytes is more than the %d a tasks file may hold", file, len(data), MaxTasksFile)
	}
	var out []Feature
	var cur *Feature
	seenID := map[string]bool{}
	seenField := map[string]bool{}
	field := ""
	fenced := false
	for i, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		n := i + 1
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := taskHeading.FindStringSubmatch(line); m != nil && strings.ContainsAny(m[1], "0123456789") {
			if seenID[m[1]] {
				return nil, fmt.Errorf("%s:%d: task id %s is used twice", file, n, m[1])
			}
			seenID[m[1]] = true
			if len(out) == MaxTasks {
				return nil, fmt.Errorf("%s:%d: more than %d tasks; a plan this big needs splitting", file, n, MaxTasks)
			}
			out = append(out, Feature{ID: m[1], Title: m[2], Line: n})
			cur, field, seenField = &out[len(out)-1], "", map[string]bool{}
			continue
		}
		if anyHeading.MatchString(line) { // another section: the bullets under it are not the last task's
			cur, field = nil, ""
			continue
		}
		if cur == nil {
			continue
		}
		if m := fieldLine.FindStringSubmatch(line); m != nil {
			field = strings.ToLower(m[1])
			if seenField[field] {
				return nil, fmt.Errorf("%s:%d: task %s gives %q twice", file, n, cur.ID, m[1])
			}
			seenField[field] = true
			setField(cur, field, m[2])
			continue
		}
		if m := checkbox.FindStringSubmatch(line); m != nil {
			if field != "acceptance criteria" {
				return nil, fmt.Errorf("%s:%d: a checkbox outside \"Acceptance criteria\" in task %s", file, n, cur.ID)
			}
			cur.Criteria = append(cur.Criteria, strings.TrimSpace(m[1]))
			continue
		}
		// A line that continues the last criterion, or the last field's value.
		if t := strings.TrimSpace(line); t != "" && strings.HasPrefix(line, "  ") {
			continueField(cur, field, t)
		} else if t == "" || !strings.HasPrefix(line, "-") {
			field = ""
		}
	}
	if fenced {
		return nil, fmt.Errorf("%s: a code fence is not closed", file)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no task found; a task is a heading such as \"## plan-T001: title\"", file)
	}
	for i := range out {
		finish(&out[i])
	}
	return out, nil
}

// setField records the first line of a field.
func setField(f *Feature, field, value string) {
	switch field {
	case "named test":
		f.Test = value
	case "layer":
		f.Layer = value
	case "size":
		f.Size = value
	case "depends on":
		f.DependsOn = []string{value}
	}
}

// continueField adds a continuation line to the last criterion, or to the field's value.
func continueField(f *Feature, field, text string) {
	switch field {
	case "acceptance criteria":
		if n := len(f.Criteria); n > 0 {
			f.Criteria[n-1] += " " + text
		}
	case "named test":
		f.Test += " " + text
	case "layer":
		f.Layer += " " + text
	case "size":
		f.Size += " " + text
	case "depends on":
		f.DependsOn = append(f.DependsOn, text)
	}
}

// finish turns the raw values into the fields' meanings: a layer as a path, a size as a letter, and the
// ids a task depends on.
func finish(f *Feature) {
	f.Test = strings.TrimSpace(f.Test)
	f.Layer = strings.Trim(strings.TrimSpace(f.Layer), "`")
	if rest, ok := strings.CutPrefix(f.Layer, "layer/"); ok {
		f.Layer = "repos/" + rest
	}
	if words := strings.Fields(strings.ReplaceAll(f.Size, "`", "")); len(words) > 0 {
		if letter := strings.ToUpper(strings.Trim(words[0], "().,:")); len(letter) == 1 {
			f.Size = letter
		}
	}
	raw := strings.ReplaceAll(strings.Join(f.DependsOn, " "), "`", "")
	f.DependsOn = nil
	if noneWord.MatchString(strings.TrimSpace(raw)) {
		return
	}
	for _, w := range splitIDs.Split(raw, -1) {
		if w != "" && !strings.EqualFold(w, "and") {
			f.DependsOn = append(f.DependsOn, w)
		}
	}
}
