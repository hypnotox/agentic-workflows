---
format: plan-v2
date: 2026-08-14
adrs:
  - require-git-tracking-for-generated-artifacts
status: Proposed
---
# Plan: Track Generated Artifacts in Git

## Goal

Make repository and staged drift checks fail when awf-generated artifacts are absent from Git's index, including `.awf/awf.lock`; do not turn ignore rules into tracking authority or disable filesystem-only repository checks outside Git.

## Architecture summary

Add a metadata-only, prefix-scoped index-path query to the existing `internal/git` seam, with its backend-neutral contract suite. The project layer continues to derive the required generated set from its one operation-owned `OutputPlan`, adds the separately written lock, and classifies absent index members as `untracked` before ordinary missing-file drift. Ordinary checks carry a dedicated tracking-capability note separately from aggregate-only render advisories, and project construction preserves whether an adopter root is nested so resident outputs keep their existing exclusion. Staged checks use index membership alongside staged render bytes, recognize an absent staged lock as tracking drift when drift is selected, and retain existing operational handling for invalid staged authority. Tests exercise real ignored paths, index-only changes, nested roots, and presentation boundaries; current-state claim mutations and adopter documentation travel with the implementing transactions.

## Phase 1: Enforce repository generated-artifact tracking

**Execution mode: subagent-driven.**

Advances: ["generated-tracking-enforced", "tracking-boundaries-preserved"]
Completes: ["repository-tracking-enforced"]

### Task 1.1: Add ignore-independent index membership and repository drift
Latitude: exact
Applying: ["require-git-tracking-for-generated-artifacts:generated-artifacts-must-be-indexed", "require-git-tracking-for-generated-artifacts:ownership-follows-existing-authorities", "require-git-tracking-for-generated-artifacts:no-git-degrades-narrowly"]
Paths: ["internal/git/handle.go", "internal/git/git_test.go", "internal/git/entrypoints_test.go", "internal/project/project.go", "internal/project/check.go", "internal/project/check_test.go", "cmd/awf/checkrepo.go", "cmd/awf/checkrepo_test.go", "cmd/awf/checkgroup_test.go"]

Expose one sorted, metadata-only index-path entrypoint on `internal/git.Repo`. It must return repository-relative paths rerooted to the adopted project prefix, remain independent of ignore configuration and blob readability, honor context cancellation, deterministically collapse any repeated path stages, and be registered in the source-derived semantic entrypoint suite. Its contract evidence must cover ordinary, executable, and symlink entries, a nested adopted root, tracked paths later matched by ignore rules, ignored untracked paths, non-stage-0 metadata, and cancellation.

Thread the containing-repository prefix fact through ordinary project construction without exposing Git backend types or re-opening the repository. During `Project.CheckReport`, derive the required tracking set from the same `OutputPlan.writeFiles()` used by drift and advisory projections, include `.awf/awf.lock` explicitly, and omit resident-root outputs only for a nested adopter. Compare that set with index paths before ordinary filesystem classification. Emit sorted blocking `untracked` drift with an actionable render-and-force-add hint; when a path is simultaneously absent from disk and index, suppress the secondary `missing` finding while retaining all other established drift precedence.

When the project has no Git handle, skip only index membership and return a dedicated drift-capability advisory. Carry that advisory separately from aggregate-only `CheckReport.Notes`; direct `awf check repo drift` and the repo aggregate must show the capability advisory, while direct drift must continue to omit unrelated aggregate advisories. Preserve operational errors for unreadable index metadata. Add project and command tests proving output-plan writes and the lock are checked, an effectively globally ignored rendered file fails while an already tracked ignored file passes, the no-Git note is non-failing, direct advisory scope remains narrow, deterministic finding order holds, and nested resident outputs retain their exclusion.

### Task 1.2: Apply repository tracking authority
Latitude: exact
Applying: ["require-git-tracking-for-generated-artifacts:ownership-follows-existing-authorities", "require-git-tracking-for-generated-artifacts:no-git-degrades-narrowly"]
Paths: ["docs/decisions/require-git-tracking-for-generated-artifacts.md", ".awf/topics/parts/rendering/project-output-plan/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/awf.lock", "docs/topics/rendering/project-output-plan.md", "docs/topics/tooling/cli.md", "docs/decisions/INDEX.md"]

Transition the reviewed ADR from Accepted to Implementing under the lifecycle handshake. In the same implementing transaction, apply exactly the `update rendering/project-output-plan:check-report-single-plan` and `update tooling/cli:repo-check-capability-plan` operations with matching `Revised-by` provenance and test backing. Describe the one-plan tracking projection, nested resident exclusion, separately included lock, dedicated no-Git drift note, and direct-versus-aggregate presentation boundary without claiming staged enforcement before Phase 2. Render all generated outputs and inspect the changed current-state topic prose for those exact semantics and for contradictions with existing advisory scope.

### Phase close

The phase owner verifies the Git seam contract tests, focused project and command check tests, `./awf check staged`, and the project gate, then closes one coherent repository-tracking transaction.

```commit
feat(rendering): require tracked generated outputs (applies ADR batch)
```

## Phase 2: Enforce staged generated-artifact tracking

**Execution mode: subagent-driven.**

Completes: ["generated-tracking-enforced", "tracking-boundaries-preserved", "staged-tracking-enforced", "adopter-contract-documented"]

