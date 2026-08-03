---
format: current-state-v3
slug: opt-in-commit-identity-and-signature-enforcement
status: Implementing
date: 2026-08-02
---
# ADR-opt-in-commit-identity-and-signature-enforcement: Opt-in commit identity and signature enforcement

## Context

The repository's Git-local `user.name` and `user.email` were set to the fixed identity used by Git
test fixtures. Because repository-local configuration overrides the correct global identity, agents
then created hundreds of unpublished commits as `T <t@example.com>`. Most of those commits carried
valid SSH signatures, demonstrating that signing alone authenticates a key without proving that the
recorded author and committer identities satisfy project policy. A smaller set was unsigned despite
`commit.gpgSign=true`, demonstrating that configuration defaults alone are bypassable.

The existing generated hooks validate the staged tree, commit message, merge transition, and project
gate. `pre-commit` and `commit-msg` run before Git attaches a commit signature, so neither can prove
that the exact resulting commit is signed. `post-commit` runs after the branch has moved; rejecting
there would require an automatic reset or amend, which is precisely the history rewriting this policy
must avoid. Git's `reference-transaction` hook instead exposes the new object IDs during its
`prepared` phase and may reject the ref update before it becomes visible on a branch.

A local hook is an early accidental-error boundary, not an unbypassable security boundary: a user who
controls the checkout can disable hooks or change a repository-owned trust file. Pre-push checking
catches ordinary hook bypasses before upload, while host-side signed-commit protection remains the
final publication boundary. awf renders hook payloads but never activates them, so adopters retain
ownership of whether and how the payloads are wired.

Projects need different policies. Some care only about fixture or corporate identities, some require
signed commits, and some require both. Existing history may predate opt-in and may contain commits
that cannot satisfy a new policy without rewriting, so the capability needs an explicit preview path,
clear activation guidance, and actionable failures rather than silently repairing history.

## Decision

1. awf adds an optional top-level `commitPolicy` configuration block with a required
   `grandfatheredThrough` full commit object ID whenever the block is present. Commits reachable from
   that baseline are tolerated ancestry; every commit reachable from a checked target but not from
   the baseline is policy-era work. `allowedIdentities` is a nonempty list of exact `{name, email}`
   pairs when present. `requireSignedCommits` defaults to `false`; when true, `allowedSigners` is a
   nonempty list of exact `{principal, key}` SSH signer records. `allowedSigners` while signing is
   disabled and an enabled signing requirement without signers are invalid. The two enforcement
   families are otherwise independent, and an absent block preserves current behavior.

2. `internal/config` owns parsing and structural validation, `internal/configspec` owns public field
   documentation, the render data and manifest include the block through their existing config
   projections, and schema generation advances once with a no-op migration for absent blocks. Older
   binaries refuse the newer lock generation under the existing version gate; existing adopters gain
   no policy or generated behavior until they author the block and wire its hooks. Upgrade never
   invents an identity, signer, key, or baseline.

3. Every configured name and email is nonempty valid UTF-8, has no leading or trailing whitespace,
   and contains no control character. Identity pairs are unique. `grandfatheredThrough` is lowercase
   hexadecimal of the repository's full object-ID width and must resolve to a commit at runtime.
   Each signer principal is one nonempty ASCII authorization token containing only letters, digits,
   `.`, `_`, `@`, `+`, or `-`, and signer records are unique. Each key is exactly one option-free,
   comment-free OpenSSH public-key record accepted by `ssh-keygen`, with no newline, trailing record,
   or unsupported key algorithm. A principal authorizes its associated key; it is not an identity
   asserted by the signed commit.

4. Identity enforcement evaluates the exact author and committer fields of each selected commit.
   Both must match one configured pair byte-for-byte. It does not infer identity from a signature,
   rewrite metadata, treat author and committer differently, or accept a name-only or email-only
   match.

5. Signature enforcement requires exactly the ordinary Git SSH signature semantics: the commit must
   carry a signature that `git verify-commit` validates against a temporary allowed-signers file
   derived from the configured principals and public keys. A signature header without successful
   cryptographic verification is not signed for policy purposes. GPG and X.509 signer configuration
   are outside the first capability rather than being accepted without an explicit trust model.

