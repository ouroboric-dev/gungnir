package schema

import (
	"errors"
	"strings"
	"testing"
)

func ready(id, layer string, deps ...string) Feature {
	return Feature{ID: id, Title: id, Criteria: []string{"it works"}, Test: "TestIt", Layer: layer, Size: SizeS, DependsOn: deps}
}

func TestAPlanOfReadyTasksPasses(t *testing.T) {
	fs := []Feature{ready("p-T1", "repos/api"), ready("p-T2", "repos/web", "p-T1")}
	fs[1].Size = SizeM
	if err := CheckReady(fs, []string{"repos/api", "repos/web"}); err != nil {
		t.Fatalf("CheckReady() = %v", err)
	}
	if err := CheckReady(fs, nil); err != nil {
		t.Errorf("with no leaf list, the layers are not looked up: %v", err)
	}
}

func TestCheckReadyListsEveryReasonATaskIsNotReady(t *testing.T) {
	leaves := []string{"repos/api", "repos/web"}
	bad := func(mutate func(*Feature)) []Feature {
		f := ready("p-T1", "repos/api")
		mutate(&f)
		return []Feature{f}
	}
	cases := map[string]struct {
		fs   []Feature
		want string
	}{
		"no criteria":                  {bad(func(f *Feature) { f.Criteria = nil }), "no acceptance criteria"},
		"a blank criterion":            {bad(func(f *Feature) { f.Criteria = []string{"  "} }), "no acceptance criteria"},
		"no named test":                {bad(func(f *Feature) { f.Test = " " }), "no named test"},
		"no layer":                     {bad(func(f *Feature) { f.Layer = "" }), "no layer"},
		"an unknown layer":             {bad(func(f *Feature) { f.Layer = "repos/db" }), `layer "repos/db" is not a leaf of this project (repos/api, repos/web)`},
		"no size":                      {bad(func(f *Feature) { f.Size = "" }), "no size"},
		"a size L":                     {bad(func(f *Feature) { f.Size = SizeL }), "split it"},
		"a size of words":              {bad(func(f *Feature) { f.Size = "Small" }), `size "Small" is not S, M or L`},
		"an unknown blocker":           {bad(func(f *Feature) { f.DependsOn = []string{"p-T9"} }), `depends on "p-T9", which is not a task of this plan`},
		"itself":                       {bad(func(f *Feature) { f.DependsOn = []string{"p-T1"} }), "depends on itself"},
		"a ring of two":                {[]Feature{ready("a-1", "repos/api", "a-2"), ready("a-2", "repos/api", "a-1")}, "ring"},
		"a ring of three":              {[]Feature{ready("a-1", "repos/api", "a-3"), ready("a-2", "repos/api", "a-1"), ready("a-3", "repos/api", "a-2")}, "ring"},
		"two ids that are one node id": {[]Feature{ready("A_1", "repos/api"), ready("a-1", "repos/api")}, "same node id"},
	}
	for name, c := range cases {
		err := CheckReady(c.fs, leaves)
		var re *ReadinessError
		if !errors.As(err, &re) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: CheckReady() = %v, want a ReadinessError containing %q", name, err, c.want)
		}
	}
	err := CheckReady([]Feature{{ID: "p-T1"}, {ID: "p-T2", Layer: "repos/api"}}, leaves)
	var re *ReadinessError
	if !errors.As(err, &re) || len(re.Problems) < 5 {
		t.Fatalf("CheckReady() = %v; every reason for every task is listed, not only the first", err)
	}
	if re.Problems[0].Task != "p-T1" {
		t.Errorf("problems = %+v", re.Problems)
	}
	if err := CheckReady(nil, leaves); err == nil {
		t.Error("a plan with no task was ready")
	}
}

func TestTheReadinessErrorIsBounded(t *testing.T) {
	var fs []Feature
	for i := 0; i < MaxTasks; i++ {
		fs = append(fs, Feature{ID: "p-T" + strings.Repeat("9", 1+i%7) + string(rune('a'+i%26)), Title: strings.Repeat("x", 500)})
	}
	err := CheckReady(fs, nil)
	if err == nil || len(err.Error()) > 8000 || !strings.Contains(err.Error(), "more") {
		t.Errorf("a plan of %d unready tasks gave an error of %d bytes; it goes back into a prompt, so it is cut", len(fs), len(err.Error()))
	}
}

func TestReadyOrderPutsEveryTaskAfterWhatItDependsOn(t *testing.T) {
	fs := []Feature{ready("c", "r", "b"), ready("a", "r"), ready("b", "r", "a"), ready("d", "r")}
	got, err := ReadyOrder(fs)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	if strings.Join(ids, ",") != "a,b,c,d" {
		t.Errorf("order = %v; the file's order, held back until a task's blockers are done", ids)
	}
	if _, err := ReadyOrder([]Feature{ready("a", "r", "b"), ready("b", "r", "a")}); err == nil {
		t.Error("a ring has no order")
	}
}
