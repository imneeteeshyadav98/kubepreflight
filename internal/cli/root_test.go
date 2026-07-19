package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/buildinfo"
)

// TestExitCodeForError_InfraFailureReturnsFour guards the P0 fix directly:
// an infrastructure failure (kubeconfig load, client construction, total
// collector failure — none of which produced a trustworthy report) must
// exit 4, never 1, since 1 is documented as "warnings only" and a CI gate
// reading the exit code alone could otherwise mistake a totally failed
// scan for one that merely found warnings.
func TestExitCodeForError_InfraFailureReturnsFour(t *testing.T) {
	err := infraFailure(errors.New("loading kubeconfig: no such file"))
	if got := exitCodeForError(err, 0); got != 4 {
		t.Errorf("exitCodeForError(infraFailure, 0) = %d, want 4", got)
	}
}

// TestExitCodeForError_OrdinaryErrorStillReturnsOne guards that this fix
// is scoped to infrastructure failures only — an ordinary pre-report
// error (bad flags, an unsupported flag combination) keeps its existing
// exit-1 behavior, matching normal CLI usage-error convention.
func TestExitCodeForError_OrdinaryErrorStillReturnsOne(t *testing.T) {
	err := errors.New("--target-version is required")
	if got := exitCodeForError(err, 0); got != 1 {
		t.Errorf("exitCodeForError(ordinary error, 0) = %d, want 1", got)
	}
}

// TestExitCodeForError_SuccessReturnsReportExitCode guards that a nil
// error (the command ran to completion) still returns whatever
// report-derived exit code (0-3) the subcommand already computed, not a
// hardcoded value.
func TestExitCodeForError_SuccessReturnsReportExitCode(t *testing.T) {
	for _, want := range []int{0, 1, 2, 3} {
		if got := exitCodeForError(nil, want); got != want {
			t.Errorf("exitCodeForError(nil, %d) = %d, want %d", want, got, want)
		}
	}
}

// TestInfraFailure_NilStaysNil guards that wrapping a nil error is a
// no-op, so callers can write `return infraFailure(err)` unconditionally
// without an extra nil check.
func TestInfraFailure_NilStaysNil(t *testing.T) {
	if err := infraFailure(nil); err != nil {
		t.Errorf("infraFailure(nil) = %v, want nil", err)
	}
}

// TestIsInfraFailure_UnwrapsThroughFmtErrorf guards that the marker
// survives being wrapped again by a caller's own fmt.Errorf("...: %w",
// err) — errors.As must still find it via Unwrap, not just direct
// type-assertion.
func TestIsInfraFailure_UnwrapsThroughFmtErrorf(t *testing.T) {
	marked := infraFailure(errors.New("dial tcp: connection refused"))
	wrappedAgain := fmt.Errorf("running scan: %w", marked)
	if !isInfraFailure(wrappedAgain) {
		t.Error("isInfraFailure(wrappedAgain) = false, want true")
	}
	if isInfraFailure(errors.New("plain error")) {
		t.Error("isInfraFailure(plain error) = true, want false")
	}
}

// TestRootCmd_VersionFlagPrintsBanner guards REL-TRUST-001: `--version` is
// Cobra's auto-provided flag (added because root.Version is non-empty),
// wired to print the same three-line banner as the `version` subcommand
// rather than Cobra's one-line default template.
func TestRootCmd_VersionFlagPrintsBanner(t *testing.T) {
	cmd := newRootCmd(new(int))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want success", err)
	}
	if got := out.String(); got != buildinfo.String() {
		t.Errorf("--version output = %q, want %q", got, buildinfo.String())
	}
}

// TestRootCmd_VersionSubcommandPrintsBanner guards that `kubepreflight
// version` prints the identical banner as `--version`, so the two
// surfaces can never drift apart.
func TestRootCmd_VersionSubcommandPrintsBanner(t *testing.T) {
	cmd := newRootCmd(new(int))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want success", err)
	}
	if got := out.String(); got != buildinfo.String() {
		t.Errorf("`version` output = %q, want %q", got, buildinfo.String())
	}
}

// TestRootCmd_HelpUnchanged guards that adding the version command/flag
// didn't disturb ordinary --help behavior (still exits cleanly, still
// lists the existing subcommands).
func TestRootCmd_HelpUnchanged(t *testing.T) {
	cmd := newRootCmd(new(int))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want success", err)
	}
	for _, want := range []string{"scan", "plan", "rollback", "compare", "version"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("--help output missing %q:\n%s", want, out.String())
		}
	}
}