6. A cohesive `internal/commitpolicy` package owns the typed policy, commit facts, violation set, and
   operational-refusal outcomes, and evaluates facts supplied through a consumer-local interface.
   The existing `internal/git` boundary owns object resolution, revision walking, and `verify-commit`
   mechanics. `internal/project` composes one operation-scoped verifier from project configuration;
   `internal/commitpolicy` renders its typed outcomes for humans, while `cmd/awf` selects the
   renderer, emits output, and maps exit status. Hooks and tests consume the same project operation
   rather than parsing CLI prose or duplicating evaluation.

7. One public `awf check commit-policy <revision-or-range>...` command resolves explicit targets
   through the shared Git boundary, expands each target to commits reachable after
   `grandfatheredThrough`, deduplicates them, and reports every violation. This is the common verifier
   for human preview, generated hooks, tests, and diagnostics; hook scripts do not reimplement
   identity parsing or signature verification. With no configured policy it succeeds with one
   explicit disabled-policy note.

8. The hooks singleton renders a `reference-transaction.sh` payload in addition to its existing
   payloads. In the hook's `prepared` phase it reads all `<old-oid> <new-oid> <ref>` records, selects
   policy-era commits reachable from new local branch targets but not their old targets, deduplicates
   them, and invokes the common commit-policy check before Git moves any selected ref. A new branch
   checks all commits after `grandfatheredThrough`; deletion-only and backward-only updates introduce
   no commit. Any other hook state exits without re-evaluating policy. awf still does not activate or
   edit Git configuration; adopters wire the payload as `reference-transaction` when they opt in.

9. The generated pre-push payload reads every standard ref-update record and verifies all policy-era
   commits reachable from each nonzero local target before running the configured project gate. A
   branch target is expanded directly. An annotated tag is peeled recursively and its reachable
   commits are expanded; a lightweight tag is handled by its target type. A non-commit target that
   cannot reach a commit adds no commit but is reported as such in verbose diagnostics. Missing
   objects, a peel failure, or an unresolvable baseline fails closed with reconciliation guidance.
   Deletion updates add no commit. Because `grandfatheredThrough` defines the tolerated ancestry, the
   hook may safely check a superset of commits newly introduced to the remote and never treats stale
   remote-tracking refs as authority.

10. Before enabling either hook, adopters set `grandfatheredThrough` to the last commit they
    deliberately tolerate and run the commit-policy command over every unpublished target they intend
    to retain or publish. Enabling policy never rewrites, amends, resets, or re-signs an existing
    commit. A nonconforming commit after the baseline remains the adopter's explicit choice to
    recreate, move behind a later reviewed baseline, retain only on an unprotected ref, or leave
    unpublished; diagnostics never perform that choice.

11. Every violation names the commit, observed condition, and complete relevant allowlist. Identity
    failures say `identity <name> <email> is not allowed`, distinguish author from committer, list
    `allowed identities: ...`, and direct the user to correct Git identity and rerun the refused
    operation. Missing or invalid signatures say `commits must be signed by an allowed signer`, list
    `allowed signers: ...`, and direct the user to configure `commit.gpgSign`, `gpg.format`, and
    `user.signingKey` before rerunning. Multiple violations are stable and complete.

12. `internal/commitpolicy` also owns typed operational refusals following the project's actionable
    outcome protocol: category, condition, whether refs or the index changed, preserved cause, and
    ordered next action. Configuration, baseline, object-resolution, tag-peel, linked-worktree, and
    signature-process failures remain distinguishable. A reference-transaction refusal explicitly
    reports that refs did not move and how to rerun; presentation is centralized rather than assembled
    independently by both hooks.

13. Hook path resolution is explicit for linked worktrees. The wiring script path locates only stable
    executable wiring. Policy configuration, generated payloads, and temporary trust material resolve
    from the invoking worktree root reported by Git, so an absolute shared `core.hooksPath` cannot mix
    the primary checkout's policy files into a linked branch. Tests cover absolute and relative hook
    paths with deliberately different primary and linked-worktree configurations.

14. This repository opts into both identity and SSH-signature enforcement for
    `Josua Müller <hypnotox@pm.me>` and its existing SSH signing key, with the final published MIT
    boundary as `grandfatheredThrough`. The activation transaction removes repository-local and
    worktree-local identity overrides, verifies the effective identity in every worktree, wires both
    generated payloads, and keeps GitHub's required-signed-commit protection on `main` as the final
    remote boundary.

