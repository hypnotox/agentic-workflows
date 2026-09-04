# 2026-09-04 simplify mutation safety

## Context

AWF's mutation model accumulated persistent upgrade journals, recovery-only command states, rollback and quarantine machinery, automatic backup files, and detailed effect ledgers. Those mechanisms implied transaction guarantees that Git-backed multi-file operations cannot provide. They also made recovery depend on inferred intermediate state instead of the repository and paths an operator can inspect directly.

The durable safety boundary is simpler. AWF must not overwrite or remove content unless it has the minimum evidence needed to prove ownership. It must validate the complete collision and destructive set it can derive before mutation, serialize mutable-authority decisions, confine filesystem access, and make partial progress explicit when a later step fails.

## Decision

AWF protects content it cannot prove it owns. Mutating operations acquire one writer lease before reading mutable authority, strictly parse live configuration and the lock, refuse future schemas, and completely preflight every collision and destructive path they can derive before the first write. Creation is exclusive and no-clobber. Filesystem access remains root-confined, does not follow symlinks, and refuses a symlink at the final mutation path. Destructive worktree guards remain. Directory removal remains limited to empty directories. Unchanged outputs are skipped. The authority lock is written last.

Git-backed mutations are ordered, not cross-file transactional. An operation stops on its first failed mutation, leaves every earlier successful effect visible, and reports the affected paths. The operator inspects those paths and Git state, resolves any blocking condition, and reruns the ordinary command to converge. AWF does not create or consume persistent recovery journals, expose `upgrade --recover`, embargo other commands based on a journal, infer quarantine or recovery residue, create automatic `.awf-bak` files, expose `init --force`, maintain Publisher effect or recovery ledgers, or roll back a focused operation.

The temporary live migration bridge from schema 50 through schema 53 remains until the external managed-adopter rollout permits its removal. This decision does not advance the live schema floor.

This decision supersedes only the recovery, rollback, quarantine, residue-inference, automatic-backup, recovery-ledger, and `init --force` clauses of earlier decisions. Their root confinement, no-follow and final-symlink refusal, exclusive creation, collision and destructive preflight, writer leasing, strict parsing, future-schema refusal, ownership checks, lock-last ordering, unchanged-output skips, empty-only directory removal, and destructive worktree safeguards remain in force. Earlier records remain historical.

## Consequences

Failures after mutation can leave a valid partial result that is visible in Git and on disk. Diagnostics must name affected paths without claiming restoration or inferring hidden state. A retry is the convergence mechanism after inspection and correction.

Unknown or insufficiently owned content blocks mutation instead of being overwritten, deleted, relocated, or copied to an automatic backup. Implementation and tests may delete the retired recovery surfaces while preserving the temporary schema bridge and the retained safety checks. Commit policy and hooks are unaffected.
