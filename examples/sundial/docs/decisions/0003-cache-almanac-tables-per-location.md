---
status: Proposed
date: 2026-07-09
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [performance]
related: [1]
domains: [almanac, cli]
---
# ADR-0003: Cache almanac tables per location

## Context

Every invocation recomputes seven days from scratch. That is fast today, but the
roadmap's golden-hour rows would triple the per-day work, and scripted use renders
the same location repeatedly.

## Decision

1. Cache computed `almanac.Day` values per (location, date) in a small in-process
   map behind the `schedule` package.
2. No persistence: the cache lives and dies with the process.

## Invariants

- Textual: cache hits and misses return byte-identical tables.

## Consequences

Golden-hour rows become cheap. A new seam between `schedule` and `almanac` to keep
honest — the cache must never change output, only cost.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| On-disk cache | State and invalidation for a CLI that runs in microseconds. |
| Precomputed year tables | Trades startup and memory for savings nobody measured yet. |
