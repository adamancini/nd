package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/RamXX/nd/internal/format"
	"github.com/RamXX/nd/internal/graph"
	"github.com/RamXX/nd/internal/store"
	"github.com/RamXX/nd/internal/watch"
	"github.com/spf13/cobra"
)

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show blocked issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchMode && watchInterval <= 0 {
			return fmt.Errorf("--interval must be greater than 0")
		}
		run := func() error {
			s, err := store.Open(resolveVaultDir())
			if err != nil {
				return err
			}
			defer s.Close()

			all, err := s.ListIssues(store.FilterOptions{})
			if err != nil {
				return err
			}

			g := graph.Build(all)
			blocked := g.Blocked()

			if jsonOut {
				return format.JSON(os.Stdout, blocked)
			}

			if len(blocked) == 0 {
				fmt.Println("No blocked issues.")
				return nil
			}

			format.Table(os.Stdout, blocked)

			// Show blockers for each.
			if verbose {
				fmt.Println()
				for _, issue := range blocked {
					blockers := g.BlockersOf(issue.ID)
					ids := make([]string, len(blockers))
					for i, b := range blockers {
						ids[i] = b.ID
					}
					fmt.Printf("  %s blocked by: %s\n", issue.ID, strings.Join(ids, ", "))
				}
			}
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
	addWatchFlags(blockedCmd)
	rootCmd.AddCommand(blockedCmd)
}
