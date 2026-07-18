#!/usr/bin/env bash
# One command to reproduce the whole benchmark: capture the machine, measure
# single-file latency across tools, measure peak memory across projects, and
# write results.md. Assumes setup.sh has already cloned the subjects and
# installed the competitor tools.
#
#   scripts/setup.sh     # once, downloads subjects and tools (~a few minutes)
#   scripts/run.sh       # measure and report

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$here/config.sh"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 not found; the measurement wrapper (measure.py) needs it." >&2
  exit 1
fi
if [ -z "$PYPLS_BIN" ]; then
  echo "pypls binary not found. Build it first: (cd ../.. && go build -o pypls .)" >&2
  exit 1
fi

bash "$here/machine.sh"
bash "$here/latency.sh"
bash "$here/memory.sh"

# Optional: the pypls internal analyzer numbers (lex+parse+check with no process
# startup), straight from the Go benchmark, if the Go toolchain is present.
if command -v go >/dev/null 2>&1 && [ -d "$SUBJECTS_DIR/django-4.2" ]; then
  section() { echo "== $* =="; }
  echo "== pypls internal analyzer (go test) =="
  ( cd "$BENCH_ROOT/../.." && DJANGO_SRC="$SUBJECTS_DIR/django-4.2" \
      go test ./internal/analyzer -run TestDjango -v 2>&1 | tee "$RAW_DIR/pypls_internal.txt" ) || true
  ( cd "$BENCH_ROOT/../.." && DJANGO_SRC="$SUBJECTS_DIR/django-4.2" \
      go test ./internal/analyzer -run xxx -bench BenchmarkDjangoCheck -benchmem 2>&1 | tee -a "$RAW_DIR/pypls_internal.txt" ) || true
fi

python3 "$here/aggregate.py"
echo ""
echo "Done. Raw logs and results.md are in: $RAW_DIR"
