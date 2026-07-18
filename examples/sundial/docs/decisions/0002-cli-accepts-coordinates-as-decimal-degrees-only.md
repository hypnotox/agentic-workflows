---
status: Implemented
date: 2026-07-07
tags: [cli]
related: [1]
domains: [cli]
---
# ADR-0002: CLI accepts coordinates as decimal degrees only

## Context

Coordinates arrive in many formats (DMS, cardinal suffixes, geo URIs). Every
format the CLI accepts is parsing surface to test and document, against ADR-0001's
small-and-instant stance.

## Decision

1. `sundial <latitude> <longitude>`, both decimal degrees, parsed with
   `strconv.ParseFloat`; anything else is exit 2 with a usage line.
2. No DMS, no cardinal suffixes, no flags for alternate formats.

## Invariants

- Textual: the CLI accepts coordinates exclusively as decimal degrees; no DMS
  parsing exists.

## Consequences

One trivially testable input path. Users with DMS coordinates convert them first:
accepted friction, documented in the README usage line.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Accept DMS too | Doubles the parse surface for a conversion any map app does. |
| Named flags (`--lat`, `--lon`) | Two positional arguments are unambiguous; flags add ceremony. |
