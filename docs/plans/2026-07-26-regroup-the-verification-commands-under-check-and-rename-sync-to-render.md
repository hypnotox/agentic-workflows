---
date: 2026-07-26
adrs: [159]
status: Proposed
---
# Plan: Regroup the verification commands under check and rename sync to render

## Goal

Ship ADR-0159: rename `awf sync` to `awf render`, fold `awf invariants`, `awf prose-gate`, `awf memory-gate`, and `awf commit-gate` into an `awf check` group alongside two new `drift` and `state` children, and give the driver the per-child gating and per-child project-state exemption that regrouping requires.

Non-goals: bare `awf check` does not change what it runs or what it returns, no ran/skipped report is added, no `*Cmd` var key is renamed, and no file under `docs/decisions/` or `docs/plans/` is rewritten.

## Architecture summary

Three mechanisms carry the change; the rest is mechanical rename.

**Gating gains an inherit state.** `clispec.Gating`'s zero value is `Ungated` today, so a child that declares nothing is indistinguishable from a child that declares `Ungated`. Phase 3 renumbers the enum so the zero value is `Inherit`, making "says nothing" and "says Ungated" distinct. The driver then resolves gating from the child, falling back to the parent. That is what lets `check prose`, `check memory`, and `check commit` stay ungated under a gated `check` while `metrics` and `new` children keep inheriting.

**Project-state exemption becomes a command property.** `guardProjectState` switches on a hardcoded list of top-level names. Phase 3 replaces that list with a `StateExempt bool` on `clispec.Command`, read from the resolved node, so the three regrouped gates keep the exemption they hold today and a commit-msg hook keeps working during a committed journal or an attested lock.

**`check` becomes a group with a default leaf.** `metrics` is the existing precedent: a group whose bare form does work and whose children add more. `check`'s handler dispatches on `c.sub`, with the empty sub running today's `runCheck` unchanged.

`--staged` stays a flag on the bare form only. The three predicates that key on `top.Name == "check"` are re-scoped to bare check, and the handler rejects the flag on any child.

Phases are ordered so each closing commit passes `./x gate` alone, and so every ADR-0159 operation lands in the phase whose reality it describes. Phase boundaries are not release boundaries: only the completed plan is releasable, because the schema migration that moves adopters lands in Phase 4.

## File structure

- **Created:**
  - `internal/migrate/renameretiredcommands.go` - the schema-19 migration rewriting retired subcommand tokens in config var values
  - `internal/migrate/renameretiredcommands_test.go` - its table test
- **Modified:**
  - `internal/clispec/clispec.go` - the command table: `sync` to `render`, the `check` group and its six children, the `Inherit` gating zero value, the `StateExempt` field, `GatedCommandNames`
  - `internal/clispec/clispec_test.go` - replace `TestGroupChildrenCarryNoGating`, update `TestGatedCommandNames` and `TestLookup`
  - `cmd/awf/main.go` - `globalHelp` child recursion, resolved-gating dispatch, `guardProjectState` per-child exemption, bare-check staged predicates
  - `cmd/awf/dispatch.go` - handler registry keys, the `check` group handler
  - `cmd/awf/check.go` - the drift and state entry points, the version-ahead note text
  - `cmd/awf/invariants.go`, `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`, `cmd/awf/commitgate.go` - user-facing message prefixes
  - `cmd/awf/help_test.go`, `cmd/awf/check_test.go`, `cmd/awf/gate_test.go`, `cmd/awf/run_test.go`, `cmd/awf/invariants_test.go`, `cmd/awf/prosegate_test.go`, `cmd/awf/memorygate_test.go`, `cmd/awf/commitgate_test.go`, `cmd/awf/failure_paths_test.go`, `cmd/awf/dashboardread_test.go` - invocations and expected output
  - `internal/project/banner.go` - `bannerText`
  - `internal/project/gatedcommands.go` - the per-child projection
  - `internal/catalog/standard.go` - five var descriptors' descriptions and `Options`
  - `internal/configspec/spec.go` - five key entries' `Description` / `Availability`
  - `internal/migrate/migrate.go` - register migration 19
  - `templates/hooks/pre-commit.sh.tmpl`, `templates/hooks/commit-msg.sh.tmpl` - the three unset-var fallbacks
  - `x` - the `sync` verb and five retired-command call sites
  - `.awf/config.yaml` - `activeMdRegenCmd`, `proseGateCmd`, `memoryGateCmd`, `commitGateCmd`
  - the authored `.awf/` inputs naming a retired command (Tasks 1.6 and 4.3 give the exact sets)
  - `.awf/topics/parts/**/current-state.md` - the eighteen ADR-0159 claim operations
  - `docs/decisions/0159-regroup-the-verification-commands-under-check-and-rename-sync-to-render.md` - status history
  - this plan - the status flip
