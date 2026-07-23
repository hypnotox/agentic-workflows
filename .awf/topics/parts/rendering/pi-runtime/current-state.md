The Pi runtime floor and its boundaries: child-process safety, tool boundaries, target rendering, and real-runtime smoke coverage.

## Claims

### `invariant: pi-pinned-development-runtime`

Without an enabled bootstrap, the generated Pi dashboard tries `awf` on PATH and, after absence, execution failure, project-version refusal, or protocol refusal, uses only an advertised repository `dashboard-awf-path` fallback. Bootstrap remains authoritative and never falls through. The fallback receives the absolute project root through `AWF_DASHBOARD_PROJECT_ROOT`, dual failure retains both bounded causes, and one successfully handshaken launcher is captured for the Pi session so a later ref advance cannot hot-swap it.
Origin: ADR-0150
Backing: test

### `invariant: pi-child-process-safety`

In the generated Pi subagent extension, every child exit path removes the temporary role prompt and its listeners, cancellation escalates from TERM to KILL based on the observed process exit, and child errors preserve bounded diagnostics.
Origin: ADR-0148
Backing: test

### `invariant: pi-child-tool-boundaries`

Pi subagent children use an explicitly selected validated model, a validated configured preference, or inherit the parent; inherit the parent's thinking level; receive fixed role allowlists excluding extension tools; and enforce fixed retained-output limits with explicit truncation diagnostics.
Origin: ADR-0148
Revised-by: ADR-0151
Backing: test

### `invariant: pi-extension-target-render`

Enabling the Pi target renders exactly five governed extension files with valid TypeScript provenance comments and target-sensitive config hashes: two under awf-subagents, the separate awf-handoff entrypoint, and the awf-dashboard index plus its descriptor-derived protocol projection. The protocol output declares and hashes the Go-owned descriptor as an attributed input, only the dashboard index consumes widget configuration, a target set without Pi renders none, and every output participates in ordinary check, sync, and manifest-cleanup semantics.
Origin: ADR-0148
Backing: test

### `invariant: pi-implementation-state-boundary`

Pi implementation subagent calls serialize against one another, enforce the caller-selected commit permission - reporting a changed HEAD under a no-commit permission as a policy violation without auto-reverting - and report starting and ending git state, marking commit verification unavailable outside a git checkout.
Origin: ADR-0148
Backing: test

### `invariant: pi-minimum-runtime`

All three generated Pi extension factory entrypoints require Pi 0.81.1 or newer with the event, queued-command, persisted custom-entry, widget, overlay, and shutdown APIs their contracts use. They share one actionable compatibility notification on an older or core-API-incompatible runtime and fail before registering commands, tools, or other functional hooks; context-only APIs are checked before use and degrade visibly rather than being guessed.
Origin: ADR-0148
Backing: test

### `invariant: pi-real-runtime-smoke`

The containerized fixtures are the deterministic gate for every Pi extension, and release readiness additionally requires subagent, router, structured replacement, handoff, and dashboard smoke runs on the exact compatible fork for Pi 0.81.1 or a later build verified to expose the required queued-command, persisted-session, custom-entry, widget, overlay, and shutdown APIs. The smoke covers router-enforced phase transitions and recovery, explicit effort resume through session replacement, memory-identity association across a parent handoff, dashboard refresh, widget and overlay behavior, durable lifecycle writes, shutdown drain, and visible degraded operation when canonical binary resolution is unavailable.
Origin: ADR-0148
Revised-by: ADR-0149
Backing: unbacked
Verify: Before a release, follow the documented real-Pi smoke on `hypnotox/pi` `fork-v0.81.1-awf.3` for Pi 0.81.1, or on a later compatible build: load `brainstorming` through `awf_workflow`, load its routed successor and verify one transactional phase transition, cancel then retry `/awf-resume-effort <effort-id>` and verify association in the replacement, create a memory file with the same `Effort: <id>`, cross one parent handoff and verify association before kickoff, interrupt and retry one provisional or transition settlement to verify idempotent recovery, fork and resume a trajectory, open and refresh the dashboard, exercise its widget and overlay plus a canceled destructive action, durably complete the lifecycle, verify shutdown drain and retained history, then repeat with canonical binary resolution unavailable and confirm visible degraded non-blocking telemetry; record every command, observed state, and compatibility finding in the release work.
