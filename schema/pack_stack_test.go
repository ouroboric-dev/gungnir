package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// writeContext writes files (path: body) under dir/devcontainer, with a Containerfile unless one is given.
func writeContext(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if _, ok := files["Containerfile"]; !ok {
		files["Containerfile"] = "FROM " + digest + "\n"
	}
	for name, body := range files {
		p := filepath.Join(dir, "devcontainer", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

var localStack = regexp.MustCompile(`^localhost/ygg-stack-toy:[0-9a-f]{12}$`)

func TestPackSandboxTakesABuildContextOrAPinnedImage(t *testing.T) {
	dir := writePack(t, "sandbox:\n  build: devcontainer\n  egress: [proxy.golang.org]\n")
	writeContext(t, dir, map[string]string{"checks/go-unit.sh": "#!/bin/sh\n"})
	p, err := LoadPack(dir)
	if err != nil || p.Sandbox == nil || p.Sandbox.Build != "devcontainer" || p.Sandbox.Stack != "" {
		t.Fatalf("LoadPack(build form) = %+v, %v", p, err)
	}
	ref, err := p.StackRef(dir)
	if err != nil || !localStack.MatchString(ref) {
		t.Fatalf("StackRef(build form) = %q, %v; want a local tag", ref, err)
	}
	pinned, err := LoadPack(writePack(t, "sandbox:\n  stack: "+digest+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if ref, err := pinned.StackRef(t.TempDir()); err != nil || ref != digest {
		t.Errorf("StackRef(digest form) = %q, %v; want %q", ref, err, digest)
	}
	for name, sb := range map[string]string{
		"both forms": "sandbox:\n  stack: " + digest + "\n  build: devcontainer\n",
		"neither":    "sandbox:\n  egress: [proxy.golang.org]\n",
	} {
		if _, err := LoadPack(writePack(t, sb)); err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		}
	}
}

func TestAStackTagFollowsTheContext(t *testing.T) {
	refOf := func(files map[string]string, mode os.FileMode) string {
		dir := writePack(t, "sandbox:\n  build: devcontainer\n")
		writeContext(t, dir, files)
		if mode != 0 {
			if err := os.Chmod(filepath.Join(dir, "devcontainer", "run.sh"), mode); err != nil {
				t.Fatal(err)
			}
		}
		p, err := LoadPack(dir)
		if err != nil {
			t.Fatal(err)
		}
		ref, err := p.StackRef(dir)
		if err != nil {
			t.Fatal(err)
		}
		return ref
	}
	base := refOf(map[string]string{"run.sh": "echo a\n"}, 0)
	if again := refOf(map[string]string{"run.sh": "echo a\n"}, 0); again != base {
		t.Errorf("the same context gave %q and %q", base, again)
	}
	if other := refOf(map[string]string{"run.sh": "echo b\n"}, 0); other == base {
		t.Error("a changed file gave the same tag")
	}
	if other := refOf(map[string]string{"run.sh": "echo a\n"}, 0o755); other == base {
		t.Error("a file made executable gave the same tag")
	}
	if other := refOf(map[string]string{"run.sh": "echo a\n", "more": ""}, 0); other == base {
		t.Error("an added file gave the same tag")
	}
}

func TestABuildContextIsRefusedWhenItCouldReachOutsideItself(t *testing.T) {
	fill := func(t *testing.T, dir string, n, size int) {
		for i := 0; i <= n; i++ {
			if err := os.WriteFile(filepath.Join(dir, "devcontainer", fmt.Sprintf("f%d", i)), make([]byte, size), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cases := map[string]func(t *testing.T, dir string){
		"no Containerfile": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "devcontainer", "Containerfile")); err != nil {
				t.Fatal(err)
			}
		},
		"a symlink in the context": func(t *testing.T, dir string) {
			if err := os.Symlink("/etc/passwd", filepath.Join(dir, "devcontainer", "passwd")); err != nil {
				t.Fatal(err)
			}
		},
		"too many files":                func(t *testing.T, dir string) { fill(t, dir, MaxContextFiles, 0) },
		"a context bigger than allowed": func(t *testing.T, dir string) { fill(t, dir, MaxContextBytes>>20, 1<<20) },
	}
	for name, mutate := range cases {
		dir := writePack(t, "sandbox:\n  build: devcontainer\n")
		writeContext(t, dir, map[string]string{})
		mutate(t, dir)
		if _, err := LoadPack(dir); err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		}
	}
	out := t.TempDir() // a valid context beside the realm: only its place is wrong
	writeContext(t, out, map[string]string{})
	for _, build := range []string{"../" + filepath.Base(out) + "/devcontainer", "/etc", "."} {
		dir := writePack(t, "sandbox:\n  build: "+build+"\n")
		if _, err := LoadPack(dir); err == nil {
			t.Errorf("build %q: LoadPack() = nil, want an error", build)
		}
	}
}

func TestAnAcceptanceStackIsAnImageOrABuildToo(t *testing.T) {
	dir := writePack(t, "sandbox:\n  build: devcontainer\n  acceptance:\n    build: devcontainer\n    target: acceptance\n")
	writeContext(t, dir, map[string]string{})
	p, err := LoadPack(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := p.AcceptanceRef(dir)
	lead, _ := p.StackRef(dir)
	if err != nil || ref == lead || !regexp.MustCompile(`^localhost/ygg-stack-toy-acceptance:[0-9a-f]{12}$`).MatchString(ref) {
		t.Errorf("AcceptanceRef() = %q, %v; the lead stack is %q", ref, err, lead)
	}
	plain, err := LoadPack(writePack(t, "sandbox:\n  stack: "+digest+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if ref, err := plain.AcceptanceRef(t.TempDir()); err != nil || ref != "" {
		t.Errorf("AcceptanceRef() with none = %q, %v; want empty", ref, err)
	}
}

func TestATargetNamesAStageOfTheContextAndIsPartOfTheTag(t *testing.T) {
	refs := map[string]string{}
	for _, target := range []string{"", "web", "acceptance"} {
		sb := "sandbox:\n  build: devcontainer\n"
		if target != "" {
			sb += "  target: " + target + "\n"
		}
		dir := writePack(t, sb)
		writeContext(t, dir, map[string]string{"checks/go-unit.sh": "#!/bin/sh\n"})
		p, err := LoadPack(dir)
		if err != nil {
			t.Fatalf("target %q: %v", target, err)
		}
		ref, err := p.StackRef(dir)
		if err != nil {
			t.Fatal(err)
		}
		refs[target] = ref
	}
	if refs[""] == refs["web"] || refs["web"] == refs["acceptance"] || refs[""] == refs["acceptance"] {
		t.Errorf("two stages of one context share a tag: %v", refs)
	}
	for name, sb := range map[string]string{
		"a target with an image":              "sandbox:\n  stack: " + digest + "\n  target: web\n",
		"an acceptance target with an image":  "sandbox:\n  build: devcontainer\n  acceptance:\n    stack: " + digest + "\n    target: web\n",
		"a target that is not a stage name":   "sandbox:\n  build: devcontainer\n  target: \"Web Stage\"\n",
		"an acceptance stack of neither form": "sandbox:\n  stack: " + digest + "\n  acceptance: {}\n",
	} {
		d := writePack(t, sb)
		writeContext(t, d, map[string]string{})
		if _, err := LoadPack(d); err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		}
	}
}
