package gungnir

import "testing"

func TestValidStack(t *testing.T) {
	const pinned = "docker.io/library/golang:1.26.7-bookworm@sha256:e8c859f5632dcfde7b32d2012b4351728f6437930887c2f6a91ea242459e5514"
	for ref, want := range map[string]bool{
		pinned: true,
		"localhost/ygg-stack-midgard-react:0123456789ab":            true,
		"localhost/ygg-stack-midgard-react-acceptance:0123456789ab": true,
		"localhost/ygg-stack-x:latest":                              false,
		"localhost/ygg-stack-X:0123456789ab":                        false,
		"localhost/other:0123456789ab":                              false,
		"docker.io/library/golang:1.26.7":                           false,
		"":                                                          false,
	} {
		if got := ValidStack(ref); got != want {
			t.Errorf("ValidStack(%q) = %v, want %v", ref, got, want)
		}
	}
	spec := SandboxSpec{Run: "r1", Name: "repos/app", Parent: "/p", Leaf: "/p/repos/app", GitDir: "/p/.git/modules/app", Stack: "localhost/ygg-stack-midgard-react:0123456789ab"}
	if err := spec.Validate(); err != nil {
		t.Errorf("a spec with a local stack: %v", err)
	}
	spec.Stack = "localhost/ygg-stack-midgard-react:latest"
	if err := spec.Validate(); err == nil {
		t.Error("a spec with an unpinned local stack was accepted")
	}
}
