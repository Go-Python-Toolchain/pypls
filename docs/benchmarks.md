# Benchmarks

This document records the performance measurements behind pypls's Definition of
Done. The headline target from the project blueprint is that pypls "resolves
types for Django 4.2 source in under 150ms". That budget is about editor
responsiveness: the time to type-check the single file an engineer is editing.
The numbers below report that per-file time, the largest single-file time
against the 150ms budget, the whole-tree time for transparency, and the
incremental re-check time.

## What was measured

- Subject: Django 4.2.0, cloned from the official repository at tag `4.2`
  (`git clone --depth 1 --branch 4.2 https://github.com/django/django`). The
  version was confirmed from `django/__init__.py`, which reads
  `VERSION = (4, 2, 0, "final", 0)`.
- Operation: `analyzer.Check(file, source)`, the same entry point the language
  server calls to produce diagnostics. It runs the full pipeline: lex, parse,
  and type-check.
- Harness: `internal/analyzer/django_bench_test.go`. The tests are guarded by the
  `DJANGO_SRC` environment variable and skip when it is unset, so continuous
  integration stays offline and deterministic. Per-file times are the median of
  7 runs to damp scheduler noise.

## How to reproduce

```
git clone --depth 1 --branch 4.2 https://github.com/django/django /tmp/django-4.2
export DJANGO_SRC=/tmp/django-4.2
go test ./internal/analyzer -run TestDjango -v
go test ./internal/analyzer -run xxx -bench BenchmarkDjangoCheck -benchmem
```

## Environment

- CPU: 12th Gen Intel Core i9-12900H
- OS/arch: linux/amd64
- Go: local toolchain (go 1.26), module targets go 1.23

Absolute times depend on the machine. The comparison that matters is the
per-file time versus the 150ms budget, which holds with a wide margin.

## Per-file responsiveness (the 150ms target)

Median `Check` time over 7 runs for representative large Django modules:

| File | Lines | Check time |
| --- | ---: | ---: |
| django/db/models/query.py | 2632 | 4.044 ms |
| django/db/models/base.py | 2532 | 3.956 ms |
| django/db/models/fields/__init__.py | 2815 | 4.606 ms |
| django/forms/forms.py | 540 | 0.831 ms |
| django/forms/models.py | 1661 | 2.677 ms |
| django/core/management/base.py | 689 | 0.985 ms |
| django/contrib/admin/options.py | 2502 | 4.229 ms |
| django/db/models/sql/query.py | 2649 | 4.663 ms |

**Largest single-file check: django/db/models/sql/query.py at 4.663 ms, against
the 150 ms budget.** That is roughly 32 times faster than the target. The
`go test` benchmark on the same file agrees:

```
BenchmarkDjangoCheck-20    260    4854055 ns/op    5994697 B/op    26650 allocs/op
```

about 4.85 ms per check.

## Whole-tree time (transparency, not the budget)

Checking every `.py` file in the Django 4.2 checkout, one run per file:

- Files checked: 2761
- Total check time: 0.9 s (1.0 s wall including file IO)
- Mean per file: 0.323 ms
- Slowest single file across the whole tree: tests/admin_views/tests.py at
  22.706 ms

The slowest file in the entire tree is a large test module, and it is still well
under the 150 ms per-file budget. The 150 ms target applies to a single file, not
to the whole tree; the whole-tree figure is reported only to show the full corpus
is handled quickly.

## Incremental re-check

The language server gives each open document an incremental analyzer that
re-type-checks only the top-level units whose text changed, which is the latency
an editor pays on each edit. For django/db/models/query.py:

- Cold analyze (first pass, all units): 6.452 ms
- Warm re-check after a one-line edit: 5.579 ms, with 1 top-level unit rechecked

The warm re-check stays in the single-digit-millisecond range and touches only
the edited unit, so editing a large Django file remains responsive.

## Summary

The Definition of Done target is met with a wide margin. The largest
representative Django 4.2 file type-checks in 4.663 ms, well under the 150 ms
per-file editor-responsiveness budget, and even the slowest file in the whole
tree stays under it.
