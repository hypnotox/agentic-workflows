---
format: plan-v2
date: 2026-08-24
adrs: [hybrid-raw-coverage-ratchet-and-targeted-mutation-regression]
status: Proposed
---
# Plan: Implement RF-009 coverage policy and regression control

## Goal

Replace the repository's filtered 100 percent coverage blocker with a generated exact raw-miss
ratchet, independently blocking critical selectors, honest ignore admission, and a bounded mutation
regression check for changes owned by `cmd/covercheck`. Preserve one whole-module profile, raw and
filtered reporting, behavior-first tests, and advisory mutation everywhere else. Do not add a second
profile, threshold or rigor layer, production metric seam, broad mutation gate, adopter-facing
configuration, or audit-program change.

## Architecture summary

Extend `internal/coverage` from a filtered percentage calculator into the single owner of typed raw
block identities, whole-profile selector derivation, directive inventory, canonical policy loading
and regeneration, and fail-closed policy evaluation. Keep `cmd/covercheck` as its CLI adapter and
keep the existing merged whole-module profile as the only measurement input. The tracked root
`coverage-baseline.json` is the generated policy artifact: unchanged misses retain their reviewed
reasons, covered misses disappear automatically, and added or moved misses cannot enter without an
explicit reason and independent review. Production and test directives remain separate populations;
the four Darwin and Windows publication rollback directives remain explicitly unmeasured static
entries.

Build the mutation trust path on the existing gremlins v0.6.0 and `cmd/mutants` parser. First close
the two known assertion gaps without changing production. Then run one ordinary uncached
whole-repository preflight followed by hermetic one-worker normal-mode dry discovery and actual
execution over the complete whole `./cmd/covercheck` target. Execute each mutant against the
package-owned `go test ./cmd/covercheck` behavior-test universe, require exact dry-to-actual identity
equality and complete timeout-free reports, and compare exact survivors with the reviewed equivalent
set in the coverage baseline. One fail-conservative gate-owned path detector consumes either the local
staged snapshot or an explicit CI range and selects the exact `cmd/covercheck` blocker. It does not
change the broader test-lane classifier and never turns global advisory mutation into a blocker.

Keep new machinery non-authoritative until directive adjudication, the assertion fixes, and three
identical complete mutation runs fit the renewal budget. Activate the generated baseline, repository
and six exact whole-derived selector checks, targeted mutation selection, audit warnings, current
claims, local runner, and CI range wiring together. Before the first application transaction, the
execution owner must obtain the ordinary ADR lifecycle approval and move the linked ADR through
Accepted to Implementing; it remains Implementing, and this plan remains Proposed, through terminal
implementation assurance.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan records the best known route at authoring time, not a binding implementation choreography. A commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale listed path need not be touched. Reapproval is required only when the protected contract would change or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material instructions. Inconsequential and independently local edits require no deviation record. A delegated owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from route flexibility.

## Phase 1: Establish the canonical coverage policy owner

**Execution mode: inline.**

Advances: ["raw-identity-ratchet", "critical-selector-protection", "ignore-accountability", "reporting-compatibility"]

### Task 1.1: Add typed whole-profile policy evaluation and canonical baseline mechanics
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:raw-identity-ratchet", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:generated-baseline-owner", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:critical-selectors", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:retained-ignore-admission", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:platform-ledger", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["internal/coverage/coverage.go", "internal/coverage/coverage_test.go", "internal/coverage/policy.go", "internal/coverage/policy_test.go"]
Post-check: "Observe focused policy tests fail before their production implementation, then require `go test ./internal/coverage` to exit zero. Fixture matrices prove exact file, line, column, and statement-count identity; duplicate whole-profile merging; all six ADR-defined selector path sets derived from that same profile; strict canonical JSON; automatic removal-only improvement; reasoned additions and moves; malformed, missing, noncanonical, or unavailable evidence rejection; separate production and test directive inventories; static unmeasured platform entries; and no local-owner profile satisfying any blocker."

Introduce one typed block identity and one policy model rather than parallel profile parsers. Preserve
module-relative normalization, deterministic set ordering, and the existing duplicate merge rule.
Canonical regeneration must read the prior baseline, drop only identities absent because they became
covered, preserve review metadata for unchanged identities, and refuse every new or moved identity
without an explicit stored reason. Validate the exact selector roots from the ADR, directive classes
and evidence, platform constraints, and reviewed equivalent-mutant identities. Keep percentage
calculation and filtered-profile emission as views over the same parsed blocks and directive
interpretation.

