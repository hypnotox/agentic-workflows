## Current state

Per-project configuration lives in a `.claude/awf/` tree: a skeleton `config.yaml` (prefix, vars, flat enable arrays for skills/agents/docs/domains/hooks, invariants) plus per-target sidecars and convention parts. The config is strict-parsed (`KnownFields`), and drift is tracked in a schema-versioned `awf.lock`. Schema migrations are an ordered registry applied by `awf upgrade`; `awf sync`/`check` gate a stale layout. Additive optional fields (like `domains`) are backward-safe and need no version bump.
