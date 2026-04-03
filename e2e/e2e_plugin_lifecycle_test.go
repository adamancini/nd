// Package e2e runs end-to-end tests for the nd plugin lifecycle.
//
// These tests exercise the Makefile plugin targets (install, update,
// uninstall) against a real Claude Code installation. They verify
// both success and idempotency.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	// e2e/ is one level below repo root
	dir, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err != nil {
		t.Fatalf("Makefile not found at %s -- run tests from repo root or e2e/", dir)
	}
	return dir
}

func TestPluginLifecycle(t *testing.T) {
	root := repoRoot(t)
	script := filepath.Join(root, "e2e", "plugin_lifecycle_test.sh")

	cmd := exec.Command("bash", script)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("plugin lifecycle e2e test failed: %v", err)
	}
}
