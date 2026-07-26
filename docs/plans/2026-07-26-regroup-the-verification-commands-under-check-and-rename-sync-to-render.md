---
date: 2026-07-26
adrs: [159]
status: Proposed
---
# Plan: Regroup the verification commands under check and rename sync to render

## Goal

Ship ADR-0159: rename `awf sync` to `awf render`, fold `awf invariants`, `awf prose-gate`, `awf memory-gate`, and `awf commit-gate` into an `awf check` group alongside two new `drift` and `state` children, and give the driver the per-child gating and per-child project-state exemption that regrouping requires.

Non-goals: bare `awf check` does not change what it runs or what it returns, no ran/skipped report is added, no `*Cmd` var key is renamed, and no retained record is rewritten (the ADR and plan files themselves, the dated analyses under `docs/research/`, and the released version sections of `changelog/`). The rule covers records, not whole directories: the awf-managed files that live among them (`docs/decisions/INDEX.md`, both `README.md` and `template.md` pairs) are rendered output and are regenerated like any other.

## Architecture summary

Three mechanisms carry the change; the rest is mechanical rename.

**Gating gains an inherit state.** `clispec.Gating`'s zero value is `Ungated` today, so a child that declares nothing is indistinguishable from a child that declares `Ungated`. Phase 3 renumbers the enum so the zero value is `Inherit`, making "says nothing" and "says Ungated" distinct. The driver then resolves gating from the child, falling back to the parent. That is what lets `check prose`, `check memory`, and `check commit` stay ungated under a gated `check` while `metrics` and `new` children keep inheriting.

**Project-state exemption becomes a command property.** `guardProjectState` switches on a hardcoded list of top-level names. Phase 3 replaces that list with a `StateExempt bool` on `clispec.Command`, read from the resolved node, so the three regrouped gates keep the exemption they hold today and a commit-msg hook keeps working during a committed journal or an attested lock.

**`check` becomes a group with a default leaf.** `metrics` is the existing precedent: a group whose bare form does work and whose children add more. `check`'s handler dispatches on `c.sub`, with the empty sub running today's `runCheck` unchanged.

`--staged` stays a flag on the bare form only. The three predicates that key on `top.Name == "check"` are re-scoped to bare check, and the handler rejects the flag on any child. The children still declare the flag in their spec: `parseArgs` validates against the resolved child, so an undeclared flag dies with a generic unknown-flag error before the handler can produce a useful one.

Phases are ordered so each closing commit passes `./x gate` alone, and so every ADR-0159 operation lands in the phase whose reality it describes. Phase boundaries are not release boundaries: only the completed plan is releasable, because the schema migration that moves adopters lands in Phase 4.

**Operation batching.** ADR-0159 declares nineteen operations, applied in four batches: ten in Phase 1 (the render rename, alongside the `Implementing` flip), one in Phase 2, three in Phase 3, and five in Phase 4 (alongside the `Implemented` flip). The lifecycle rules pin the ends: `internal/adr/format.go` refuses an `Implementing` event not immediately followed by the first `Applied` event and an explicit `Implemented` event not immediately preceded by a final `Applied` event, and `internal/adr/application.go` refuses `Implementing` without both applied and remaining operations. Every intermediate state satisfies that: 10/9, 11/8, 14/5, then 19/0.

## File structure

- **Created:**
  - `internal/migrate/renameretiredcommands.go` - the schema-19 migration rewriting retired subcommand tokens in config var values
  - `internal/migrate/renameretiredcommands_test.go` - its table test
- **Modified:**
  - `internal/clispec/clispec.go` - the command table: `sync` to `render`, the `check` group and its six children, the `Inherit` gating zero value, the `StateExempt` field, `GatedCommandNames`, the new weaker-child projection
  - `internal/clispec/clispec_test.go` - `TestLookup`, `TestGatedCommandNames`, and the replacement for `TestGroupChildrenCarryNoGating`
  - `cmd/awf/main.go` - `globalHelp` child recursion, resolved-gating dispatch, `guardProjectState` per-child exemption, bare-check staged predicates
  - `cmd/awf/dispatch.go` - handler registry keys, the `check` group handler
  - `cmd/awf/check.go` - the drift and state entry points, the version-ahead note text
  - `cmd/awf/invariants.go`, `cmd/awf/prosegate.go`, `cmd/awf/memorygate.go`, `cmd/awf/commitgate.go` - user-facing message prefixes
  - `internal/audit/audit.go`, `internal/config/config.go`, `internal/project/render.go`, `internal/project/currentstate.go` - Go doc comments naming retired commands
  - `internal/config/edit.go` - the string-valued mapping editor the migration needs
  - the `cmd/awf` test files carrying a retired command name or bare token (Tasks 1.6 and 3.12 give discovery commands rather than a fixed list)
  - `internal/project/banner.go` - `bannerText`
  - `internal/project/gatedcommands.go`, `internal/project/gatedcommands_test.go` - the projection and its pinned expectation
  - `internal/catalog/standard.go` - five var descriptors' descriptions and `Options`
  - `internal/configspec/spec.go` - five key entries' `Description` / `Availability`
  - `internal/migrate/migrate.go` - register migration 19
  - `templates/hooks/pre-commit.sh.tmpl`, `templates/hooks/commit-msg.sh.tmpl` - the three unset-var fallbacks
  - `templates/docs/working-with-awf.md.tmpl` - the `gatedCommands` render-key description
  - `x`, `.githooks/pre-commit` - the runner verb and the hook stub's remediation message
  - `README.md`, `examples/sundial/README.md` - the public CLI command table and the example's usage notes (both hand-authored and outside the lock, so nothing else catches them)
  - `examples/sundial/.awf/docs/parts/testing/layout.md` - a raw convention part spliced verbatim into the example's rendered testing doc, so a re-render propagates the retired name rather than fixing it
  - `changelog/CHANGELOG.md` - a new `## [Unreleased]` entry only
  - `.awf/config.yaml` - `activeMdRegenCmd`, `proseGateCmd`, `memoryGateCmd`, `commitGateCmd`
  - `.awf/docs/parts/architecture/components.md`, `.awf/domains/parts/config/current-state.md`, `internal/config/edit_test.go` - the two authored enumerations of the config serialization funnel and the claim's proof file, all reached by no discovery grep
  - the authored `.awf/` inputs naming a retired command (Tasks 1.6 and 4.5 give the discovery commands)
  - `.awf/topics/parts/**/current-state.md` - the nineteen ADR-0159 claim operations
  - `docs/decisions/0159-regroup-the-verification-commands-under-check-and-rename-sync-to-render.md` - status history
  - this plan - the status flip
