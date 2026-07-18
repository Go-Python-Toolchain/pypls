#!/usr/bin/env bash
# Prepare the benchmark workspace: clone the subject projects at their pinned
# refs and install the competitor tools (pyright, mypy) locally under the work
# directory. Everything lands under examples/benchmark/work, which is git
# ignored, so nothing here pollutes the repo or the global environment.
#
# Re-running is safe: existing clones and installs are left in place.

source "$(dirname "${BASH_SOURCE[0]}")/config.sh"

echo "Workspace: $WORK_DIR"

# --- Subjects -------------------------------------------------------------
for entry in "${SUBJECTS[@]}"; do
  IFS='|' read -r name repo ref <<<"$entry"
  dest="$SUBJECTS_DIR/$name"
  if [ -d "$dest/.git" ] || [ -d "$dest/django" ] || [ -d "$dest" ] && [ -n "$(ls -A "$dest" 2>/dev/null)" ]; then
    echo "subject $name: already present, skipping"
    continue
  fi
  echo "subject $name: cloning $repo @ $ref"
  git clone --depth 1 --branch "$ref" "$repo" "$dest"
done

# --- pyright (via npm, pinned) -------------------------------------------
if [ -x "$TOOLS_DIR/node_modules/.bin/pyright" ]; then
  echo "pyright: already installed ($("$TOOLS_DIR/node_modules/.bin/pyright" --version))"
elif command -v npm >/dev/null 2>&1; then
  echo "pyright: installing $PYRIGHT_VERSION"
  ( cd "$TOOLS_DIR" && npm install "pyright@$PYRIGHT_VERSION" )
else
  echo "pyright: npm not found; skipping (latency/memory tables will omit pyright)" >&2
fi

# --- mypy (via venv, pinned) ---------------------------------------------
if [ -x "$TOOLS_DIR/mypy-venv/bin/mypy" ]; then
  echo "mypy: already installed ($("$TOOLS_DIR/mypy-venv/bin/mypy" --version))"
elif command -v python3 >/dev/null 2>&1; then
  echo "mypy: installing $MYPY_VERSION"
  python3 -m venv "$TOOLS_DIR/mypy-venv"
  "$TOOLS_DIR/mypy-venv/bin/pip" install --quiet --upgrade pip
  "$TOOLS_DIR/mypy-venv/bin/pip" install --quiet "mypy==$MYPY_VERSION"
else
  echo "mypy: python3 not found; skipping (latency/memory tables will omit mypy)" >&2
fi

echo ""
echo "Setup complete."
echo "  subjects: $SUBJECTS_DIR"
echo "  tools:    $TOOLS_DIR"
