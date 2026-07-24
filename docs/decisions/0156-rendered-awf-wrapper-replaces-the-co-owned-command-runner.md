---
format: current-state-v2
status: Implementing
date: 2026-07-23
---
# ADR-0156: Rendered awf wrapper replaces the co-owned command runner

## Context

ADR-0101 made the `runner` singleton render a co-owned `x` at the adopter repo root: awf-owned
per-verb forwarding arms (one `case` arm per runner-forwarded clispec command, each delegating to
the bootstrap-resolved pinned binary) plus two `awf:edit-in-place` sections holding the adopter's
project verbs. Living with that shape surfaced three structural frictions:

1. **Co-ownership fights the adopter's own runner paradigm.** A project that drives its tasks
   through `make`, `just`, or npm scripts gets a second, awf-shaped runner it must adopt to hold
   its project verbs, and first adoption of a pre-existing hand-written runner is lossy by design
   (ADR-0100/0101). The awf-facing half and the project-facing half have no reason to live in one
   file.
2. **The verb list is a standing rot risk that needed its own machinery.** The per-verb arms must
   track the clispec command table, so `tooling/cli:managed-runner-command-parity` and the
   `RunnerDisposition` metadata exist solely to police a list that a pure forwarder would not have
   at all.
3. **Missing project commands degrade silently.** With `gateCmd` unset, the rendered pre-push
   payload falls back to a shim running bare `awf check` (`templates/hooks/pre-push.sh.tmpl:6-10`)
   and pre-commit simply omits the gate step: a hooks-enabled adopter whose config never set
   `gateCmd` gets hooks that run but do not gate, and nothing tells them. The user direction is
   explicit: "We should not assume a default runner, some projects might be missing this completely
   yet, and they don't know that they have to set it potentially, if they don't review the diff."

Verified couplings that shape the redesign:

- **Descriptor defaults are not render-effective.** `VarDescriptor.Default` feeds only init
  prompting and the configspec/reference docs; render data exposes only `config.yaml` vars
  (`internal/project/render.go:97`), and 17 templates carry `{{ with .vars.X }}...{{ else }}awf
  <verb>{{ end }}` fallback arms. Making a wrapper path the effective default needs a render-data
  mechanism, not a descriptor edit.
- **awf-the-repo has a path collision with the wrapper.** `.gitignore:1` ignores `/awf` because
  `./x build` compiles the binary to exactly that path; self-adoption requires relocating the
  build output and un-ignoring the path, and the foreign-file backup (`awf-bak`) handles any stale
  binary on the first sync.
- **The lock prune deletes a renamed output's old path unconditionally**
  (`internal/project/project.go:271-292`), so a same-path hand-written replacement must be created
  only after the pruning sync.
- The user wants awf-the-repo to adopt the wrapper for real: "I want to excercise what a normal
  adopter would use. I know this splits it into two runners, but that's exactly what an adopter
  would have. ./x will have repo-audit and all that specific to our repo-local things, and the
  ./awf wrapper is solely awf."

## Decision

1. **The `runner` singleton renders a pure awf wrapper.** With the singleton enabled, `awf sync`
   renders exactly one executable file `awf` at the repo root (executable via the existing shebang
   rule). The file is fully awf-owned: no `awf:edit-in-place` sections, no per-verb dispatch. Its
   body resolves an awf invocation and `exec`s it with all arguments forwarded verbatim, so every
   current and future CLI verb is available through `./awf <verb>` with no rendered verb list to
   maintain.
2. **Resolution is pinned-then-PATH, overridable by one var.** A new command var `awfInvokeCmd`
   holds the invocation command; when set, the wrapper uses it verbatim (awf-the-repo sets
   `go run ./cmd/awf`). When unset, the rendered default resolves the bootstrap-pinned binary when
   `.awf/bootstrap.sh` exists and falls back to PATH `awf` otherwise, centralising in one file the
   resolution logic today duplicated across the three hook-payload shims.
3. **The wrapper is default-on.** `awf init` scaffolding seeds `runner.enabled: true` (alongside
   the existing bootstrap/hooks seeding), and a schema-generation migration seeds an absent
   `runner` key to enabled on `awf upgrade`, respecting an explicit `enabled: false` (precedent:
   `applyEnableBootstrap`).
4. **Templates learn the effective awf command through render data, not descriptor defaults.**
   Render data exposes the runner-enabled state; every awf-verb fallback arm (hooks, skills, docs,
   plans template) renders `./awf <verb>` when the runner is enabled and the generic `awf <verb>`
   otherwise, keeping publication-safe degradation under empty data. The hook payloads drop their
   inline resolution shims entirely.