- **Deleted:** none.

## Phase 1: Rename `awf sync` to `awf render`

Self-contained: the rename completes within this phase, so `./x gate` passes at its close. No `check` work happens here.

- [ ] **Task 1.1: Rename the command in the spec table and its two pinned tests.** In `internal/clispec/clispec.go`, replace the `sync` entry with:

  ```go
  	{
  		Name: "render", Summary: "Re-render after a template or config change",
  		MaxPos: 0, Gating: Gated,
  		HelpBody: `Usage: awf render

  Re-render every enabled target after a template or config change and update .awf/awf.lock.
  `,
  	},
  ```

  Two tests in `internal/clispec/clispec_test.go` pin the old name and fail at this phase's gate unless both change here: `TestLookup` looks up `"sync"` (change the argument to `"render"`), and `TestGatedCommandNames` pins `want := []string{"sync", "check", "invariants", ...}` (change the first element to `"render"` only; Task 3.5 makes the remaining edits to this literal).

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

  In `.githooks/pre-commit`, the drift-refusal message tells the developer what to run, and is read exactly when a commit has just been refused:

  ```diff
  -    echo "pre-commit: the staged slice has drift in $label - run ./x sync and stage the result" >&2
  +    echo "pre-commit: the staged slice has drift in $label - run ./x render and stage the result" >&2
  ```

- [ ] **Task 1.5: Update the authored config and descriptor naming `awf sync`.** In `.awf/config.yaml`, change `activeMdRegenCmd: ./x sync` to `activeMdRegenCmd: ./x render`. In `internal/catalog/standard.go`:

  ```go
  		{Key: "activeMdRegenCmd", Kind: "string", Description: "Command that regenerates the generated ADR decision index (INDEX.md).", Default: "", Options: []string{"./awf render", "awf render"}},
  ```

- [ ] **Task 1.6: Batch - update every authored source naming the retired command.** One transformation (`awf sync` becomes `awf render`; `./x sync` becomes `./x render`) applied across authored inputs.

  **Representative** - a convention part naming the command in prose:

  ```diff
  -Every `awf sync` unconditionally renders `.awf/memory/.gitignore` with no config gate,
  +Every `awf render` unconditionally renders `.awf/memory/.gitignore` with no config gate,
  ```

  **Edge** - a Go test passing the command as a bare token, with no `awf ` prefix for a prose grep to find. `cmd/awf/help_test.go` is the sharpest case, because its failure is a `Lookup` miss rather than a string mismatch:

  ```diff
  -	run([]string{"awf", "help", "sync"}, &out, &errb)
  -	spec, _ := clispec.Lookup("sync")
  +	run([]string{"awf", "help", "render"}, &out, &errb)
  +	spec, _ := clispec.Lookup("render")
  ```

  **Affected-site set** - the union of two discovery commands, because the prose grep alone misses every bare-token Go site:

  ```
  git grep -lE 'awf sync|\./x sync' -- .awf .githooks templates cmd internal x tools .github README.md examples/sundial/README.md examples/sundial/.awf/docs examples/sundial/.awf/config.yaml
  git grep -ln '"sync"' -- cmd internal
  ```

  A git pathspec is a leading-path match, so `.awf` never reaches `examples/sundial/.awf`; the example's authored inputs must be named separately, and its `README.md` is hand-authored with no provenance banner, so nothing re-renders it. Exclude the rendered payloads that a re-render rewrites: `.awf/hooks/*.sh`, `.awf/memory/.gitignore`, and `.awf/metrics/.gitignore` all match through their banner line and must never be hand-edited. Do not run either command over `docs/decisions`, `docs/plans`, `docs/research`, or the released sections of `changelog/`: those are retained records naming the old command. In the second command's output, `internal/telemetry` uses `"sync"` in an unrelated sense; inspect before editing rather than replacing blind.

  **Post-check** - after the edits *and* the Task 1.7 re-render (which is what corrects the banner-bearing payloads), `go test ./...` passes and this command produces no output:

  ```
  git grep -nE 'awf sync|\./x sync' -- .awf .githooks templates cmd internal x tools .github README.md examples/sundial/README.md examples/sundial/.awf/docs examples/sundial/.awf/config.yaml
  ```

