package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/RamXX/nd/internal/format"
	"github.com/RamXX/nd/internal/store"
	"github.com/RamXX/nd/internal/watch"
	"github.com/spf13/cobra"
)

var childrenCmd = &cobra.Command{
	Use:   "children <id>",
	Short: "List child issues of a parent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchMode && watchInterval <= 0 {
			return fmt.Errorf("--interval must be greater than 0")
		}
		id := args[0]

		run := func() error {
			s, err := store.Open(resolveVaultDir())
			if err != nil {
				return err
			}
			defer s.Close()

			issues, err := s.ListIssues(store.FilterOptions{Parent: id})
			if err != nil {
				return err
			}

			if jsonOut {
				return format.JSON(os.Stdout, issues)
			}
			format.Table(os.Stdout, issues)
			return nil
		}
		if watchMode {
			return watch.Run(time.Duration(watchInterval)*time.Second,
				strings.Join(os.Args[1:], " "), run)
		}
		return run()
	},
}

func init() {
	addWatchFlags(childrenCmd)
	rootCmd.AddCommand(childrenCmd)
}