### Task 1.2: Expose policy generation and diagnostics through the existing command owner
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:generated-baseline-owner", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:percentages-report-only", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:executed-ignore-error", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["cmd/covercheck/main.go", "cmd/covercheck/main_test.go", "internal/testsupport/testsupport.go"]
Post-check: "Observe focused CLI fixtures fail for the new policy modes before implementation, then require `go test ./cmd/covercheck ./internal/coverage` to exit zero. The CLI reports raw and filtered statement percentages without blocking on either, emits a byte-stable filtered profile, diagnoses every positively executed ignored guarded body, regenerates only canonical policy output, and exits nonzero for unavailable or invalid policy evidence without adding a second profile or production seam."

Keep existing invocation and `--emit-filtered` compatibility while adding the narrow generation and
policy-diagnostic interfaces needed by later phases. Generation writes only complete canonical bytes
and reads them back before success. A measured positive ignored body is an Error in policy
evaluation; this phase supplies the diagnostic but does not yet replace the repository gate.

### Phase close

Land the tested policy model, canonical generator, selector derivation, and compatible CLI while the
old gate remains authoritative and green.

```commit
feat(tooling): establish raw coverage policy owner
```

### Completed Phase 1 inventory and required freshness barrier

Phase 1 landed the typed identity and policy model, exact selector derivation, canonical regeneration,
separate directive inventories, static platform ledger validation, policy diagnostics, and compatible
CLI in `cadc5688c`, then settled its exact-range mechanical review in `5c2b96a6b`. The completed
mapper incorrectly treated every positive same-line condition or evaluation block as guarded-body
entry, so its executed-ignore diagnostic is affected by the ADR amendment.

Before any Phase 2 marker or assertion change, correct that completed Phase 1 mapper to canonically
OR-merge duplicate exact identities and match exact syntax-position guarded-body entry. First add
failing fixtures that distinguish same-line condition and evaluation blocks from body entry and that
require duplicate mode-set counts to combine by logical OR. Then make those fixtures pass, capture a
fresh uncached diagnostic and its exact live set, run the focused tests, staged check, and full gate,
and obtain renewed independent implementation assurance over the complete corrected Phase 1 range.
Phase 2 cannot begin until that correction and renewed assurance settle.

## Phase 2: Settle directive truth and behavior-first assertions

**Execution mode: inline.**

Advances: ["ignore-accountability", "mutation-regression", "reporting-compatibility"]

