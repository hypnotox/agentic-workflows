---
status: Implemented
date: 2026-07-02
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [hooks, rendering, adoption, tooling]
related: [23, 32, 35, 36, 39, 40, 45, 47]
domains: [rendering, config, tooling]
---
# ADR-0048: Rendered git-hook payloads singleton

## Context

[ADR-0032](0032-remove-automatic-hook-handling.md) removed automatic git-hook handling
entirely: both the rendered `.githooks/` files (the `hook` kind) and the activation side
effect (`awf setup` mutating `core.hooksPath`). The friction it named was the activation —
silently hijacking an adopter's git setup — but the removal also took the rendered hook
*content* with it. The adopter-floor analysis (2026-07-01) flagged the result: a fresh
`awf init` ships an agent guide declaring "green gate before every commit" with zero
enforcement wiring; adopters hand-author hooks from README prose.

The ecosystem has since grown the pieces that change the calculus:

- `awf commit-gate` ([ADR-0036](0036-deterministic-commit-message-gate.md)) gives hooks a
  deterministic commit-message gate to call.
- The self-pinning bootstrap ([ADR-0040](0040-self-pinning-rendered-bootstrap.md),
  relocated to `.awf/bootstrap.sh` by
  [ADR-0047](0047-bootstrap-relocation-into-the-config-tree.md)) lets a hook run the exact
  pinned awf version via `"$(bash .awf/bootstrap.sh)" check`.
- ADR-0047 Decision item 2 established `.awf/` as the home for rendered awf-owned tooling,
  and the bootstrap singleton (`bootstrap: enabled:` block, rendered `0o644`, no sections,
  no sidecar, invoked explicitly with `bash`) is a proven shape to mirror.
- The graceful-fallback contract ([ADR-0045](0045-out-of-box-render-completeness.md))
  makes var-parameterised templates publication-safe when vars are unset.

Grounding discoveries that shape the design (all mechanically verified):

- Git ignores non-executable hooks, and every rendered file is written `0o644` with no
  chmod (`internal/project/project.go:147`; ADR-0047 Decision item 3 forbids a mode
  special-case). A rendered file therefore can never *be* a live hook — it can only be a
  payload that the adopter's own hook wiring invokes.
- The bootstrap singleton renders via `renderTarget("bootstrap", "", bootstrapTID, nil,
  config.Sidecar{}, …)` (`internal/project/render.go:259-269`) — no sections, no sidecar —
  and has a bespoke CLI path outside `kindDescriptors` (`cmd/awf/list_add.go:26-51`)
  writing `bootstrap.enabled` via `config.SetMappingScalar`.
- Reusing the `hooks:` config key is parse-safe for gated commands: the schema-4
  `drop-hooks` migration strips the old array shape, migrations edit raw YAML (never the
  strict parser), and `gate(root)` precedes `project.Open` in sync/check/list/new/
  invariants/audit. Ungated commands (`awf add`/`remove`, `awf commit-gate`, `awf init`)
  do reach the strict parser with an un-migrated config, but fail loudly there today
  ("field hooks not found") and would fail loudly after this change (a YAML type error) —
  never a silent misparse.
- The orphan scanner iterates only `kindDescriptors` and walks `.awf/<kind>/` +
  `.awf/<kind>/parts/` (`internal/project/check.go:121-186`); the singleton is not a
  descriptor, so rendered `.awf/hooks/*.sh` can never be misflagged as orphan
  sidecars/parts.
- Sync's prune loop, `awf uninstall` (lock-driven), init's collision preflight, and the
  foreign-file backup (ADR-0023/0035, via `PlannedOutputs`) all cover new render outputs
  with no code — disabling the singleton removes the payloads and the emptied
  `.awf/hooks/` directory.
- `isManagedMarkdown(tid)` (`internal/project/check.go:200-202`) gates both the
  dead-reference and skill-reference scans; the three payload template ids join
  `bridgeTID`/`bootstrapTID` there.
- Two enumerating surfaces must be *extended*, not just satisfied:
  `TestVarDescriptorParity` derives the referenced-var set only from skills/agents/docs
  template paths (`internal/project/descriptor_parity_test.go:31-49`), and
  `ScaffoldConfig`'s var collection walks the same pools
  (`internal/project/scaffold.go:32-47`) — without adding the hook templates to both, a
  new `commitGateCmd` descriptor fails parity and a fresh init would prompt for the var
  then silently drop the answer.
- awf's own hand-maintained hooks are exactly `./x check` + `./x gate` (pre-commit),
  `./x commit-gate "$1"` (commit-msg), `./x gate full` (pre-push) — reproducible from its
  existing `checkCmd`/`gateCmd`/`gateCmdFull` vars plus one new var. The pre-0032 hook
  template used exactly the `{{ if .vars.checkCmd }}…{{ else }}awf check{{ end }}`
  fallback idiom this design specifies; a commit-msg payload is genuinely new.
