# Shared measurement helpers for the pypls benchmark harness.
#
# Every measurement records the same two numbers the same way: wall clock
# seconds (microsecond resolution) and peak resident set size in kilobytes.
# measure.py provides both from a single process launch. Results are appended to
# CSV files that aggregate.py turns into the published tables.

MEASURE_PY="$(dirname "${BASH_SOURCE[0]}")/measure.py"

# run_timed LABEL OUT_CSV -- CMD...
#
# Runs CMD once under measure.py, appends one CSV row to OUT_CSV as
#   label,wall_seconds,max_rss_kb,exit_code
# and echoes the row to the .rawlog beside the CSV for auditing. The command's
# own output is discarded; only the timing is kept.
run_timed() {
  local label="$1"; shift
  local out_csv="$1"; shift
  [ "$1" = "--" ] && shift

  local line
  line="$(python3 "$MEASURE_PY" "$@" 2>/dev/null || echo "NA,NA,127")"

  # The label itself contains a comma (mode,tool or tool,subject), so it must be
  # quoted to stay a single CSV field. measure.py emits wall,rss,exit.
  printf '"%s",%s\n' "$label" "$line" >>"$out_csv"
  printf '"%s",%s\n' "$label" "$line" >>"${out_csv%.csv}.rawlog"
  return 0
}

# have_tool NAME PATH: report whether a tool is present, printing a skip note if
# not so a partial run is self documenting rather than silently incomplete.
have_tool() {
  local name="$1" path="$2"
  if [ -z "$path" ] || [ ! -x "$path" ]; then
    # dmypy/mypy live inside a venv; -x on the shim is enough. pyright is a
    # node shim. If the path is a bare command, fall back to command -v.
    if [ -n "$path" ] && command -v "$path" >/dev/null 2>&1; then
      return 0
    fi
    echo "  skip: $name not found (looked for '${path:-<empty>}')" >&2
    return 1
  fi
  return 0
}

# section HEADER: print a labeled banner to stderr for progress.
section() { echo "" >&2; echo "== $* ==" >&2; }