- **Deleted:** none.

## Phase 1: Rename `awf sync` to `awf render`

Self-contained: the rename completes within this phase, so `./x gate` passes at its close. No `check` work happens here.

- [ ] **Task 1.1: Rename the command in the spec table.** In `internal/clispec/clispec.go`, replace the `sync` entry with:

  ```go
  	{
  		Name: "render", Summary: "Re-render after a template or config change",
  		MaxPos: 0, Gating: Gated,
  		HelpBody: `Usage: awf render

  Re-render every enabled target after a template or config change and update .awf/awf.lock.
  `,
  	},
  ```

  In `internal/clispec/clispec_test.go`, `TestLookup` looks up `"sync"`; change that argument to `"render"`.

- [ ] **Task 1.2: Rename the handler key and the ahead-note text.** In `cmd/awf/dispatch.go`, change the registry key:

  ```go
  	"render":      func(c *cmdCtx) error { return runSync(c.root, c.stdout) },
  ```

  Keep the Go identifier `runSync` and the file name `cmd/awf/sync.go`: ADR-0159 renames the command surface, not internal identifiers, and renaming both here doubles the reviewable diff for no reader benefit.

  In `cmd/awf/check.go`, the version-ahead note names the command a reader must run:

  ```go
  		fmt.Fprintf(stdout, "note: awf %s is ahead of this project (rendered by %s); run awf render to re-pin\n",
  ```

- [ ] **Task 1.3: Change the provenance banner.** In `internal/project/banner.go`:

  ```go
  const bannerText = "GENERATED by awf: do not edit; change .awf/ and run `awf render`"
  ```

  This rewrites the first line of every managed file, so the re-render in Task 1.7 touches essentially the whole rendered tree. That diff is content-free and expected.

- [ ] **Task 1.4: Rename the runner's verb and its awf invocations.** In `x`: rename the `sync)` case label to `render)`; update the usage line in the `*)` default case so `render` replaces `sync` in the pipe-separated verb list; change `./awf sync "$@"` to `./awf render "$@"`; change `(cd examples/sundial && "$bindir/awf" sync)` to `(cd examples/sundial && "$bindir/awf" render)`. Leave the `check)` and `gate)` cases untouched.

- [ ] **Task 1.5: Update the authored config and descriptor naming `awf sync`.** In `.awf/config.yaml`, change `activeMdRegenCmd: ./x sync` to `activeMdRegenCmd: ./x render`. In `internal/catalog/standard.go`:

  ```go
  		{Key: "activeMdRegenCmd", Kind: "string", Description: "Command that regenerates the generated ADR decision index (INDEX.md).", Default: "", Options: []string{"./awf render", "awf render"}},
  ```

- [ ] **Task 1.6: Batch - update every authored source naming `awf sync`.** One transformation (`awf sync` becomes `awf render`; `./x sync` becomes `./x render`) applied across authored inputs.

  **Representative** - a convention part naming the command in prose:

  ```diff
  -Every `awf sync` unconditionally renders `.awf/memory/.gitignore` with no config gate,
  +Every `awf render` unconditionally renders `.awf/memory/.gitignore` with no config gate,
  ```

  **Edge** - a template that spells the banner out in its own text, where the surrounding comment syntax and the `awf:` prefix must survive:

  ```diff
  -<!-- GENERATED by awf: do not edit; change .awf/ and run `awf sync` -->
  +<!-- GENERATED by awf: do not edit; change .awf/ and run `awf render` -->
  ```

  **Affected-site set** - the output of:

  ```
  git grep -lE 'awf sync|\./x sync' -- .awf templates cmd internal x tools .github
  ```

  Apply the identical shape at every site. Do not run it over `docs/decisions` or `docs/plans`: ADR-0159 Decision 10 leaves retained records naming the old command.

  **Post-check** - this command produces no output:

  ```
  git grep -nE 'awf sync|\./x sync' -- .awf templates cmd internal x tools .github
  ```

