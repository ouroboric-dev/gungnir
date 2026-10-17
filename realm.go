package gungnir

import "regexp"

var (
	realmName    = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	realmVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
)

// ValidRealm reports whether s names a realm leaf: lower case words joined by dashes, such as
// midgard-react (change realm-leaf-naming).
func ValidRealm(s string) bool { return realmName.MatchString(s) }

// ValidVersion reports whether s is a version of the form 1.2.3.
func ValidVersion(s string) bool { return realmVersion.MatchString(s) }
