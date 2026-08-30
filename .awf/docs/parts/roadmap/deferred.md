## Structured context results

Revisit a structured context result only when a demonstrated consumer can define its contract. The current focused text projections remain sufficient.

## Verify-line tightenings

The `code-design/outcome-modeling:actionable-outcome-protocol` and `code-design/package-composition:package-owns-one-sentence` Verify lines still need tighter evidence. Apply the improvements with the next change to either claim.

## Init collision over-refusal

The init collision probe conservatively refuses artifacts that a `--set` trim would deselect. Revisit this accepted behavior only after an adopter reports it as a problem.

## Decomposing `internal/project`

Resident-root policy and anchoring have moved into `internal/resident`, while `code-design/dependency-composition` owns generic dependency direction. Still open are a future package-cohesion pattern for methods that do not read receiver state and any further core decomposition justified by measured coupling.

## The rendered pre-commit payload validates the worktree, not the staged slice

A partial staging can leave a drift-inconsistent staged subset while the worktree passes pre-commit, landing broken HEAD. The repository-local hook uses a checkout-index check, but the shipped payload still checks the worktree. A language-agnostic fix needs a safe temporary-index execution design and an explicit latency tradeoff.

## Live-agent outcome evals

Deterministic fixture evaluations cover handoffs and skill parity. Live-agent golden-task outcome evaluations remain cost-prohibitive; revisit only with a concrete budget and scoring harness.

## Unmanaged Go `t.TempDir` directories survive abrupt process death

Managed TestMain homes are bounded under a recoverable root, but arbitrary Go `t.TempDir` paths survive abrupt death. The manager deliberately excludes them; any broader cleanup policy requires a separate safety decision.
