package gungnir

// Need is a service a build realm's leaves need from the realm that runs the product (a runtime realm).
type Need struct {
	Name        string // the name a pack's needs and provides use
	Description string
}

// Needs lists every need a pack may name, in the order a listing shows them.
var Needs = []Need{
	{Name: "postgres", Description: "a Postgres-compatible database"},
	{Name: "s3", Description: "an S3 API"},
}

// ValidNeed reports whether name is in Needs.
func ValidNeed(name string) bool {
	for _, n := range Needs {
		if n.Name == name {
			return true
		}
	}
	return false
}
