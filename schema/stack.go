package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ouroboric-dev/gungnir"
)

// A build context is read whole into a hash and into an image, so it is bounded.
const (
	MaxContextFiles = 500
	MaxContextBytes = 64 << 20
)

// PackStack is the stack of a sandbox in one of its two forms: an image pinned by digest, or a build
// context in the realm, a folder with a Containerfile that `ygg sandbox build` builds (change realm-content).
type PackStack struct {
	Stack  string `yaml:"stack"`  // an image pinned by digest
	Build  string `yaml:"build"`  // a folder of the realm: the build context, with a Containerfile at its top
	Target string `yaml:"target"` // with build: the stage of the Containerfile to build; empty is its last stage
}

// StackRef returns the image leaves of this realm start on: the pinned image, or the local tag that a
// build of the realm's context gets. dir is the realm's folder. It is "" for a pack with no sandbox.
func (p *Pack) StackRef(dir string) (string, error) {
	if p.Sandbox == nil {
		return "", nil
	}
	return stackRef(p.Name, PackStack{Stack: p.Sandbox.Stack, Build: p.Sandbox.Build, Target: p.Sandbox.Target}, dir)
}

// AcceptanceRef is StackRef for the stack that acceptance checks run on, in a sandbox that holds a
// snapshot of the whole project. It is "" when the pack names none.
func (p *Pack) AcceptanceRef(dir string) (string, error) {
	if p.Sandbox == nil || p.Sandbox.Acceptance == nil {
		return "", nil
	}
	return stackRef(p.Name+"-acceptance", *p.Sandbox.Acceptance, dir)
}

func stackRef(name string, s PackStack, dir string) (string, error) {
	if s.Build == "" {
		return s.Stack, nil
	}
	h, err := StackHash(filepath.Join(dir, s.Build), s.Target)
	if err != nil {
		return "", err
	}
	return gungnir.LocalStack(name, h), nil
}

// StackHash is the hash in the tag of a stack built from the context at dir up to target: the context's
// hash (HashContext) and the stage built, so two stages of one Containerfile are two images.
func StackHash(dir, target string) (string, error) {
	h, err := HashContext(dir)
	if err != nil {
		return "", err
	}
	if target == "" {
		return h, nil
	}
	sum := sha256.Sum256([]byte(h + "\x00" + target))
	return hex.EncodeToString(sum[:])[:12], nil
}

// HashContext returns the first 12 hex digits of a hash of every file of the build context at dir: their
// names, whether each is executable, and their contents. It refuses a context without a Containerfile at
// its top, one with a symlink or any file that is not a regular file, and one over the limits.
func HashContext(dir string) (string, error) {
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("the build context %s must be a folder of the realm", dir)
	}
	if st, err := os.Lstat(filepath.Join(dir, "Containerfile")); err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("the build context %s has no Containerfile (it must be a regular file)", dir)
	}
	h := sha256.New()
	files, total := 0, int64(0)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		st, err := d.Info()
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("the build context holds %s, which is not a regular file; a symlink could lead the build outside the realm", rel)
		}
		files++
		total += st.Size()
		if files > MaxContextFiles || total > MaxContextBytes {
			return fmt.Errorf("the build context is over its limits (%d files, %d MiB)", MaxContextFiles, MaxContextBytes>>20)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fmt.Fprintf(h, "%s\x00%v\x00%d\x00", filepath.ToSlash(rel), st.Mode()&0o111 != 0, st.Size())
		_, err = io.Copy(h, f)
		return err
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}
