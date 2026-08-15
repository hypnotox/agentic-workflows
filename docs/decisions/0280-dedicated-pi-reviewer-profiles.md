---
format: current-state-v4
slug: dedicated-pi-reviewer-profiles
status: Proposed
date: 2026-08-16
---
# ADR-0280: Dedicated Pi reviewer profiles


## Context

ADR-0123 introduced one `subagent_review` tool whose required `kind` argument dispatches to the
ADR, plan, or code reviewer contract. The adapter later moved onto pi-tools' profile protocol, but
retained that earlier shape as one `awf-review` profile. The profile now owns runtime identity,
scheduling, schema, and preparation, while an internal kind switch still selects the actual
reviewer. As a result, runtime details identify every review as the generic profile, the public
schema permits three behaviors instead of describing one reviewer, and reviewer-specific policy
remains coupled behind the switch.

The three reviewers continue to share the same report-only capability boundary and the `review`
model-preference role established by ADR-0151. Separating runtime identity does not require
separating model policy. pi-tools schedules concurrency by profile identity, so splitting the
profile also makes the existing review concurrency limit apply independently to each reviewer.

## Decision

1. `decision: one-profile-per-reviewer` Register the ADR, plan, and code reviewers as distinct
   pi-tools profiles exposed respectively as `subagent_review_adr`, `subagent_review_plan`, and
   `subagent_review_code`. Each tool has a closed schema containing the review task and optional
   exact model override, with no reviewer-kind argument.
2. `decision: retire-generic-review-tool` Remove the generic `subagent_review` tool and its
   `awf-review` profile without a compatibility alias. Rendered workflow guidance names the
   dedicated tool at each governed dispatch site.
3. `decision: preserve-shared-review-policy` All three reviewer profiles retain the shared
   report-only tool boundary and `review` model-preference role. Each profile independently permits
   ten active calls under pi-tools' profile-scoped scheduler.

## State changes

- update `rendering/pi-workflows:pi-structured-exploration-contract`

## Consequences

Runtime profile details and queues identify the reviewer that actually ran. Each public tool has a
narrow schema, invalid kind-to-contract combinations disappear, and reviewer-specific behavior can
change without expanding a shared dispatch switch. Existing callers must adopt the dedicated tool
names immediately because no legacy alias remains.

The model-preference file and wizard stay stable: one `review` preference still governs every
reviewer. The maximum number of simultaneous reviews across different kinds can exceed the prior
shared total because each profile owns its own ten-call limit; this is an accepted consequence of
making each reviewer a genuine scheduling identity.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep one review profile and the `kind` argument | Preserves a dispatch layer that hides the actual reviewer from profile identity and keeps an unnecessarily broad schema. |
| Add dedicated profiles but retain `subagent_review` as an alias | Leaves two public routes for the same behavior and prolongs the ambiguous generic contract. |
| Give each reviewer a separate model-preference role | Profile identity and model policy are separate concerns; no reviewer-specific model policy is required. |

## Status history

- 2026-08-16: Proposed
