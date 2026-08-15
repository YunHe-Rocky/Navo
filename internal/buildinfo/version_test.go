package buildinfo

import "testing"

func TestNormalizedVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	Version = " 1.0.29 "
	if got := NormalizedVersion(); got != "1.0.29" {
		t.Fatalf("NormalizedVersion() = %q", got)
	}

	Version = " "
	if got := NormalizedVersion(); got != "dev" {
		t.Fatalf("empty NormalizedVersion() = %q", got)
	}
}