### Task 2.1: Adjudicate executed ignores and prepare the retained directive inventory
Kind: batch
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:retained-ignore-admission", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:platform-ledger", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:executed-ignore-error", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["glob:**/*.go", "internal/coverage/policy_test.go", "coverage-review.json"]
Representative: "A production directive remains only when direct source and behavioral evidence place it in one admitted class; an executed guarded body loses its false ignore while its existing behavior oracle remains unchanged."
Edge: "The Darwin and Windows publication rollback entries remain present as static platform-only evidence and are not claimed as measured by the Linux profile; `_test.go` directives remain visible only in the test inventory."
Post-check: "Run an uncached whole-module profile and the policy diagnostic after confirming the profile command succeeded. Before marker correction, capture its nonzero result and exact positively executed ignored guarded-body findings; after correction it reports a zero terminal set of those findings, a zero terminal set of unclassified or unsupported retained production directives, disjoint production and test inventories, and exactly the source-proven platform ledger entries from the ADR. `coverage-review.json` loads strictly and maps every terminal production directive identity to one admitted class and concrete source/caller/test evidence. Focused affected-package tests and `go test ./internal/coverage ./cmd/covercheck` exit zero; the source diff contains no control-flow, export, seam, or assertion weakening introduced for a metric."

Use the canonical diagnostic to adjudicate every member of the live positively executed guarded-body
set from the fresh canonical OR-merged profile, without freezing a count or preselecting identities.
Remove a false marker rather than changing behavior or manufacturing reachability. Review every
retained production directive against one of the four admitted classes and record direct evidence
for later baseline generation; never infer admission from reason text alone. Treat the historical
eight only as context, not as an expected identity list. Do not edit a test-source directive to make
it satisfy a production entry.

### Task 2.2: Close the two known `cmd/covercheck` mutation survivors in tests only
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:targeted-mutation-blocker", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["cmd/covercheck/main_test.go", "cmd/covercheck/main.go"]
Post-check: "Retain the invalid current Gremlins cutoff evidence only as corroboration that the two immutable-base arithmetic gaps recur before cutoff; never treat it as complete, timeout-free, survivor-complete, baseline, qualification, or blocker evidence. Strengthen the exact partial-profile diagnostic assertion without changing `cmd/covercheck/main.go`. Temporarily replace its unchanged subtraction with addition, require the focused test to fail on the exact diagnostic, fully restore production, and require the same test to pass. `git diff --exit-code b8905f02c -- cmd/covercheck/main.go` succeeds."

Assert the exact covered, total, and reported percentage relationship instead of merely asserting
that subtraction fails. The temporary, fully restored subtraction-to-addition mutation and exact
diagnostic are the current-tip red/green oracle for both duplicate-semantic arithmetic gaps. Preserve
every production branch and interface.

### Phase close

Land truthful ignore markers and stronger behavior assertions with focused and full old-gate evidence
green; do not generate or activate the baseline yet.

```commit
fix(tooling): remove false coverage exclusions
```

## Phase 3: Qualify the exact targeted mutation blocker

**Execution mode: inline.**

Advances: ["mutation-regression"]

### Task 3.1: Make the existing mutation report owner enforce the trust contract
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-trust-contract", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-renewal", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["cmd/mutants/main.go", "cmd/mutants/main_test.go", "go.mod", "go.sum"]
Post-check: "Observe focused parser fixtures fail before implementation, then require `go test ./cmd/mutants` to exit zero. Fixtures cover gremlins v0.6.0; all eleven explicit true and false operator booleans; exact module-relative file, line, column, and mutator identities; reviewed-equivalent acceptance; exact dry/actual identity equality; and rejection of duplicate, missing, extra, empty, malformed, structurally incomplete, `NOT COVERED`, `NOT VIABLE`, `SKIPPED`, `TIMED OUT`, unknown-status, and timed-out reports. Renewal evidence is rejected unless three complete runs have one identical trusted status set, each aggregate run stays within 900 seconds, and their total stays within 25 minutes."

Extend the existing parser rather than introducing a second mutation report owner. Distinguish a
complete clean report from a pre-created file that gremlins never completed. Accept only `KILLED` or
a `LIVED` mutant whose exact stable identity is in the baseline's independently reviewed equivalent
set. Keep the general `./x mutants` path advisory while giving the targeted caller a blocking result.

### Task 3.2: Share fail-conservative owned-path selection across staged and range inputs
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:targeted-mutation-blocker", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-trust-contract"]
Paths: ["x", "internal/project/gate_runner_test.go", ".github/workflows/ci.yml"]
Post-check: "Run the focused gate-runner contract tests and require zero failures for staged and explicit-range additions, modifications, deletions, rename pairs, non-owned paths, empty input, malformed NUL input, unavailable Git evidence, missing endpoints, and uncertain classifications. Both input modes call one exact `cmd/covercheck` ownership predicate; every uncertainty selects mutation, while a proven non-owned set skips it. The ordinary test-lane selection matrix and global advisory `./x mutants` behavior remain unchanged."

Add an explicit range input suitable for CI without weakening the staged snapshot used locally. The
shared detector owns only `cmd/covercheck` path selection and must not infer from the broader Go-test
categories. CI must fetch or otherwise possess both named endpoints and provide the event-appropriate
base and head; inability to establish the range runs the blocker. Keep the actual CI blocker wiring
dormant until Phase 4 activation.

### Task 3.3: Run and preserve deterministic mutation qualification evidence
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-trust-contract", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-renewal"]
Paths: ["x", "internal/project/gate_runner_test.go", "cmd/covercheck", "cmd/mutants"]
Post-check: "First add fake-command gate-runner fixtures and observe them fail before orchestration implementation, then require focused tests to pass for exact preflight, census, discovery, execution, validation, and cleanup order; all operator flags and environment isolation; configuration, census, and dependency refusals; aggregate timeout and incomplete-report failure; and cleanup confined to the proven owned subtree. Stage the complete Phase 3 transaction, require no qualification-relevant unstaged or untracked path, record its `git write-tree` identity and base commit, and keep that staged tree byte-identical through Phase 3 close. Run the exact hermetic recipe three times against that staged tree. Each run proves a uniquely owned external `/tmp` root has adequate capacity and lies outside Git discovery; rejects every `GREMLINS_*` override; supplies an empty temporary config; explicitly sets all eleven operator booleans; verifies the exact filesystem versus no-tag `TestGoFiles` and `XTestGoFiles` census plus direct and compiled-test `internal/coverage` dependencies; runs `go test -count=1 ./...` once; and uses one-worker gremlins normal mode to dry-discover and actually execute the complete whole `./cmd/covercheck` target against `go test ./cmd/covercheck` per mutant. Every dry identity is unique and `RUNNABLE`; every actual identity exactly matches the dry set and is `KILLED` or an exact reviewed-equivalent `LIVED`; every other status and incomplete report fails. Retain evidence in ignored effort scratch, clean only the owned temporary subtree after proving no process or descriptor owns it, require each aggregate run to stay within 900 seconds, require the three runs to total at most 25 minutes, and require identical trusted status sets."

Expose one focused local command for the exact blocker so qualification and later gate invocation use
the same preflight, census, hermetic environment, dry-discovery, execution, validation, and owned
cleanup recipe. Keep its orchestration behind fake-command contract tests so the live runs qualify
behavior already protected by durable red-then-green oracles. Hash or canonicalize the three parsed
status sets and record commands, tool version, all operator values, base commit, staged-tree identity,
elapsed segments, identity equality, report disposition, and cleanup evidence in Notes. Recheck the
same staged-tree identity and absence of qualification-relevant unstaged or untracked paths after each
run and immediately before the Phase 3 commit. Do not track a repository Gremlins config or baseline
an equivalent mutant merely to make qualification pass. Any recipe, tool, operator, package-test
census, dependency contract, or staged-tree identity change after this evidence invalidates it and
requires a fresh three-run renewal.

### Phase close

Land the trusted parser, shared selector, focused command, and qualification evidence while mutation
remains advisory outside exact `cmd/covercheck` ownership and automatic activation remains deferred.

```commit
feat(tooling): qualify covercheck mutation blocker
```

## Phase 4: Generate and atomically activate the policy

**Execution mode: inline.**

Completes: ["raw-identity-ratchet", "critical-selector-protection", "ignore-accountability", "mutation-regression", "reporting-compatibility", "current-authority"]

### Task 4.1: Generate and independently review the initial canonical baseline
Kind: batch
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:generated-baseline-owner", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:critical-selectors", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:retained-ignore-admission", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:platform-ledger", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:mutation-trust-contract"]
Paths: ["coverage-baseline.json", "coverage-review.json", "glob:**/*.go"]
Representative: "Each raw repository miss appears by exact identity with its reviewed admission reason and in every applicable whole-derived selector; each retained production directive has one admitted class and direct evidence."
Edge: "A moved span remains an addition requiring review even when an unrelated baseline identity disappears; the four platform entries remain unmeasured; the clean qualified mutation run contributes no fabricated equivalent survivor."
Post-check: "After a successful uncached whole-module profile, regenerate and read back `coverage-baseline.json`; a second regeneration produces an empty diff. Deterministic set comparisons report zero unbaselined repository or selector identities, zero stale identities, zero unsupported production directives, zero production/test overlap, zero positively executed ignored bodies, zero noncanonical fields, and zero unreviewed equivalent survivors. Independently review every initial raw identity, directive admission, platform entry, selector membership, and mutation entry by exact range rather than aggregate count, and record the review range and disposition in Notes."