- [ ] **Task 1.7: Apply the render-rename operations, open the Implementing sequence, and commit.** Update the prose of these ten claims so each names `awf render` or `./x render` where it named the retired command. The first eight name it as an invocation; the last two enumerate "load, render, sync, or check" and would otherwise name one concept twice, so their enumeration collapses to "load, render, or check":

  `config/migrations-and-locks:noop-autobump`, `config/migrations-and-locks:upgrade-gate`, `rendering/companion-scripts:runner-singleton-toggle`, `rendering/doc-outputs:topic-output-complete`, `rendering/inplace-and-placeholders:part-placeholder-sandboxed`, `rendering/singletons-and-payloads:memory-gitignore-always-on`, `rendering/sync-and-drift:sync-always-writes-active-md`, `rendering/sync-and-drift:sync-backs-up-foreign`, `config/configuration:awf-config-root`, `config/migrations-and-locks:legacy-read-isolation`.

  Each gains `Revised-by: ADR-0159`. Claim slugs and the `rendering/sync-and-drift` topic id are identities and do not change (ADR-0159 Decision 11).

  In the ADR, set `status: Implementing` and append two adjacent events in this order: an `Implementing` event carrying the frozen content digest, then an `Applied` event carrying the next state sequence and these ten operations in the ADR's `State changes` declaration order. `internal/adr/format.go` refuses an `Implementing` event not immediately followed by the first `Applied` event, and `internal/adr/application.go` refuses an `Implementing` status without both applied and remaining operations; ten applied of eighteen leaves eight remaining, satisfying both.

  Run `./x render` (the verb renamed in Task 1.4), then `./x check` (expected: clean, no drift entries and no error-severity findings). Stage the transaction, run `./awf check --staged` (expected: clean), then `./x gate` (expected: every step passes). Commit:

  ```commit
  feat(rendering): rename awf sync to awf render
  ```

## Phase 2: `awf help` lists group children

Self-contained and independently useful: `metrics` and `new` have undiscoverable children today, before `check` becomes a group at all.

- [ ] **Task 2.1: Recurse into children in the global help.** In `cmd/awf/main.go`, `globalHelp` prints only top-level names. Required behaviour after the change:
  - Iterate `clispec.Commands` in table order, printing each command's name and summary exactly as today, so existing top-level assertions keep passing.
  - After a command with a non-empty `Children`, print each child in `Children` order on its own line, indented deeper than the parent, showing the child's own name and its `Summary`.
  - The child line's name column renders the child's own name (`export`, `adr`, `drift`), not the `parent child` pair; indentation carries the relationship.

  Forbidden: any parallel list of group names in `cmd/awf`. The recursion reads `clispec.Commands` only, so `cli-command-spec-single-source` continues to hold.

- [ ] **Task 2.2: Pin the property, declare its claim, and commit.** In `cmd/awf/help_test.go`, extend `TestCliCommandSpecSingleSource` (today it asserts only that top-level names appear in clispec order) with a child assertion: for every command in `clispec.Commands` with children, every child's name appears in `awf help` output after its parent's line and before the next top-level command's line. Acceptance: the test fails if any group child is absent from the overview. Add the proof marker on that test:

  ```go
  // invariant: tooling/cli:help-lists-group-children
  ```

  Author `tooling/cli:help-lists-group-children` in `.awf/topics/parts/tooling/cli/current-state.md` as an invariant with `Backing: test` and `Origin: ADR-0159`, stating that `awf help` lists every group command's children beneath their parent, so no command is reachable only by knowing to ask a parent for help.

  In the ADR, append an `Applied` event with the next state sequence and the single operation `add \`tooling/cli:help-lists-group-children\``. Status stays `Implementing`: eleven applied, seven remaining.

  Run `./x render && ./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): list group children in awf help
  ```

## Phase 3: Regroup the verification commands under `awf check`

The largest phase, and it cannot be sliced further: the group, the gating machinery, the guard exemption, and every call site invoking a retired command must land together, because `./x gate` itself runs `./awf prose-gate` and would fail at any intermediate state. No claim operations land here; the claims describing this reality apply in Phase 4, which is what `Remaining` means.

- [ ] **Task 3.1: Give gating an inherit state.** In `internal/clispec/clispec.go`, renumber the enum so the zero value means "not declared":

  ```go
  const (
  	Inherit        Gating = iota // a group child that declares nothing: resolve from the parent
  	Ungated                      // never gates (version, changelog, upgrade, uninstall, init, and the three regrouped hook checks)
  	Gated                        // the driver gates before the handler
  	GatedInHandler               // the handler gates itself (config/context/topic after their static-fallback check; new after name validation)
  )
  ```

  Add a resolver beside `Child`:

  ```go
  // ResolvedGating returns the child's own gating when it declares one, else the
  // parent's. A child that says nothing inherits; a child that says Ungated under
  // a Gated parent lowers it deliberately (ADR-0159 Decision 4).
  func ResolvedGating(top, cmd Command) Gating {
  	if cmd.Gating != Inherit {
  		return cmd.Gating
  	}
  	return top.Gating
  }
  ```

  Audit the table and give any top-level entry lacking an explicit `Gating` its correct explicit value: a top-level command must never be `Inherit`, having no parent to inherit from.

