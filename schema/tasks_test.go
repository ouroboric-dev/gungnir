package schema

import (
	"slices"
	"strings"
	"testing"
)

const planTasks = `---
title: Tasks of the gallery
status: draft
---

# Tasks

Some words before the first task.

## gallery-T001: the api owns galleries

- **Files:** ` + "`repos/api/gallery.go`" + `
- **Acceptance criteria:**
  - [ ] a client can list the galleries they own
  - [x] another client's galleries are never listed,
        even by id
- **Named test:** ` + "`repos/api/gallery_test.go`" + ` ` + "`TestAClientListsOnlyTheirGalleries`" + `
- **Layer:** ` + "`repos/api`" + `
- **Size:** S
- **Depends on:** nothing

### Task gallery-T002: the web shows them

- **Acceptance criteria:**
  - [ ] the page lists the galleries
- **Named test:** gallery.spec.ts: lists galleries
- **Layer:** layer/web
- **Size:** M (about 200 lines)
- **Depends on:** gallery-T001, ` + "`gallery-T003`" + `

` + "```" + `
## gallery-T999: a heading inside a code block is not a task
` + "```" + `

## gallery-T003: the api signs download urls
- **Acceptance criteria:**
  - [ ] a signed url expires
- **Named test:** TestSignedURLsExpire
- **Layer:** repos/api
- **Size:** S
- **Depends on:** none
`

func TestParseTasksReadsTheCanonicalLayout(t *testing.T) {
	fs, err := ParseTasks("tasks.md", []byte(planTasks))
	if err != nil {
		t.Fatalf("ParseTasks() = %v", err)
	}
	if len(fs) != 3 {
		t.Fatalf("got %d tasks, want 3 (a heading in a code block is not one): %+v", len(fs), fs)
	}
	a, b, c := fs[0], fs[1], fs[2]
	if a.ID != "gallery-T001" || a.Title != "the api owns galleries" || a.Layer != "repos/api" || a.Size != "S" || len(a.DependsOn) != 0 {
		t.Errorf("first task = %+v", a)
	}
	if len(a.Criteria) != 2 || !strings.HasSuffix(a.Criteria[1], "even by id") || !strings.Contains(a.Criteria[1], "never listed,") {
		t.Errorf("criteria = %q", a.Criteria)
	}
	if !strings.Contains(a.Test, "TestAClientListsOnlyTheirGalleries") {
		t.Errorf("test = %q", a.Test)
	}
	if b.ID != "gallery-T002" || b.Layer != "repos/web" || b.Size != "M" || !slices.Equal(b.DependsOn, []string{"gallery-T001", "gallery-T003"}) {
		t.Errorf("second task = %+v", b)
	}
	if c.ID != "gallery-T003" || len(c.DependsOn) != 0 || c.Line == 0 {
		t.Errorf("third task = %+v", c)
	}
	if a.Slug() != "gallery-t001" || (Feature{ID: "007"}).Slug() != "t007" || (Feature{ID: "A_b.C"}).Slug() != "a-b-c" {
		t.Errorf("slugs: %q %q %q", a.Slug(), (Feature{ID: "007"}).Slug(), (Feature{ID: "A_b.C"}).Slug())
	}
}

func TestAHeadingThatIsNotATaskEndsTheTaskBeforeIt(t *testing.T) {
	fs, err := ParseTasks("tasks.md", []byte("## a-T1: x\n- **Layer:** repos/a\n\n## Notes: extra\n- **Layer:** repos/elsewhere\n- **Size:** L\n"))
	if err != nil || len(fs) != 1 || fs[0].Layer != "repos/a" || fs[0].Size != "" {
		t.Errorf("ParseTasks() = %+v, %v; the notes' bullets are not task a-T1's", fs, err)
	}
}

func TestParseTasksRefusesWhatItCannotReadExactly(t *testing.T) {
	cases := map[string]string{
		"an id used twice":            "## a-T1: x\n- **Layer:** repos/a\n## a-T1: y\n",
		"a field twice":               "## a-T1: x\n- **Layer:** repos/a\n- **Layer:** repos/b\n",
		"a criterion outside a field": "## a-T1: x\n  - [ ] stray\n- **Layer:** repos/a\n",
		"no tasks":                    "# Tasks\n\nNothing here.\n",
		"a file that is too big":      "## a-T1: x\n" + strings.Repeat("a line of words\n", 70000),
		"an id with a slash":          "## a/T1: x\n- **Layer:** repos/a\n",
		"an unclosed code fence":      "## a-T1: x\n```\n- **Layer:** repos/a\n",
	}
	for name, md := range cases {
		if _, err := ParseTasks("tasks.md", []byte(md)); err == nil {
			t.Errorf("%s: ParseTasks() = nil error", name)
		}
	}
}

func TestParseTasksLeavesMissingFieldsToTheReadinessCheck(t *testing.T) {
	fs, err := ParseTasks("tasks.md", []byte("## a-T1: only a title\n"))
	if err != nil || len(fs) != 1 || fs[0].Layer != "" || fs[0].Size != "" || len(fs[0].Criteria) != 0 {
		t.Errorf("ParseTasks() = %+v, %v; a task with fields missing is the readiness check's to refuse", fs, err)
	}
}
