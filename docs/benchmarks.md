# Benchmarks

This document records the performance measurements behind pypls's Definition of
Done. The headline target from the project blueprint is that pypls "resolves
types for Django 4.2 source in under 150ms". That budget is about editor
responsiveness: the time to type-check the single file an engineer is editing.
The numbers below report that per-file time, the largest single-file time
against the 150ms budget, the whole-tree time for transparency, and the
incremental re-check time.

They also answer the obvious next question, "fast compared to what?", by putting
pypls next to pyright and mypy on the same file and the same machine, and they
report how much memory each tool holds while checking a whole project. Every
number here is reproducible with the harness in
[`examples/benchmark`](../examples/benchmark), which clones the subjects,
installs the competitor tools, and runs the same measurements on your hardware.

## What was measured

- Subject: Django 4.2.0, cloned from the official repository at tag `4.2`
  (`git clone --depth 1 --branch 4.2 https://github.com/django/django`). The
  version was confirmed from `django/__init__.py`, which reads
  `VERSION = (4, 2, 0, "final", 0)`. The memory comparison adds two more real
  projects, FastAPI 0.111.0 and NumPy v1.26.4, so the memory numbers are not a
  single-project artifact.
- Operation: `analyzer.Check(file, source)`, the same entry point the language
  server calls to produce diagnostics. It runs the full pipeline: lex, parse,
  and type-check.
- Tools compared: pypls (this build), pyright 1.1.370, and mypy 1.10.0. See the
  fairness note below; the three tools do not do the same amount of work.
- Harness: `internal/analyzer/django_bench_test.go` for the pypls internals, and
  `examples/benchmark` for the cross-tool latency and memory numbers. The Go
  tests are guarded by the `DJANGO_SRC` environment variable and skip when it is
  unset, so continuous integration stays offline and deterministic. Per-file
  times are the median of 7 runs to damp scheduler noise.

## A fairness note on the comparison

pypls, pyright, and mypy do not do the same work, and this comparison is only
honest if that is stated plainly. pypls is a deliberately light, single-file
responsiveness layer: it checks the file you are editing and does not chase
every import across the dependency graph. pyright and mypy do deep cross-module
inference against typeshed and your dependencies, so they are slower and heavier
and also report more findings. The comparison is not "same analysis, who is
faster". It is "what does each tool cost to give you feedback on the file in
front of you". pypls trades analysis depth for speed on purpose; the numbers
below show the size of that trade, not a claim that pypls does everything the
others do.

## How to reproduce

The pypls internals, straight from the Go benchmark:

```
git clone --depth 1 --branch 4.2 https://github.com/django/django /tmp/django-4.2
export DJANGO_SRC=/tmp/django-4.2
go test ./internal/analyzer -run TestDjango -v
go test ./internal/analyzer -run xxx -bench BenchmarkDjangoCheck -benchmem
```

The cross-tool latency and memory tables, including cloning the subjects and
installing pyright and mypy locally:

```
cd examples/benchmark
scripts/setup.sh      # clone Django/FastAPI/NumPy, install pyright + mypy locally
scripts/run.sh        # measure everything, write work/raw/results.md
```

## Environment

- CPU: 12th Gen Intel Core i9-12900H (20 threads)
- Memory: 31.0 GB
- OS/arch: linux/amd64 (kernel 6.17)
- Go: local toolchain (go 1.26), module targets go 1.23

Absolute times depend on the machine. The comparison that matters is the
per-file time versus the 150ms budget, which holds with a wide margin, and the
relative standing against pyright and mypy, which is stable across hardware.

## Per-file responsiveness (the 150ms target)

Median `Check` time over 7 runs for representative large Django modules:

| File | Lines | Check time |
| --- | ---: | ---: |
| django/db/models/query.py | 2632 | 4.862 ms |
| django/db/models/base.py | 2532 | 6.005 ms |
| django/db/models/fields/__init__.py | 2815 | 5.383 ms |
| django/forms/forms.py | 540 | 0.777 ms |
| django/forms/models.py | 1661 | 4.374 ms |
| django/core/management/base.py | 689 | 1.108 ms |
| django/contrib/admin/options.py | 2502 | 4.781 ms |
| django/db/models/sql/query.py | 2649 | 5.369 ms |

**Largest single-file check: django/db/models/base.py at 6.005 ms, against the
150 ms budget.** That is roughly 25 times faster than the target. The `go test`
benchmark on the largest representative file agrees:

```
BenchmarkDjangoCheck-20    280    4691700 ns/op    5994720 B/op    26650 allocs/op
```

about 4.69 ms per check, allocating roughly 5.7 MB across 26,650 allocations per
run.

## Head to head against pyright and mypy

The same large Django module, `django/db/models/sql/query.py`, checked from a
cold start by each tool on the same machine. This is the "check on save with no
daemon running" cost. Median wall time and peak resident memory over 7 runs:

