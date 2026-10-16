package gungnir

import (
	"strings"
	"testing"
)

// The named test of change task remote-creates-repos-T001.
func TestRepoAdminIsHard(t *testing.T) {
	if GateRepoAdmin != "repo-admin" || !GateRepoAdmin.Known() || !GateRepoAdmin.Hard() {
		t.Errorf("repo-admin: known %v, hard %v; want a known, hard approval point", GateRepoAdmin.Known(), GateRepoAdmin.Hard())
	}
}

func TestRepoValidate(t *testing.T) {
	good := Repo{Owner: "ouroboric-dev", Name: "gallery-api", Visibility: VisibilityPrivate,
		Description: "the gallery's API", Path: "repos/api", Files: map[string][]byte{"LICENSE": []byte("MIT\n")}}
	if err := good.Validate(); err != nil {
		t.Fatalf("good repo: %v", err)
	}
	bad := map[string]func(*Repo){
		"no owner":            func(r *Repo) { r.Owner = "" },
		"a name with a slash": func(r *Repo) { r.Name = "a/b" },
		"unknown visibility":  func(r *Repo) { r.Visibility = "internal" },
		"a path that climbs":  func(r *Repo) { r.Path = "../api" },
		"no first commit":     func(r *Repo) { r.Files = nil },
		"a file that climbs":  func(r *Repo) { r.Files = map[string][]byte{"../x": nil} },
		"a file in .git":      func(r *Repo) { r.Files = map[string][]byte{".git/hooks/pre-commit": nil} },
	}
	for name, change := range bad {
		r := good
		change(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
	if err := (Repo{}).Validate(); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Errorf("empty repo: Validate() = %v, want it to name the owner", err)
	}
}
