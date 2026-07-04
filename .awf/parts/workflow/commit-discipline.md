## Commit discipline

Use Conventional Commits, one concern per commit. Stage files explicitly rather than `git add -A`, so each commit is a deliberate, reviewable unit.

The allowed commit scopes are stored once, in `audit.allowedScopes` (ADR-0051), and enforced by `awf commit-gate`. awf uses an eight-scope, domain-aligned taxonomy:

| scope | use it for |
|---|---|
| `adr` | ADR markdown documents (`docs(adr)`) |
| `plans` | plan markdown documents (`docs(plans)`) |
| `awf` | genuinely cross-cutting or repo-meta work (version bump, top-level README) — the umbrella of last resort, not a default |
| `adr-system` | the ADR machinery code (ACTIVE.md generation, lifecycle) |
| `config` | the `.awf` config tree, schema, migrations |
| `invariants` | invariant backing and checks |
| `rendering` | the render engine and templates |
| `tooling` | CLI, audit/gate, coverage, CI, `./x`, changelog, evals |

The five code scopes (`adr-system`, `config`, `invariants`, `rendering`, `tooling`) mirror the `domains:` entries in `.awf/config.yaml` by convention — see [the domain docs](domains) for what each area covers. The correspondence is hand-maintained, not machine-enforced (ADR-0055): adding a domain does not add a scope, so decide the scope separately and edit `audit.allowedScopes` too. The gate only checks set membership; it cannot catch a wrong-but-valid pick (`adr` where `adr-system` was meant), so pick the scope that names the area you actually changed.