- [ ] **Task 1.7: Apply the render-rename operations, open the Implementing sequence, and commit.** Update the prose of these ten claims so each names `awf render` or `./x render` where it named the retired command. The first eight name it as an invocation; the last two enumerate "load, render, sync, or check" and would otherwise name one concept twice, so their enumeration collapses to "load, render, or check":

  `config/migrations-and-locks:noop-autobump`, `config/migrations-and-locks:upgrade-gate`, `rendering/companion-scripts:runner-singleton-toggle`, `rendering/doc-outputs:topic-output-complete`, `rendering/inplace-and-placeholders:part-placeholder-sandboxed`, `rendering/singletons-and-payloads:memory-gitignore-always-on`, `rendering/sync-and-drift:sync-always-writes-active-md`, `rendering/sync-and-drift:sync-backs-up-foreign`, `config/configuration:awf-config-root`, `config/migrations-and-locks:legacy-read-isolation`.

  Each gains `Revised-by: ADR-0159`. Claim slugs and the `rendering/sync-and-drift` topic id are identities and do not change (ADR-0159 Decision 11).

  In the ADR, set `status: Implementing` and append two adjacent events in this order: an `Implementing` event carrying the frozen content digest, then an `Applied` event carrying the next state sequence and these ten operations in the ADR's `State changes` declaration order.

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

  In the ADR, append an `Applied` event with the next state sequence and the single operation `add \`tooling/cli:help-lists-group-children\``. Status stays `Implementing`: eleven applied, eight remaining.

  Run `./x render && ./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): list group children in awf help
  ```

## Phase 3: Regroup the verification commands under `awf check`

The largest phase, and it cannot be sliced further: the group, the gating machinery, the guard exemption, and every call site invoking a retired command must land together, because `./x gate` itself runs `./awf prose-gate` and `./awf memory-gate` (`x:24-25`) and would fail at any intermediate state.

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

- [ ] **Task 3.3: Replace the children-carry-no-gating guard and pin both new properties.** In `internal/clispec/clispec_test.go`, delete `TestGroupChildrenCarryNoGating` and its doc comment: it asserts the property this phase deliberately removes. Replace it with a test asserting resolution instead, covering: every top-level command has `Gating != Inherit`; `ResolvedGating` returns the parent's gating for a child left at `Inherit` (assert against a `metrics` or `new` child); `ResolvedGating` returns `Ungated` for `check`'s `prose`, `memory`, and `commit` children while `check` itself is `Gated`. Add its proof marker:

  ```go
  // invariant: tooling/cli:group-child-gating-honored
  ```

  Add a second test, in `cmd/awf`, asserting that each of `check prose`, `check memory`, and `check commit` succeeds under a committed current-state journal and under an attested lock while bare `check` refuses in both. Add its proof marker:

  ```go
  // invariant: tooling/cli:group-child-project-guard-exemption
  ```

  Both claims are authored in Task 3.14, in this same commit. A proof marker naming an undeclared claim fails the corpus load with `unknown claim ID`, so marker and claim must land together; keeping both in Phase 3 also avoids shipping a commit whose claim prose contradicts its own proof test.

  `cmd/awf/main_test.go`'s `TestResolveReturnsTopLevel` keeps passing but its doc comment states the retired rule ("so run() gates ... off the top-level node rather than the child, whose Gating is an unset Ungated zero"). Correct it here: the body is still right, only the stated rationale is wrong.

