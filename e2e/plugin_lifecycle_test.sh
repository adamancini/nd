#!/usr/bin/env bash
set -euo pipefail

# e2e test: nd plugin lifecycle via Makefile targets
# Exercises install, update, uninstall, and idempotency of each.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PASS=0
FAIL=0

phase() {
  echo ""
  echo "=== PHASE: $1 ==="
}

check() {
  local description="$1"
  shift
  if "$@"; then
    echo "  PASS: $description"
    PASS=$(( PASS + 1 ))
  else
    echo "  FAIL: $description"
    FAIL=$(( FAIL + 1 ))
  fi
}

# ── Phase 1: install-plugin ─────────────────────────────────────────────
phase "install-plugin (first run)"

OUTPUT="$(make install-plugin 2>&1)" || {
  echo "  FAIL: make install-plugin exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "output contains 'Marketplace registered.' or 'Marketplace already registered.'" \
  grep -qE "Marketplace registered\.|Marketplace already registered\." <<< "$OUTPUT"

# ── Phase 2: install-plugin idempotency ──────────────────────────────────
phase "install-plugin (idempotency)"

OUTPUT="$(make install-plugin 2>&1)" || {
  echo "  FAIL: make install-plugin (idempotent) exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "idempotent install exits 0" true  # already guaranteed by set -e above

# ── Phase 3: update ─────────────────────────────────────────────────────
phase "update"

OUTPUT="$(make update 2>&1)" || {
  echo "  FAIL: make update exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "make update exits 0" true  # guaranteed by set -e

# ── Phase 4: uninstall-plugin ────────────────────────────────────────────
phase "uninstall-plugin (first run)"

OUTPUT="$(make uninstall-plugin 2>&1)" || {
  echo "  FAIL: make uninstall-plugin exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "output contains 'Plugin uninstalled.' or 'Plugin was not installed.'" \
  grep -qE "Plugin uninstalled\.|Plugin was not installed\." <<< "$OUTPUT"

# ── Phase 5: uninstall-plugin idempotency ────────────────────────────────
phase "uninstall-plugin (idempotency)"

OUTPUT="$(make uninstall-plugin 2>&1)" || {
  echo "  FAIL: make uninstall-plugin (idempotent) exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "idempotent uninstall output contains 'Plugin was not installed.'" \
  grep -q "Plugin was not installed\." <<< "$OUTPUT"

# ── Phase 6: re-install to restore state ─────────────────────────────────
phase "re-install plugin (restore state)"

OUTPUT="$(make install-plugin 2>&1)" || {
  echo "  FAIL: make install-plugin (restore) exited non-zero"
  echo "$OUTPUT"
  exit 1
}
echo "$OUTPUT"

check "restore install exits 0" true

# ── Summary ──────────────────────────────────────────────────────────────
echo ""
echo "=============================="
echo "  Results: $PASS passed, $FAIL failed"
echo "=============================="

if (( FAIL > 0 )); then
  echo "FAIL"
  exit 1
fi

echo "PASS"
exit 0
