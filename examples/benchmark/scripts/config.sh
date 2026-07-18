# Shared configuration for the pypls benchmark harness.
#
# Every script sources this file. It pins the subject versions and tool
# versions so a run on any machine measures the same code, and it defines the
# directory layout. Nothing here is machine specific; the machine details are
# captured at run time by machine.sh.

set -euo pipefail

# Root of the benchmark harness (the directory that holds this scripts folder).
BENCH_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Where heavy, uncommitted artifacts live: the cloned subjects, the installed
# competitor tools, and the raw measurement logs. This directory is git ignored.
WORK_DIR="${PYPLS_BENCH_WORK:-$BENCH_ROOT/work}"
SUBJECTS_DIR="$WORK_DIR/subjects"
TOOLS_DIR="$WORK_DIR/tools"
RAW_DIR="${PYPLS_BENCH_RAW:-$WORK_DIR/raw}"

# Pinned subjects. Each entry is "name|repo|ref". These are the projects an
# engineer opens in an editor, so they stand in for real memory and latency
# pressure. Django 4.2 is the blueprint's Definition of Done subject.
SUBJECTS=(
  "django-4.2|https://github.com/django/django|4.2"
  "fastapi|https://github.com/fastapi/fastapi|0.111.0"
  "numpy|https://github.com/numpy/numpy|v1.26.4"
)

# The single file used for the head to head latency comparison. It is the
# largest of Django's representative modules and is pypls's worst case in the
# per-file table, so it is the fairest hard case to publish.
REP_FILE="$SUBJECTS_DIR/django-4.2/django/db/models/sql/query.py"

# The representative Django modules pypls reports per-file times for. Paths are
# relative to the Django source root.
REP_FILES=(
  "django/db/models/query.py"
  "django/db/models/base.py"
  "django/db/models/fields/__init__.py"
  "django/forms/forms.py"
  "django/forms/models.py"
  "django/core/management/base.py"
  "django/contrib/admin/options.py"
  "django/db/models/sql/query.py"
)

# Pinned competitor versions. pypls is deliberately a fast, single-file
# responsiveness layer; pyright and mypy do deep cross-module analysis, so they
# are heavier and slower and also report more. The comparison is honest only
# when the versions are known, so they are pinned here.
PYRIGHT_VERSION="1.1.370"
MYPY_VERSION="1.10.0"

# Number of repetitions per cold latency sample. The reported figure is the
# median, which is stabler than a single sample against scheduler noise.
LATENCY_RUNS="${PYPLS_BENCH_RUNS:-7}"

# Tool entry points. pypls is expected on PATH or as ../../pypls relative to
# the tool repo; the competitors are installed under TOOLS_DIR by setup.sh.
find_pypls() {
  if command -v pypls >/dev/null 2>&1; then command -v pypls; return; fi
  for c in "$BENCH_ROOT/../../pypls" "$BENCH_ROOT/../../pypls.exe"; do
    [ -x "$c" ] && { echo "$c"; return; }
  done
  echo ""
}
PYPLS_BIN="${PYPLS_BIN:-$(find_pypls)}"
PYRIGHT_BIN="${PYRIGHT_BIN:-$TOOLS_DIR/node_modules/.bin/pyright}"
MYPY_BIN="${MYPY_BIN:-$TOOLS_DIR/mypy-venv/bin/mypy}"
DMYPY_BIN="${DMYPY_BIN:-$TOOLS_DIR/mypy-venv/bin/dmypy}"

mkdir -p "$WORK_DIR" "$SUBJECTS_DIR" "$TOOLS_DIR" "$RAW_DIR"