- [ ] **Task 3.2: Add the project-state exemption as a command property.** In `internal/clispec/clispec.go`, add to `Command`:

  ```go
  	StateExempt bool // bypasses the current-state journal/attestation guard (ADR-0159 Decision 5)
  ```

  Set `StateExempt: true` on the top-level `version` and `changelog` entries and on the `check` children `prose`, `memory`, and `commit` added in Task 3.4. Leave it false everywhere else, `check` itself and `check drift`/`state`/`invariants` included, matching `check`'s treatment today.

- [ ] **Task 3.3: Replace the children-carry-no-gating guard.** In `internal/clispec/clispec_test.go`, delete `TestGroupChildrenCarryNoGating` and its doc comment: it asserts the property this phase deliberately removes. Replace it with a test asserting resolution instead, covering: every top-level command has `Gating != Inherit`; `ResolvedGating` returns the parent's gating for a child left at `Inherit` (assert against a `metrics` or `new` child); `ResolvedGating` returns `Ungated` for `check`'s `prose`, `memory`, and `commit` children while `check` itself is `Gated`.

  Write this test without a proof-marker comment. Its marker is added in Phase 4 Task 4.4 alongside the claim it proves, because a marker naming an undeclared claim fails the corpus load.

- [ ] **Task 3.4: Restructure the command table.** In `internal/clispec/clispec.go`, delete the top-level `invariants`, `commit-gate`, `prose-gate`, and `memory-gate` entries, and replace the `check` entry with a group in its current table position, so `awf help` order changes only by the four removals. `MaxPos` widens to `-1` so the handler owns the unknown-subcommand message (the `new` treatment):

  ```go
  	{
  		Name: "check", Summary: "Verify the project: drift, current state, and the opt-in scans",
  		BoolFlags: []string{"--staged"}, MaxPos: -1, Gating: Gated,
  		HelpBody: `Usage: awf check [--staged]
         awf check <subcommand>

  With no subcommand, re-render in memory and fail if any rendered file is stale or
  hand-edited (drift), then check current-state authority over the working tree.

  With --staged, skip the drift check and instead validate the staged transition:
  the HEAD-to-index ADR status changes and claim add/update/remove mutations must
  correspond, and the index is checked for topic coverage. It reads only committed
  and staged content, never the working tree, so a pre-commit hook can invoke it.
  --staged applies to the bare form only; a subcommand rejects it.

  Subcommands:
    drift        report stale or hand-edited rendered output
    state        report current-state authority findings
    invariants   report each invariant claim's backing and proof sites
    prose        scan tracked text files for typographic punctuation, blocking
    memory       scan staged decision records for working-memory citations, blocking
    commit       validate one commit message (Conventional Commits), blocking
  `,
  		Children: []Command{
  			{Name: "drift", Summary: "Report stale or hand-edited rendered output", MaxPos: 0,
  				HelpBody: `Usage: awf check drift

  Re-render in memory and report every rendered file that is stale or hand-edited,
  including the config-tree hygiene sweep. Does not accept --staged.
  `},
  			{Name: "state", Summary: "Report current-state authority findings", MaxPos: 0,
  				HelpBody: `Usage: awf check state

  Check current-state authority over the working tree. Does not accept --staged;
  the staged transition is awf check --staged.
  `},
  			{Name: "invariants", Summary: "Report each invariant claim's backing and proof sites", MaxPos: 0,
  				HelpBody: `Usage: awf check invariants

  Report each invariant claim's backing mode, an unbacked claim's Verify guidance,
  and a test-backed claim's proof-marker sites.
  `},
  			{Name: "prose", Summary: "Scan tracked text files for typographic punctuation, blocking", MaxPos: 0,
  				Gating: Ungated, StateExempt: true,
  				HelpBody: `Usage: awf check prose

  Report every typographic punctuation substitute in the project's tracked text
  files and exit non-zero on any finding. Exits zero without scanning unless
  proseGate.enabled is true, so a hook or a runner may invoke it unconditionally.
  Permit a character that is genuinely being written about with
  proseGate.exemptions. awf installs no hook; wire this into your own pre-commit
  hook (the rendered .awf/hooks/pre-commit.sh payload runs it when the hooks
  artifact is enabled).
  `},
  			{Name: "memory", Summary: "Scan staged decision records for working-memory citations, blocking", MaxPos: 0,
  				Gating: Ungated, StateExempt: true,
  				HelpBody: `Usage: awf check memory

  Report every citation of a specific working-memory file in the staged decisions
  and plans directories and exit non-zero on any finding: the convention says a
  decision record may name the directory or a placeholder, never an actual file.
  Exits zero without scanning unless memoryCite.enabled is true, so a hook or a
  runner may invoke it unconditionally; the same knob makes awf check commit scan
  the commit-message body for the same thing. Permit a path that genuinely needs
  one with memoryCite.exemptions. awf installs no hook; wire this into your own
  pre-commit hook (the rendered .awf/hooks/pre-commit.sh payload runs it when the
  hooks artifact is enabled).
  `},
  			{Name: "commit", Summary: "Validate one commit message (Conventional Commits), blocking", MaxPos: 1,
  				Gating: Ungated, StateExempt: true,
  				HelpBody: `Usage: awf check commit [FILE]

  Validate one commit message against the Conventional Commits rules (type, scope,
  72-char subject) and exit non-zero on a violation. Reads FILE (the path a
  commit-msg hook passes as $1) or stdin; cleans the message git-style and exempts
  merge/autosquash subjects. awf installs no hook; wire this into your own
  commit-msg hook (the rendered .awf/hooks/commit-msg.sh payload runs it when the
  hooks artifact is enabled).
  `},
  		},
  	},
  ```