Generate only after Phase 2 adjudication and Phase 3 qualification remain valid at the candidate tip.
Use the current raw profile as measurement, not the filtered profile. The authoring-time measured
figures in the ADR are diagnostics, never generation targets. A changed identity must be explained
and reviewed; an unrelated disappearance cannot offset it.

### Task 4.2: Add repo-local warning evidence for policy additions and moves
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:generated-baseline-owner"]
Paths: ["cmd/repoaudit/main.go", "cmd/repoaudit/main_test.go", "coverage-baseline.json"]
Post-check: "Require `go test ./cmd/repoaudit` to exit zero for explicit-range fixtures proving one Warning per added or moved raw identity with its stored reason, no Warning for removal-only improvement, preservation of `coverage-ignore-added`, Error severity for unavailable or invalid baseline evidence, deterministic ordering, and unchanged historical aggregate context."

Extend the existing repository audit registry rather than adding another policy layer. Baseline
addition and move review is visible Warning evidence and remains complementary to the gate's
fail-closed baseline requirement. Keep fresh-profile executed-ignore Error evaluation in
`internal/coverage` and the coverage gate, and do not weaken the existing ignore-addition warning.

### Task 4.3: Apply all claim changes and switch local and CI enforcement together
Kind: batch
Applying: ["hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:raw-identity-ratchet", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:percentages-report-only", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:executed-ignore-error", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:targeted-mutation-blocker", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:test-backed-policy", "hybrid-raw-coverage-ratchet-and-targeted-mutation-regression:behavior-first-oracles"]
Paths: ["x", ".github/workflows/ci.yml", ".awf/topics/parts/tooling/quality-gates/current-state.md", ".awf/topics/metadata/tooling/quality-gates.yaml", ".awf/docs/parts/development/command-runner.md", "docs/topics/tooling/quality-gates.md", "docs/development.md", "docs/decisions/hybrid-raw-coverage-ratchet-and-targeted-mutation-regression.md", "docs/decisions/INDEX.md", ".awf/awf.lock", "internal/coverage/policy_test.go", "cmd/covercheck/main_test.go", "cmd/mutants/main_test.go", "internal/project/gate_runner_test.go"]
Post-check: "After applying all seven declared State changes in one lifecycle transaction and rendering, `./awf context --show pending docs/decisions/hybrid-raw-coverage-ratchet-and-targeted-mutation-regression.md` reports no Remaining operation. Focused invariant proof checks resolve the four new claims to direct tests; the removed `coverage-gate-100` claim and stale blocking-percentage prose are absent; raw and filtered reports remain visible; local staged and CI explicit-range selection exercise the same fail-conservative owned-path detector; `./x check`, `./awf check staged`, and the full gate exit zero. Generated prose review finds no second profile, package-local blocker, broad mutation gate, or claim that platform entries were Linux-measured."

