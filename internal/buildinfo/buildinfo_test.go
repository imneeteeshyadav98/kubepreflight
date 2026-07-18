package buildinfo

import "testing"

// TestString_ContainsSafeDefaults guards the local-build fallback: with no
// -ldflags set (as in `go test`), String() must fall back to the
// documented dev/unknown placeholders rather than an empty or malformed
// string.
func TestString_ContainsSafeDefaults(t *testing.T) {
	want := "KubePreflight dev\ncommit: unknown\nbuilt: unknown\n"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
