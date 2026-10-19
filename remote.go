package gungnir

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Remote creates and protects the repos of an instance tree (spec §2.2; change
// remote-creates-repos). v0 ships the local remote, which keeps every repo on this host; the github
// remote is iteration 2 (spec §15.1). Committing, branching, merging and moving pointers on the
// local tree are Odin's own host git; the github remote adds push, pull requests and merges.
type Remote interface {
	Name() string
	// Create makes repo r with one commit holding r.Files and nothing else. It refuses, and
	// creates nothing, if the repo exists. On a hosted remote it is a repo-admin action.
	Create(ctx context.Context, r Repo) error
	// Protect applies the standard settings and the protect-main ruleset, fixing drift. It is safe
	// to repeat. applied is false when the remote has nothing to protect (the local remote).
	Protect(ctx context.Context, r Repo) (applied bool, err error)
}

// Visibility is who can see a repo.
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

// Repo is a repo a Remote creates or protects.
type Repo struct {
	Owner       string            // the org or account
	Name        string            // the repo's name on the remote
	Visibility  Visibility        // private or public
	Description string            // one line
	Path        string            // where its working copy goes, relative to its parent umbrella
	Files       map[string][]byte // the first commit: path in the repo to content
	// Realm and RealmVersion pin the realm leaf that provides the new leaf's recipe, and Consumes lists
	// the sibling leaves it calls (paths from the parent umbrella). A remote records them with the
	// leaf, in the same step, so a leaf never exists without its pin. A Repo with none uses the
	// graph pack's recipe, if it holds one.
	Realm        string
	RealmVersion string
	Consumes     []string
}

var repoName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Validate refuses a repo that a remote couldn't create exactly as described.
func (r Repo) Validate() error {
	var errs []error
	if !repoName.MatchString(r.Owner) {
		errs = append(errs, fmt.Errorf("owner %q is not an org or account name", r.Owner))
	}
	if !repoName.MatchString(r.Name) {
		errs = append(errs, fmt.Errorf("name %q is not a repo name", r.Name))
	}
	if r.Visibility != VisibilityPrivate && r.Visibility != VisibilityPublic {
		errs = append(errs, fmt.Errorf("visibility %q is neither private nor public", r.Visibility))
	}
	if !filepath.IsLocal(r.Path) {
		errs = append(errs, fmt.Errorf("path %q must be a relative path inside the parent umbrella", r.Path))
	}
	if len(r.Files) == 0 {
		errs = append(errs, errors.New("the first commit needs at least one file"))
	}
	for p := range r.Files {
		if !filepath.IsLocal(p) || p == ".git" || strings.HasPrefix(p, ".git/") {
			errs = append(errs, fmt.Errorf("first-commit file %q must be a relative path outside .git", p))
		}
	}
	switch {
	case r.Realm == "" && r.RealmVersion == "":
	case r.Realm == "" || r.RealmVersion == "":
		errs = append(errs, errors.New("a realm pin is a realm and its version, together"))
	case !ValidRealm(r.Realm):
		errs = append(errs, fmt.Errorf("realm %q is not a realm name", r.Realm))
	case !ValidVersion(r.RealmVersion):
		errs = append(errs, fmt.Errorf("realm version %q is not a version such as 1.2.3", r.RealmVersion))
	}
	seen := map[string]bool{}
	for _, c := range r.Consumes {
		switch {
		case !filepath.IsLocal(c):
			errs = append(errs, fmt.Errorf("consumed leaf %q must be a relative path inside the parent umbrella", c))
		case c == r.Path:
			errs = append(errs, fmt.Errorf("leaf %s can't consume itself", c))
		case seen[c]:
			errs = append(errs, fmt.Errorf("consumed leaf %s is listed twice", c))
		}
		seen[c] = true
	}
	return errors.Join(errs...)
}
