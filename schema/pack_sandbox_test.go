package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const digest = "docker.io/library/golang:1.26.7-bookworm@sha256:e8c859f5632dcfde7b32d2012b4351728f6437930887c2f6a91ea242459e5514"

func writePack(t *testing.T, sandbox string) string {
	t.Helper()
	dir := t.TempDir()
	body := "name: toy\nversion: 1.0.0\ngraph: graph.yaml\n" + sandbox
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPackSandbox(t *testing.T) {
	p, err := LoadPack(writePack(t, "sandbox:\n  stack: "+digest+"\n  egress: [proxy.golang.org, sum.golang.org]\n"))
	if err != nil || p.Sandbox == nil || p.Sandbox.Stack != digest || len(p.Sandbox.Egress) != 2 {
		t.Fatalf("LoadPack() = %+v, %v", p, err)
	}
	if p, err := LoadPack(writePack(t, "")); err != nil || p.Sandbox != nil {
		t.Errorf("a pack without a sandbox: %+v, %v", p, err)
	}
	for name, sb := range map[string]string{
		"a stack without a digest": "sandbox:\n  stack: docker.io/library/golang:1.26.7-bookworm\n",
		"no stack":                 "sandbox:\n  egress: [proxy.golang.org]\n",
		"an egress URL":            "sandbox:\n  stack: " + digest + "\n  egress: [\"https://proxy.golang.org\"]\n",
		"an egress address":        "sandbox:\n  stack: " + digest + "\n  egress: [\"10.0.0.1\"]\n",
	} {
		if _, err := LoadPack(writePack(t, sb)); err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		} else if strings.Contains(err.Error(), "no such file") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