Use `awf-using-awf` for the authored current-state and documentation sources, then render their
projections and lock. Add valid proof annotations owned by the direct test surfaces for
`coverage-raw-identity-ratchet`, `coverage-ignore-admission`,
`coverage-executed-ignore-errors`, and `covercheck-mutation-regression`. Remove the old 100 percent
claim; update `covered-profile-honors-ignores` and `gate-severity-by-protected-property`; leave
`coverage-ignore-reason`, `mutants-timeout-untrusted`, and the audit-command claims unchanged except
for correcting prose already contradicted by their implementation if required by review.

Replace the local gate's percentage blocker with raw identity and selector evaluation and targeted
mutation selection. Supply CI's explicit event range to the same detector and fail closed when it
cannot be established. Keep Codecov's raw and filtered uploads informational and sourced from the
single profile. Activation must not land unless the canonical baseline, audit evidence, mutation
qualification, local gate, CI path, claims, proofs, and generated documentation are coherent in this
same phase.

### Phase close

Land the generated baseline, repo-local warning evidence, complete claim transaction, local and CI
blockers, proof annotations, documentation, and rendered outputs as one independently green
activation. Keep the ADR Implementing and plan Proposed pending terminal assurance. After the closing
commit exists, run the repo-local audit over the complete implementation range and include that
result in independent terminal assurance over the exact candidate tip.

```commit
feat(tooling): activate RF-009 regression policy (applies ADR batch)
```

## Definition of done

