package schema

import (
	"strings"
	"testing"
	"time"
)

const graderBlock = "grader:\n  tasks_dir: /home/op/mimir\n  suite_dir: /home/op/suites\n  timeout: 30m\n  memory: 4g\n  cpus: 4\n"

func TestFactoryNamesTheGrader(t *testing.T) {
	f, err := ParseFactory("factory.yaml", []byte(factoryHead+graderBlock))
	if err != nil {
		t.Fatalf("ParseFactory() = %v", err)
	}
	g := f.Grader
	if g == nil || g.TasksDir != "/home/op/mimir" || g.SuiteDir != "/home/op/suites" || time.Duration(g.Timeout) != 30*time.Minute || g.Memory != "4g" || g.CPUs != 4 {
		t.Errorf("Grader = %+v", g)
	}
	if f, err := ParseFactory("factory.yaml", []byte(factoryHead)); err != nil || f.Grader != nil {
		t.Errorf("a config with no grader = %+v, %v", f, err)
	}
}

func TestTheGraderBlockIsRefusedWhenItIsNotExact(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"a relative tasks folder":     {strings.Replace(graderBlock, "/home/op/mimir", "mimir", 1), "tasks_dir"},
		"a relative suite folder":     {strings.Replace(graderBlock, "/home/op/suites", "suites", 1), "suite_dir"},
		"a folder that isn't clean":   {strings.Replace(graderBlock, "/home/op/suites", "/home/op/../op/suites", 1), "suite_dir"},
		"the root as tasks folder":    {strings.Replace(graderBlock, "/home/op/mimir", "/", 1), "tasks_dir"},
		"one folder for both":         {strings.Replace(graderBlock, "/home/op/suites", "/home/op/mimir", 1), "grader"},
		"the suites inside the tasks": {strings.Replace(graderBlock, "/home/op/suites", "/home/op/mimir/suites", 1), "grader"},
		"the tasks inside the suites": {strings.Replace(graderBlock, "/home/op/mimir", "/home/op/suites/tasks", 1), "grader"},
		"a timeout of zero":           {strings.Replace(graderBlock, "30m", "0s", 1), "timeout"},
		"a negative timeout":          {strings.Replace(graderBlock, "30m", "-5m", 1), "timeout"},
		"a timeout with no unit":      {strings.Replace(graderBlock, "30m", "30", 1), "timeout"},
		"no timeout":                  {strings.Replace(graderBlock, "  timeout: 30m\n", "", 1), "timeout"},
		"no memory":                   {strings.Replace(graderBlock, "  memory: 4g\n", "", 1), "memory"},
		"a memory with no unit":       {strings.Replace(graderBlock, "4g", "4", 1), "memory"},
		"no cpus":                     {strings.Replace(graderBlock, "  cpus: 4\n", "", 1), "cpus"},
		"zero cpus":                   {strings.Replace(graderBlock, "cpus: 4", "cpus: 0", 1), "cpus"},
		"no suite folder":             {strings.Replace(graderBlock, "  suite_dir: /home/op/suites\n", "", 1), "suite_dir"},
		"an unknown field":            {graderBlock + "  network: true\n", "network"},
	}
	for name, c := range cases {
		_, err := ParseFactory("factory.yaml", []byte(factoryHead+c.yaml))
		if err == nil {
			t.Errorf("%s: ParseFactory() = nil error", name)
		} else if !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "no complete budget caps") {
			t.Errorf("%s: ParseFactory() = %v, want an error naming %q", name, err, c.want)
		}
	}
}
