# pypls benchmark

Reproduce the numbers behind pypls's performance claims on your own machine, and
compare pypls against the two most common Python type checkers, pyright and
mypy. Everything here is scripted so you do not have to take our word for it: run
it and read the results.

The published figures live in [`docs/benchmarks.md`](../../docs/benchmarks.md).
This directory is the harness that produces them.

## What it measures

Three things, all from the point of view of an editor giving you feedback as you
type:

1. **Single-file latency.** How long each tool takes to check one file, cold,
   and warm (with a resident daemon already running). This is the "check on
   save" and "per keystroke" cost.
2. **Peak resident memory.** How much RAM each tool holds while checking a whole
   project. Editor tooling lives in your machine's memory all day, so this
   matters almost as much as latency.
3. **pypls internals.** The lex, parse, and type-check time with no process
   startup, straight from the Go benchmark, plus the incremental re-check time.

The subjects are three real projects an engineer actually opens: Django 4.2
(the blueprint's Definition of Done subject), FastAPI, and NumPy.

## A fair-comparison note

pypls, pyright, and mypy do not do the same work, and the tables say so. pypls is
a deliberately light, single-file responsiveness layer: it checks the file you
are editing and does not chase every import across the dependency graph. pyright
and mypy do deep cross-module inference against typeshed and your dependencies,
so they are slower and heavier and also report more findings. The comparison is
not "same analysis, who is faster". It is "what does each tool cost to give you
feedback on the file in front of you". pypls trades depth for speed on purpose;
these numbers show the size of that trade.

## Requirements

- `python3` (runs the measurement wrapper and the aggregator; standard library
  only, no pip packages needed for the harness itself)
- `git` (clones the subject projects)
- `node` and `npm` (installs pyright locally; optional, the harness skips
  pyright if npm is absent)
- `go` (only for the pypls-internals step; optional)
- A built `pypls` binary. From the repo root: `go build -o pypls .`

The subjects and competitor tools are installed into `work/`, which is git
ignored. Nothing touches your global environment.

## Run it

```
# From this directory (examples/benchmark).

# 1. One time: clone the subjects and install pyright + mypy locally (~minutes).
scripts/setup.sh

# 2. Measure everything and write work/raw/results.md.
scripts/run.sh
```

Or run a single stage:

```
scripts/machine.sh    # record CPU / OS / tool versions
scripts/latency.sh    # single-file latency across tools
scripts/memory.sh     # peak RSS across projects
python3 scripts/aggregate.py   # rebuild results.md from the raw CSVs
```

Results, raw logs, and the machine description land in `work/raw/`:

- `results.md` - the assembled tables
- `latency.csv`, `memory.csv` - one row per sample, so you can re-aggregate
- `*.rawlog` - every raw measurement line for auditing
- `machine.txt` - the CPU, memory, OS, and tool versions for this run
- `pypls_internal.txt` - the Go benchmark output

## Tuning

Environment variables (all optional):

- `PYPLS_BENCH_RUNS` - repetitions per latency sample (default 7, median wins)
- `PYPLS_BENCH_WORK` - where subjects, tools, and raw output live (default
  `./work`)
- `PYPLS_BENCH_MEM_TIMEOUT` - per-run ceiling for the memory sweep in seconds
  (default 600)
- `PYPLS_BIN`, `PYRIGHT_BIN`, `MYPY_BIN` - point at specific tool binaries

## Contribute a machine

The numbers depend on the hardware. If you run this on a Ryzen desktop, an Apple
M-series laptop, or a low-end machine, the `machine.txt` plus `results.md` from
your run are exactly what a cross-hardware table needs. Open an issue or a pull
request against the pypls repo with both files and we will add your row.

## Pinned versions

So a run measures the same code everywhere:

| Piece | Version |
| --- | --- |
| Django | 4.2 (`VERSION = (4, 2, 0, "final", 0)`) |
| FastAPI | 0.111.0 |
| NumPy | v1.26.4 |
| pyright | 1.1.370 |
| mypy | 1.10.0 |

These are set in `scripts/config.sh`.
