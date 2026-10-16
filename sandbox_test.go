package gungnir

import (
	"context"
	"testing"
)

func validSpec() SandboxSpec {
	return SandboxSpec{
		Run: "20261016-101500-a1b2c3", Name: "repos/app",
		Parent: "/work/project", Leaf: "/work/project/repos/app", GitDir: "/work/project/.git/modules/repos/app",
		Egress: []string{"api.anthropic.com", "proxy.golang.org"},
	}
}

func TestSandboxSpecValidate(t *testing.T) {
	if err := validSpec().Validate(); err != nil {
		t.Fatalf("good spec: %v", err)
	}
	bad := map[string]func(*SandboxSpec){
		"no run":                         func(s *SandboxSpec) { s.Run = "" },
		"a run id with a slash":          func(s *SandboxSpec) { s.Run = "a/b" },
		"a name that climbs":             func(s *SandboxSpec) { s.Name = "../app" },
		"a relative leaf":                func(s *SandboxSpec) { s.Leaf = "repos/app" },
		"a leaf outside the parent":      func(s *SandboxSpec) { s.Leaf = "/elsewhere/app" },
		"a git dir outside .git/modules": func(s *SandboxSpec) { s.GitDir = "/work/project/repos/app/.git" },
		"an egress URL":                  func(s *SandboxSpec) { s.Egress = []string{"https://api.anthropic.com"} },
		"an egress wildcard":             func(s *SandboxSpec) { s.Egress = []string{"*.example.com"} },
		"an egress with a port":          func(s *SandboxSpec) { s.Egress = []string{"example.com:22"} },
		"an egress IP address":           func(s *SandboxSpec) { s.Egress = []string{"10.0.0.1"} },
	}
	for name, change := range bad {
		s := validSpec()
		change(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}

func TestSandboxLayoutMirrorsTheUmbrella(t *testing.T) {
	l := validSpec().Layout()
	if l.Leaf != "repos/app" || l.GitDir != ".git/modules/repos/app" {
		t.Errorf("Layout() = %+v, want the leaf and git dir relative to the parent umbrella", l)
	}
}

type fakeSandbox struct{ got Cmd }

func (f *fakeSandbox) Up(context.Context, SandboxSpec) (Env, error) { return Env{}, nil }
func (f *fakeSandbox) Exec(_ context.Context, _ Env, c Cmd) (ExecResult, error) {
	f.got = c
	return ExecResult{ExitCode: 3}, nil
}
func (f *fakeSandbox) Down(context.Context, Env) error { return nil }

func TestBindRunsInThatEnvironment(t *testing.T) {
	f := &fakeSandbox{}
	r := Bind(f, Env{Container: "c1"})
	res, err := r.Run(context.Background(), Cmd{Argv: []string{"true"}})
	if err != nil || res.ExitCode != 3 || len(f.got.Argv) != 1 {
		t.Errorf("Run() = %+v, %v; the command did not reach the sandbox", res, err)
	}
	if err := (Cmd{}).Validate(); err == nil {
		t.Error("an empty command validated")
	}
}
