// Command exemptioncheck validates the false-positive governance registry,
// audit inventory, documentation anchors, referenced regression tests, and
// known production callsites. It is not part of the public CLI; CI invokes it
// through scripts/check-exemption-governance.sh.
package main

import (
	"fmt"
	"os"

	"github.com/imneeteeshyadav98/kubepreflight/internal/exemptions"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "exemption governance check failed to determine working directory: %v\n", err)
		os.Exit(1)
	}
	report := exemptions.Check(root)
	fmt.Print(report.String())
	if !report.OK() {
		os.Exit(1)
	}
}
