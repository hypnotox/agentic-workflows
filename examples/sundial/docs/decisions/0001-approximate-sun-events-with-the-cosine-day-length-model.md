---
status: Implemented
date: 2026-07-06
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [model]
related: []
domains: [almanac]
---
# ADR-0001: Approximate sun events with the cosine day-length model

## Context

sundial needs sunrise/sunset times good enough to plan a walk. Real ephemeris
computation drags in a dependency and precision nobody asked for; the CLI's whole
value is being small and instant.

## Decision

1. Day length comes from the cosine model: seasonal solar declination drives the
   day/night terminator angle; solar noon shifts four minutes per degree of
   longitude.
2. Latitude is clamped to [-90, 90] before the model runs; garbage input degrades
   to the pole.
3. Polar day and night collapse to full- or zero-length days, never an error.

## Invariants

- `invariant: almanac-clamped-latitude`: latitude is clamped to [-90, 90] before the
  day-length model; out-of-range input degrades to the pole, never to a domain
  error.
- Textual: `internal/almanac` stays standard-library-only.

## Consequences

Minutes-level accuracy at temperate latitudes; wrong near the poles and useless
for navigation, stated in the package doc. Zero dependencies. Sub-minute accuracy
is out of scope unless a successor decision accepts an ephemeris dependency.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| NOAA solar-position algorithm | An order of magnitude more code for accuracy the use case does not need. |
| Ephemeris library | A dependency for a toy; contradicts the CLI's instant-and-small value. |