- [ ] **Task 3.5: Project the gated-command list per child.** In `internal/clispec/clispec.go`, `GatedCommandNames` iterates top-level commands only. Required behaviour after the change: emit each top-level command whose gating is not `Ungated`, in table order, exactly as today; additionally emit, in table order beneath its parent, each child whose `ResolvedGating` differs from its parent's, spelled `parent child`; a child that inherits emits nothing, so `new` and `metrics` children never appear. Under the restructured table the result is the twelve gated top-level names plus the three ungated `check` children.

  In `internal/clispec/clispec_test.go`, `TestGatedCommandNames` pins a literal list; update the literal to the post-change set. This is the test that fails until edited. The marker-carrying `TestGatedCommandsDisplay` in `internal/project` derives its expectation from `GatedCommandNames()` and passes unchanged, so it is not the one to edit.

- [ ] **Task 3.6: Render the gated list with its exclusions.** In `internal/project/gatedcommands.go`, the projection feeds the agent guide's binary-version-gate line. Required behaviour: render the gated top-level names as today, and render the differing children as a trailing exclusion clause naming them, so a reader sees which `check` subcommands do not gate. Forbidden: presenting an ungated child as a member of the gated set.

- [ ] **Task 3.7: Resolve gating and the state guard from the child.** In `cmd/awf/main.go`, the driver reads `top.Gating`; resolve from the child and re-scope the staged predicate to bare check:

  ```go
  	if g := clispec.ResolvedGating(top, cmd); g == clispec.Gated {
  		gateFn := gate
  		if top.Name == "check" && sub == "" && inv.bools["--staged"] {
  			gateFn = gateStaged
  		}
  		if err := gateFn(cwd); err != nil {
  			return dispatchErr(stderr, err)
  		}
  	}
  ```

  `cmd` and `sub` are already returned by `resolve`; thread them to this call site. Replace `guardProjectState`'s top-level-name switch with the resolved command's property:

  ```go
  func guardProjectState(root string, cmd clispec.Command, top clispec.Command, sub string, inv invocation) error {
  	if cmd.StateExempt {
  		return nil
  	}
  	if top.Name == "init" && inv.bools["--describe"] {
  		return nil
  	}
  	staged := top.Name == "check" && sub == "" && inv.bools["--staged"]
  	// ... remainder unchanged
  ```

  Update the doc comment above `guardProjectState`, which enumerates the exempt commands by name: state instead that exemption is the resolved command's `StateExempt` property, and that the three regrouped `check` children carry it so a commit-msg hook keeps working during a journal or an attested lock. In `cmd/awf/check.go`, `checkLockVsBinary` branches on the staged flag; its caller must pass the bare-check-only value, never the raw flag, so a child can never select the staged lock.

- [ ] **Task 3.8: Add the drift and state entry points.** In `cmd/awf/check.go`, keep `runCheck` as the bare-form entry point, unchanged in behaviour, and extract two new entry points from the parts it already calls:
  - `runCheckDrift(root string, stdout io.Writer) error` - open the project, run `p.Check()`, print each drift entry in the existing format, print `awf check drift: clean` when empty, else return an error naming the drift count. It prints neither the advisory notes nor the version-ahead note; ADR-0159 Decision 2 keeps both on the bare form.
  - `runCheckState(root string, stdout io.Writer) error` - open the project, run `p.CheckCurrentState()`, print the report's notes and findings in the existing format, print `awf check state: clean` when empty, else return an error naming the finding count.

  Forbidden: duplicating the drift or current-state logic. Both call the same `project` methods `runCheck` calls. `runCheckStaged` is unchanged, and its clean line stays `awf check --staged: clean`.

