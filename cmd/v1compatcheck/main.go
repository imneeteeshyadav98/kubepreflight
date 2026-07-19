// Command v1compatcheck validates KubePreflight's documented v1 compatibility
// contract against the current implementation.
package main

import (
	"fmt"
	"os"

	"github.com/imneeteeshyadav98/kubepreflight/internal/cli"
	"github.com/imneeteeshyadav98/kubepreflight/internal/v1compat"
)

func main() {
	report := v1compat.Check(cli.V1CompatibilityActual())
	fmt.Print(report.String())
	if !report.OK() {
		os.Exit(1)
	}
}
