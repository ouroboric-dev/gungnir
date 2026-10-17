package gungnir

import (
	"strings"
	"testing"
)

func TestAServiceMayNameItsCommandAsAnArgumentVector(t *testing.T) {
	ok := Service{Name: "seaweedfs", Image: pg, Ports: []int{8333}, Command: []string{"server", "-s3", "-dir=/data"}}
	if err := ValidateServices([]Service{ok}); err != nil {
		t.Fatalf("ValidateServices() = %v", err)
	}
	long := make([]string, MaxServiceCommand+1)
	for i := range long {
		long[i] = "x"
	}
	bad := map[string][]string{
		"an empty argument":          {"server", ""},
		"an argument with a dollar":  {"server", "-dir=$HOME"},
		"an argument with a NUL":     {"server\x00x"},
		"an argument with a newline": {"server\n-s3"},
		"an argument too long":       {strings.Repeat("a", MaxServiceArg+1)},
		"too many arguments":         long,
	}
	for name, cmd := range bad {
		s := ok
		s.Command = cmd
		err := ValidateServices([]Service{s})
		if err == nil {
			t.Errorf("%s: ValidateServices() = nil, want an error", name)
		} else if strings.Contains(err.Error(), "$HOME") {
			t.Errorf("%s: the error echoes the argument: %v", name, err)
		}
	}
}