- [ ] **Task 3.9: Rewire the handler registry.** In `cmd/awf/dispatch.go`, delete the `invariants`, `commit-gate`, `prose-gate`, and `memory-gate` keys and replace the `check` value with a group handler. Required behaviour:
  - `sub == ""` with no positionals: run `runCheck(c.root, c.inv.bools["--staged"], c.stdout)`, today's behaviour.
  - `sub` names a child: reject `--staged` with a usage error stating the flag applies to the bare form only, then dispatch to `runCheckDrift`, `runCheckState`, `runInvariants`, `runProseGate`, `runMemoryGate`, or `runCommitGate` (the last taking `firstPos(c.inv.positionals)` and `c.stdin`).
  - `sub == ""` with a positional: `resolve` tests only `args[1]` for a child name, so `awf check --staged drift` arrives here with `drift` as a positional. When that positional names a valid child, return a usage error saying the subcommand must come first (`awf check drift`); otherwise return a usage error listing the valid subcommands.

  Forbidden: listing a valid child among "unknown subcommands" when the user spelled one in the wrong position. `TestHandlerRegistryParity` asserts registry keys match clispec top-level names, so removing four keys alongside four table entries keeps it green.

- [ ] **Task 3.10: Batch - update the user-facing message prefixes.** Each regrouped command prints its own name in at least one message.

  **Representative** - `cmd/awf/prosegate.go`:

  ```diff
  -		return fmt.Errorf("prose-gate: cannot read staged files: %w", err)
  +		return fmt.Errorf("check prose: cannot read staged files: %w", err)
  ```

  **Edge** - `cmd/awf/invariants.go`, where the prefix carries the `awf` word too:

  ```diff
  -		fmt.Fprintln(stdout, "awf invariants: no invariant claims")
  +		fmt.Fprintln(stdout, "awf check invariants: no invariant claims")
  ```

  **Affected-site set** - `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`, `cmd/awf/commitgate.go`, `cmd/awf/invariants.go`, and the tests asserting those strings: `cmd/awf/prosegate_test.go`, `cmd/awf/memorygate_test.go`, `cmd/awf/commitgate_test.go`, `cmd/awf/invariants_test.go`, `cmd/awf/run_test.go`, `cmd/awf/failure_paths_test.go`.

  **Post-check** - `go test ./cmd/awf/...` passes and this command produces no output:

  ```
  git grep -nE '"(prose-gate|memory-gate|commit-gate|awf invariants):' -- cmd internal
  ```

- [ ] **Task 3.11: Rewire the hook templates, the runner, and this project's config.** In `templates/hooks/pre-commit.sh.tmpl`:

  ```diff
  -{{ with .vars.proseGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} prose-gate{{ end }}
  -{{ with .vars.memoryGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} memory-gate{{ end }}
  +{{ with .vars.proseGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} check prose{{ end }}
  +{{ with .vars.memoryGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} check memory{{ end }}
  ```

  In `templates/hooks/commit-msg.sh.tmpl` - the fallback ADR-0159 Decision 9 singles out, because a hook degrading to a nonexistent command is what the publication-safe-templates invariant forbids:

  ```diff
  -{{ with .vars.commitGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} commit-gate{{ end }} "$1"
  +{{ with .vars.commitGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} check commit{{ end }} "$1"
  ```

  In `x`:

  ```diff
  -    ./awf prose-gate
  -    ./awf memory-gate
  +    ./awf check prose
  +    ./awf check memory
  ```

  ```diff
  -    (cd examples/sundial && "$bindir/awf" invariants)
  +    (cd examples/sundial && "$bindir/awf" check invariants)
  ```

  In `.awf/config.yaml`, update `proseGateCmd`, `memoryGateCmd`, and `commitGateCmd` to their new spellings. `checkCmd` names this repo's own runner verb and stays as it is.

