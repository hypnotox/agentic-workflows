---
name: awf-proposing-adr
description: >
  Use to propose a new ADR under docs/decisions for awf.
  Writes the ADR file with required frontmatter and sections, regenerates
  ACTIVE.md, commits, and hands off to awf-reviewing-adr.
---

# awf-proposing-adr

Writes a new ADR to `docs/decisions/NNNN-kebab-title.md` (status `Proposed`) and commits it. Scope each record to a single load-bearing commitment — one decision per ADR. See `docs/workflow.md` for the full lifecycle rules and `awf-adr-lifecycle` for state transitions.

## When to invoke

Per `docs/workflow.md`: load-bearing decisions only — one ADR per decision. Bugfixes and routine refactors do not need an ADR. When in doubt, write the ADR.

Load-bearing triggers include:
- Introducing a new internal package or changing package boundaries
- Changing the render pipeline (template engine, section overlay, missingkey behaviour)
- Changing the config-tree schema (new top-level keys or the sidecar/parts shape)
- Adopting a new external dependency
- Changing the manifest / lock file format


## Conventions enforced

- **Next number:** list `docs/decisions/NNNN-*.md`, take the highest existing number plus one. Never reuse numbers.
- **Filename:** `NNNN-kebab-title.md`.
- **Template:** copy section structure from `docs/decisions/template.md` (inline the content via `Write`; do not shell-copy the file).
- **Required frontmatter:** `status` (`Proposed` — initial state), `date` (today, ISO-8601), `supersedes` (array of ADR numbers or `[]`), `superseded_by` (number or `null`), `tags` (≥1 keyword label), `related` (array of ADR numbers or `[]`), `domains` (≥1 coarse domain key — drives the per-domain `docs/domains/<domain>.md` index).
- **Required sections:** Context, Decision, Invariants, Consequences, Alternatives Considered, in that order. Delete the authoring checklist before committing.
- **Predecessor flip:** if fully superseding an earlier ADR, update its `status:` frontmatter field to `Superseded by ADR-NNNN` in the same commit.

## Procedure

1. **Pick the next ADR number.** List `docs/decisions/NNNN-*.md` and take the maximum number plus one.

1. **Write the ADR file** using `Write`. Copy section structure from `docs/decisions/template.md` and fill all sections:
   - **Context:** the problem, couplings, prior discoveries. Mutable while `Proposed`.
   - **Decision:** numbered items, each a discrete commitment. Numbers matter for partial-item supersedence.
   - **Invariants:** testable textual contracts. Each bullet must be a verifiable property the codebase must maintain.
   - **Consequences:** honest about trade-offs accepted, operational implications, downstream work created or unblocked.
   - **Alternatives Considered:** real options weighed and the one-line reason each was set aside. Skip if there were no genuine alternatives.
   - Delete the authoring checklist before committing.

1. **Update or create the relevant domain doc** under `docs/domains` if the ADR materially shifts a domain's current state. Include this file in the same commit as the ADR.

1. **Flip predecessor status** if fully superseding an earlier ADR: update its `status:` frontmatter field to `Superseded by ADR-NNNN` in the same commit.

1. **Update the README index.** Add a row in the appropriate area table of `docs/decisions/README.md`. Columns: ADR link, title, Status, Notes.

1. **Tag enforceable Invariants and back them with a test.** Give each machine-checkable Invariants bullet an explicit slug, ``- `inv: <slug>` — …``, and back it with a comment tag — your project's comment marker followed by `invariant: <slug>` (e.g. `// invariant: <slug>` in Go/Rust/TS, `# invariant: <slug>` in Python/Ruby) — in a source file matching a glob in your `.claude/awf/config.yaml` `invariants.sources`, shipping in the same commit. `awf-check` fails once the ADR is `Implemented` if a tagged slug is unbacked, or if `invariants` is unconfigured (set `invariants.sources` or `invariants.disabled: true`). Bullets without a slug remain textual contracts. Run `./x gate` and `./x check` to confirm.

1. **Regenerate ACTIVE.md.** Run `./x sync` to regenerate `docs/decisions/ACTIVE.md`. Stage the regenerated file. Do not hand-edit `ACTIVE.md`.

1. **Commit everything in one commit.** Format: `docs(adr): propose NNNN <short title>`. The commit body names the load-bearing decision and explains why it warrants an ADR. The gate must pass — if the drift test fails, regenerate and re-stage `ACTIVE.md` before retrying.

1. **Autonomous continuation.** After the commit, continue to the next chain step without waiting for further approval, per the project's autonomous post-brainstorm rule.

1. **Terminal step: invoke `awf-reviewing-adr`** via the `Skill` tool, passing the ADR path. The reviewer runs the lens panel and reports findings; route them per the reviewing skill's procedure.

## Notes

- The ADR stays `status: Proposed` through the implementation sequence. It flips to `Accepted` (design final, implementation follows) or directly to `Implemented` (design and implementation land together) in a later commit — that is handled by `awf-adr-lifecycle`, not this skill.
- `docs/decisions/ACTIVE.md` is never hand-edited; it is always regenerated via `./x sync`.
- Decision items are numbered so future ADRs can override a specific item via partial-item supersedence (`related:` frontmatter, predecessor status stays live, successor cites "ADR-NNNN Decision item M" in prose).
- For the full ADR lifecycle, see `docs/workflow.md`.
