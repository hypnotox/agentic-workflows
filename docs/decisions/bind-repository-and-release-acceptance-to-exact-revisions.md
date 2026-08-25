---
format: current-state-v4
slug: bind-repository-and-release-acceptance-to-exact-revisions
status: Proposed
date: 2026-08-26
---
# ADR-bind-repository-and-release-acceptance-to-exact-revisions: Bind Repository and Release Acceptance to Exact Revisions


## Context

The CI workflow runs gate and release-configuration jobs, but the live main-branch ruleset requires neither result. A signed direct push can therefore advance main while either job is failing, pending, skipped, or absent. Local hooks are optional and cannot substitute for remote acceptance.

The pre-commit hook materializes the index and checks its build and rendered state, then runs behavioral verification in the original working tree. Unstaged compensating changes can make a broken staged candidate pass. The local test selector also treats broad documentation families as test-free even though Go tests consume the README, architecture documentation, and changelog.

The release workflow verifies ancestry and runs the ordinary gate in a clean tag checkout. Its mutation selector reads staged changes, so it skips the release range. Successful CI on some main revision plus ancestry does not prove that the exact tagged revision passed the required jobs.

Repository workflow changes and hosted-repository settings form one acceptance boundary but have different operators. Workflow identities must stabilize before required checks, tag protection, or a release environment is enabled, or the settings can deadlock delivery.

## Decision

1. `decision: exact-main-revision` Main may advance only after the required gate and release-configuration jobs succeed for the exact candidate revision. Signed commits and fast-forward policy remain additional controls rather than substitutes for required status checks.
2. `decision: staged-candidate-is-test-authority` Pre-commit behavioral verification evaluates the materialized staged candidate, not working-tree bytes. It preserves the original parent-to-index changed set explicitly or runs complete behavioral lanes, so committing the temporary candidate cannot erase selection evidence.
3. `decision: evidence-based-test-selection` A path may skip behavioral suites only when repository dependencies prove that it cannot affect them. Broad documentation families are not test-free while repository tests or embedded inputs consume their contents.
4. `decision: exact-tagged-revision` Release publication is bound to successful required CI conclusions for the exact tagged revision. Main ancestry alone is insufficient. Tag creation or publication uses hosted protection that prevents an unverified revision from bypassing that boundary.
5. `decision: release-range-mutation-selection` Release verification runs the targeted mutation blocker over the previous release tag through the candidate tag revision. A clean tag index never substitutes for that release range.
6. `decision: documented-hosted-state-is-verified` Maintained documentation may claim that a hosted acceptance control is enforced only when operator verification confirms that the live setting names the stable workflow identity it protects.

## State changes

- update `tooling/quality-gates:covercheck-mutation-regression`
- update `tooling/quality-gates:staged-test-selection`
- add `tooling/quality-gates:exact-revision-repository-acceptance`
- add `tooling/quality-gates:hosted-main-acceptance-settings`
- update `tooling/changelog-and-release:release-gate-on-tag`
- add `tooling/changelog-and-release:hosted-release-protection`

## Consequences

A commit cannot rely on unstaged compensation, and a future main or release revision cannot be accepted solely because CI happened to run somewhere nearby. Release verification pays the targeted mutation cost for the actual release range.

Documentation and other repository inputs run the behavioral suites they can affect. Local gates may take longer, but skips become evidence-based rather than family-wide assumptions.

Hosted settings remain an operator transaction outside an ordinary commit. Repository workflow contracts land as invariants with test backing and `./internal/...` proof annotations; the exact-tag release wiring remains owned by the existing test-backed release-gate claim. Live required-status and release-protection claims land separately as rules with no repository-test backing and require operator verification. A failed settings update leaves the implementation incomplete rather than being represented as enforced.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep CI observational and rely on signed pushes | Authentication does not establish that the exact revision passed its required verification. |
| Run behavioral checks in the working tree after staged drift checks | Unstaged bytes can compensate for a broken staged candidate. |
| Treat all documentation as test-free | Repository tests and embedded inputs already consume specific documentation files. |
| Accept a tag whose commit is merely reachable from main | Reachability does not bind publication to successful CI conclusions for that SHA. |
| Use the staged mutation selector in a release checkout | A clean tag index contains no release-range selection evidence. |
| Rely only on the release workflow's exact-tag checkout and pre-publication gate | Tag-time self-verification does not establish the approved pre-existing CI conclusion or prevent an unprotected publication path from bypassing it. |

## Status history

- 2026-08-26: Proposed