- [ ] **Task 3.12: Update and extend the command tests, then commit.** Update every test invoking a retired command name to its new spelling across `cmd/awf/check_test.go`, `cmd/awf/gate_test.go`, `cmd/awf/run_test.go`, `cmd/awf/invariants_test.go`, `cmd/awf/prosegate_test.go`, `cmd/awf/memorygate_test.go`, `cmd/awf/commitgate_test.go`, `cmd/awf/help_test.go`, and `cmd/awf/dashboardread_test.go`. Add tests for the behaviour this phase introduces, each asserting a terminal state:
  - `awf check drift` and `awf check state` each run alone and print their clean line on a clean tree.
  - `awf check --staged` is unchanged; `awf check state --staged` and `awf check drift --staged` each exit non-zero with the bare-form-only usage message.
  - `awf check --staged drift` exits non-zero with the subcommand-order message, not the unknown-subcommand message.
  - `awf check bogus` exits non-zero listing the valid subcommands.
  - `awf check prose` and `awf check memory` succeed against a project whose lock is behind the binary, where bare `awf check` refuses. This is the per-child gating property.

  Run `./x render && ./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): regroup the verification commands under awf check
  ```

## Phase 4: Migration, descriptor prose, docs, and the terminal flip

Closes the ADR: the remaining seven operations apply here, and the `Implemented` event follows the final `Applied` event immediately, as `internal/adr/format.go` requires.

- [ ] **Task 4.1: Add the schema-19 migration.** Create `internal/migrate/renameretiredcommands.go` with `applyRenameRetiredCommands`. Required behaviour:
  - For each config var value that is a string, match the shape `<invocation> <retired-subcommand>[ <trailing args>]`, where `<invocation>` is exactly `awf`, `./awf`, or a path ending in `/awf`.
  - Rewrite the subcommand token: `sync` to `render`, `invariants` to `check invariants`, `prose-gate` to `check prose`, `memory-gate` to `check memory`, `commit-gate` to `check commit`. Preserve the invocation token and every trailing argument verbatim.
  - Leave every other value untouched, including a value naming a runner awf does not own (`./x check`, `make gate`) and any value whose first token is not an awf invocation.
  - Write through `internal/config`'s existing mapping editor; do not hand-roll YAML emission.

  Forbidden: rewriting a value that merely contains a retired word in prose or later in a longer pipeline. The match is anchored at the invocation token.

  Register it in `internal/migrate/migrate.go`:

  ```go
  	{To: 19, Name: "rename-retired-commands", Apply: applyRenameRetiredCommands},
  ```

- [ ] **Task 4.2: Test the migration.** Create `internal/migrate/renameretiredcommands_test.go` as a table test covering, at minimum: each of the five rewrites; all three invocation spellings; trailing-argument preservation; `./x check` left untouched; a non-awf first token left untouched; an absent var left untouched; a value containing a retired word in prose left untouched.

- [ ] **Task 4.3: Correct the descriptor and configspec prose.** In `internal/catalog/standard.go`, update the four remaining descriptors naming a retired command: `commitGateCmd`, `proseGateCmd`, and `memoryGateCmd` (their `Options` entries become `./awf check commit`, `./awf check prose`, `./awf check memory`) and `commitScopes` (its description says "enforced by awf commit-gate/audit"). `activeMdRegenCmd` was corrected in Task 1.5.

  In `internal/configspec/spec.go`, update the `Description` and `Availability` prose of `audit.allowedTypes`, `audit.allowedScopes`, `audit.subjectMaxLength`, `proseGate.enabled`, and `memoryCite.enabled`. These render verbatim into `docs/config-reference.md`, so regenerating without correcting them reproduces the retired names.

- [ ] **Task 4.4: Batch - update the remaining authored sources.** The mirror of Task 1.6 for the regrouped commands.

  **Representative** - a workflow convention part naming a gate command in prose, where only the command name moves and the surrounding sentence stands:

  ```diff
  -The opt-in `awf memory-gate` (on in this repo, wired into `./x gate` and the
  +The opt-in `awf check memory` (on in this repo, wired into `./x gate` and the
  ```

  **Edge** - `.awf/docs/glossary.yaml`, where the command name is the entry's own term, so the term key and its definition prose must move together rather than only the first occurrence on the line.

  **Affected-site set** - the output of:

  ```
  git grep -lE 'awf (invariants|prose-gate|memory-gate|commit-gate)' -- .awf templates tools .github
  ```

  excluding the rendered payloads under `.awf/hooks/`, which a re-render rewrites.

  **Post-check** - after the edits and a re-render, this command produces no output:

  ```
  git grep -nE 'awf (invariants|prose-gate|memory-gate|commit-gate)' -- .awf templates cmd internal x tools .github
  ```

