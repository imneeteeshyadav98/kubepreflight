package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/imneeteeshyadav98/kubepreflight/internal/buildinfo"
)

// newVersionCmd wires `kubepreflight version`, printing the same banner as
// `kubepreflight --version` (see newRootCmd, which sets root.Version and a
// matching version template from the same buildinfo.String()) so both
// surfaces can never drift apart.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), buildinfo.String())
			return nil
		},
	}
}