5. **Project-verb vars have no default, and unresolvable hook commands are errors.** `gateCmd`,
   `gateCmdFull`, and `testCmd` keep empty descriptor defaults with no rendered fallback that
   pretends a runner exists. Config validation fails `awf sync` and `awf check` (and therefore the
   terminal sync of `awf upgrade`, deliberately a hard error with an actionable message naming the
   exact var) when: (a) the hooks singleton is enabled and `gateCmd` is unset, or (b) the hooks
   singleton is enabled, the runner singleton is disabled, and a hook-referenced awf-verb var
   (exactly `checkCmd`, `commitGateCmd`, or `proseGateCmd`) is unset. Silent gate-skipping
   degradation is removed.
6. **The per-verb forwarding machinery is removed, not orphaned.** The clispec
   `RunnerDisposition`/`Forwarded` surface, the runner in-place section machinery, the
   runner-usage render data, and the runner command-parity checks lose their purpose and are
   deleted in the same effort; the dead-code gate enforces completeness of the removal. The
   `tooling/cli:metrics-command-contract` and `tooling/cli:doctor-command-contract` claims shed
   their now-meaningless "runner-forwarded" qualifier as part of the same change.
7. **awf-the-repo adopts the wrapper.** Its config enables the runner with
   `awfInvokeCmd: go run ./cmd/awf`, so `./awf` is the rendered from-source forwarder. The
   hand-maintained `./x` sheds every awf-verb forwarding arm (`invariants`, `audit`, `metrics`,
   `doctor`, `commit-gate`, `prose-gate`, `new`, `list`, `config`, `topic`, `context`,
   `changelog`, `version`) and keeps only repo-local verbs: `gate`, `lint`, `fmt`, `test`,
   `deadcode`, `pi-test`, `dashboard-awf-path`, `dashboard-awf-advance`, `mutants`, `audit-local`,
   `install`, `build` (output relocated off the root `awf` path, `.gitignore` updated
   accordingly), and the `sync`/`check` composites that wrap the example-adopter oracle
   (ADR-0090) and now invoke `./awf` internally. The command vars follow the split:
   `commitGateCmd`/`proseGateCmd` move to `./awf` forms, while `checkCmd`/`activeMdRegenCmd` stay
   on the `./x` composites so the sundial oracle remains in the gate path.
8. **The example adopter models the new split.** `examples/sundial` keeps the runner singleton
   enabled (now rendering `awf`), drops its awf-verb vars entirely so it dogfoods the
   runner-aware rendered defaults a fresh adopter gets (`./awf` forms supplied by the
   templates, not by explicit var values), and gains a small hand-written `./x` carrying its
   `gate`/`test` bodies with `gateCmd: ./x gate` and `testCmd: ./x test`; the hand-written
   file is added only after the sync that prunes the old rendered `x`.
9. **The outgoing co-owned runner is backed up, not silently deleted.** When a sync's lock prune
   removes the previously rendered co-owned runner path `x` (the only output that carried
   in-place sections), the file is backed up in place of deletion through the standard backup
   path (`x.awf-bak`, collision-suffixed like every awf backup), so an adopter's hand-authored
   in-place verb bodies stay on disk for the one-time hand-port instead of vanishing into git
   history.
10. **The four added claims are invariant claims with `Backing: test`.**
    `runner-pure-forwarder`, `runner-resolution-pinned-first`, `runner-prune-backup`, and
    `hooks-commands-resolvable` are each directly testable (wrapper render shape, resolution
    behaviour, prune backup, validation errors) and are proven by markers on the corresponding
    render/validation tests when their prose is authored at application time.
11. **Documentation travels with the change.** The same effort refreshes every authored and
    rendered surface that describes the co-owned runner contract: the tooling domain narrative
    part, the working-with-awf and agents-md-standard template prose on the managed runner, the
    AGENTS.md runner-toggle sentence, the configspec `runner.enabled` description, and the
    generated config reference for the new `awfInvokeCmd` descriptor.
12. **This redefines the runner contract ADR-0101 established.** ADR-0101 remains frozen history;
    the change lands entirely through the State changes below, and incremental (batched) V2
    application is expected given the breadth. The declaration order is the batched application
    order: the prune-backup mechanism lands first (it must exist before any tree's sync prunes a
    co-owned runner), the core reshape and both adoptions follow, this repository's runner
    slimming next, and the hook-payload/validation rework closes the sequence.

## State changes

- add `rendering/companion-scripts:runner-prune-backup`
- add `rendering/companion-scripts:runner-pure-forwarder`
- add `rendering/companion-scripts:runner-resolution-pinned-first`
- update `rendering/companion-scripts:runner-singleton-toggle`
- update `rendering/companion-scripts:runner-example-adopted`
- update `rendering/catalog-and-targets:var-descriptor-set-pinned`
- update `tooling/cli:cli-command-spec-single-source`
- update `tooling/cli:metrics-command-contract`
- update `tooling/cli:doctor-command-contract`
- remove `rendering/companion-scripts:runner-awf-verbs-owned`
- remove `rendering/companion-scripts:runner-project-verbs-in-place`
- remove `tooling/cli:managed-runner-command-parity`
- update `rendering/companion-scripts:dashboard-development-runtime-commands`
- add `config/validation:hooks-commands-resolvable`
- update `rendering/companion-scripts:hook-payloads-fallback-safe`

