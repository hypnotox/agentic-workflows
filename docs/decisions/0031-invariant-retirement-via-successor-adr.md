---
status: Implemented
date: 2026-06-29
supersedes: []
superseded_by: ""
tags: [invariants, adr-system]
related: [8]
domains: [invariants, adr-system]
---
# ADR-0031: Invariant Retirement via Successor ADR

## Context

[ADR-0008](0008-invariant-backing-tooling.md) made invariant backing enforce-by-default:
every `inv: <slug>` declared in the Invariants section of an **Implemented** ADR must be
backed by a `<marker> invariant: <slug>` comment in a configured source file, or `awf check`
fails. The checker is `internal/invariants/Check`, which builds a `required` set by scanning
Implemented ADRs' `Invariants` sections with `declRe` and then verifies each slug has a backing
tag (`internal/invariants/invariants.go`). ADR frontmatter is parsed by `internal/adr`
(`internal/adr/adr.go` — currently `status`, `domains`, `superseded_by`).

This model has no way to **retire** an invariant whose backing code is intentionally removed,
when the ADR that declared the slug must remain `Implemented` because it still holds *other*
live invariants. Two escape routes both fail:

- **Delete the backing comment.** The checker then reports the slug `Unbacked` and the gate
  fails — the declaring ADR is still Implemented and still lists the `inv:` bullet.
- **Edit the declaring ADR to drop the bullet.** That rewrites an Implemented ADR's Invariants
  section, violating the project's append-only-ADRs invariant (an Implemented ADR is a stable
  historical record; only its `status`/`superseded_by` frontmatter flips on supersedence).
- **Move the declaring ADR out of `Implemented`.** That un-enforces *every* slug it declares,
  including ones whose code is untouched.

The gap is concrete and imminent. The forthcoming hook-removal ADR deletes
`cmd/awf/setup.go`, which backs `inv: setup-guards-hookspath` — a slug declared by
[ADR-0023](0023-safe-adoption-existing-repos.md) alongside `inv: init-force-backs-up` and
`inv: uninstall-removes-lock-tracked`, both of which stay live and backed. ADR-0023 must remain
`Implemented`; only the one slug needs to stop being enforced. There is no precedent in this
repo for retiring a backed invariant, so this ADR establishes the mechanism before the
hook-removal change needs it.

The project already records *partial-item supersedence* successor-side (a later ADR cites
"ADR-NNNN Decision item M" via `related` + prose, leaving the predecessor's text untouched —
ADR-0023 item 5 did exactly this to ADR-0003 and ADR-0016). Invariant retirement should follow
the same shape: declared by the successor, predecessor left intact.

## Decision

1. **A successor ADR retires an invariant via a frontmatter list.** Introduce an optional
   `retires_invariants: [<slug>, ...]` ADR frontmatter field (default `[]`). The slugs name
   `inv:` invariants that this ADR's change removes from enforcement. The ADR that originally
   declared the slug is **left untouched** — its Invariants section remains the historical
   record; the retirement is recorded successor-side, mirroring the existing partial-item
   supersedence convention.

2. **Retirement takes effect only from an `Implemented` ADR.** A `retires_invariants` entry in a
   `Proposed` or `Accepted` ADR is inert (the slug stays required and backed). Retirement
   therefore activates in the same commit that flips the retiring ADR to `Implemented` and
   deletes the backing code, keeping the checker internally consistent at every commit.

3. **A dangling retirement is an error.** If a slug listed in any Implemented ADR's
   `retires_invariants` matches no `inv:` slug declared by any Implemented ADR, `awf check`
   fails with a message naming the slug and the retiring ADR. This catches typos and stale
   retirements (e.g. a predecessor later restructured) rather than letting them pass silently.

4. **Parse and enforce.** `internal/adr` parses `retires_invariants` into the ADR struct.
   `internal/invariants/Check` collects the union of `retires_invariants` across Implemented
   ADRs, applies the dangling-retirement guard (item 3), then subtracts the retired set from
   `required` before checking backing.

5. **Discoverability.** The rendered ADR template frontmatter gains `retires_invariants: []` so
   the field is visible to ADR authors, consistent with how `supersedes`/`related` already
   appear in the template.

## Invariants

Tagged slugs are backed by tests landing with implementation (enforced by `awf check` once this
ADR is `Implemented`).

- `invariant: inv-retirement-drops-slug` — a slug listed in `retires_invariants` of an `Implemented`
  ADR is not required to be backed: `awf check` is clean when that slug's backing comment is
  absent, given the slug is declared `inv:` by some Implemented ADR.
- `invariant: inv-retirement-implemented-only` — a `retires_invariants` entry in a non-`Implemented`
  ADR does not drop the slug; it remains required (and its absence of backing is still reported).
- `invariant: inv-retirement-dangling-errors` — a slug in an Implemented ADR's `retires_invariants`
  that no Implemented ADR declares as `inv:` makes `awf check` fail.

## Consequences

Easier:
- Invariant-backed features can be removed cleanly: delete the code, declare the retirement in
  the removing ADR, and the gate stays green — without rewriting an Implemented predecessor or
  un-enforcing its other invariants.
- The retirement is auditable: `retires_invariants` is greppable frontmatter pointing from the
  removing ADR to the slug it dropped, and the predecessor's Invariants section still shows the
  original contract.

Harder / accepted trade-offs:
- ADR-0008's enforcement contract is extended (recorded via `related: [0008]`, no full
  supersedence): "every `inv:` slug in an Implemented ADR is backed" now reads "…unless retired
  by an Implemented successor ADR's `retires_invariants`." ADR-0008 keeps its `Implemented`
  status and all its own invariants.
- The checker gains one collection pass plus the dangling-retirement guard; `internal/adr`
  gains one parsed field. Adopters' ADR template frontmatter gains an optional line.
- New behaviour needs coverage to clear the 100% gate: the drop, the Implemented-only gating,
  and the dangling-retirement error path.

Doc-currency obligations the implementing commit(s) must satisfy:
- The `invariants` domain narrative (`docs/domains/invariants.md`, via its convention part)
  gains the retirement mechanism.
- The agent guide's "Backed invariants" rule (rendered into `AGENTS.md`) is reworded to note the
  retirement exception.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Edit the declaring ADR to drop the `inv:` bullet | Rewrites an Implemented ADR's Invariants section — violates append-only ADRs and erases the original contract. |
| Move the declaring ADR out of `Implemented` | Un-enforces every slug it declares, including ones whose backing code is untouched. |
| Section-level marker in the successor (e.g. `- retired-inv: <slug>`) | Less discoverable than frontmatter and asymmetric with `supersedes`/`related`, which are already frontmatter ADR-cross-references. |
| Silently drop a retired slug with no dangling-retirement guard | A typo or stale retirement would pass unnoticed, weakening the backing guarantee ADR-0008 established. |
