package watch

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run executes fn immediately and then on every interval tick until
// SIGINT/SIGTERM. Before each call it clears the screen and prints a
// header line modeled after watch(1).
func Run(interval time.Duration, header string, fn func() error) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	execute := func() error {
		ClearScreen()
		PrintHeader(interval, header)
		return fn()
	}

	if err := execute(); err != nil {
		return err
	}

	for {
		select {
		case <-sig:
			return nil
		case <-ticker.C:
			if err := execute(); err != nil {
				return err
			}
		}
	}
}

// ClearScreen emits ANSI escape codes to move the cursor home and clear
// the entire screen.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// PrintHeader prints a watch(1)-style header line showing the refresh
// interval, the reconstructed command, hostname, and current time.
func PrintHeader(interval time.Duration, cmd string) {
	hostname, _ := os.Hostname()
	now := time.Now().Format("Mon Jan 2 15:04:05 2006")
	fmt.Printf("Every %.1fs: %s\t%s  %s\n\n",
		interval.Seconds(), cmd, hostname, now)
}