## Consequences

Easier:

- Verb rot is structurally impossible: a forwarder with no verb list cannot fall behind the CLI,
  and the parity machinery that policed it disappears with it.
- The concern split is clean: `./awf` is solely awf's, fully rendered and drift-checked; the
  project's runner paradigm (make, just, a hand-written `./x`) is untouched and unopinionated.
- A missing gate command surfaces as one loud, actionable sync/check error instead of a silently
  toothless hook.
- Binary resolution logic lives in one rendered file instead of three hook shims.
- awf-the-repo finally exercises the adopter-facing runner primitive itself (from-source via
  `awfInvokeCmd`), closing ADR-0101's accepted dogfooding gap.

Harder / accepted trade-offs:

- **Existing hooks-enabled adopters with `gateCmd` unset hard-fail at upgrade.** The migration can
  seed the runner but cannot invent a project gate command; the terminal sync error names the var
  to set. Accepted deliberately over a warn-first release: the silent-gate hazard is worse than a
  one-time loud stop.
- **A root file named `awf` can collide with adopter trees.** An existing file at that path is
  backed up as a foreign file on first sync, and an adopter `.gitignore` matching `awf` keeps the
  rendered wrapper out of their commits while disk-based `awf check` stays green; `awf check
  --staged` surfaces the split. Noted as a known edge, not separately validated.
- The removal slice (clispec dispositions, in-place runner machinery, parity tests) plus the
  fallback-arm rework across 17 templates is a wide mechanical surface; the dead-code and coverage
  gates bound it.
- Adopters who used the co-owned runner's in-place sections (the example adopter is the known
  case) must hand-port their project verbs to their own runner file once; the pruned file stays
  on disk as `x.awf-bak` (Decision item 9), with git history as the durable fallback.
- **Runner-absent adopters gain an unsolicited root file on upgrade.** Seeding an absent `runner`
  key to enabled means an adopter who never opted into a runner receives a new tracked `awf` file
  at the root; accepted because the wrapper is the new baseline contract the rendered hooks and
  skills point at, and an explicit `enabled: false` is respected.
- ADR-0155 (Proposed, in flight) edits some of the same skill templates; textual coordination is a
  sequencing concern for the plans, not a semantic conflict.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the co-owned `x` (ADR-0101 status quo) | The co-ownership, verb-list parity machinery, and silent gate degradation are the frictions this decision removes. |
| Two singletons (co-owned `x` and a wrapper) | Two overlapping runner concepts to document and maintain; the user rejected coexistence. |
| Rename the co-owned runner to `awf` without reshaping | Keeps every friction and adds a name that misleadingly suggests pure awf ownership. |
| Strict bootstrap-only resolution in the wrapper | Breaks runner-on/bootstrap-off adopters at runtime or demands another validation rule; pinned-then-PATH matches the proven hook-shim behaviour. |
| A non-colliding wrapper path or name (e.g. `bin/awf`, `awfw`) | The user explicitly directed the root `./awf` form; the tool's own name at the root maximises discoverability, and the collision costs are bounded (foreign-file backup, one relocated build output in this repo). |
| Warn-first rollout of the gate-command validation | Prolongs the silent-gate hazard a full release cycle and needs a second behavioural change; config smell is treated as an error state in this project. |
| Two ADRs (wrapper reshape vs validation tightening) | The validation tightening is motivated entirely by the reshape; splitting fragments one coherent redefinition of the runner contract. |

## Status history

- 2026-07-23: Proposed
- 2026-07-24: Implementing; content-sha256: 0c43c72549cffd1c08956711be7ea28978501f690b2aa78836fc651901c4f723
- 2026-07-24: Applied; state-sequence: 40; operations: add `rendering/companion-scripts:runner-prune-backup`
- 2026-07-24: Applied; state-sequence: 41; operations: add `rendering/companion-scripts:runner-pure-forwarder`, add `rendering/companion-scripts:runner-resolution-pinned-first`, update `rendering/companion-scripts:runner-singleton-toggle`, update `rendering/companion-scripts:runner-example-adopted`, update `rendering/catalog-and-targets:var-descriptor-set-pinned`, update `tooling/cli:cli-command-spec-single-source`, update `tooling/cli:metrics-command-contract`, update `tooling/cli:doctor-command-contract`, remove `rendering/companion-scripts:runner-awf-verbs-owned`, remove `rendering/companion-scripts:runner-project-verbs-in-place`, remove `tooling/cli:managed-runner-command-parity`
- 2026-07-24: Applied; state-sequence: 42; operations: update `rendering/companion-scripts:dashboard-development-runtime-commands`
