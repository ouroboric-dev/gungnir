package gungnir

import (
	"slices"
	"strings"
	"testing"
)

const pg = "docker.io/library/postgres:17.2@sha256:1111111111111111111111111111111111111111111111111111111111111111"

func specWithServices() SandboxSpec {
	s := validSpec()
	s.Stack = "docker.io/library/golang:1.26.7-bookworm@sha256:e8c859f5632dcfde7b32d2012b4351728f6437930887c2f6a91ea242459e5514"
	s.Services = []Service{{Name: "postgres", Image: pg, Env: map[string]string{"POSTGRES_DB": "app"}, Ports: []int{5432}}}
	return s
}

func TestASandboxSpecMayCarryAStackAndServices(t *testing.T) {
	if err := specWithServices().Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	bad := map[string]func(*SandboxSpec){
		"a stack with no digest":                         func(s *SandboxSpec) { s.Stack = "docker.io/library/golang:1.26.7" },
		"a service with no digest":                       func(s *SandboxSpec) { s.Services[0].Image = "docker.io/library/postgres:17.2" },
		"a service named work":                           func(s *SandboxSpec) { s.Services[0].Name = "work" },
		"a service named fw":                             func(s *SandboxSpec) { s.Services[0].Name = "fw" },
		"a service name in capitals":                     func(s *SandboxSpec) { s.Services[0].Name = "Postgres" },
		"a service twice":                                func(s *SandboxSpec) { s.Services = append(s.Services, s.Services[0]) },
		"a service with no port":                         func(s *SandboxSpec) { s.Services[0].Ports = nil },
		"a port out of range":                            func(s *SandboxSpec) { s.Services[0].Ports = []int{70000} },
		"a port twice":                                   func(s *SandboxSpec) { s.Services[0].Ports = []int{5432, 5432} },
		"an env name with an equals":                     func(s *SandboxSpec) { s.Services[0].Env = map[string]string{"A=B": "c"} },
		"an empty env name":                              func(s *SandboxSpec) { s.Services[0].Env = map[string]string{"": "c"} },
		"an env that names a credential":                 func(s *SandboxSpec) { s.Services[0].Env = map[string]string{"POSTGRES_PASSWORD": "dev"} },
		"an env value that expands from the environment": func(s *SandboxSpec) { s.Services[0].Env = map[string]string{"POSTGRES_DB": "${HOME}"} },
		"an env value with a bare dollar variable":       func(s *SandboxSpec) { s.Services[0].Env = map[string]string{"POSTGRES_DB": "$HOME"} },
		"nine services":                                  func(s *SandboxSpec) { s.Services = manyServices(9) },
	}
	for name, change := range bad {
		s := specWithServices()
		s.Services = slices.Clone(s.Services)
		change(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
	if err := func() error { s := specWithServices(); s.Services = manyServices(8); return s.Validate() }(); err != nil {
		t.Errorf("eight services = %v", err)
	}
}

func TestAServiceEnvValueWithADollarIsRefusedWithoutEchoingIt(t *testing.T) {
	err := ValidateServices([]Service{{Name: "postgres", Image: pg, Env: map[string]string{"POSTGRES_DB": "pre-${SECRET_VALUE}"}, Ports: []int{5432}}})
	if err == nil {
		t.Fatal("ValidateServices() = nil, want an error")
	}
	for _, want := range []string{"postgres", "POSTGRES_DB"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "SECRET_VALUE") {
		t.Errorf("error %q echoes the value", err)
	}
}

func manyServices(n int) []Service {
	var out []Service
	for i := 0; i < n; i++ {
		out = append(out, Service{Name: "svc" + string(rune('a'+i)), Image: pg, Ports: []int{8000 + i}})
	}
	return out
}

func TestAScratchSandboxHoldsOneFolderAndNoGitDir(t *testing.T) {
	s := SandboxSpec{Run: "20261016-101500-a1b2c3", Name: "scratch/design", Scratch: true,
		Parent: "/work/project/.factory/runs/r1", Leaf: "/work/project/.factory/runs/r1/scratch/design"}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if l := s.Layout(); l.Leaf != "scratch/design" || l.GitDir != "" {
		t.Errorf("Layout() = %+v", l)
	}
	for name, change := range map[string]func(*SandboxSpec){
		"a git dir":                   func(s *SandboxSpec) { s.GitDir = "/work/project/.git/modules/x" },
		"a folder outside the parent": func(s *SandboxSpec) { s.Leaf = "/elsewhere" },
		"a relative folder":           func(s *SandboxSpec) { s.Leaf = "scratch/design" },
	} {
		c := s
		change(&c)
		if c.Validate() == nil {
			t.Errorf("%s: Validate() = nil", name)
		}
	}
	notScratch := validSpec()
	notScratch.GitDir = ""
	if notScratch.Validate() == nil {
		t.Error("a leaf sandbox with no git dir validated")
	}
}

func TestValidImagePinsByDigest(t *testing.T) {
	for img, want := range map[string]bool{
		pg: true, "ghcr.io/x/y@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789": true,
		"postgres:17": false, "": false, "docker.io/library/postgres@sha256:short": false, "Docker.io/x@sha256:" + "1111111111111111111111111111111111111111111111111111111111111111": false,
	} {
		if got := ValidImage(img); got != want {
			t.Errorf("ValidImage(%q) = %v, want %v", img, got, want)
		}
	}
}