- ADR-0032's Invariants include the textual contract "the catalog and config schema
  declare no `hook` kind or `hooks` key" — reintroducing rendered hook content therefore
  requires explicit partial supersedence. Its `inv: hooks-config-dropped` slug stays
  valid: `Upgrade` skips migrations with `To <= from`, so a modern `hooks:` mapping never
  passes through `drop-hooks`. The retired `setup-guards-hookspath` stays retired —
  activation is not coming back.

**Decision in brief:** render inert git-hook payload scripts under `.awf/hooks/` as a
default-on singleton mirroring the bootstrap. awf supplies drift-checked hook *content*;
adopters own all hook *wiring* — awf never touches `.git/` or git config.

## Decision

1. **New `hooks` singleton.** A `hooks:` mapping block in `.awf/config.yaml` with
   `enabled: bool`; nil/absent and `enabled: false` both mean "do not render" (the
   `BootstrapConfig` semantics). Toggled via `awf add hooks` / `awf remove hooks` — a
   bespoke CLI path mirroring the bootstrap's, plus an `awf list hooks` row. The kind
   token is plural `hooks` (a deliberate break from the singular kind convention: the
   toggle governs a three-file set as a unit). No schema migration ships: absent key means
   disabled, and existing adopters opt in explicitly.

2. **Three payloads render as a unit** at `.awf/hooks/pre-commit.sh`,
   `.awf/hooks/commit-msg.sh`, and `.awf/hooks/pre-push.sh` — tracked, drift-checked,
   written `0o644` like every rendered file. No `awf:section` markers, no sidecar, no
   parts (bootstrap precedent; the marker syntax is HTML comments, which have no place in
   shell scripts). Adopters wanting different content disable the singleton and own their
   hooks. The three template ids are excluded from the managed-markdown scans alongside
   `bootstrapTID`, and `artifactLabel` distinguishes the payloads (not three lines labeled
   `hooks`) in unset-var advisories.

3. **Payload contents** follow the ADR-0045 graceful-fallback contract. Each script is
   `#!/usr/bin/env bash` + `set -euo pipefail`, then:
   - **pre-commit:** `checkCmd` (fallback `awf check`), then `gateCmd` when set.
   - **commit-msg:** a **new `commitGateCmd` catalog var** (fallback `awf commit-gate`),
     with the positional appended outside the substitution:
     `{{ with .vars.commitGateCmd }}{{ . }}{{ else }}awf commit-gate{{ end }} "$1"`.
   - **pre-push:** `gateCmdFull`, else `gateCmd`, else the `checkCmd`/`awf check`
     fallback.
   - **Pin-aware fallback resolution:** the fallback branches resolve `awf` through the
     pinned bootstrap when it is present and working — a runtime shell-function shim
     guarded by `[ -f .awf/bootstrap.sh ]` that falls back to PATH `awf` when the
     bootstrap fetch fails (offline clones and unsupported platforms must not block
     commits). No render-context change is needed.

4. **awf never activates hooks.** ADR-0032's activation removal is reaffirmed in full: no
   code path writes to `.git/`, runs `git config`, or ships an installer. The payloads are
   inert until the adopter invokes them from wiring they own — a hand-written stub, a
   `core.hooksPath` directory, husky/lefthook, or CI. This ADR **partially supersedes
   ADR-0032** (recorded via `related`; ADR-0032 stays `Implemented`): its Decision item 1
   (no rendered hook files) and the `hooks`-key clause of its textual invariant "the
   catalog and config schema declare no `hook` kind or `hooks` key" are superseded by
   items 1-2 above — the rest of that invariant stays true: the catalog still declares no
   `hook` kind, and `.githooks/` still receives no rendered files. Its Decision
   items 2 (no activation) and 6 (`setup-guards-hookspath` retired) remain in force, and
   `inv: hooks-config-dropped` remains valid for the legacy array shape.

5. **`awf init` scaffolds the singleton enabled** (like the bootstrap), fixing the
   adopter floor out of the box. `ScaffoldConfig`'s referenced-var collection and the
   descriptor-parity test are both extended to enumerate the hook templates, so
   `commitGateCmd` is seeded and prompted like any other var.

6. **Dogfood.** awf enables `hooks:`, sets `commitGateCmd: ./x commit-gate`, and its
   rendered payloads reproduce today's hand-maintained hook bodies exactly. The three
   `.githooks/*` files shrink to hand-maintained executable stubs
   (`exec bash .awf/hooks/<name>.sh "$@"`) — the worked example of an adopter wiring
   their own hooks onto the rendered payloads.

7. **Docs travel with the change.** The implementing commits update every surface
   enumerated under Consequences, reference payload paths in code-spans only (never
   markdown links — the ADR-0047 dead-reference lesson), and the commit that flips this
   ADR's status regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Invariants

- `inv: hook-payloads-rendered` — with the singleton enabled, exactly three payloads
  render at `.awf/hooks/pre-commit.sh`, `.awf/hooks/commit-msg.sh`, and
  `.awf/hooks/pre-push.sh`; with it absent or disabled, no path under `.awf/hooks/`
  renders.
- `inv: hook-payloads-fallback-safe` — with `checkCmd`, `gateCmd`, `gateCmdFull`, and
  `commitGateCmd` all unset, every payload renders as a runnable script whose commands
  degrade to the generic `awf` forms, with no unresolved-value token.
