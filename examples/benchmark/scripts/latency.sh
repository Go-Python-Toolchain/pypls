#!/usr/bin/env bash
# Single-file latency: how long each tool takes to check one file from a cold
# start, and the warm re-check where a resident daemon exists. This is the
# "check on save" and "per keystroke" cost an editor pays.
#
# Fairness note: pypls checks one file. pyright and mypy follow imports and do
# deep cross-module inference, so they do strictly more work and also report
# more. The comparison is not "same analysis, different speed"; it is "what does
# each tool cost to give feedback on the file you are editing". pypls is a
# deliberately light responsiveness layer, which is the point being measured.

source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

csv="$RAW_DIR/latency.csv"
: >"$csv"
: >"${csv%.csv}.rawlog"
echo "label,wall_seconds,max_rss_kb,exit_code" >"$csv"

if [ ! -f "$REP_FILE" ]; then
  echo "representative file missing: $REP_FILE" >&2
  echo "run setup.sh first" >&2
  exit 1
fi

runs="$LATENCY_RUNS"
echo "representative file: ${REP_FILE#$SUBJECTS_DIR/}" >&2
echo "runs per sample: $runs (median is reported)" >&2

# --- Cold single-file check ----------------------------------------------
section "cold single-file check"

if have_tool pypls "$PYPLS_BIN"; then
  for i in $(seq 1 "$runs"); do
    run_timed "cold,pypls" "$csv" -- "$PYPLS_BIN" check --no-cache "$REP_FILE"
  done
fi

if have_tool mypy "$MYPY_BIN"; then
  for i in $(seq 1 "$runs"); do
    run_timed "cold,mypy" "$csv" -- "$MYPY_BIN" --no-incremental --follow-imports=skip --ignore-missing-imports --cache-dir=/dev/null "$REP_FILE"
  done
fi

if have_tool pyright "$PYRIGHT_BIN"; then
  for i in $(seq 1 "$runs"); do
    run_timed "cold,pyright" "$csv" -- "$PYRIGHT_BIN" "$REP_FILE"
  done
fi

# --- Warm re-check (resident daemon already running) ---------------------
# The truest editor latency: a server is already up, one file is re-checked.
section "warm re-check"

if have_tool pypls "$PYPLS_BIN"; then
  # Prime the on-disk cache, then measure a warm re-check.
  "$PYPLS_BIN" check "$REP_FILE" >/dev/null 2>&1 || true
  for i in $(seq 1 "$runs"); do
    run_timed "warm,pypls" "$csv" -- "$PYPLS_BIN" check "$REP_FILE"
  done
fi

if have_tool dmypy "$DMYPY_BIN"; then
  "$DMYPY_BIN" stop >/dev/null 2>&1 || true
  "$DMYPY_BIN" start -- --follow-imports=skip --ignore-missing-imports >/dev/null 2>&1 || true
  "$DMYPY_BIN" check "$REP_FILE" >/dev/null 2>&1 || true
  for i in $(seq 1 "$runs"); do
    run_timed "warm,dmypy" "$csv" -- "$DMYPY_BIN" check "$REP_FILE"
  done
  "$DMYPY_BIN" stop >/dev/null 2>&1 || true
fi

echo "" >&2
echo "latency samples written to $csv" >&2
