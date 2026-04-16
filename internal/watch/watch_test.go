package watch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// captureStdout redirects os.Stdout to a pipe, runs fn, and returns
// whatever fn wrote to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func TestRun_ExecutesImmediately(t *testing.T) {
	var calls atomic.Int32

	errDone := errors.New("done")
	fn := func() error {
		calls.Add(1)
		return errDone
	}

	err := Run(time.Hour, "test cmd", fn)
	if !errors.Is(err, errDone) {
		t.Fatalf("expected errDone, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected fn called 1 time before first tick, got %d", got)
	}
}

func TestRun_RespectsInterval(t *testing.T) {
	var calls atomic.Int32

	fn := func() error {
		n := calls.Add(1)
		if n >= 3 {
			return errors.New("stop")
		}
		return nil
	}

	start := time.Now()
	_ = Run(50*time.Millisecond, "test cmd", fn)
	elapsed := time.Since(start)

	got := calls.Load()
	if got < 3 {
		t.Fatalf("expected at least 3 calls, got %d", got)
	}
	// Two ticks at 50ms each = at least ~100ms total.
	if elapsed < 80*time.Millisecond {
		t.Fatalf("elapsed %v is too short for 2 ticks at 50ms", elapsed)
	}
}

func TestRun_StopsOnSignal(t *testing.T) {
	var calls atomic.Int32

	fn := func() error {
		n := calls.Add(1)
		if n == 1 {
			// After first execution, send SIGINT to ourselves.
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGINT)
		}
		return nil
	}

	err := Run(50*time.Millisecond, "test cmd", fn)
	if err != nil {
		t.Fatalf("expected nil on signal exit, got %v", err)
	}
}

func TestRun_PropagatesError(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("boom")

	fn := func() error {
		n := calls.Add(1)
		if n >= 2 {
			return want
		}
		return nil
	}

	err := Run(50*time.Millisecond, "test cmd", fn)
	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}
}

func TestClearScreen(t *testing.T) {
	out := captureStdout(t, func() {
		ClearScreen()
	})
	if !strings.Contains(out, "\033[H\033[2J") {
		t.Fatalf("expected ANSI clear sequence, got %q", out)
	}
}

func TestPrintHeader(t *testing.T) {
	out := captureStdout(t, func() {
		PrintHeader(2*time.Second, "nd list --watch")
	})

	hostname, _ := os.Hostname()

	checks := []string{
		"Every 2.0s:",
		"nd list --watch",
		hostname,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q; got %q", want, out)
		}
	}

	// Verify the timestamp looks like a date.
	now := time.Now()
	yearStr := fmt.Sprintf("%d", now.Year())
	if !strings.Contains(out, yearStr) {
		t.Errorf("header missing current year %s; got %q", yearStr, out)
	}
}
