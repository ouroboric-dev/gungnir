package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ouroboric-dev/gungnir"
)

func TestPackNeedsAndProvides(t *testing.T) {
	services := "services:\n  postgres:\n    image: " + "docker.io/library/postgres:18.0-bookworm@sha256:" + strings.Repeat("a", 64) + "\n    ports: [5432]\n" +
		"  s3:\n    image: docker.io/chrislusf/seaweedfs:4.45@sha256:" + strings.Repeat("b", 64) + "\n    ports: [8333]\n"
	load := func(t *testing.T, extra string) (*Pack, error) {
		t.Helper()
		return LoadPack(writePack(t, extra))
	}

	p, err := load(t, "needs: [postgres, s3]\n")
	if err != nil || len(p.Needs) != 2 || p.Needs[0] != "postgres" || p.Needs[1] != "s3" {
		t.Fatalf("needs: LoadPack() = %+v, %v", p, err)
	}
	for name, c := range map[string]struct{ body, want string }{
		"an unknown need":                   {"needs: [redis]\n", "redis"},
		"a need named twice":                {"needs: [s3, postgres, s3]\n", "s3"},
		"an empty need":                     {"needs: [\"\"]\n", "needs"},
		"needs as a string":                 {"needs: postgres\n", "needs"},
		"provides an unknown":               {services + "provides: {redis: postgres}\n", "redis"},
		"provides a service the pack lacks": {services + "provides: {postgres: mysql}\n", "mysql"},
		"provides another name":             {services + "provides: {s3: postgres}\n", "s3"},
		"provides with no services":         {"provides: {postgres: postgres}\n", "postgres"},
		"provides an empty service":         {services + "provides: {postgres: \"\"}\n", "provides"},
	} {
		if _, err := load(t, c.body); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: LoadPack() = %v, want an error naming %q", name, err, c.want)
		}
	}

	p, err = load(t, services+"provides: {postgres: postgres, s3: s3}\n")
	if err != nil {
		t.Fatalf("provides: LoadPack() = %v", err)
	}
	if svc, ok := p.Provider("s3"); !ok || svc.Name != "s3" || svc.Ports[0] != 8333 {
		t.Errorf("Provider(s3) = %+v, %v", svc, ok)
	}
	if _, ok := p.Provider("redis"); ok {
		t.Error("Provider(redis) = ok, want none")
	}
	if p, err := load(t, services+"provides: {s3: s3}\n"); err != nil {
		t.Errorf("a partial provides: %v", err)
	} else if _, ok := p.Provider("postgres"); ok {
		t.Error("Provider(postgres) = ok for a pack that does not provide it")
	}

	// A runtime realm provides no leaf.
	dir := writePack(t, services+"provides: {s3: s3}\nproduct:\n  start: start.sh\n")
	if err := os.WriteFile(filepath.Join(dir, "start.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPack(dir); err != nil {
		t.Fatalf("provides and product: %v", err)
	}
	for name, body := range map[string]string{
		"leaf and provides": "provides: {s3: s3}\n",
		"leaf and product":  "product:\n  start: start.sh\n",
	} {
		dir := writePack(t, services+body+"leaf:\n  description: a leaf\n  template: tpl\n  first_commit: [a]\n  implementer: impl.md\n  checks: [c]\nchecks:\n  c: [\"true\"]\n")
		if err := os.MkdirAll(filepath.Join(dir, "tpl"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"tpl/a", "impl.md", "start.sh"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := LoadPack(dir); err == nil || !strings.Contains(err.Error(), "a runtime realm provides no leaf") {
			t.Errorf("%s: LoadPack() = %v, want \"a runtime realm provides no leaf\"", name, err)
		}
	}

	if _, err := load(t, services+"provides: {s3: s3}\nsandbox:\n  stack: "+digest+"\n"); err == nil || !strings.Contains(err.Error(), "a runtime realm provides no leaf and no stack") {
		t.Errorf("provides and sandbox: LoadPack() = %v, want \"a runtime realm provides no leaf and no stack\"", err)
	}
	if _, err := load(t, "product:\n  start: start.sh\nsandbox:\n  stack: "+digest+"\n"); err == nil || !strings.Contains(err.Error(), "no stack") {
		t.Errorf("product and sandbox: LoadPack() = %v, want the no-stack refusal", err)
	}

	if p, err := load(t, ""); err != nil || p.Needs != nil || p.Provides != nil {
		t.Errorf("a pack with neither field: %+v, %v", p, err)
	}
	if len(gungnir.Needs) != 2 || !gungnir.ValidNeed("postgres") || !gungnir.ValidNeed("s3") || gungnir.ValidNeed("redis") {
		t.Errorf("gungnir.Needs = %v", gungnir.Needs)
	}
}
