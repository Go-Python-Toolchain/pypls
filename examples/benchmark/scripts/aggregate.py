#!/usr/bin/env python3
"""Turn the raw benchmark CSVs into the published markdown tables.

Reads latency.csv and memory.csv from the raw directory, computes the median
per label, and writes results.md. Kept dependency free (standard library only)
so it runs anywhere the harness runs.
"""

import csv
import os
import statistics
import sys

RAW = os.environ.get("PYPLS_BENCH_RAW")
if not RAW:
    here = os.path.dirname(os.path.abspath(__file__))
    RAW = os.environ.get("PYPLS_BENCH_WORK") or os.path.join(here, "..", "work", "raw")
    if os.path.basename(os.path.normpath(RAW)) != "raw":
        RAW = os.path.join(RAW, "raw")

TOOL_LABEL = {
    "pypls": "pypls",
    "mypy": "mypy",
    "dmypy": "mypy (dmypy)",
    "pyright": "pyright",
}


def load(path):
    rows = []
    if not os.path.exists(path):
        return rows
    with open(path, newline="") as fh:
        for r in csv.DictReader(fh):
            try:
                r["wall_seconds"] = float(r["wall_seconds"])
                r["max_rss_kb"] = float(r["max_rss_kb"])
            except (ValueError, KeyError):
                continue
            rows.append(r)
    return rows


def group_median(rows, key):
    """Median wall (ms) and RSS (MB) per label value at position `key`."""
    buckets = {}
    for r in rows:
        parts = r["label"].split(",")
        gkey = tuple(parts)
        buckets.setdefault(gkey, {"wall": [], "rss": []})
        buckets[gkey]["wall"].append(r["wall_seconds"] * 1000.0)
        buckets[gkey]["rss"].append(r["max_rss_kb"] / 1024.0)
    out = {}
    for gkey, v in buckets.items():
        out[gkey] = (
            statistics.median(v["wall"]),
            statistics.median(v["rss"]),
            len(v["wall"]),
        )
    return out


def latency_table(rows):
    med = group_median(rows, 0)
    lines = []
    lines.append("### Single-file latency (Django `db/models/sql/query.py`)\n")
    lines.append("Median wall time from a cold start, and warm re-check where a")
    lines.append("resident daemon exists. Lower is better.\n")
    lines.append("| Mode | Tool | Median wall | Peak RSS | Samples |")
    lines.append("| --- | --- | ---: | ---: | ---: |")
    order = [
        ("cold", "pypls"), ("cold", "mypy"), ("cold", "pyright"),
        ("warm", "pypls"), ("warm", "dmypy"),
    ]
    for mode, tool in order:
        key = (mode, tool)
        if key not in med:
            continue
        wall, rss, n = med[key]
        lines.append(
            f"| {mode} | {TOOL_LABEL.get(tool, tool)} | {wall:.1f} ms | {rss:.0f} MB | {n} |"
        )
    return "\n".join(lines) + "\n"


def memory_table(rows):
    med = group_median(rows, 0)
    subjects, tools = [], []
    for (tool, subj) in med:
        if subj not in subjects:
            subjects.append(subj)
        if tool not in tools:
            tools.append(tool)
    tool_order = [t for t in ("pypls", "mypy", "pyright") if t in tools]
    lines = []
    lines.append("### Peak resident memory over a whole project\n")
    lines.append("Peak RSS while checking every source file in the project once.")
    lines.append("Lower is better.\n")
    header = "| Project | " + " | ".join(TOOL_LABEL.get(t, t) for t in tool_order) + " |"
    sep = "| --- | " + " | ".join("---:" for _ in tool_order) + " |"
    lines.append(header)
    lines.append(sep)
    for subj in subjects:
        cells = []
        for t in tool_order:
            key = (t, subj)
            if key in med:
                _, rss, _ = med[key]
                cells.append(f"{rss:.0f} MB")
            else:
                cells.append("n/a")
        lines.append(f"| {subj} | " + " | ".join(cells) + " |")
    return "\n".join(lines) + "\n"


def main():
    lat = load(os.path.join(RAW, "latency.csv"))
    mem = load(os.path.join(RAW, "memory.csv"))
    parts = ["# Benchmark results\n"]
    machine = os.path.join(RAW, "machine.txt")
    if os.path.exists(machine):
        parts.append("```\n" + open(machine).read().rstrip() + "\n```\n")
    if lat:
        parts.append(latency_table(lat))
    else:
        parts.append("_No latency samples found. Run latency.sh._\n")
    if mem:
        parts.append(memory_table(mem))
    else:
        parts.append("_No memory samples found. Run memory.sh._\n")
    report = "\n".join(parts)
    out = os.path.join(RAW, "results.md")
    with open(out, "w") as fh:
        fh.write(report)
    sys.stdout.write(report)
    sys.stderr.write(f"\nwrote {out}\n")


if __name__ == "__main__":
    main()
