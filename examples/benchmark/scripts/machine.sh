#!/usr/bin/env bash
# Capture the machine identity for a benchmark run so results from different
# hardware can be compared. Prints a short human readable block and writes a
# machine.txt into the raw directory. Absolute times depend on the machine, so
# every published number should carry this context.

source "$(dirname "${BASH_SOURCE[0]}")/config.sh"

cpu_model() {
  if [ -r /proc/cpuinfo ]; then
    grep -m1 'model name' /proc/cpuinfo | sed 's/.*: //'
  elif command -v sysctl >/dev/null 2>&1; then
    sysctl -n machdep.cpu.brand_string 2>/dev/null
  else
    echo "unknown"
  fi
}

cpu_cores() {
  if command -v nproc >/dev/null 2>&1; then nproc
  elif command -v sysctl >/dev/null 2>&1; then sysctl -n hw.ncpu 2>/dev/null
  else echo "unknown"; fi
}

mem_total() {
  if [ -r /proc/meminfo ]; then
    awk '/MemTotal/ { printf "%.1f GB\n", $2/1024/1024 }' /proc/meminfo
  elif command -v sysctl >/dev/null 2>&1; then
    awk -v b="$(sysctl -n hw.memsize 2>/dev/null)" 'BEGIN { printf "%.1f GB\n", b/1024/1024/1024 }'
  else echo "unknown"; fi
}

out="$RAW_DIR/machine.txt"
{
  echo "CPU:        $(cpu_model)"
  echo "Cores:      $(cpu_cores)"
  echo "Memory:     $(mem_total)"
  echo "OS/arch:    $(uname -s -m)"
  echo "Kernel:     $(uname -r)"
  echo "Go:         $(go version 2>/dev/null | awk '{print $3}' || echo 'n/a')"
  echo "pypls:      $("$PYPLS_BIN" version 2>/dev/null || echo 'n/a')"
  echo "pyright:    ${PYRIGHT_VERSION}"
  echo "mypy:       ${MYPY_VERSION}"
} | tee "$out"
