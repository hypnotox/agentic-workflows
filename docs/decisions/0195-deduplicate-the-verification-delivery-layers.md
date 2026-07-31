---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0195: Deduplicate the verification delivery layers

## Context

A 2026-07-31 inventory of this repository's verification surfaces measured how often each
enforcement step runs for one typical single-commit change. The full gate (test suite with
coverage, containerized Pi-extension check, vet, lint, cross-compile matrix, dead-code and
workflow-pin checks) runs four times on the identical tree: once manually because the agent
guide's staged-authority invariant and several skills instruct "Stage the complete
transaction, run `awf check --staged`, then run `./x gate`" before every commit, once in the
pre-commit hook payload (`.awf/hooks/pre-commit.sh`), once in the pre-push payload, and once
in CI. The prose and working-memory scans run five times, twice within the same pre-commit
script: `./x gate`'s tail invokes `./awf check prose` and `./awf check memory` (a repo-local
composition choice) seconds before the payload's own standalone invocations scan the
identical staged tree. CI additionally reruns the entire test suite a second time in the
same job solely to produce a `coverage.out` artifact for a Codecov upload that never blocks,
while the gate's own profile is written to a discarded `mktemp` path.

Constraints that shape the fix:

- The staged-authority sentence has no shared template source. It is hand-duplicated across
  eight shipped surfaces: `templates/agents-doc/AGENTS.md.tmpl` (the invariant bullet),
  `templates/skills/adr-lifecycle/SKILL.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`,
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`, and variant restatements in
  `templates/agents/implementer.md.tmpl`, `templates/plans-template/template.md.tmpl`,
  `templates/plans-readme/README.md.tmpl`, and `templates/skills/writing-plans/SKILL.md.tmpl`.
  Extraction into a partial is unavailable: partials may not reference `.vars`
  (`internal/project/configreference_test.go`), and each copy interpolates command vars.
- awf never wires hooks (ADR-0048): the rendered payloads are inert until an adopter
  activates them. Shipped default text therefore must not assert that the pre-commit hook
  enforces anything; for a fresh adopter that assertion is false.
- In this repository the `.githooks/` stubs are wired and there is no `pre-merge-commit`
  hook, so a clean `git merge` has never run the pre-commit payload; a conflicted merge
  commits through `git commit` and does run it. The staged-tree scans read the full index,
  which mirrors HEAD in a fresh CI checkout, so a CI invocation of the scans validates every
  pushed tree, merge commits included.
- No current-state claim pins the manual-instruction sentence, `./x`'s gate composition, or
  `.github/workflows/ci.yml`. One claim-backed test (`TestAgentsDocGuide`,
  `internal/project/spine_test.go`) counts the sentence's occurrence in the rendered guide;
  the owning claim's prose does not mention the sentence, so that is a proof-side assertion
  update, not a claim change.

The user settled the direction in the workflow-friction-reduction effort: conditional
wording for the instruction layer, the pre-push payload untouched, the scan dedup resolved
in favor of the shipped payload, and every accepted loss named explicitly in this record
rather than left implicit.

## Decision

1. The staged-authority instruction rewords, in place, in all eight shipped template
   copies. The normative content of every copy: the staged check and the gate must both
   pass for every commit; a wired pre-commit hook enforces both at commit time; a manual
   run before committing is required only in a clone without wired hooks. Each copy keeps
   its surface-appropriate shape (invariant bullet, skill step, plan-template note) and its
   existing command-var interpolations. No copy asserts unconditional hook enforcement, and
   no copy demands an unconditional manual run.
2. The agent guide's invariant bullet keeps its "Staged authority and green gate before
   every commit" title; its body carries the conditional wording of item 1 and drops the
   "the hook repeats the staged check as defense in depth" framing, which described the
   manual-first model this decision retires.
3. This repository's `./x gate` drops its two tail steps `./awf check prose` and
   `./awf check memory`. The rendered pre-commit payload's standalone scan lines become the
   single local enforcement point for both scans. The shipped payload template is unchanged
   by this item: its scan lines already render unconditionally and no-op at runtime when
   the corresponding opt-in knob is disabled.
4. `.github/workflows/ci.yml` gains explicit `check prose` and `check memory` steps, making
   CI the hook-independent backstop for both scans on every pushed tree.
5. The pre-push hook payload is unchanged. Consequence accepted and named: after item 3 the
   pre-push gate no longer covers the two scans transitively, so their local coverage is
   pre-commit only, with CI as the backstop for clean merges and hook-bypassed commits
   (clean merges never had local pre-commit coverage in the first place).
6. `./x gate` writes its coverage profile to a durable `coverage.out` at the repository
   root instead of a discarded `mktemp` path, and CI's standalone duplicate
   `go test ./... -coverpkg=./... -coverprofile=coverage.out` step is removed; the Codecov
   upload consumes the gate-written profile. The `coverage.covered.out` derivation via
   `covercheck --emit-filtered` is unchanged.
7. Documentation and proofs travel with the change: `.awf/parts/workflow/composing-the-gate.md`
   stops listing the two scans as gate steps and documents the payload/CI split, the gate
   descriptions in `.awf/docs/parts/testing/gate.md` and
   `.awf/docs/parts/development/command-runner.md` (rendered `docs/testing.md` and
   `docs/development.md`) are reworded the same way, the plain-punctuation invariant
   bullet's "wired into `./x gate`" phrasing in `.awf/agents-doc.yaml` is corrected to name
   the pre-commit payload and CI, the `TestAgentsDocGuide` sentence-count assertion is
   updated to the reworded text, and the `examples/sundial` fan-out re-renders in the same
   commits.

## State changes

None.

## Consequences

- Where hooks are wired, the full gate drops from four runs per change to one per commit
  (the pre-commit hook) plus two per push (the pre-push hook and CI, which runs on push
  events); the instruction layer stops demanding a manual duplicate of work a mechanical
  layer performs seconds later.
- Codecov's reported numbers now derive from the gate's own test invocation instead of an
  independently reproduced run. The command line is identical and the upload is
  informational, so the coupling is accepted.
- The shipped guide and skills become truthful for a fresh adopter: today's default text
  instructs a manual run and then calls the hook "defense in depth" even where no hook is
  wired; the reworded text states the actual enforcement topology.
- An adopter with neither wired hooks nor CI keeps the manual instruction as the only
  layer, exactly as today; nothing is removed for that posture.
- Local defense-in-depth for the prose and memory scans shrinks from two points to one;
  a hook-bypassed commit carrying a violation is now caught at CI rather than at pre-push.
  This is the honest cost of item 3, accepted per the effort's settled decisions.
- The rendered skill and guide surfaces change, so adopters receive the rewording at their
  next sync; the changelog carries an entry.
- The ceremony half of the friction-reduction effort (conditional verify pass, checkpoint
  and spill-notice weight, Record-block materiality) is deliberately out of scope here and
  lands as its own decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Extract the staged-authority sentence into a shared partial | Structurally blocked: partials may not reference `.vars`, and every copy interpolates command vars; a recorded pitfall documents this exact trap |
| Drop the manual instruction unconditionally | False economy for adopters without wired hooks or CI, who would lose their only enforcement layer |
| Keep the scans in `./x gate` and drop them from the payload | The payload is the shipped authority and cannot know whether an adopter's gate composition includes the scans; deduplicating in the payload's favor keeps the standard self-contained |
| Drop the pre-push gate as well | Declined by user decision: it stays as the local backstop for hook-bypassed commits and post-commit history rewrites |
| Add the two scans to the pre-push payload to restore two local points | Ships a template change to every adopter to close a gap CI already covers; rejected as new machinery in a decision that removes machinery |
| Keep CI's standalone coverage test run | Duplicates the gate's own test execution for no benefit once the gate's profile is durable and reusable |

## Status history

- 2026-07-31: Proposed
