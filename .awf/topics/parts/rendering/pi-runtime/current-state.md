The Pi runtime floor and its boundaries: child-process safety, tool boundaries, target rendering, and real-runtime smoke coverage.

## Claims

### `invariant: pi-child-process-safety`

In the generated Pi subagent extension, every child exit path removes the temporary role prompt and its listeners, cancellation escalates from TERM to KILL based on the observed process exit, and child errors preserve bounded diagnostics.
Origin: ADR-0148
Backing: test

### `invariant: pi-child-tool-boundaries`

Pi subagent children use an explicitly selected validated model or inherit the parent, inherit the parent's thinking level, receive fixed role allowlists excluding extension tools, and enforce fixed retained-output limits with explicit truncation diagnostics.
Origin: ADR-0148
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

The containerized fixtures are the deterministic gate for every Pi extension, and release readiness additionally requires subagent, handoff, and dashboard smoke runs on the exact compatible fork for Pi 0.81.1 or a later build verified to expose the required queued-command, persisted-session, custom-entry, widget, overlay, and shutdown APIs. The smoke covers association across a parent handoff, dashboard refresh, widget and overlay behavior, durable lifecycle writes, shutdown drain, and visible degraded operation when canonical binary resolution is unavailable.
Origin: ADR-0148
Backing: unbacked
Verify: Before a release, follow the documented real-Pi smoke on `hypnotox/pi` `fork-v0.81.1-awf.3` for Pi 0.81.1, or on a later compatible build: create and route an effort, cross one parent handoff and verify active-branch association, fork and resume a trajectory, open and refresh the dashboard, exercise its widget and overlay plus a canceled destructive action, durably complete the lifecycle, verify shutdown drain and retained history, then repeat with canonical binary resolution unavailable and confirm visible degraded non-blocking telemetry; record any compatibility finding in the release work.