- `inv: init-hooks-default-on` — `awf init`'s scaffolded config enables the `hooks`
  singleton.
- awf never activates hooks: no code path writes under `.git/` or mutates git config
  (textual contract, reaffirming ADR-0032).
- Rendered prose references the payload paths in code-spans only, never as markdown links
  (textual contract; the dead-reference scan enforces it wherever the singleton is
  disabled).

## Consequences

Easier:
- A fresh `awf init` ships working, drift-maintained gate wiring content: the adopter's
  remaining step is one stub or one `core.hooksPath` directory of their own, documented in
  the rewritten guidance. The "green gate before every commit" invariant stops being
  prose-only.
- Hook content stays current automatically: a `checkCmd` change or an awf upgrade
  re-renders the payloads, and the per-artifact config hash flags them stale like any
  rendered file.
- Composability: husky/lefthook users call `bash .awf/hooks/pre-commit.sh` from their
  existing manager; nothing competes for `core.hooksPath`.

Harder / accepted trade-offs:
- Activation stays manual, per clone, forever — that is the point. The payloads are dead
  weight for an adopter who never wires them; they are also three small inert files.
- **No migration for existing adopters** (asymmetric with the bootstrap's `To:5`
  enable-bootstrap migration, deliberately): hook payloads are more invasive than a cache
  script, so upgraded projects stay hooks-off until they run `awf add hooks`. Fresh inits
  are hooks-on.
- A fresh init with unset vars emits ADR-0045 unset-var advisories for the payloads on
  every `awf check` until the adopter sets `checkCmd`/`gateCmd` — intended nudge, but new
  default-on noise.
- The payloads assume githooks(5) semantics: git runs pre-commit/commit-msg/pre-push from
  the top of the working tree in a non-bare repo, so relative invocations
  (`bash .awf/bootstrap.sh`, `./x …`) resolve. Exotic invocation (calling a payload by
  hand from a subdirectory) is on the adopter.
- The pin-aware shim shadows PATH `awf` only when the bootstrap resolves successfully;
  on Windows Git Bash or offline it falls back to PATH `awf`, so a broken bootstrap
  degrades pinning rather than blocking commits.
- Old binaries do not know the `hooks:` key. Gated commands are protected by the
  ADR-0039 version gate once a newer binary has stamped the lock; ungated commands
  (`awf add`, `awf commit-gate`) on an old binary fail with a loud strict-parse error
  ("field hooks not found") — acceptable for a pre-1.0 tool.
- The render error path for the hooks block cannot carry the bootstrap's
  `coverage-ignore` justification verbatim (these templates *do* reference vars); the
  justification rests on every reference being `with`/`else`-wrapped, making an
  unresolved-value render unreachable. New CLI branches need the same coverage the
  bootstrap CLI tests provide.

Doc-currency obligations the implementing commits must satisfy:
- `templates/docs/workflow.md.tmpl` `local-hooks` section rewritten: awf renders inert
  payload scripts under `.awf/hooks/`; wiring is yours; examples for a stub, a
  `core.hooksPath` directory, and a hook manager.
- `README.md` hook passage and the adopter-guide awf-setup section updated to the same
  story.
- Kind enumerations gain `hooks`: the agent-guide template's toggle line, `awf --help`
  and the `unknown kind` error in `cmd/awf`, and the `awf list` row.
- `templates/catalog.yaml` gains the `commitGateCmd` var descriptor;
  `templates/embed.go` re-adds the hooks template directory.
- The `rendering`, `config`, and `tooling` domain narratives reflect the singleton.
- awf's own `.awf/config.yaml`, rendered tree, and `.githooks/` stubs land together with
  the dogfood flip, and its `.awf/agents-doc.yaml` invariants entries gain the new
  hook-payload invariants (the ADR-0047 precedent: the agent guide's Invariants list
  travels with the dogfooded change).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Payloads plus a rendered `install.sh` writing stubs into `.git/hooks/` | The user rejected any awf-supplied installation step — adopters own their git-hook setup entirely; awf supplies content only. |
| An `awf hooks install` subcommand | Resurrects the imperative-setup code and clash-guards ADR-0032 deleted; hook content would live in the binary, untracked and not drift-checked. |
| Executable rendered `.githooks/` + documented `core.hooksPath` one-liner | Requires the `0o755` mode special-case ADR-0032 removed and ADR-0047 explicitly declined; re-pollutes the repo root. |
| An enable-hooks schema migration (bootstrap precedent) | Hook payloads are more invasive than a cache script; existing adopters opt in explicitly. The asymmetry is recorded above. |
| Per-hook enable array (resurrect the `hook` kind) | Full kind machinery (catalog entries, per-name add/remove, orphan semantics) for payloads that are inert anyway; rendering all three is harmless. |
| Kind token `hook` (singular convention) | Misleading — the toggle governs the set, not one hook; the deliberate plural is recorded in Decision item 1. |
| Documentation-only (status quo) | Leaves the adopter floor gap: every adopter hand-authors the same three scripts from prose, with no drift maintenance when commands change. |
