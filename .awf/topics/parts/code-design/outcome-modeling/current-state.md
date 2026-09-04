This topic governs outcomes introduced by new work and sites deliberately converted under
its authority, failure and success alike. Existing message-text identities, shallow
predicates, operation-taxonomy category literals, and exported types no caller matches
remain bounded future candidates until a deliberate conversion brings them into scope; so
does the hard-safety refusal family (`git.HardSafetyError`), which predates the protocol
with its own category kinds and no protocol shape, and whose extension at a new
construction site stays a candidate, not a violation, until the family is deliberately
converted.
The rendering implementation of the protocol follows `code-design/presentation-ownership`
(the package owning the outcome model renders it); the moment a second package implements
the numbered-step format, `code-design/single-home` requires the shared helper.

## Claims

### `invariant: actionable-outcome-protocol`

A new or deliberately converted refusal or partial-progress outcome that observes
repository, worktree, or effort state carries the actionable outcome protocol: a category
from the closed state-kind vocabulary (`cleanliness`, `operation`, `topology`, `ancestry`,
`repository-identity`, `merge-conflict`), a present-tense condition stating the observed
state rather than what the command attempted, the exact affected paths known from the
operation, an ordered remedy whose steps are each independently executable and render
through central `Steps` as `step 1: ...`, `step 2: ...`, and so on, and a cause present
exactly when the condition observes a failed call. A mutation failure does not infer hidden
state or claim rollback. Its steps direct the operator to inspect the reported paths and
Git state, correct the blocking condition, and rerun the ordinary command to converge.
Backing: test
### `invariant: typed-outcome-for-caller-branching`

A cause a caller must branch on in new or deliberately converted code is a distinct error
type or sentinel, exported when the branching caller sits outside the defining package,
carrying `Unwrap` when it wraps a cause, and matched with `errors.Is` or `errors.As`;
production control flow never branches on message substrings.
Backing: unbacked
Verify: For each changed branching site, confirm the branch tests identity through errors.Is or errors.As on a declared type or sentinel, and that no substring match on an error message decides production control flow.
### `invariant: errors-is-over-os-predicates`

New or deliberately converted code matches a standard-library condition with the
`errors.Is` identity family (`fs.ErrNotExist`, `fs.ErrExist`, `fs.ErrPermission`), never
the shallow `os.IsNotExist`, `os.IsExist`, or `os.IsPermission` predicates, which do not
unwrap.
Backing: unbacked
Verify: Search the changed files for os.IsNotExist, os.IsExist, and os.IsPermission; any occurrence in a new or deliberately converted site fails.
### `invariant: consumed-identity`

A new or deliberately converted exported error identity arrives in the same green
transaction as at least one consumer that branches on it through `errors.Is` or
`errors.As`; it may land without an in-repo branching consumer only when its consuming
caller is named and documented in the same transaction. This specializes
`code-design/dependency-composition:concrete-first-consumer` to error identity.
Backing: unbacked
Verify: For each newly exported error type or sentinel in the diff, find the errors.Is or errors.As consumer in the same commit, or the named and documented consuming caller; absent both, the identity fails.
### `invariant: test-identity-assertions`

A new or deliberately converted test asserts a produced error's identity through
`errors.Is`, `errors.As`, or the exported type, and asserts message text only where the
rendered message is itself the contract, such as CLI or report output.
Backing: unbacked
Verify: For each changed test that asserts an error, confirm identity flows through Is, As, or a typed match, and that any message-text assertion targets output whose exact rendered text is the contract under test.
