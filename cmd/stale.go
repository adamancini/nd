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

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "List stale issues (not updated recently)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchMode && watchInterval <= 0 {
			return fmt.Errorf("--interval must be greater than 0")
		}
		days, _ := cmd.Flags().GetInt("days")

		run := func() error {
			cutoff := time.Now().UTC().AddDate(0, 0, -days)

			s, err := store.Open(resolveVaultDir())
			if err != nil {
				return err
			}
			defer s.Close()

			issues, err := s.ListIssues(store.FilterOptions{
				Status:        "!closed",
				UpdatedBefore: cutoff,
				Sort:          "updated",
			})
			if err != nil {
				return err
			}

			if jsonOut {
				return format.JSON(os.Stdout, issues)
			}
			if len(issues) == 0 {
				fmt.Printf("No stale issues (threshold: %d days).\n", days)
				return nil
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
	staleCmd.Flags().Int("days", 30, "days since last update to consider stale")
	addWatchFlags(staleCmd)
	rootCmd.AddCommand(staleCmd)
}
