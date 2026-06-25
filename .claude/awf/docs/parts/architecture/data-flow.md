## Data flow

A `sync` loads the config tree, resolves each enabled target's sections (sidecar overrides and
convention parts layered over template defaults, precedence
`drop > explicit replaceWith > convention part > template default`), executes `text/template`
under `missingkey=zero`, rejects output carrying an unresolved-variable placeholder, writes the rendered files, and stamps
each one's per-target `ConfigHash` into `.claude/awf/awf.lock`. A `check` re-renders in memory and
compares against the lock — reporting drift, orphaned sidecars/parts, and stale `ACTIVE.md` — while
a stale schema generation hard-fails with a "run `awf upgrade`" gate; `awf upgrade` runs the
registered migrations up to current and re-syncs.
