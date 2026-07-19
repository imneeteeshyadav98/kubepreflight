// Package cli wires the Cobra command tree for the kubepreflight binary.
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/imneeteeshyadav98/kubepreflight/internal/buildinfo"
)

// Execute builds and runs the root command, returning the process exit
// code — the documented CI exit-code contract:
//
//	0 clean
//	1 warnings only
//	2 blockers present
//	3 incomplete coverage / partial evidence
//	4 scan infrastructure failure — no trustworthy report was produced
//
// Codes 0-3 are report-derived (see findings.Report.ExitCode) and are only
// reachable once a report actually exists. Code 4 is for everything
// upstream of that — a failure while merely trying to collect evidence
// (bad kubeconfig, can't construct a Kubernetes client, the collector
// itself failed outright) — which must never collide with 1's documented
// "warnings only" meaning. Any other pre-report error (bad flags, an
// unsupported flag combination) still exits 1, matching ordinary CLI
// usage-error convention. Called from cmd/kubepreflight, which is
// expected to os.Exit with the returned code.
func Execute() int {
	exitCode := 0
	root := newRootCmd(&exitCode)
	return exitCodeForError(root.Execute(), exitCode)
}

// newRootCmd builds the full command tree. Split out from Execute so the
// root command — including its Cobra-provided --version flag and the
// `version` subcommand — is directly unit-testable via SetArgs/SetOut,
// the same pattern every other subcommand test in this package already
// uses, rather than only reachable through os.Args.
func newRootCmd(exitCode *int) *cobra.Command {
	root := &cobra.Command{
		Use:          "kubepreflight",
		Short:        "Know what will break before your Kubernetes or EKS upgrade",
		Long:         "KubePreflight is a read-only CLI that correlates deprecated APIs, extension API health, admission webhooks, PodDisruptionBudgets, node/kubelet skew, and optional EKS provider constraints into a go/no-go upgrade readiness report.",
		SilenceUsage: true,
		Version:      buildinfo.Version,
	}
	// Same banner for `--version` and `version` — see newVersionCmd.
	root.SetVersionTemplate(buildinfo.String())
	root.AddCommand(newScanCmd(exitCode))
	root.AddCommand(newPlanCmd(exitCode))
	root.AddCommand(newRollbackCmd(exitCode))
	root.AddCommand(newCompareCmd(exitCode))
	root.AddCommand(newVersionCmd())
	return root
}

// exitCodeForError maps root.Execute()'s returned error (nil on success)
// to the final process exit code, given the report-derived exit code the
// subcommand already computed (0 if no report-producing subcommand ran at
// all, e.g. --help). Split out from Execute so this mapping is directly
// unit-testable without touching os.Args.
func exitCodeForError(err error, exitCode int) int {
	if err == nil {
		return exitCode
	}
	if isInfraFailure(err) {
		return 4
	}
	return 1
}

// infraFailureError marks a pre-report error as an infrastructure
// failure — evidence collection couldn't even be attempted, so no
// trustworthy scan result exists. This must exit with a code distinct
// from the report-derived "1 = warnings only" contract; without this
// marker, a totally failed scan (unreachable cluster, bad kubeconfig)
// would be indistinguishable from "the scan ran and found only warnings,"
// which a CI gate reading the exit code alone could easily misread as
// safe to proceed.
type infraFailureError struct{ err error }

func (e *infraFailureError) Error() string { return e.err.Error() }
func (e *infraFailureError) Unwrap() error { return e.err }

// infraFailure wraps err (if non-nil) so Execute maps it to exit code 4
// instead of the generic 1.
func infraFailure(err error) error {
	if err == nil {
		return nil
	}
	return &infraFailureError{err: err}
}

func isInfraFailure(err error) bool {
	var e *infraFailureError
	return errors.As(err, &e)
}
