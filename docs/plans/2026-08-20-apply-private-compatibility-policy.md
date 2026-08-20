---
format: plan-v2
date: 2026-08-20
adrs: [bound-compatibility-support-to-managed-reality]
status: Implemented
---
# Plan: Apply Private Compatibility Policy

## Goal

Make the owner-approved private compatibility policy active, inventory-backed repository authority
and document unsupported installations clearly. Do not remove compatibility machinery, change
runtime behavior, or upgrade any managed adopter.

## Architecture summary

Each existing semantic owner receives only its policy claim: config owns the live schema floor, ADR
and plan topics own represented authored formats, audit owns the managed-history horizon, effort owns
resident formats, and upgrade owns installed releases, cutover formats, and the removal gate. The
working-with-awf source supplies user-facing installation support guidance. Accept the linked ADR
before application, then apply all declared claims in one source-driven render transaction while the
ADR remains Implementing; terminal lifecycle closure stays deferred until assurance and integration.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.


## Phase 1: Authorize the reviewed policy

**Execution mode: inline.**

Completes: ["policy-authorized"]

### Task 1.1: Accept the linked decision
Paths: ["docs/decisions/bound-compatibility-support-to-managed-reality.md", "docs/decisions/INDEX.md", ".awf/awf.lock"]

Use the ADR lifecycle transition to append only the Accepted status event to the review-settled
pending record. Preserve every reviewed byte and leave all State changes unapplied. Render the
index from source and verify that the ADR remains pending, Accepted, and amendable before any policy
claim is authored.

### Phase close

Close with the accepted decision and a clean rendered tree.

```commit
docs(adr): accept private compatibility policy
```

## Phase 2: Apply policy authority

**Execution mode: inline.**

Completes: ["active-policy-claims", "unsupported-installations-documented", "compatibility-boundary-preserved"]

### Task 2.1: Author claims in their semantic homes
Kind: batch
Applying: ["bound-compatibility-support-to-managed-reality:rolling-installed-release-floor", "bound-compatibility-support-to-managed-reality:live-source-schema-floor", "bound-compatibility-support-to-managed-reality:actual-managed-history-horizon", "bound-compatibility-support-to-managed-reality:represented-authored-formats", "bound-compatibility-support-to-managed-reality:managed-removal-gate"]
Paths: [".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/topics/parts/adr-system/adr-lifecycle/current-state.md", ".awf/topics/parts/adr-system/plan-artifacts/current-state.md", ".awf/topics/parts/tooling/audit-and-snapshots/current-state.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/tooling/upgrade-runtime/current-state.md"]
Representative: The config claim states only the live schema-46 floor; each authored-format claim states only the represented formats its package owns.
Edge: The upgrade removal-gate claim must not imply that floor advancement authorizes deletion, and the audit claim must distinguish pre-.awf empty history from the schema-3 lower bound.
Post-check: Run `./awf topic config/migrations-and-locks --history`, `./awf topic adr-system/adr-lifecycle --history`, `./awf topic adr-system/plan-artifacts --history`, `./awf topic tooling/audit-and-snapshots --history`, `./awf topic tooling/effort-management --history`, and `./awf topic tooling/upgrade-runtime --history`; require the eight expected claims to resolve exactly once with the pending ADR as Origin and no claim text to authorize RF-008B or RF-014B removal.

Add the eight declared claims with concise current-state prose and the pending ADR as `Origin`. Keep
component inventory detail in the ADR; claims state the durable policy relevant to their existing
owner. Do not change selectors, production code, migration tables, parsers, resident readers, bridge
machinery, or tests.

### Task 2.2: Document unsupported installations
Applying: ["bound-compatibility-support-to-managed-reality:rolling-installed-release-floor", "bound-compatibility-support-to-managed-reality:unsupported-boundaries"]
Paths: [".awf/parts/working-with-awf/commands.md", "docs/working-with-awf.md"]

Extend the commands guidance from its rendered default to state that owner-managed installations
support current plus one previous release, the floor advances only after all managed pins upgrade,
and older installed releases are unsupported. Direct readers to the existing upgrade command and
avoid promising runtime refusal for an old installed binary.

### Task 2.3: Apply and render the claim transaction
Applying: ["bound-compatibility-support-to-managed-reality:rolling-installed-release-floor", "bound-compatibility-support-to-managed-reality:live-source-schema-floor", "bound-compatibility-support-to-managed-reality:actual-managed-history-horizon", "bound-compatibility-support-to-managed-reality:represented-authored-formats", "bound-compatibility-support-to-managed-reality:managed-removal-gate", "bound-compatibility-support-to-managed-reality:unsupported-boundaries"]
Paths: ["docs/decisions/bound-compatibility-support-to-managed-reality.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "docs/topics/config/migrations-and-locks.md", "docs/topics/adr-system/adr-lifecycle.md", "docs/topics/adr-system/plan-artifacts.md", "docs/topics/tooling/audit-and-snapshots.md", "docs/topics/tooling/effort-management.md", "docs/topics/tooling/upgrade-runtime.md", "docs/working-with-awf.md"]

Transition the ADR from Accepted to Implementing and append one Applied event whose payload is
exactly, in declaration order:

- add `config/migrations-and-locks:live-source-compatibility-floor`
- add `adr-system/adr-lifecycle:managed-adr-format-support`
- add `adr-system/plan-artifacts:managed-plan-format-support`
- add `tooling/audit-and-snapshots:managed-history-decode-horizon`
- add `tooling/effort-management:managed-effort-format-support`
- add `tooling/upgrade-runtime:installed-release-compatibility-floor`
- add `tooling/upgrade-runtime:managed-compatibility-removal-gate`
- add `tooling/upgrade-runtime:managed-cutover-format-support`

Apply that batch in the same transaction as its source claims. Render all generated outputs, inspect
the six topic documents and working guide for semantic fidelity, and keep the final ADR status
Implementing even though Remaining is empty. Verification must establish clean drift, valid staged
claim provenance and lifecycle, and the full project gate.

### Phase close

Close with every declared claim active, unsupported installation guidance visible, the ADR
Implementing with no Remaining operations, and no compatibility code or managed adopter changed.

```commit
docs(config): apply private compatibility policy
```

## Definition of done

- `dod: policy-authorized` The review-settled pending ADR is Accepted before any declared State change is applied.
- `dod: active-policy-claims` All eight declared claims are active in their semantic topic owners with correct pending-ADR provenance, and the ADR is Implementing with all operations Applied.
- `dod: unsupported-installations-documented` Working-with-awf clearly states the rolling current-plus-previous installation policy, pin-upgrade precondition, unsupported older releases, and existing recovery command.
- `dod: compatibility-boundary-preserved` The transaction removes no compatibility machinery, changes no runtime behavior, upgrades no adopter, leaves clean rendered drift and the full gate green, and records any authorized residual scope honestly.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material cross-owner revisions rather than editing the plan; the parent supplies the report to phase review and reconciles required plan changes with findings in one focused post-review settlement commit before checkpointing or later execution. Record spike answers, follow-ups, and findings surfaced during implementation.
