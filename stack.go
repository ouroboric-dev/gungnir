package gungnir

import "regexp"

// localStack is the tag Odin gives the image it builds from a realm's build context: the realm's name and
// a hash of every file of the context, so a changed context is a new image and never a changed one.
var localStack = regexp.MustCompile(`^localhost/ygg-stack-[a-z][a-z0-9-]*:[0-9a-f]{12}$`)

// ValidStack reports whether ref names a stack a sandbox may start on: an image pinned by digest, or the
// local tag of an image built from a realm's build context.
func ValidStack(ref string) bool { return ValidImage(ref) || localStack.MatchString(ref) }

// LocalStack returns the tag of the image built from a context whose hash is hash, for the realm name
// (with "-acceptance" added for a realm's acceptance stack).
func LocalStack(name, hash string) string { return "localhost/ygg-stack-" + name + ":" + hash }
