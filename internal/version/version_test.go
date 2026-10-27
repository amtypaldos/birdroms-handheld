package version

import "testing"

func TestStringIncludesNameAndVersion(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "1.2.3"
	if got, want := String(), "birdROMs 1.2.3"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