15. Each implementation batch updates its authored `.awf/` sources, schema and migration, config
    reference, CLI help and command documentation, hook guidance, AGENTS conventions, README,
    architecture and testing docs, changelog, example adopter, destination claims and backing tests,
    templates, rendered outputs, and `docs/decisions/INDEX.md` in the same commit that makes the
    corresponding behavior true.

16. `config/validation:commit-policy`, `tooling/commit-policy:exact-commit-enforcement`, the updated
    `rendering/singletons-and-payloads:hook-payloads-rendered`, and
    `rendering/singletons-and-payloads:commit-policy-hook-payloads` are all `Backing: test` claims.
    Their proof units are respectively `TestCommitPolicyValidation`,
    `TestExactCommitEnforcement`, the existing `TestHookPayloadsRendered`, and
    `TestCommitPolicyHookPayloads`; each matching test file carries the required invariant marker.

17. Every affected template remains publication-safe when the new variables and records are absent:
    `missingkey=zero` renders coherent generic output, no unresolved or no-value token is emitted,
    and coverage exercises empty-policy rendering for every generated hook and documentation surface.

## State changes

- add `config/validation:commit-policy`
- add `tooling/commit-policy:exact-commit-enforcement`
- update `rendering/singletons-and-payloads:hook-payloads-rendered`
- add `rendering/singletons-and-payloads:commit-policy-hook-payloads`

## Consequences

A normal commit with a fixture identity, a disallowed author or committer, a missing signature, or a
signature from an untrusted key fails before the branch moves. The user's staged transaction remains
available for reconciliation rather than becoming a bad commit that must be amended. Pre-push and
host protection cover progressively later bypass boundaries.

The same typed verifier owns preview and hook verdicts, keeping error messages and trust semantics
consistent. Projects can opt into identity checks without maintaining signing keys, or into both.
An opt-in project must maintain its allowlists deliberately when adding contributors, bots, or key
rotations.

The reference-transaction hook runs for more Git operations than ordinary commit hooks. Candidate
selection and linked-worktree resolution therefore require real Git integration tests across commit,
amend, merge, rebase, reset, branch creation, deletion, multi-ref transactions, and hook states. A
copied-tree spike must establish that refusal leaves refs unchanged and preserves recoverable staged
state before the capability is implemented.

A checkout owner can still alter hooks, configuration, or trust material. The local mechanism is not
presented as protection against that owner; remote repository protection remains the authoritative
publication control. Branch-creation enumeration may check more locally-unpublished commits than the
remote strictly needs, but it must never omit a commit merely because a stale cache was treated as
fresh authority.

The config schema, command table, generated hook set, documentation, and adopters' rendered outputs
all grow. Existing adopters remain unchanged because the block is optional and awf does not wire
hooks automatically.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rely only on `commit.gpgSign=true` and Git identity configuration | Both defaults can be overridden, and signing does not validate author or committer policy. |
| Check signing in `pre-commit` or `commit-msg` | The final commit signature does not exist yet. |
| Detect failures in `post-commit` and reset or amend automatically | Moves the branch first and creates the history-rewrite hazard the feature is meant to avoid. |
| Enforce only at pre-push | Protects publication but permits bad local history to accumulate until reconciliation is expensive. |
| Ship a `git commit` wrapper | Does not cover direct Git, IDEs, agents, merges, rebases, or other ref-writing operations. |
| Treat the committed allowlist as protection against a malicious checkout owner | That owner can modify or disable both policy and hook; only an independent remote boundary can provide that guarantee. |
| Require signatures without configured trusted keys | Proves only the presence of signature-shaped data, not authorization by a project-approved signer. |

## Status history

- 2026-08-02: Proposed
- 2026-08-03: Accepted; content-sha256: bbd924bd577ba8878c4c2be3319f4d6fad5dd1ae1e2901256d32c59884097aa0
- 2026-08-03: Implementing; content-sha256: bbd924bd577ba8878c4c2be3319f4d6fad5dd1ae1e2901256d32c59884097aa0
- 2026-08-03: Applied; operations: add `config/validation:commit-policy`
- 2026-08-03: Applied; operations: add `tooling/commit-policy:exact-commit-enforcement`
- 2026-08-03: Reapplied; operations: add `tooling/commit-policy:exact-commit-enforcement`
- 2026-08-03: Reapplied; operations: add `tooling/commit-policy:exact-commit-enforcement`