### Task 2.1: Classify absent staged generated artifacts
Latitude: exact
Applying: ["require-git-tracking-for-generated-artifacts:generated-artifacts-must-be-indexed", "require-git-tracking-for-generated-artifacts:ownership-follows-existing-authorities"]
Paths: ["internal/project/staged_drift.go", "internal/project/staged_drift_test.go", "cmd/awf/checkstaged.go", "cmd/awf/check_test.go", "cmd/awf/gate.go", "cmd/awf/gate_test.go"]

Extend staged drift with explicit index membership while preserving the staged config, lock, and rendered-output universe. Require every staged `OutputPlan.writeFiles()` path, plus `.awf/awf.lock`, to be indexed; preserve the existing nested-adopter resident exclusion. An expected path absent from the index yields sorted blocking `untracked` drift and suppresses content-derived `missing` or freshness-neutral behavior for that path. A staged deletion and an ignored working-tree replacement must therefore fail even when the planned hashes remain fresh.

Reshape staged preparation only as far as needed to make an absent staged lock an actionable `untracked` staged-drift finding whenever drift is selected. Direct `awf check staged state` retains its operational refusal because no staged authority can be loaded; invalid or unreadable staged locks also remain operational failures. The staged aggregate must retain the tracking finding rather than returning before report construction. Keep `stagedLock` and gate-version behavior single-homed and do not consult working-tree bytes or ignore rules.

Add focused tests for an absent expected output, staged deletion, ignored untracked replacement, tracked-but-ignored output, absent staged lock in direct drift and aggregate checks, state-only absent-lock behavior, invalid-lock behavior, nested resident exclusion, deterministic order, and coexistence with existing stale and hand-edited findings.

### Task 2.2: Apply staged authority and document the adopter contract
Latitude: exact
Applying: ["require-git-tracking-for-generated-artifacts:generated-artifacts-must-be-indexed"]
Paths: ["docs/decisions/require-git-tracking-for-generated-artifacts.md", ".awf/topics/parts/rendering/sync-and-drift/current-state.md", ".awf/parts/working-with-awf/sync-and-drift.md", ".awf/awf.lock", "docs/topics/rendering/sync-and-drift.md", "docs/working-with-awf.md", "docs/decisions/INDEX.md"]

While the ADR remains Implementing, apply exactly the `add rendering/sync-and-drift:generated-artifacts-tracked` and `update rendering/sync-and-drift:staged-drift-rendered-output` operations with matching Origin or Revised-by provenance and test backing. State the complete repository-and-staged invariant, lock inclusion, ignore independence, `untracked`-over-`missing` precedence, absent staged lock behavior, no-Git advisory, and nested resident exclusion. Update adopter guidance to say that `awf check` verifies generated files are indexed and that a global ignore may require an explicit forced add. Render generated documentation and inspect both rendered sections for accurate command scope, actionable wording, and absence of the retired "exactly stale and hand-edited" claim.

### Phase close

The phase owner runs the focused staged and command suites, confirms both direct drift commands report representative untracked paths with the intended check labels, runs `./awf check staged` and the project gate, and closes one coherent staged-tracking and documentation transaction.

```commit
feat(rendering): verify staged generated tracking
```

## Definition of done

- `dod: repository-tracking-enforced` `awf check repo drift` fails on any present or missing planned output or lock absent from the index, reports one `untracked` root-cause finding per path, and continues with a dedicated non-failing advisory outside Git.
- `dod: staged-tracking-enforced` `awf check staged drift` and the staged aggregate fail on staged deletion, ignored untracked replacement, or an absent staged lock without consulting working-tree bytes.
- `dod: generated-tracking-enforced` Every non-excluded output-plan write and `.awf/awf.lock` is covered by ignore-independent index membership in both verification universes, while already tracked ignored files remain valid.
- `dod: tracking-boundaries-preserved` Index access stays in `internal/git`, output-set ownership stays in `internal/project`, nested resident outputs keep their exclusion, direct advisory scope remains narrow, and existing stale and hand-edited classifications remain intact.
- `dod: adopter-contract-documented` Current-state sources and rendered adopter guidance describe the implemented tracking behavior, all declared ADR operations are Applied while the ADR remains Implementing, `./x check` is clean, and `./x gate` passes.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, spike answers, follow-ups, and findings surfaced during implementation.

- Corrected Phase 1 lifecycle wording from Proposed to Accepted to Accepted to Implementing, because the reviewed ADR status is already Accepted. The phase close subject now carries `(applies ADR batch)` to match the lifecycle authority.
- Phase 1 review found the exported drift-only `Project.Check` projection had no legitimate production consumer after tracking advisories required `CheckReport`; with user approval it was retired, the `check-report-single-plan` claim was Reapplied, and all check callers now retain the complete report. Review also required stronger nested composition, global-ignore, aggregate-note, and index-rerooting evidence plus an adopter-facing changelog entry.
- Renewed Phase 1 review narrowed untracked-over-missing suppression to absent files and strengthened the named claim proofs for heterogeneous outputs, the lock, top-level and nested resident handling, direct and aggregate tracking notes, the retired projection, and root versus nested Git handles.
- Final Phase 1 review required exact whole-plan tracking-set proofs for top-level and nested adopters, a production-wired no-Git presentation proof, and a required tracking-drift input on locked-file classification; the settlement applies all three mechanical findings.

After implementation assurance settles, `awf-effort-workflow` owns the terminal artifact transaction: reconcile final deviations and review settlement here, append only the ADR's Implemented status event, change this plan to `status: Implemented`, regenerate the decision index and lock, and commit those lifecycle-only changes together.