- `dod: raw-identity-ratchet` The gate compares exact module-relative raw uncovered-block identities, automatically accepts removal-only improvements, blocks every unreasoned addition or moved span, rejects invalid baseline evidence, and treats raw and filtered percentages as reports only.
- `dod: critical-selector-protection` The repository and all six exact ADR-defined critical path sets derive identities from the same merged whole-module profile, block independently, and cannot be satisfied by a local-owner profile or an aggregate-count swap.
- `dod: ignore-accountability` Every retained production directive has one admitted class and explicit evidence, test directives are separately inventoried, static platform entries remain honestly unmeasured, future executed ignored bodies are Errors, and repo-local addition or move and ignore-addition Warnings remain visible.
- `dod: reporting-compatibility` The existing profile invocation and `--emit-filtered` interface remain compatible, use one directive interpretation, report raw and filtered statement percentages, and preserve separate informational Codecov raw and covered uploads.
- `dod: mutation-regression` Exact `cmd/covercheck` owned-path changes trigger the pinned, hermetic, whole-target normal-mode recipe from both local staged and explicit CI range evidence; an ordinary whole-repository preflight and exact package-test and dependency censuses guard the narrowed per-mutant execution boundary, dry and actual identities must match, uncertainty runs the blocker, only reviewed exact equivalents pass, three-run renewal stays within budget, and mutation elsewhere remains advisory.
- `dod: current-authority` All declared State changes are applied with valid direct test backing, authored and rendered current-state and runner documentation match enforcement, the canonical artifact is independently range-reviewed, and focused checks, drift, staged check, and the gate pass at the activation tip.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material
cross-owner revisions rather than editing the plan; the parent supplies the report to phase review
and reconciles required plan changes with findings in one focused post-review settlement commit
before checkpointing or later execution. Record directive adjudications, test-only mutation red/green
evidence, all three qualification runs, initial-baseline range review, semantic render inspection,
phase and terminal review findings, and material route deviations here.

Plan review disposition: keep executed-ignore Error evaluation under the canonical fresh-profile
`internal/coverage` owner and limit `cmd/repoaudit` to range-based baseline addition and move evidence.
This avoids a stale or parallel executed-body policy while preserving the ADR's Error and Warning
boundaries.

Phase 2 directive adjudication: an uncached terminal whole profile reports raw 22,049/22,776,
filtered 21,914/21,914, 748 production directives, 35 disjoint test directives, and zero positively
executed ignored bodies after removing every member of the fresh 27-member canonical live set. Direct
source, caller, and owned-test review is recorded by exact identity in `coverage-review.json`: 17
directly tested process-exit seams, 427 revalidated impossible states, 300 safely uninducible
deterministic faults, and four platform-only rollback branches. The platform set is exactly lines 23
and 41 of `internal/effort/publication_darwin.go` and lines 73 and 94 of
`internal/effort/publication_windows.go`; no retained entry is unclassified or unsupported. Strict
loading and exact-set reconciliation against the terminal analysis pass; the evidence artifact has
SHA-256 `c348e0c89069c61d61bce9a073465eb293f78eb79f6ed9f247c982a1d9dc8c59` and remains the Phase 4
baseline-generation input.

Phase 2 reasoned deviation: after Phase 1 expanded `cmd/covercheck`, two operator-pinned,
single-worker integration runs reached the 900-second cutoff without a report, and one diagnostic run
remained incomplete at 1,800 seconds after processing 21 mutants. These runs are invalid for baseline,
qualification, completeness, timeout, survivor, or blocker claims. Combined with the prior three
complete immutable-base runs, they corroborate only the two recurring arithmetic gaps at the unchanged
coverage subtraction. Phase 2 therefore uses a fully restored subtraction-to-addition mutation and the
exact partial-profile diagnostic assertion as its current-tip red/green oracle. The controlled mutant
reports three uncovered statements instead of one and makes `TestRunBelowHundred` fail; the restored
production subtraction makes the same focused test pass, and `cmd/covercheck/main.go` is byte-unchanged
from the phase base.

Recipe-amendment freshness: the independently reviewed Accepted ADR now replaces only the future
integration-mode Phase 3 route with the D16-proven package-owned normal-mode route. D16 ran at the
clean completed-Phase-2 tip `69a374aec`, proved the exact two-file compiled and filesystem test census,
the direct and compiled-test `internal/coverage` dependency contract, dry/actual equality for the
complete 23-identity target, and 23 killed mutants with no other status in 96 aggregate seconds. The
amendment changes no Phase 1 or Phase 2 implementation contract: Phase 1 still owns the target and
dependency behavior, and Phase 2 still owns marker truth and test-only assertion strength. Their landed
code already supplied the successful D16 calibration universe, while no completed phase implemented
or qualified the rejected integration recipe. Existing completed-phase assurance therefore remains
fresh; D16 is single-run route evidence only and does not satisfy Phase 3's three-run qualification.
Ordinary linked-plan review must confirm this affected-phase inventory before Phase 3 can be
authorized.

