// Package buildinfo holds the release identity injected into the binary at
// build time via -ldflags (see .github/workflows/release.yml). The
// defaults below are what an ordinary `go build`/`go run` gets, since no
// linker flags are set for local or test builds.
package buildinfo

import "fmt"

var (
	// Version is the released version string, e.g. "0.16.0" or
	// "0.16.0-redaction".
	Version = "dev"
	// Commit is the Git SHA the binary was built from.
	Commit = "unknown"
	// BuildDate is the RFC3339 build timestamp.
	BuildDate = "unknown"
)

// String renders the version banner shared by `kubepreflight version` and
// `kubepreflight --version`, so both surfaces always agree.
func String() string {
	return fmt.Sprintf("KubePreflight %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildDate)
}
