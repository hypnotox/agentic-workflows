---
format: current-state-v2
status: Implemented
date: 2026-07-30
---
# ADR-0185: Severity-key removal preserves values, not bytes

## Context

`config/migrations-and-locks:severity-keys-dropped` promises that schema generation 25 "leaves every
other configured key byte-identical". That is false for any `config.yaml` not already in awf's
canonical form.

Every removal routes through `config.RemoveMappingKey`, which mutates a `yaml.Node` tree and then
re-encodes the whole document through `internal/config/edit.go`'s `encode` funnel at `SetIndent(2)`.
The re-encode is the mechanism ADR-0026 chose so that comments and untouched keys survive, and it does
preserve every key, every value, and every comment. It does not preserve layout. Verified against the
migration at HEAD:

- a four-space-indented block comes back two-space indented, so a surviving sibling's own line bytes
  change even though nothing about that key was touched;
- a sequence item under a surviving key moves from two-space to four-space indent, since yaml.v3 at
  `SetIndent(2)` indents sequence items under their key;
- blank lines anywhere in the file are dropped, while comments survive attached to the following key.

The reachable population is an adopter who hand-edited `config.yaml` into a non-canonical layout. An
awf-written config goes through `MarshalSkeleton` and is already canonical, which is precisely why
every fixture in `internal/migrate/dropseveritysettings_test.go` uses canonical sources and no test can
fail on the clause.

The wording is inherited: it entered with ADR-0183 and ADR-0184 corrected the second half of the same
sentence, adding the seed, while leaving the first half unexamined. This is the residual-over-breadth
shape the repository already recognizes, found one layer deeper than the pass that introduced the
correction.

The over-promise is specific rather than systemic. Four other current-state claims use
"byte-identical", and each describes awf's own generated output, where both sides are canonical and the
promise holds. This is the only claim promising byte-identity about a file the adopter authored.

## Decision

1. `config/migrations-and-locks:severity-keys-dropped` is narrowed from byte preservation to value
   preservation: the migration leaves every other configured key and its value intact. What the
   migration guarantees is that nothing an adopter configured is lost or altered in meaning, not that
   the file's layout is untouched.

2. The claim does not attempt to describe the canonical re-encode. That behaviour belongs to every
   `internal/config` editor under ADR-0026, not to this migration, and pinning it here would attach a
   package-wide property to one caller.

3. `internal/migrate/dropseveritysettings_test.go` gains a non-canonically-formatted source case
   asserting that every surviving key and value is present with its value unchanged. Without it the
   narrowed clause would be reworded but still unfalsifiable, which is the defect this decision exists
   to correct rather than repeat.

## State changes

- update `config/migrations-and-locks:severity-keys-dropped`

## Consequences

The claim describes what the migration guarantees. An adopter reading it learns that their configured
values survive, and is not promised a clean diff that a non-canonical file will not get.

What stays unpinned is the re-encode itself: no claim states that awf's config editors canonicalize
layout while preserving comments and values. That is a real gap, and it is deliberately not closed
here, because it governs `internal/config` as a whole and every migration that has ever used those
editors. Recorded as a roadmap idea rather than smuggled in through a migration's claim.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Accept "key ... byte-identical" as meaning the key/value pair | The surviving key's own line bytes change under re-indent, so the reading does not hold even charitably. |
| Make the migration genuinely byte-preserving | Would require abandoning the `yaml.Node` round-trip ADR-0026 chose, for every editor, to fix one adjective. |
| Reword without adding a fixture case | Leaves the clause unfalsifiable, which is the defect being corrected rather than a fix for it. |
| Add a claim describing the canonical re-encode here | It is a property of every `internal/config` editor, not of this migration; it belongs to its own decision. |

## Status history

- 2026-07-30: Proposed
- 2026-07-30: Implemented; content-sha256: 7c34609b6c835bc2b3fbf7776102ab2ef2a00d2c691438503ce4dd6819f3f007; state-sequence: 97
