## Commit discipline

Use Conventional Commits, one concern per commit. Stage files explicitly rather than `git add -A`, so each commit is a deliberate, reviewable unit.

The allowed commit scopes are stored once, in `audit.allowedScopes` (ADR-0051), and enforced by `awf commit-gate`. awf uses a domain-aligned taxonomy:

{{=awf:commitScopeTable}}

The code scopes mirror the domain vocabulary in `.awf/config.yaml`; see [the domain docs](domains) for what each area covers. The correspondence is hand-maintained, not machine-enforced (ADR-0055): adding a domain does not add a scope. The gate only checks set membership; it cannot catch a wrong-but-valid pick (a docs scope where a code scope was meant), so pick the scope that names the area you actually changed.
