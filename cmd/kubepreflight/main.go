// Command kubepreflight is the CLI entrypoint. All logic lives in
// internal/cli; this file only wires it to os.Exit. Cobra-level errors are
// printed by Cobra itself; internal/cli.Execute returns the documented
// exit-code contract (0 clean, 1 warnings, 2 blockers) for everything else.
package main

import (
	"os"

	"kubepreflight/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
