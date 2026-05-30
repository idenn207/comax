package version

import "testing"

// These tests intentionally do NOT run with t.Parallel(): they share the
// package-level Version variable and the override case mutates it.

func TestStringReturnsNonEmpty(t *testing.T) {
	if got := String(); got == "" {
		t.Fatalf("String() returned empty string; want non-empty default")
	}
}

func TestStringHonoursOverride(t *testing.T) {
	prev := Version
	t.Cleanup(func() { Version = prev })

	Version = "v1.2.3"
	if got, want := String(), "v1.2.3"; got != want {
		t.Fatalf("String() = %q; want %q", got, want)
	}
}
