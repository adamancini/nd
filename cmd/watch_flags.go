package cmd

import (
	"github.com/spf13/cobra"
)

var (
	watchMode     bool
	watchInterval int
)

// addWatchFlags registers --watch and --interval on a read command.
// --interval intentionally has no short flag to avoid conflict with
// -n (--limit) registered by addFilterFlags.
func addWatchFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&watchMode, "watch", "w", false,
		"watch mode: clear and refresh output on interval")
	cmd.Flags().IntVar(&watchInterval, "interval", 2,
		"watch refresh interval in seconds (used with --watch)")
}
