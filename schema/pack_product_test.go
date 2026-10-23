package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackProductStart(t *testing.T) {
	pack := func(t *testing.T, product string) string {
		t.Helper()
		dir := writePack(t, product)
		if err := os.MkdirAll(filepath.Join(dir, "templates", "product"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "templates", "product", "start.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "adir"), 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	p, err := LoadPack(pack(t, "product:\n  start: templates/product/start.sh\n"))
	if err != nil || p.Product == nil || p.Product.Start != "templates/product/start.sh" {
		t.Fatalf("LoadPack() = %+v, %v", p, err)
	}
	if p, err := LoadPack(pack(t, "")); err != nil || p.Product != nil {
		t.Errorf("a pack without product: %+v, %v", p, err)
	}
	// These name a file that exists, so only the path check refuses them.
	dir := pack(t, "")
	for name, product := range map[string]string{
		"an absolute path": "product:\n  start: " + filepath.Join(dir, "templates", "product", "start.sh") + "\n",
		"a path with ..":   "product:\n  start: ../" + filepath.Base(dir) + "/templates/product/start.sh\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte("name: toy\nversion: 1.0.0\n"+product), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPack(dir); err == nil || !strings.Contains(err.Error(), "product.start") {
			t.Errorf("%s: LoadPack() = %v, want an error naming product.start", name, err)
		}
	}
	for name, product := range map[string]string{
		"an empty start":     "product:\n  start: \"\"\n",
		"no start":           "product:\n  {}\n",
		"an unknown key":     "product:\n  start: templates/product/start.sh\n  stop: x\n",
		"a file not in pack": "product:\n  start: templates/product/missing.sh\n",
		"a directory":        "product:\n  start: adir\n",
	} {
		_, err := LoadPack(pack(t, product))
		if err == nil {
			t.Errorf("%s: LoadPack() = nil, want an error", name)
		} else if !strings.Contains(err.Error(), "product") {
			t.Errorf("%s: %v; want the field in the message", name, err)
		}
	}
}
