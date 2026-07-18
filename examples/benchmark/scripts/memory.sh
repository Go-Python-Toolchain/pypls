#!/usr/bin/env bash
# Peak resident memory: how much RAM each tool holds while checking a whole
# project. People building editor tooling care about memory almost as much as
# latency, so this reports the peak resident set size for each tool over Django,
# FastAPI, and NumPy.
#
# All three tools check the project's own files in a single pass. mypy and
# pyright are told not to chase third-party dependencies, so the comparison
# stays to the same corpus (the project's source) rather than each tool's
# typeshed and dependency graph. Each run is measured once; peak RSS is stable
# across runs because it is a high water mark, not an average.

source "$(dirname "${BASH_SOURCE[0]}")/config.sh"
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

csv="$RAW_DIR/memory.csv"
: >"$csv"
: >"${csv%.csv}.rawlog"
echo "label,wall_seconds,max_rss_kb,exit_code" >"$csv"

# A generous ceiling so a heavy tool on a heavy project cannot hang the run.
# `timeout` is an executable so measure.py can launch it directly.
TIMEOUT="${PYPLS_BENCH_MEM_TIMEOUT:-600}"
TIMEOUT_BIN="$(command -v timeout || true)"
cap() { if [ -n "$TIMEOUT_BIN" ]; then echo "$TIMEOUT_BIN $TIMEOUT"; fi; }

for entry in "${SUBJECTS[@]}"; do
  IFS='|' read -r name repo ref <<<"$entry"
  proj="$SUBJECTS_DIR/$name"
  if [ ! -d "$proj" ]; then
    echo "skip subject $name: not cloned" >&2
    continue
  fi
  section "peak RSS over $name"

  if have_tool pypls "$PYPLS_BIN"; then
    echo "  pypls check $name" >&2
    run_timed "pypls,$name" "$csv" -- $(cap) "$PYPLS_BIN" check --no-cache "$proj"
  fi

  if have_tool mypy "$MYPY_BIN"; then
    echo "  mypy $name" >&2
    run_timed "mypy,$name" "$csv" -- $(cap) "$MYPY_BIN" --no-incremental --follow-imports=skip --ignore-missing-imports --cache-dir=/dev/null "$proj"
  fi

  if have_tool pyright "$PYRIGHT_BIN"; then
    echo "  pyright $name" >&2
    run_timed "pyright,$name" "$csv" -- $(cap) "$PYRIGHT_BIN" "$proj"
  fi
done

echo "" >&2
echo "memory samples written to $csv" >&2
