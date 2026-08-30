Opt-in commit policy evaluates exact Git commit objects against project-configured author, committer, and SSH-signature requirements. The topic owns policy facts and typed outcomes; Git object and signature mechanics remain behind the Git boundary.

## Claims

### `invariant: exact-commit-enforcement`

The common exact-commit verifier evaluates every unique explicit target or range commit after the configured repository-width full-OID baseline against exact author and committer identity pairs and native Git-verified allowed SSH signatures, returning all stable violations or an actionable typed refusal without mutation. It selects configuration and temporary trust material from the Git-resolved invoking worktree, recursively peels tag targets, preserves operational causes by category, removes temporary trust material on every return path, and reports refs and index unchanged. An absent policy reports one successful disabled note; `awf check commit-policy` is the command adapter for this verifier.
Backing: test
