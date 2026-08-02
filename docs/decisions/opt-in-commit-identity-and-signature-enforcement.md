---
format: current-state-v3
slug: opt-in-commit-identity-and-signature-enforcement
status: Proposed
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

1. awf adds an optional top-level `commitPolicy` configuration block. `allowedIdentities` is a
   nonempty list of exact `{name, email}` pairs when present. `requireSignedCommits` defaults to
   `false`; when true, `allowedSigners` is a nonempty list of exact `{principal, key}` SSH signer
   records. `allowedSigners` while signing is disabled and an enabled signing requirement without
   signers are invalid. The two enforcement families are otherwise independent, and an absent block
   preserves current behavior.

2. Identity enforcement evaluates the exact author and committer fields of each selected commit.
   Both must match one configured pair byte-for-byte. It does not infer identity from a signature,
   rewrite metadata, treat author and committer differently, or accept a name-only or email-only
   match.

3. Signature enforcement requires exactly the ordinary Git SSH signature semantics: the commit must
   carry a signature that `git verify-commit` validates against a temporary allowed-signers file
   derived from the configured principals and public keys. A signature header without successful
   cryptographic verification is not signed for policy purposes. GPG and X.509 signer configuration
   are outside the first capability rather than being accepted without an explicit trust model.

4. One public `awf check commit-policy <revision-or-range>...` command resolves explicit commits
   through the shared Git boundary, deduplicates them, and reports every violating commit. This is
   the common verifier for human preview, generated hooks, tests, and diagnostics; hook scripts do
   not reimplement identity parsing or signature verification. With no configured policy it succeeds
   with one explicit disabled-policy note.

5. The hooks singleton renders a `reference-transaction.sh` payload in addition to its existing
   payloads. In the hook's `prepared` phase it reads all `<old-oid> <new-oid> <ref>` records, selects
   commit objects introduced onto local branch refs, deduplicates them, and invokes the common
   commit-policy check before Git moves any selected ref. Deletion-only updates and backward-only
   movement introduce no commit and remain outside the check. Any other hook state exits without
   re-evaluating policy. awf still does not activate or edit Git configuration; adopters wire the
   payload as `reference-transaction` when they opt in.

6. The generated pre-push payload reads the standard ref-update stream and verifies policy-applicable
   commits being introduced to the named remote before running the configured project gate. Updates
   use the remote-old to local-new range; branch creation excludes commits reachable from the local
   remote-tracking namespace and may conservatively inspect a documented superset when the cache is
   stale. Missing objects or an unresolvable range fail closed with reconciliation guidance. Deletion
   updates add no commit. The verifier does not claim that local remote-tracking refs are an
   authoritative copy of every advertised remote ref.

7. Before enabling either hook, adopters run the commit-policy command over the exact unpublished
   range they intend to publish. Enabling policy never rewrites, amends, resets, or re-signs an
   existing commit. A nonconforming pre-existing commit remains the adopter's explicit choice to
   publish before activation, recreate, retain only on an unprotected ref, or leave unpublished.

8. Every refusal names the violating commit and condition, prints the complete configured allowlist
   relevant to that condition, and gives a concrete next action. Identity failures say that the
   observed identity is not allowed and list the allowed identities. Missing or invalid signatures
   say that commits must be signed by an allowed signer and list the allowed principals. Configuration,
   object-resolution, linked-worktree, and remote-range failures distinguish their cause rather than
   collapsing into a generic hook failure.

9. Hook path resolution is explicit for linked worktrees. The executing payload derives its own
   payload and trust-material directory from its script path, while project checks run against the
   invoking worktree's root. Tests cover a primary checkout and a linked worktree so an absolute
   shared `core.hooksPath` cannot silently mix one branch's policy files with another branch's
   generated payload.

10. This repository opts into both identity and SSH-signature enforcement for
    `Josua Müller <hypnotox@pm.me>` and its existing SSH signing key, wires both generated payloads,
    and keeps GitHub's required-signed-commit protection on `main` as the final remote boundary.

## State changes

- add `config/validation:commit-policy`
- add `tooling/commit-policy:exact-commit-enforcement`
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