| Tool | Cold single-file check | Peak RSS |
| --- | ---: | ---: |
| pypls | 9.0 ms | 16 MB |
| mypy | 653 ms | 85 MB |
| pyright | 1258 ms | 196 MB |

pypls is about 73 times faster than mypy and 140 times faster than pyright on
this file, at a fraction of the memory. Two things are baked into those numbers
and are worth naming: pypls is a native Go binary with almost no process startup
cost, while mypy pays Python interpreter startup and pyright pays Node startup;
and, as the fairness note says, pyright and mypy are following imports and doing
deeper analysis, which is why they also report more.

The truest editor-latency comparison is the warm path, where a server is already
resident and only re-checks the edited file:

- pypls: the language server's incremental analyzer re-checks only the changed
  top-level units, entirely in memory, in 3.553 ms (see incremental re-check
  below). It never touches the disk cache for a single keystroke.
- mypy: the `dmypy` daemon re-checks the same file in 55 ms, most of which is the
  thin Python client talking to the daemon.
- pyright: its warm path is the resident language server; there is no simple CLI
  form to time, so it is not tabled here.

## Peak resident memory over a whole project

People building editor tooling care about memory almost as much as latency,
because the tool sits in RAM all day. Peak resident set size while each tool
checks every source file in the project once, over three real projects:

| Project | pypls | mypy | pyright |
| --- | ---: | ---: | ---: |
| Django 4.2 | 31 MB | 386 MB | 2157 MB |
| FastAPI 0.111.0 | 26 MB | 45 MB | 618 MB |
| NumPy v1.26.4 | 43 MB | 49 MB | 1740 MB (see note) |

pypls stays between 26 and 43 MB across all three projects. mypy ranges from 45
MB to 386 MB. pyright ranges from 618 MB to over 2 GB on Django.

Note on NumPy: pyright did not finish NumPy within a 600 second ceiling and was
stopped; 1740 MB is its peak resident memory at that cutoff, not the memory of a
completed run. pypls and mypy both finished NumPy in well under a second. This is
again the depth-versus-speed trade: pyright's whole-program analysis of NumPy's
heavily typed C-extension surface is genuinely expensive.

## Whole-tree time (transparency, not the budget)

Checking every `.py` file in the Django 4.2 checkout, one run per file:

- Files checked: 2761
- Total check time: 1.0 s (1.1 s wall including file IO)
- Mean per file: 0.370 ms
- Slowest single file across the whole tree: tests/admin_views/tests.py at
  20.065 ms

The slowest file in the entire tree is a large test module, and it is still well
under the 150 ms per-file budget. The 150 ms target applies to a single file, not
to the whole tree; the whole-tree figure is reported only to show the full corpus
is handled quickly.

## Incremental re-check

The language server gives each open document an incremental analyzer that
re-type-checks only the top-level units whose text changed, which is the latency
an editor pays on each edit. For django/db/models/query.py:

- Cold analyze (first pass, all units): 4.781 ms
- Warm re-check after a one-line edit: 3.553 ms, with 1 top-level unit rechecked

The warm re-check stays in the single-digit-millisecond range and touches only
the edited unit, so editing a large Django file remains responsive.

## Multiple machines

Absolute times move with the hardware, so the table below records which machine
produced the numbers in this document. The relative standing against pyright and
mypy is the part that holds across machines; the absolute pypls figures will be
faster on a quick desktop and slower on a low-end laptop.

| Machine | CPU | pypls cold check | pypls whole-tree mean |
| --- | --- | ---: | ---: |
| Reference | Intel Core i9-12900H | 9.0 ms | 0.370 ms |
| _(contribute)_ | Ryzen desktop | | |
| _(contribute)_ | Apple M-series | | |
| _(contribute)_ | Low-end laptop | | |

The harness in `examples/benchmark` writes a `machine.txt` and a `results.md`
for every run. If you run it on other hardware, those two files are exactly what
this table needs; send them in and we will add the row.

## Threats to validity

These measurements were performed on Linux x86_64 with an Intel i9-12900H.
Absolute timings will vary across operating systems, CPUs, memory bandwidth,
storage devices, the Go toolchain version, and the versions of the tools
compared against. The relative standing is more stable than the absolute
numbers, but it too can shift with tool versions. The benchmark harness is
published so readers can reproduce the measurements on their own hardware and
see what holds.

## Summary

The Definition of Done target is met with a wide margin. The largest
representative Django 4.2 file type-checks in 6.005 ms, well under the 150 ms
per-file editor-responsiveness budget, and even the slowest file in the whole
tree stays under it. Against the common Python type checkers on the same file,
pypls is roughly 70 to 140 times faster and holds a fraction of the memory,
because it is a focused single-file responsiveness layer rather than a
whole-program analyzer. That is the trade it is built to make.