- [ ] **Task 3.4: Restructure the command table.** In `internal/clispec/clispec.go`, delete the top-level `invariants`, `commit-gate`, `prose-gate`, and `memory-gate` entries, and replace the `check` entry with a group in its current table position, so `awf help` order changes only by the four removals. `MaxPos` widens to `-1` so the handler owns the unknown-subcommand message (the `new` treatment).

  Every child declares `BoolFlags: []string{"--staged"}` even though none accepts it: `resolve` returns the child as `cmd` and `parseArgs` validates against the child's spec, so an undeclared `--staged` dies with a generic `unknown flag` error before the handler can produce the bare-form-only message Task 3.9 specifies. Declaring the flag is what lets the handler own the diagnostic, exactly as `MaxPos: -1` does for the unknown-subcommand message.

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
  			{Name: "drift", Summary: "Report stale or hand-edited rendered output",
  				BoolFlags: []string{"--staged"}, MaxPos: 0,
  				HelpBody: `Usage: awf check drift

  Re-render in memory and report every rendered file that is stale or hand-edited,
  including the config-tree hygiene sweep. Does not accept --staged.
  `},
  			{Name: "state", Summary: "Report current-state authority findings",
  				BoolFlags: []string{"--staged"}, MaxPos: 0,
  				HelpBody: `Usage: awf check state

  Check current-state authority over the working tree. Does not accept --staged;
  the staged transition is awf check --staged.
  `},
  			{Name: "invariants", Summary: "Report each invariant claim's backing and proof sites",
  				BoolFlags: []string{"--staged"}, MaxPos: 0,
  				HelpBody: `Usage: awf check invariants

  Report each invariant claim's backing mode, an unbacked claim's Verify guidance,
  and a test-backed claim's proof-marker sites.
  `},
  			{Name: "prose", Summary: "Scan tracked text files for typographic punctuation, blocking",
  				BoolFlags: []string{"--staged"}, MaxPos: 0,
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
  			{Name: "memory", Summary: "Scan staged decision records for working-memory citations, blocking",
  				BoolFlags: []string{"--staged"}, MaxPos: 0,
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
  			{Name: "commit", Summary: "Validate one commit message (Conventional Commits), blocking",
  				BoolFlags: []string{"--staged"}, MaxPos: 1,
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

- [ ] **Task 3.5: Split the gated projection into two lists.** A flat `[]string` gives a renderer no way to tell a gated member from an ungated exclusion, and "differs from the parent" would wrongly classify a hypothetical gated child under an ungated parent. In `internal/clispec/clispec.go`, keep `GatedCommandNames()` returning only genuinely gated names, unchanged in contract, and add a sibling:

  ```go
  // UngatedGroupChildren returns, in table order, each group child whose resolved
  // gating is Ungated under a parent that gates - the exclusions a reader needs
  // beside the gated set. Spelled "parent child". A child that is not weaker than
  // its parent is simply not an exclusion; the gated list stays top-level-only
  // (ADR-0159 Decision 4).
  func UngatedGroupChildren() []string
  ```

  The comment says "not an exclusion" rather than "a member of the gated set", because `GatedCommandNames()` iterates top-level commands with no child recursion and this task keeps that contract unchanged. A hypothetical gated child under an ungated parent would therefore appear in neither projection. No such child exists in the table, so nothing is lost today; the wording just avoids promising a classification the code does not make.

  In `internal/clispec/clispec_test.go`, finish the `TestGatedCommandNames` literal begun in Task 1.1: remove `"invariants"`, leaving the twelve gated top-level names. Add a test pinning `UngatedGroupChildren()` to exactly `check prose`, `check memory`, `check commit`.

- [ ] **Task 3.6: Render the gated list with its exclusions.** The rendered value is interpolated into a shipped adopter-facing sentence at `templates/agents-doc/AGENTS.md.tmpl`, which reads `Every gated command ({{ $.gatedCommands }}) refuses to run when the binary is behind...`, so the exact string matters. In `internal/project/gatedcommands.go`, `gatedCommandsDisplay()` must produce:

  ```
  `render`, `check`, `audit`, `metrics`, `doctor`, `list`, `config`, `context`, `topic`, `new`, `enable`, `disable`, except `check prose`, `check memory`, and `check commit`
  ```

  giving the rendered sentence: "Every gated command (`render`, `check`, ..., `disable`, except `check prose`, `check memory`, and `check commit`) refuses to run when the binary is behind the project on schema generation or lock `awfVersion`". Build the exclusion clause from `UngatedGroupChildren()`, never from a literal.

  `internal/project/gatedcommands_test.go` asserts `gatedCommandsDisplay()` equals the comma-joined `GatedCommandNames()` exactly, so it fails once the clause is added; update its expectation to the two-part form. Its proof marker for `tooling/cli:gated-commands-generated` stays on that test.

  In `templates/docs/working-with-awf.md.tmpl`, the `gatedCommands` render-key description reads "the backticked, comma-separated list of binary-version-gated commands", which is now false for every adopter's rendered doc. Replace it with a description naming both parts: the backticked gated top-level list plus the named ungated-child exclusions.

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

  Replace `guardProjectState`'s top-level-name switch with the resolved command's property, and update its call site, which today passes `(cwd, top, inv)`:

  ```go
  	if err := guardProjectState(cwd, cmd, top, sub, inv); err != nil {
  		return dispatchErr(stderr, err)
  	}
  ```

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

  Four doc comments assert the rule this task inverts and are false in every clause afterwards. None is reachable by any discovery grep in this plan, because none names a retired command:
  - `cmd/awf/main.go`, above the gating block: "Gating is read from top (the top-level command), not the resolved child: a group's children never set Gating, so a future Gated group must gate from its top-level node rather than silently inherit a child's Ungated zero value."
  - `cmd/awf/main.go`, above `guardProjectState`: it enumerates the exempt commands by name. State instead that exemption is the resolved command's `StateExempt` property, and that the three regrouped `check` children carry it so a commit-msg hook keeps working during a journal or an attested lock.
  - `cmd/awf/dispatch.go`, above `resolve`: "the driver reads gating and the handler key from top, since both are top-level properties a child never overrides." The handler key is still top-level; the gating is not.
  - `internal/clispec/clispec.go`, above `GatedCommandNames`: "This is the single source of the doc-published gated-command list", false once `UngatedGroupChildren` becomes the second source.

  In `cmd/awf/check.go`, `checkLockVsBinary` branches on the staged flag; its caller must pass the bare-check-only value, never the raw flag, so a child can never select the staged lock.

- [ ] **Task 3.8: Add the drift and state entry points.** In `cmd/awf/check.go`, keep `runCheck` as the bare-form entry point, unchanged in behaviour, and extract two new entry points from the parts it already calls:
  - `runCheckDrift(root string, stdout io.Writer) error` - open the project, run `p.Check()`, print each drift entry in the existing format, print `awf check drift: clean` when empty, else return an error naming the drift count. It prints neither the advisory notes nor the version-ahead note; ADR-0159 Decision 2 keeps both on the bare form.
  - `runCheckState(root string, stdout io.Writer) error` - open the project, run `p.CheckCurrentState()`, print the report's notes and findings in the existing format, print `awf check state: clean` when empty, else return an error naming the finding count.

  Forbidden: duplicating the drift or current-state logic. Both call the same `project` methods `runCheck` calls. `runCheckStaged` is unchanged, and its clean line stays `awf check --staged: clean`.

- [ ] **Task 3.9: Rewire the handler registry.** In `cmd/awf/dispatch.go`, delete the `invariants`, `commit-gate`, `prose-gate`, and `memory-gate` keys and replace the `check` value with a group handler. Required behaviour:
  - `sub == ""` with no positionals: run `runCheck(c.root, c.inv.bools["--staged"], c.stdout)`, today's behaviour.
  - `sub` names a child: if `--staged` is present, return a usage error stating the flag applies to the bare form only (the child declares the flag in Task 3.4 solely so this message is reachable); otherwise dispatch to `runCheckDrift`, `runCheckState`, `runInvariants`, `runProseGate`, `runMemoryGate`, or `runCommitGate` (the last taking `firstPos(c.inv.positionals)` and `c.stdin`).
  - `sub == ""` with a positional: `resolve` tests only `args[1]` for a child name, so `awf check --staged drift` arrives here with `drift` as a positional. When that positional names a valid child, return a usage error saying the subcommand must come first (`awf check drift`); otherwise return a usage error listing the valid subcommands.

  Forbidden: listing a valid child among "unknown subcommands" when the user spelled one in the wrong position. `TestHandlerRegistryParity` asserts registry keys match clispec top-level names, so removing four keys alongside four table entries keeps it green.

- [ ] **Task 3.10: Batch - update the message prefixes and doc comments.** Each regrouped command prints its own name in at least one message, and several Go doc comments name one too.

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

  **Affected-site set** - the four command files (`cmd/awf/prosegate.go`, `memorygate.go`, `commitgate.go`, `invariants.go`), the five Go doc comments outside `cmd/awf` (`internal/audit/audit.go`, `internal/config/config.go` in two places, `internal/project/render.go`, `internal/project/currentstate.go`), and every site the discovery command in Task 3.12 returns.

  **Post-check** - `go test ./...` passes and this command produces no output:

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

- [ ] **Task 3.12: Batch - update the command tests.** Discovery rather than a fixed list, because the retired names appear as bare tokens in argv fixtures that no prose grep reaches.

  **Representative** - a test invoking the command through the driver:

  ```diff
  -	code := run([]string{"awf", "prose-gate"}, &out, &errb)
  +	code := run([]string{"awf", "check", "prose"}, &out, &errb)
  ```

  **Edge** - a fixture listing command names as data, where only some elements move (`cmd/awf/failure_paths_test.go`):

  ```diff
  -	for _, name := range []string{"sync", "check", "invariants", "audit", "list"} {
  +	for _, name := range []string{"render", "check", "audit", "list"} {
  ```

  **Affected-site set** - the output of:

  ```
  git grep -ln '"invariants"\|"prose-gate"\|"memory-gate"\|"commit-gate"' -- cmd internal
  ```

  **Post-check** - `go test ./...` passes and that same grep returns nothing outside `internal/migrate`.

- [ ] **Task 3.13: Add the tests for the new behaviour.** Each asserts a terminal state:
  - `awf check drift` and `awf check state` each run alone and print their clean line on a clean tree.
  - `awf check --staged` is unchanged; `awf check state --staged` and `awf check drift --staged` each exit non-zero with the bare-form-only usage message (reachable only because Task 3.4 declares the flag on each child).
  - `awf check --staged drift` exits non-zero with the subcommand-order message, not the unknown-subcommand message.
  - `awf check bogus` exits non-zero listing the valid subcommands.
  - `awf check prose` and `awf check memory` succeed against a project whose lock is behind the binary, where bare `awf check` refuses. This is the per-child gating property.
  - `awf help` lists the six `check` children (extending the Phase 2 assertion to the new group).

  The two entry points Task 3.8 adds bring new statements that the clean-path assertions above never reach: a `project.Open` error return, a `p.Check()` / `p.CheckCurrentState()` error return, the per-entry print loop, and the non-zero-count error return. ADR-0012's coverage gate fails below 100%, so each needs either a test or a reasoned ignore. Add `awf check drift` against a tree with a drifted rendered file and `awf check state` against a tree with a current-state finding, each asserting the non-zero exit and the count-naming error message. Any branch that is genuinely unreachable because the driver pre-gates it carries a `// coverage-ignore: <reason>` comment, following the precedent already in `cmd/awf/check.go` on `runCheck`'s corrupt-lock path.

- [ ] **Task 3.14: Author the two new claims, apply the third batch, and commit.** Author in `.awf/topics/parts/tooling/cli/current-state.md`, both `Backing: test` with `Origin: ADR-0159`, their proof markers already placed in Task 3.3:
  - `tooling/cli:group-child-gating-honored` - a group child's gating classification resolves from the child when it declares one and from the parent otherwise, so an ungated child under a gated parent is honoured rather than silently gated.
  - `tooling/cli:group-child-project-guard-exemption` - the current-state journal and attestation guard reads the resolved command's exemption property, so `check prose`, `check memory`, and `check commit` stay runnable in the states where a hook must still function.

  Update `tooling/cli:gated-commands-generated`'s prose for the two-list projection (gated top-level names plus the separately-reported ungated group children), gaining `Revised-by: ADR-0159`. Applying it here keeps the claim consistent with its own proof test, which Task 3.6 rewrites in this commit.

  In the ADR, append an `Applied` event with the next state sequence and these three operations in declaration order. Status stays `Implementing`: fourteen applied, five remaining.

  Run `./x render && ./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): regroup the verification commands under awf check
  ```

## Phase 4: Migration, descriptor prose, docs, and the terminal flip

Closes the ADR: the remaining five operations apply here, and the `Implemented` event follows the final `Applied` event immediately.

- [ ] **Task 4.1: Add the string-valued config editor.** `internal/config`'s funnel writes a bool (`SetMappingScalar`), an int (`SetMappingInteger`), sequences (`SetArray`, `SetArrayMember`), and a string into an *absent* key (`SeedVarKey`); nothing rewrites a present string value. Add the string sibling in `internal/config/edit.go`, matching the existing editors' shape and routing through the same `encode` funnel:

  ```go
  // SetMappingString writes value at key.child, replacing a present string value.
  // The string sibling of SetMappingScalar and SetMappingInteger (ADR-0159
  // Decision 8), added for the retired-command value migration.
  func SetMappingString(src []byte, key, child, value string) ([]byte, error)
  ```

  Forbidden: hand-rolled YAML emission. The funnel exists so no caller emits YAML directly, and ADR-0159 Decision 8 says so explicitly.

  Two authored docs enumerate the funnel's members exactly and go stale the moment it gains one. Neither discovery grep reaches them, because neither names a retired command, so both are corrected here in the same commit as the editor:
  - `.awf/docs/parts/architecture/components.md` lists "typed mutation (`SetArrayMember`, `SetArray`, `SetMappingScalar`, and `SetMappingInteger`, comment-preserving `yaml.Node` round trips) behind one `encode` funnel". Add `SetMappingString` to that list.
  - `.awf/domains/parts/config/current-state.md` carries the same enumeration and adds "all behind a single two-space `encode` funnel that retires the former hand-rolled emitter and string editor". Add the new member and reword the trailing clause, which now reads as if no string editor exists.

  The precedent is exact: the ADR-0144 batch that added `SetMappingInteger` updated both surfaces in the same commit as the editor.

  Add a `SetMappingString` case to `internal/config/edit_test.go`, which carries the proof marker for `config/configuration:config-serialization-owned`: assert that a present string value is replaced at the funnel's two-space indent. The migration's own table test exercises the editor indirectly and probably satisfies the coverage gate on its own, but the claim's backing lives in this file, and Task 4.7 updates that claim's prose to name the new member.

- [ ] **Task 4.2: Add the schema-19 migration.** Create `internal/migrate/renameretiredcommands.go` with `applyRenameRetiredCommands`. Required behaviour:
  - For each config var value that is a string, match the shape `<invocation> <retired-subcommand>[ <trailing args>]`, where `<invocation>` is exactly `awf`, `./awf`, or a path ending in `/awf`.
  - Rewrite the subcommand token: `sync` to `render`, `invariants` to `check invariants`, `prose-gate` to `check prose`, `memory-gate` to `check memory`, `commit-gate` to `check commit`. Preserve the invocation token and every trailing argument verbatim.
  - Leave every other value untouched, including a value naming a runner awf does not own (`./x check`, `make gate`) and any value whose first token is not an awf invocation.
  - Write through `config.SetMappingString` from Task 4.1.

  Forbidden: rewriting a value that merely contains a retired word in prose or later in a longer pipeline. The match is anchored at the invocation token.

  Register it in `internal/migrate/migrate.go`:

  ```go
  	{To: 19, Name: "rename-retired-commands", Apply: applyRenameRetiredCommands},
  ```

- [ ] **Task 4.3: Test the migration.** Create `internal/migrate/renameretiredcommands_test.go` as a table test covering, at minimum: each of the five rewrites; all three invocation spellings; trailing-argument preservation; `./x check` left untouched; a non-awf first token left untouched; an absent var left untouched; a value containing a retired word in prose left untouched.

  These fixtures must contain the retired spellings literally, since they are what the migration rewrites. This file and `renameretiredcommands.go` are the one sanctioned home for those spellings, and both are excluded from the repo-wide post-checks below.

- [ ] **Task 4.4: Correct the descriptor and configspec prose.** In `internal/catalog/standard.go`, update the four remaining descriptors naming a retired command: `commitGateCmd`, `proseGateCmd`, and `memoryGateCmd` (their `Options` entries become `./awf check commit`, `./awf check prose`, `./awf check memory`) and `commitScopes` (its description says "enforced by awf commit-gate/audit"). `activeMdRegenCmd` was corrected in Task 1.5.

  In `internal/configspec/spec.go`, update the `Description` and `Availability` prose of `audit.allowedTypes`, `audit.allowedScopes`, `audit.subjectMaxLength`, `proseGate.enabled`, and `memoryCite.enabled`. These render verbatim into `docs/config-reference.md`, so regenerating without correcting them reproduces the retired names.

- [ ] **Task 4.5: Batch - update the remaining authored sources.** The mirror of Task 1.6 for the regrouped commands.

  **Representative** - a workflow convention part naming a gate command in prose:

  ```diff
  -The opt-in `awf memory-gate` (on in this repo, wired into `./x gate` and the
  +The opt-in `awf check memory` (on in this repo, wired into `./x gate` and the
  ```

  **Edge** - `.awf/docs/glossary.yaml`, where the command name is the entry's own term, so the term key and its definition prose must move together rather than only the first occurrence on the line.

  **Affected-site set** - the output of:

  ```
  git grep -lE 'awf (invariants|prose-gate|memory-gate|commit-gate)' -- .awf .githooks templates cmd internal x tools .github README.md examples/sundial/README.md examples/sundial/.awf/docs examples/sundial/.awf/config.yaml ':!internal/migrate/renameretiredcommands*.go'
  ```

  `README.md` is hand-authored and absent from `.awf/awf.lock`, so no drift check will catch it: its public CLI command table is the project's front door and names five retired commands. The example adopter needs its own pathspec entries for the same reason a leading-path `.awf` never reaches `examples/sundial/.awf`; in particular `examples/sundial/.awf/docs/parts/testing/layout.md` is a raw convention part spliced verbatim into the example's rendered testing doc, so a re-render propagates the retired name rather than fixing it, and the model adopter would ship a doc naming a command that no longer exists. Do not widen the pathspec to `examples/sundial` wholesale: that sweeps in the rendered adapter trees and the example's own retained plan. Exclude the rendered payloads under `.awf/hooks/`, which a re-render rewrites.

  **Post-check** - after the edits and a re-render, that same command produces no output.

- [ ] **Task 4.6: Write the Unreleased changelog entry.** `docs/releasing.md` requires entries added as they land so the changelog stays release-ready, and ADR-0159's Consequences name the changelog as the mitigation for adopters whose var value is neither migratable shape. Under `## [Unreleased]`, add a breaking-changes entry covering: the `sync` to `render` rename with no alias; the four regrouped commands and their new spellings; the forced schema-19 upgrade, including for projects with no matching var value; the `rename-retired-commands` value migration and the shapes it deliberately does not cover; and the banner-driven whole-tree re-render on first `awf render` after upgrading.

  Do not touch any released version section: ADR-0159 Decision 10 treats them as retained history, on the same grounds as decisions and plans.

- [ ] **Task 4.7: Apply the final batch, flip both statuses, and close.** Update the prose of these five claims, each gaining `Revised-by: ADR-0159`. They are listed here in the ADR's declaration order, which is the order the Applied event must record: `internal/adr/history.go` rejects an event whose operations do not ascend by declaration position, so writing them in the order below is load-bearing, not cosmetic.
  1. `config/configuration:config-serialization-owned` - its enumeration of the serialization funnel gains `SetMappingString`
  2. `tooling/quality-gates:example-adopter-checked`
  3. `tooling/quality-gates:prose-gate-refuses-without-git`
  4. `tooling/quality-gates:memory-citation-gate`
  5. `tooling/audit-and-snapshots:commit-gate-shared-rule`

  Slugs do not change.

  `tooling/quality-gates:prose-gate-tracked-file-scan` is deliberately absent from every operation list: its body says "the prose scanner" and never names the command, so only its slug carries the old word, and a slug is an identity (ADR-0159 Decision 11).

  Registering migration 19 makes `migrate.Current()` return 19 while both `.awf/awf.lock` and `examples/sundial/.awf/awf.lock` still carry generation 18, so the version gate refuses `render` and `check` in both trees until each is upgraded. Run the upgrade before the render, exactly as the previous schema bump did:

  ```
  go run ./cmd/awf upgrade
  bindir="$(mktemp -d)"
  go build -o "$bindir/awf" ./cmd/awf
  (cd examples/sundial && "$bindir/awf" upgrade)
  ```

  The example must be upgraded with a built binary, not `go run ../../cmd/awf`: it is its own Go module with no requirement on this one, so a relative package path outside the module fails with "directory ../../cmd/awf outside main module or its selected dependencies". This is the build-once-then-run shape `x` already uses for the example, and its comment there says why.

  Expected terminal state: both locks stamped at schema generation 19, after which `./x render` and `./x check` run again.

  Run `./x render` so AGENTS.md, `docs/decisions/INDEX.md`, and `docs/config-reference.md` regenerate from the corrected authored inputs. In the ADR, set `status: Implemented` and append, in this order and in the same commit, an `Applied` event carrying the next state sequence and these five operations in the ADR's declaration order, then an `Implemented` event carrying the frozen content digest. In this plan's frontmatter, set `status: Implemented`.

  Run `./x check` (expected: clean), stage, `./awf check --staged` (expected: clean), `./x gate` (expected: pass). Commit:

  ```commit
  feat(awf): migrate retired command names and close ADR-0159
  ```

## Verification

- [ ] `./x check` is clean and `./x gate` passes at the close of every phase, not only at the end.
- [ ] This command produces no output:

  ```
  git grep -nE 'awf (sync|invariants|prose-gate|memory-gate|commit-gate)|\./x sync' -- .awf .githooks templates cmd internal x tools .github README.md examples/sundial/README.md examples/sundial/.awf/docs examples/sundial/.awf/config.yaml ':!internal/migrate/renameretiredcommands*.go'
  ```

  Excluded by design and still carrying the old spellings: the ADR and plan records, the dated analyses under `docs/research/`, the released sections of `changelog/`, and the migration's own fixtures.
- [ ] `go test ./...` passes, which is what catches the bare-token argv fixtures no prose grep reaches.
- [ ] `awf help` lists every group child, including `check`'s six and the existing `new` and `metrics` children.
- [ ] `awf check` on a clean tree produces the same verdict and exit status as before the change; only the version-ahead note's command name differs.
- [ ] `awf check prose` and `awf check memory` succeed against a project whose lock is behind the binary, where bare `awf check` refuses.
- [ ] Each of `awf check prose`, `awf check memory`, and `awf check commit` succeeds under a committed current-state journal and under an attested lock, where bare `awf check` refuses. This is the property whose absence would break commit-msg hooks.
- [ ] A fixture project whose config carries `proseGateCmd: ./awf prose-gate` has that value rewritten to `./awf check prose` by `awf upgrade`, while a sibling `checkCmd: ./x check` is left untouched.
- [ ] The ADR reaches `status: Implemented` with every declared operation applied and none remaining.

## Notes

- **Phase boundaries are not release boundaries.** The schema-19 migration that carries adopters across the rename lands in Phase 4; a release cut at the close of Phase 1 or Phase 3 would rename commands with no migration behind them. Only the completed plan is releasable.
- **Retained records keep the old names, but the rule is about records, not directories.** The ADR and plan files, the dated analyses under `docs/research/`, and the released version sections of `changelog/` are not rewritten (ADR-0159 Decision 10). A released entry states what shipped under that version; rewriting it would describe a command that version never had. Only `## [Unreleased]` gains an entry. Five awf-managed files live inside `docs/decisions/` and `docs/plans/` and *are* rewritten by the re-render, because they are rendered output, not records: `docs/decisions/INDEX.md`, and the `README.md` and `template.md` in each directory. `docs/decisions/README.md` changes beyond its banner, because its authored source under `.awf/parts/` carries retired mentions that Task 1.6 corrects. Reverting them would produce drift that fails the phase's own `./x check`.
- **The Phase 1 diff is dominated by the banner.** Changing `bannerText` rewrites the first line of every managed file, so Phase 1's commit touches essentially the whole rendered tree. That churn is content-free; review the authored files and treat the banner lines as mechanical.
- **Prose greps miss argv fixtures.** The retired names appear in Go tests as bare tokens (`resolve([]string{"sync"})`, `clispec.Lookup("sync")`), which no `awf sync` grep finds. Both rename batches therefore pair a prose grep with a bare-token grep, and both post-checks require `go test ./...` rather than a clean grep alone.
- **The Go symbol `runSync` and the file `cmd/awf/sync.go` keep their names.** ADR-0159 renames the command surface, not internal identifiers. A symbol rename is a legitimate follow-up but would double Phase 1's reviewable diff.
- **The `--staged` widening is the subtle risk.** The three predicates keying on `top.Name == "check"` compile and pass their existing tests after regrouping while meaning something different, so the tests in Task 3.13 for `check state --staged` and `check drift --staged` are what actually pin the correction.
- **Follow-on decision, deliberately out of scope.** Making bare `awf check` run every enabled check with a ran/skipped report owns the git precondition on the prose and memory scans (both call `snapshot.IndexTree` before consulting their knob), the duplicate invocations in the pre-commit payload, and the exit-code contract when every check is skipped.
- **Also noted during ADR review, out of scope here.** ADR-0100's in-place-editable-sections primitive has had no consumer since ADR-0156 replaced ADR-0101's `x` with a single-section wrapper. Worth its own look.
