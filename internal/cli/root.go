// Package cli wires the Cobra command tree for the kubepreflight binary.
package cli

import (
	"github.com/spf13/cobra"
)

// Execute builds and runs the root command, returning the process exit
// code: 0 clean, 1 warnings only, 2 blockers present — the documented CI
// exit-code contract. A Cobra-level failure (bad flags, an error before a
// report is even produced) also exits 1. Called from cmd/kubepreflight,
// which is expected to os.Exit with the returned code.
func Execute() int {
	exitCode := 0

	root := &cobra.Command{
		Use:   "kubepreflight",
		Short: "Know what will break before your EKS upgrade",
		Long:  "KubePreflight is a read-only CLI that correlates deprecated APIs, admission webhooks, PodDisruptionBudgets, EKS add-ons, node/kubelet skew, and AWS provider constraints into a go/no-go upgrade readiness report.",
	}
	root.AddCommand(newScanCmd(&exitCode))
	root.AddCommand(newPlanCmd(&exitCode))

	if err := root.Execute(); err != nil {
		return 1
	}
	return exitCode
}