Phase 3 qualification evidence handling: D18 requires the complete tracked transaction to be staged
before qualification and its `git write-tree` identity to remain byte-identical through the phase
commit. The three run-specific status digests, timings, reports, cleanup proofs, base commit, and
staged-tree identity therefore remain in ignored effort evidence and the phase checkpoint rather than
being appended to this tracked plan after the runs. This preserves the stricter authorized identity
oracle while phase review receives the complete evidence inventory and exact retained locations. The
first three clean runs at staged tree `ebee6a3e4598724e896d04c601c348afeb30385c` are diagnostic only:
the old filtered-coverage gate then exposed missing parser branch tests, and the required test-only
repair changed the staged identity, invalidating that entire set before phase close. A second clean
set at staged tree `ec8e4fe2d07261d1734073069c0499a6a2c45b3b` is also diagnostic only: the subsequent
full gate exposed two ineffectual increments immediately before fail-closed parser returns, and the
minimal lint correction changed the staged identity again.

Phase 3 exact-range review at `b2c8cc99e` found seven authority-preserving trust gaps. Settlement
removes stale dry and actual reports before execution, validates every renewal as three dry/actual
pairs, binds reports to the repository module, rejects non-finite durations, keeps all post-root
recipe commands under one exported temporary environment, and makes cleanup consume caller paths as
data rather than trap source. The mutation checker now loads the canonical generated baseline through
`internal/coverage`; the explicit `-` input is the no-equivalents qualification path before Phase 4
activation. These corrections invalidate the earlier qualification evidence and require one fresh
three-run set against the complete corrected staged tree. The single verify pass found one residual
mechanical oracle gap: renewal fixtures now use distinct paths and durations, reject corrupt second
and third pairs, and fail under a controlled constant-index reader mutation.

Phase 4 final baseline evidence: after all Go changes, the uncached whole profile has SHA-256
`f6e8f834795a8f1aa2d5cdebf801dd270d8b257b873dbfc49d1e016f75a509c0`, raw
22,285/23,017, filtered 22,150/22,155, and 715 exact raw-miss identities representing 732
statements. The independently reviewed input has SHA-256
`ef86d1d84d443cd9cbb9c29f3f937fcb1e2862f22c1c7f21e0f816d614ae34dc`; every miss has a
unique exact-range, owning-behavior, visible-debt reason. The generated baseline has SHA-256
`c5c0d0b4e38d5f7a35edc9caf9939a8abc15f77918808cafbb1c6b31b71df9c8` and is byte-identical
after a second regeneration. Exact reconciliation proves all six selector projections, 748 admitted
production directives, 35 disjoint test directives, four unmeasured platform entries, zero executed
ignored bodies, and zero equivalent mutants. Exhaustive independent review first exposed generic miss
reasons and two stale same-key directive citations, then six weak brace-only snippets and seven empty
source citations. Fresh-from-absence regeneration plus exact evidence corrections settled every
finding; the final targeted renewal passed with no residual aggregate-only assumption or evidence
quality defect.

Phase 4 reporting compatibility preserves the qualified mutation target: the unchanged legacy
`covercheck <profile>` invocation retains its historical filtered-threshold exit behavior but is no
longer the repository gate. The gate calls `covercheck --policy`, where raw and filtered percentages
are reports and exact policy findings determine exit status; `--emit-filtered` remains compatible.
This satisfies the Accepted replacement of the repository percentage blocker without silently
breaking the existing standalone interface.

After the Phase 4 commit exists, run the repo-local audit over the complete implementation range.
After terminal implementation assurance settles over that exact tip and audit evidence, return
integration-ready with the ADR still
Implementing, this plan still Proposed, and managed topology intact. Effort workflow owns terminal
ADR and plan closure, integration, topology removal, retrospective, and finish. The audit remediation
program, later issues, and archived evidence remain outside this effort.