- [ ] **Task 4.5: Author the two remaining claims.** Author `tooling/cli:group-child-gating-honored` in `.awf/topics/parts/tooling/cli/current-state.md`: an invariant, `Backing: test`, `Origin: ADR-0159`, stating that a group child's gating classification resolves from the child when it declares one and from the parent otherwise, so an ungated child under a gated parent is honoured rather than silently gated. Add its proof marker to the resolution test written in Task 3.3.

  Author `tooling/cli:group-child-project-guard-exemption`: an invariant, `Backing: test`, `Origin: ADR-0159`, stating that the current-state journal and attestation guard reads the resolved command's exemption property, so `check prose`, `check memory`, and `check commit` stay runnable in the states where a hook must still function. Add its proof marker to a new test asserting that each of the three children succeeds under a committed journal and under an attested lock while bare `check` refuses.

  Update the prose of these five claims so each names the new command, each gaining `Revised-by: ADR-0159`: `tooling/cli:gated-commands-generated` (reworded for the per-child projection rather than for a rename), `tooling/quality-gates:example-adopter-checked`, `tooling/quality-gates:prose-gate-refuses-without-git`, `tooling/quality-gates:memory-citation-gate`, and `tooling/audit-and-snapshots:commit-gate-shared-rule`. Slugs do not change.

  `tooling/quality-gates:prose-gate-tracked-file-scan` is deliberately absent from every operation list: its body says "the prose scanner" and never names the command, so only its slug carries the old word, and a slug is an identity (ADR-0159 Decision 11).

- [ ] **Task 4.6: Regenerate, flip both statuses, and close.** Run `./x render` so AGENTS.md, `docs/decisions/INDEX.md`, and `docs/config-reference.md` regenerate from the corrected authored inputs.

  In the ADR, set `status: Implemented` and append, in this order and in the same commit, an `Applied` event carrying the next state sequence and the seven remaining operations in the ADR's `State changes` declaration order, then an `Implemented` event carrying the frozen content digest. The `Implemented` event must be the entry immediately after the final `Applied` event, and every declared operation must now be applied.

  In this plan's frontmatter, set `status: Implemented`.

  Run `./x render && ./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): migrate retired command names and close ADR-0159
  ```

## Verification

- [ ] `./x check` is clean and `./x gate` passes at the close of every phase, not only at the end.
- [ ] `git grep -nE 'awf (sync|invariants|prose-gate|memory-gate|commit-gate)' -- .awf templates cmd internal x tools .github` produces no output. `docs/decisions` and `docs/plans` are excluded by design and still name the old commands.
- [ ] `awf help` lists every group child, including `check`'s six and the existing `new` and `metrics` children.
- [ ] `awf check` on a clean tree produces the same verdict and exit status as before the change; only the version-ahead note's command name differs.
- [ ] `awf check prose` and `awf check memory` succeed against a project whose lock is behind the binary, where bare `awf check` refuses.
- [ ] Each of `awf check prose`, `awf check memory`, and `awf check commit` succeeds under a committed current-state journal and under an attested lock, where bare `awf check` refuses. This is the property whose absence would break commit-msg hooks.
- [ ] A fixture project whose config carries `proseGateCmd: ./awf prose-gate` has that value rewritten to `./awf check prose` by `awf upgrade`, while a sibling `checkCmd: ./x check` is left untouched.
- [ ] The ADR reaches `status: Implemented` with every declared operation applied and none remaining.

## Notes

- **Phase boundaries are not release boundaries.** The schema-19 migration that carries adopters across the rename lands in Phase 4; a release cut at the close of Phase 1 or Phase 3 would rename commands with no migration behind them. Only the completed plan is releasable.
- **The Phase 1 diff is dominated by the banner.** Changing `bannerText` rewrites the first line of every managed file, so Phase 1's commit touches essentially the whole rendered tree. That churn is content-free; review the authored files and treat the banner lines as mechanical.
- **The Go symbol `runSync` and the file `cmd/awf/sync.go` keep their names.** ADR-0159 renames the command surface, not internal identifiers. A symbol rename is a legitimate follow-up but would double Phase 1's reviewable diff.
- **The `--staged` widening is the subtle risk.** The three predicates keying on `top.Name == "check"` compile and pass their existing tests after regrouping while meaning something different, so the tests added in Task 3.12 for `check state --staged` and `check drift --staged` are what actually pin the correction.
- **Follow-on decision, deliberately out of scope.** Making bare `awf check` run every enabled check with a ran/skipped report owns the git precondition on the prose and memory scans (both call `snapshot.IndexTree` before consulting their knob), the duplicate invocations in the pre-commit payload, and the exit-code contract when every check is skipped.
- **Also noted during ADR review, out of scope here.** ADR-0100's in-place-editable-sections primitive has had no consumer since ADR-0156 replaced ADR-0101's `x` with a single-section wrapper. Worth its own look.
