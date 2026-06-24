# Architecture Decision Records

An ADR captures a significant decision made about the design of this project: what was decided,
why, and what the consequences are. Write one when the decision is hard to reverse, affects
multiple components, or would otherwise be rediscovered from scratch months later.

## When to write an ADR

- Choosing between two technically viable approaches
- Establishing a constraint that binds future work (an "invariant")
- Superseding an existing decision with a changed one
- Recording why something was explicitly *not* done

## Naming & location

Files live in `docs/decisions/` and follow this pattern:

```
NNNN-kebab-title.md
```

where `NNNN` is a zero-padded sequence number (next available). Example:
`0003-drift-detection-strategy.md`.

## Frontmatter

Every ADR starts with YAML frontmatter:

```yaml
---
status: Proposed | Accepted | Implemented | Superseded
date: YYYY-MM-DD
supersedes: []        # list of ADR numbers this replaces, e.g. [0001]
superseded_by: ""     # ADR number that replaced this (empty if still active)
tags: [tooling]
related: []           # related ADR numbers
---
# ADR-NNNN: Title
```

## ACTIVE.md

`ACTIVE.md` is a generated index — **do not edit it by hand**. To regenerate it:

```
go test ./internal/adrtools/
```

The test writes the file if it is stale and then fails so you remember to re-stage it.
Run it again after staging and it will pass.
