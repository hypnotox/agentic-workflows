# Plan: command-table CLI dispatch, importable command spec, generated gated-command list

Implements **ADR-0094** (`docs/decisions/0094-command-table-cli-dispatch-importable-command-spec-and-generated-gated-command-list.md`). All design rationale lives in the ADR; this plan is the execution record only. The ADR stays `Proposed` until Phase 6's final commit flips it to `Implemented`.

## Goal

Replace the hand-rolled per-command `switch` in `cmd/awf` with a declarative command table sourced from a new importable leaf package `internal/clispec`, driven by a generic parse-once dispatcher; collapse the `enable`/`disable` and `newLocal*` twin handlers; complete ADR-0093's deferred resolver `Add`/`Remove`→`Enable`/`Disable` rename; and generate the gated-command list from `clispec` into both doc surfaces so it can no longer drift.

## Architecture summary

- `internal/clispec` (new, data-only leaf): the ordered command table — each `Command` carries flags, positional bounds, a three-valued `Gating`, help text, and (for `new`) child subcommands. Pure helpers only; no handler funcs; imports nothing from `cmd/awf` or `internal/project`.
- `cmd/awf` builds a runtime dispatcher by attaching handler funcs (keyed by command path) to `clispec.Commands`. The driver parses args once into an `invocation`, resolves the command (recursing into a group's children), applies the gating classification (driver pre-gates `Gated`; `config`/`context`/`new` self-gate in-handler), and calls the handler with the parsed `invocation`.
- `internal/project` imports `clispec` for the gated set, exposed as a `{{=awf:gatedCommands}}` placeholder and a `gatedCommands` template render-key; both doc surfaces consume the one generated value.

## Tech stack

- Go 1.26; module `github.com/hypnotox/agentic-workflows`.
- Packages touched: new `internal/clispec`; `cmd/awf` (`main.go`, `list_add.go`, `new.go`, `context.go`, and their tests); `internal/project` (`resolve.go`, `placeholders.go`, `render.go`, a new `gatedcommands.go`); `internal/catalog` (`graph.go` if any `PlanOp` refs — none found, confirm); `templates/agents-doc/AGENTS.md.tmpl`; `.awf/agents-doc.yaml`; `.awf/domains/parts/tooling/current-state.md`; `.awf/docs/parts/architecture/components.md`; changelog.
- Gates: `./x gate` (100% coverage, `deadcode`, lint, pincheck) before every commit; `./x check` (drift + invariants) for doc/config phases.
- Constraint — template-source residue (ADR-0082, `template-source-residue`): no `ADR-NNNN` four-digit citation may appear in any `templates/**` source file; ADR citations live in `.awf/*.yaml` **data** and render via `{{ with .ref }}`. This governs the Phase 5 `AGENTS.md.tmpl` edit.

## File structure

**Created**
- `internal/clispec/clispec.go` — the command table, `Command`/`Gating` types, `Commands`, `Resolve`, `GatedCommandNames`, `UsageLine`, help helpers.
- `internal/clispec/clispec_test.go` — table integrity + helper coverage.
- `cmd/awf/dispatch.go` — the generic driver, `invocation`, `cmdCtx`, `parseArgs`, the handler registry.
- `internal/project/gatedcommands.go` — `gatedCommandsDisplay()` + its test hook.

**Modified**
- `cmd/awf/main.go` — `run` becomes the driver shell; `argSpecs`/`commandOrder`/`checkArgs`/`positionals`/`hasFlag`/`valueFlag`/`setFlags`/`baseFlag`/`globalHelp`/`hasHelpFlag` removed or relocated.
- `cmd/awf/list_add.go`, `cmd/awf/new.go`, `cmd/awf/context.go`, `cmd/awf/*_test.go` — handler signatures + twin collapse + rename + test tokens.
- `internal/project/resolve.go` — resolver rename.
- `internal/project/placeholders.go`, `internal/project/render.go` — the gated-commands value.
- `templates/agents-doc/AGENTS.md.tmpl`, `.awf/agents-doc.yaml`, `.awf/domains/parts/tooling/current-state.md`, `.awf/docs/parts/architecture/components.md`.
- `changelog/CHANGELOG.md`.

**Deleted** — none (functions removed live inside modified files).

---

## Phase 1 — `internal/clispec` foundation; rewire help/usage/order/validation

Goal: `clispec` becomes the single source for the command metadata; `main.go` derives help, usage, order, and flag/arity validation from it. Dispatch (`switch`) and per-handler `gate()` calls are **unchanged** — behavior-preserving. Backs `cli-command-spec-single-source`.

### Task 1.1 — create the `clispec` package

- [ ] Create `internal/clispec/clispec.go` with the types and table below. The `Commands` table is a mechanical translation of the current `argSpecs` map (`cmd/awf/main.go:210-385`) plus `commandOrder` (`main.go:30-33`), with `Gating` assigned from the verified gate-call inventory: **`Gated`** = sync, check, invariants, audit, list, enable, disable; **`GatedInHandler`** = config, context, new; **`Ungated`** = init, commit-gate, upgrade, uninstall, changelog, version. `new` carries `Children` (adr/skill/agent/doc). Copy each command's `Summary`/`HelpBody` verbatim from the current `argSpecs[name].summary`/`.help`, and `BoolFlags`/`ValueFlags`/`MinPos`/`MaxPos` from the current spec. Mark `--set` repeatable (init). Exact skeleton:

```go
// Package clispec is the single declarative source of awf's CLI command set:
// every command's flags, positional bounds, gating, help text, and (for a group
// command) its subcommands. cmd/awf builds its runtime dispatcher by attaching
// handler funcs to these specs; internal/project reads the gated set to generate
// docs. Data only — no handler funcs and no import of cmd/awf or internal/project,
// so it stays an importable leaf.
package clispec

import "strings"

// Gating classifies when a command runs the binary-version gate (ADR-0094 Decision 3).
type Gating int

const (
	Ungated        Gating = iota // never gates (version, changelog, upgrade, uninstall, commit-gate, init)
	Gated                        // the driver gates before the handler
	GatedInHandler               // the handler gates itself (config/context after their static-fallback check; new after name validation)
)

// Command is one CLI command (or subcommand). A command with Children is a group:
// the driver dispatches on the next positional to a child; a leaf carries no
// Children and is run by its attached handler. MaxPos < 0 means unbounded.
type Command struct {
	Name       string
	Summary    string // one-line, for `awf help`
	HelpBody   string // full `awf <cmd> --help` text
	BoolFlags  []string
	ValueFlags []string // includes repeatables
	Repeatable []string // subset of ValueFlags collected into invocation.Multi
	MinPos     int
	MaxPos     int
	Gating     Gating
	Children   []Command
}

// Commands is the ordered command table — the sole source of the command set,
// `awf help` order, the usage line, and the gated-command list.
var Commands = []Command{
	// ... one Command literal per current argSpecs entry, in commandOrder order ...
}
```

- [ ] Fill `Commands` with all sixteen top-level entries in `commandOrder` order (`init, sync, check, invariants, audit, commit-gate, list, config, context, new, enable, disable, upgrade, uninstall, changelog, version`), each carrying its verbatim `Summary`/`HelpBody`/flags/bounds/`Gating` per the mapping above. `new` keeps its own `HelpBody` (the current `new` help verbatim, its group overview) and `Gating: GatedInHandler`. `new`'s `Children` are four leaves `adr`/`skill`/`agent`/`doc`, each with its own `HelpBody` split out from the current `new` help bullets (`main.go:317-320`) so `awf new <child> --help` prints child-specific help — a **new capability**. **Child arity stays loose (`MinPos: 0, MaxPos: -1`)** so `parseArgs` does not pre-empt the handlers' existing exact usage messages (`runNew`/`newADR`/`newLocal` own the "usage: awf new …" / empty-description errors); the children carry help/flags metadata only, not arity enforcement. Children are not separately gated — the single `new` handler (`runNew`) gates after name validation, so `adr-new-version-gated` stays backed on `runNew`.

- [ ] Add the pure helpers below to `clispec.go`:

```go
// Lookup returns the top-level command named name.
func Lookup(name string) (Command, bool) {
	for _, c := range Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// Child returns group command c's child named name.
func (c Command) Child(name string) (Command, bool) {
	for _, ch := range c.Children {
		if ch.Name == name {
			return ch, true
		}
	}
	return Command{}, false
}

// GatedCommandNames returns the top-level command names whose gating is not
// Ungated, in table order — a group contributes only its own name. This is the
// single source for the doc gated-command list (ADR-0094 Decision 6).
func GatedCommandNames() []string {
	var out []string
	for _, c := range Commands {
		if c.Gating != Ungated {
			out = append(out, c.Name)
		}
	}
	return out
}

// Names returns every top-level command name in table order.
func Names() []string {
	out := make([]string, len(Commands))
	for i, c := range Commands {
		out[i] = c.Name
	}
	return out
}

// UsageLine renders the `awf <a|b|...>` usage token list from the table.
func UsageLine() string { return "awf <" + strings.Join(Names(), "|") + ">" }
```

### Task 1.2 — cover the `clispec` package

- [ ] Create `internal/clispec/clispec_test.go` covering: `GatedCommandNames()` equals the expected ten-name slice in order (`sync, check, invariants, audit, list, config, context, new, enable, disable`); every `Command` and child has non-empty `Name`/`Summary`/`HelpBody`; `Lookup`/`Child` hit and miss; `Names`/`UsageLine` exact strings; no duplicate top-level names. This gives the data-only package its statements coverage (helpers) — table literals need no test, but every helper branch does.

### Task 1.3 — rewire `main.go` onto `clispec`

- [ ] In `cmd/awf/main.go`: delete `argSpecs`, `commandOrder`, and the `argSpec` type. Replace their readers:
  - `globalHelp()` iterates `clispec.Commands` (using `.Name`/`.Summary`) instead of `commandOrder`/`argSpecs`.
  - the bare-args usage line (`main.go:61`) prints `clispec.UsageLine()`.
  - `run`'s help lookups (`main.go:66-72,80-84`) resolve via `clispec.Lookup` and print `.HelpBody`.
  - `checkArgs` reads `clispec.Command` fields (pass the resolved `Command`, not loose slices).
- [ ] Keep the existing `switch args[1]` dispatch and every existing per-handler `gate()` call for this phase — only the metadata source changes.
- [ ] Update `cmd/awf/help_test.go` and any `main_test.go`/`list_add_test.go` references: iterate `clispec.Commands` instead of `argSpecs`/`commandOrder`; delete `TestCommandOrderMatchesArgSpecs` (the separate-slice parity it guarded no longer exists) and, in its place, keep `TestGlobalHelpListsAllCommands`/`TestPerCommandHelp` re-pointed at `clispec.Commands`. `TestHelpSubcommandDispatch` compares against `clispec.Lookup("sync").HelpBody`.
- [ ] Add the `cli-command-spec-single-source` backing: a test `TestCliCommandSpecSingleSource` in `cmd/awf` asserting the rendered `globalHelp()` command set+order and `clispec.UsageLine()` are byte-equal to their `clispec`-derived expectation, and place the marker comment `// invariant: cli-command-spec-single-source` on `clispec.Commands` in `clispec.go`. (The gated-list half of this invariant lands in Phase 5; the comment covers the single-table property now.)

### Task 1.4 — verify + commit

- [ ] `go build ./... && ./x gate` — expect `coverage: 100.0%`, `0 issues`, `no production dead code`, `all workflow references pinned`.
- [ ] `./x check` — expect `awf check: clean` / `awf invariants: clean` (the ADR is still `Proposed`, so the new slug is not yet enforced; the marker is present for Phase 6).
- [ ] Commit: `git add internal/clispec cmd/awf/main.go cmd/awf/help_test.go cmd/awf/main_test.go cmd/awf/list_add_test.go && git commit -m "refactor(tooling): source CLI command metadata from internal/clispec"`. Scope `tooling`.

---

## Phase 2 — the generic parse-once driver

Goal: replace the `switch` with a table driver; parse args once; relocate the gate per the classification. Behavior is pinned by the existing `run_test.go`/`failure_paths_test.go` — the driver is written to keep them green.

### Task 2.1 — driver types and the parse-once pass

- [ ] Create `cmd/awf/dispatch.go` with:

```go
package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// invocation is a command's arguments parsed once. Handlers read this; they never
// re-scan the raw args slice.
type invocation struct {
	positionals []string
	bools       map[string]bool     // every declared BoolFlag → present
	values      map[string]string   // every declared ValueFlag → value ("" if absent)
	multi       map[string][]string // every declared Repeatable flag → all values
}

// cmdCtx bundles what a handler needs: the working dir, the parsed args, the
// resolved subcommand token (for a group command; "" otherwise), and the I/O.
type cmdCtx struct {
	root   string
	sub    string
	inv    invocation
	stdout io.Writer
	stdin  io.Reader
}

// parseArgs validates rest against cmd's flag/positional spec and builds the
// invocation in one pass (folding the former checkArgs/positionals/valueFlag/
// setFlags/hasFlag/baseFlag scans). A value flag consumes its following token.
func parseArgs(cmd clispec.Command, rest []string) (invocation, error) {
	inv := invocation{bools: map[string]bool{}, values: map[string]string{}, multi: map[string][]string{}}
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case slices.Contains(cmd.ValueFlags, a):
			if i+1 >= len(rest) {
				return invocation{}, &usageErr{fmt.Sprintf("awf %s: flag %s needs a value", cmd.Name, a)}
			}
			i++
			if slices.Contains(cmd.Repeatable, a) {
				inv.multi[a] = append(inv.multi[a], rest[i])
			} else {
				inv.values[a] = rest[i]
			}
		case slices.Contains(cmd.BoolFlags, a):
			inv.bools[a] = true
		case strings.HasPrefix(a, "-"):
			return invocation{}, &usageErr{fmt.Sprintf("awf %s: unknown flag %q", cmd.Name, a)}
		default:
			inv.positionals = append(inv.positionals, a)
		}
	}
	if len(inv.positionals) < cmd.MinPos || (cmd.MaxPos >= 0 && len(inv.positionals) > cmd.MaxPos) {
		return invocation{}, &usageErr{fmt.Sprintf("awf %s: unexpected arguments", cmd.Name)}
	}
	return inv, nil
}
```

### Task 2.2 — the handler registry and resolution

- [ ] The `new` group handler reads `c.sub` — the resolved subcommand token already declared on `cmdCtx` in Task 2.1 (`"adr"` for `awf new adr`, `""` for a non-group command).
- [ ] In `dispatch.go`, add the handler registry keyed by **top-level command name** and the resolver. A group command has ONE handler (not per-child entries): `new`'s handler wraps the existing `runNew`, passing the resolved child token — so `runNew` (and its `adr-new-version-gated` marker and ~26 direct test callers) survives unchanged. Handlers adapt today's `run*` funcs to `func(*cmdCtx) error`, reading `c.inv` instead of raw `args`:

```go
type handler func(*cmdCtx) error

// handlers maps a top-level command name to its handler. A group command (new)
// has a single handler that dispatches on c.sub; children are NOT separate keys.
var handlers = map[string]handler{
	"init": func(c *cmdCtx) error { return runInit(c.root, c.inv.bools["--force"], c.inv.bools["--describe"], c.inv.multi["--set"], c.inv.values["--answers"], c.stdout) },
	"new":  func(c *cmdCtx) error { return runNew(c.root, c.sub, c.inv.positionals, c.stdout) },
	// ... one entry per top-level command ...
}
```

  A parity test (Task 2.4) asserts `handlers` keys exactly match the top-level `clispec` command names (both directions) — leaves and the `new` group.

- [ ] Add `resolve(args []string) (cmd clispec.Command, sub string, rest []string, ok bool)`: look up `args[0]` via `clispec.Lookup`; if it is a group and `args[1]` names a child (via `cmd.Child`), return the **child** as `cmd` (so `parseArgs` validates against the child's flag spec and `--help` prints the child `HelpBody`), `sub = args[1]`, `rest = args[2:]`; else a leaf/group returns itself, `sub = ""`, `rest = args[1:]`. For a group invoked with no child or an unknown child, `resolve` returns the **group** with `sub = ""` and `rest = args[1:]`; the driver still routes to the group's handler (`runNew`), whose existing empty/`default` cases emit the current `usage: awf new <kind> <title>` and unknown-kind messages verbatim. `enable`/`disable`'s kind-specific messages stay inside their handlers, reading `c.inv.positionals`.

### Task 2.3 — rewrite `run` as the driver

- [ ] Replace `run`'s body (`main.go:59-192`) and delete `hasFlag`/`valueFlag`/`setFlags`/`baseFlag`/`positionals`/`checkArgs`/`hasHelpFlag` (now folded into `parseArgs`/`dispatch.go`). New `run` flow: bare-args guard → `help`/`--help` forms (via `clispec.Lookup`) → `getwd` → `resolve` (unknown command → `usageErr`) → `--help`/`-h` intercept on `rest` → `parseArgs` → `if cmd.Gating == clispec.Gated { gate(cwd) }` → `handlers[topName](&cmdCtx{root: cwd, sub: sub, inv: inv, stdout: stdout, stdin: stdin})` → the existing `usageErr`→2 / err→1 mapping. (Note: the registry key is the top-level command name even when `resolve` returned a child spec — the child's spec drives parse/help, the group's handler drives dispatch via `c.sub`.)
- [ ] **`context` git-path resolution:** move the `--staged`/`--range`→paths logic (today at `main.go:122-141`) into `runContext`, placed **at the very top of `runContext`, before the `ConfigPath` stat / static-fallback branch** (`context.go:23-30`) — preserving today's order so that outside an adopted tree the static output still carries the git-resolved paths and a bad selector still errors (`context-static-fallback` unchanged). `runContext` reads `c.inv.positionals`, `c.inv.bools["--staged"]`, `c.inv.values["--range"]`, `c.inv.bools["--json"]` and keeps gating in-handler (it is `GatedInHandler`). No separate driver hook is needed — the resolution and gate both live in the handler.
- [ ] Adapt the remaining handlers (`runSync`, `runCheck`, `runInvariants`, `runAudit`, `runCommitGate`, `runList`, `runConfig`, `runUpgrade`, `runUninstall`, `runChangelog`, `runVersion`) to the `func(*cmdCtx) error` registry entries, reading `c.inv`. `commit-gate` reads its optional positional (`c.inv.positionals`) and still uses `stdin`. Handlers keep their own `gate()` calls only for `config`/`context`/`new` children; remove the now-driver-owned `gate()` calls from `runSync`/`runCheck`/`runInvariants`/`runAudit`/`runList` and from `runEnable`/`runDisable` prologue (the driver pre-gates them). `new` children keep their in-handler `gate()` (after name validation).

### Task 2.4 — tests + verify + commit

- [ ] Update `cmd/awf/run_test.go`, `failure_paths_test.go`, `context_test.go`, `new_test.go`, `list_add_test.go` for any raw-args assumptions; the behavior assertions (exit codes, messages) are unchanged, so most tests pass as-is once handler signatures compile. Add `TestHandlerRegistryParity` asserting `handlers` keys == `clispec` leaf paths (both directions).
- [ ] Add coverage for every new `dispatch.go` branch (unknown flag, missing value, arity under/over, unknown command, group with unknown child, each gating arm, the `context` pre-gate usage errors). A genuinely-unreachable branch takes `// coverage-ignore: <reason>`.
- [ ] `./x gate` green; `./x check` clean.
- [ ] Commit: `git commit -m "refactor(tooling): dispatch CLI commands through a parse-once driver"` (68 chars). Scope `tooling`.

---

## Phase 3 — collapse the twin handlers

Goal: `runEnable`/`runDisable` share a prologue; `newLocalArtifact`/`newLocalDoc` merge. Pure refactor; behavior pinned by `list_add_test.go`/`new_test.go`.

### Task 3.1 — enable/disable shared prologue

- [ ] Extract the shared prologue of `runEnable` (`list_add.go:145-201`) and `runDisable` (`list_add.go:226-284`) into one helper `func toggle(root, kind, name string, dir direction, flags toggleFlags, stdout io.Writer) error` that runs: driver already gated → `checkGraphFlags` → `target`/`bootstrap`/`hooks` bespoke dispatch → `PluralKind` → `project.Open` → per-direction validation → per-direction plan → `rewriteConfig` → per-direction post-notes → `runSync`. **Use the CURRENT symbol names in this phase** — `addRemoveTarget`/`addRemoveSingleton` for the bespoke arms and `ResolveAdd`/`ResolveRemove` for the plans; Phase 4 renames these (and their new call sites inside `toggle`) together. `direction` selects the resolver call, the validation (enable: catalog/domain-name + not-already-enabled; disable: is-enabled), and the post-step (enable: `scaffoldDomainCurrentState`; disable: orphan notes + `noteUnrequiredAgents` + the dependent-refusal guard). `runEnable`/`runDisable` become thin wrappers passing the direction. Keep the exact user-facing messages and note text.
- [ ] Read the per-kind bespoke behavior from `kindDescriptors` where it already lives; do not add per-kind branches. `target`/`bootstrap`/`hooks` stay the bespoke non-descriptor arms (unchanged, only renamed in Phase 4).

### Task 3.2 — new local-artifact merge

- [ ] `runNew` (the `new` group handler, keeping its `adr-new-version-gated` marker) still dispatches on kind: `case "adr"` → `newADR`; `case "skill", "agent", "doc"` → the merged `newLocal`. Merge `newLocalArtifact` (`new.go:52`) and `newLocalDoc` (`new.go:146`) into one `newLocal(root, kind string, args []string, stdout io.Writer)` parameterized by kind: kind ∈ {skill, agent} uses `ValidateArtifactName` + `localPartStub` + a `{description}` sidecar; kind == doc uses `ValidateDocName` + `localDocPartStub` + a `{title,description}` sidecar and the catalog-doc collision message. Factor the shared spine (validate name → gate → open → pool/authored-files collision guards → write sidecar+stub → `SetArrayMember` → `seedScaffoldVars` for skill/agent → `runSync`).

### Task 3.3 — verify + commit

- [ ] `./x gate` green (100% coverage — the merged helpers cover both former paths); `./x check` clean.
- [ ] Commit: `git commit -m "refactor(tooling): collapse enable/disable and new-local twin handlers"`. Scope `tooling`.

---

## Phase 4 — resolver `Add`/`Remove` → `Enable`/`Disable` rename (mechanical sweep)

Goal: complete ADR-0093's deferred rename. Mechanical; one worked exemplar + the full site inventory + a grep-zero verify.

**Exemplar** (the pattern applied at every site):

```
ResolveAdd     → ResolveEnable
ResolveRemove  → ResolveDisable
PlanOp.Add (field, bool)  → PlanOp.Enable
addRemoveTarget    → enableDisableTarget
addRemoveSingleton → enableDisableSingleton
```

Doc comments move with the identifiers (`// ResolveAdd plans enabling…` → `// ResolveEnable plans enabling…`), keeping the wording accurate.

### Task 4.1 — apply the rename at every site

- [ ] **Production sites** (from `grep -rn 'ResolveAdd\|ResolveRemove\|\.Add\b\|addRemoveTarget\|addRemoveSingleton' internal/project internal/catalog cmd/awf | grep -v _test`):
  - `internal/project/resolve.go`: `type PlanOp` field `Add bool` → `Enable bool` (lines 12-16); `ResolveAdd` (18-46) name + doc + the `PlanOp{…, Add: true, …}` literal (37); `ResolveRemove` (48-74) name + doc + the two `PlanOp{…, Add: false, …}` literals (56, 66).
  - `cmd/awf/list_add.go`: `printPlan`'s `if !op.Add` (123) → `if !op.Enable`; `addRemoveSingleton`/`addRemoveTarget` definitions + all call sites → `enableDisable*`; `ResolveAdd`/`ResolveRemove` call sites → `ResolveEnable`/`ResolveDisable`. **Note:** Phase 3 moved several of these into the new `toggle` helper — grep the whole file for the current names (`grep -n 'ResolveAdd\|ResolveRemove\|addRemove\|op\.Add' cmd/awf/list_add.go`) and rename at their post-Phase-3 homes, not the pre-refactor line numbers above.
- [ ] **Test sites** (`grep -rn 'ResolveAdd\|ResolveRemove\|\.Add\b\|addRemoveTarget\|addRemoveSingleton' internal/project/resolve_test.go cmd/awf/*_test.go`): rename identically. `PlanOp{…, Add: …}` literals and `.Add` reads in `resolve_test.go` and any `list_add_test.go` uses.
- [ ] Leave untouched (not this vocabulary): `catalog.Node`/`RequiresOf`/`Closure` (no Add/Remove naming — confirmed), the config editors `SetArray`/`SetArrayMember`/`SetMappingScalar`, `wt.Add` in `internal/testsupport/gitfixture`, and the stable invariant slugs `add-skill-pairs-agent`/`remove-agent-pairing-guard`/`add-applies-closure-plan`/`remove-refuses-dependents`/`cli-config-kinds` (slugs are identifiers, not vocabulary — ADR-0094 Decision 7).

### Task 4.2 — verify + commit

- [ ] Grep-zero verify: `grep -rn 'ResolveAdd\|ResolveRemove\|addRemoveTarget\|addRemoveSingleton' internal cmd` returns nothing; `grep -rn '\.Add\b' internal/project cmd/awf | grep -v 'wt.Add\|SetArray'` returns nothing.
- [ ] `./x gate` green (rename is type-checked; coverage unchanged); `./x check` clean.
- [ ] Commit (subject ≤72; the ADR-0093 reference goes in the body): `git commit -m "refactor(tooling): rename resolver Add/Remove to Enable/Disable" -m "Completes ADR-0093 Decision item 4 (deferred resolver rename), per ADR-0094 Decision 7."`. Scope `tooling`.

---

## Phase 5 — generate the gated-command list into both docs

Goal: single-source the gated list from `clispec`; expose it as a placeholder and a render-key; reword both doc surfaces. Backs `gated-commands-generated`.

### Task 5.1 — the generated value

- [ ] Create `internal/project/gatedcommands.go`:

```go
package project

import (
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
)

// gatedCommandsDisplay renders the gated-command list as a backticked, comma-joined
// list from the single clispec source (ADR-0094 Decision 6). It is a tool constant
// (identical for every adopter — the same awf binary), so it takes no config input.
// invariant: gated-commands-generated
func gatedCommandsDisplay() string {
	names := clispec.GatedCommandNames()
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	return strings.Join(quoted, ", ")
}
```

- [ ] In `internal/project/placeholders.go` `placeholderRegistry()` add `put("gatedCommands", gatedCommandsDisplay())` (a package func, not a `*Project` method — no config input). The residual-guard loop already rejects a value carrying `{{=awf` (the backticked list cannot).
- [ ] In `internal/project/render.go` `data()` (line 65) add `"gatedCommands": gatedCommandsDisplay(),` alongside `commitScopes`.

### Task 5.2 — the agents-doc typed marker

- [ ] In `templates/agents-doc/AGENTS.md.tmpl`, add a branch to the invariants range loop (after the `kind == "scopes"` branch at line 42), mirroring it. **The ADR citation comes from the entry's `.ref` data via `{{ with .ref }}` — NOT hardcoded in template source (ADR-0082 residue guard bans `ADR-NNNN` in `templates/**`):**

```
{{- else if eq .kind "gated-commands" }}
- **Binary-version gate.** Every gated command ({{ $.gatedCommands }}) refuses to run when the binary is behind the project on schema generation or lock `awfVersion`; `config` and `context` degrade to a static reference outside an adopted tree instead of refusing.{{ with .ref }} ({{ . }}){{ end }}
```

- [ ] In `.awf/agents-doc.yaml`, replace the hand-written `text: '**Binary-version gate.** …'` invariant entry (the enumerated-list line) with a typed marker that **keeps its `ref`** so the citation renders from data:

```yaml
        - kind: gated-commands
          ref: ADR-0039
```

  at the same list position.

### Task 5.3 — reword the doc surfaces onto the placeholder

- [ ] In `.awf/domains/parts/tooling/current-state.md`, reword the ADR-0039 sentence (line 5) that today slash-joins `sync/check/invariants/audit/list/enable/disable/new` (omitting `config`/`context`) to consume the placeholder: `A binary-version gate (ADR-0039) precedes those reads: the gated commands ({{=awf:gatedCommands}}) refuse to run against a project rendered by a *newer* awf …` — a sentence rewrite, not token substitution, so the generated list carries the correct membership.
- [ ] In `templates/docs/working-with-awf.md.tmpl`, add a `{{=awf:gatedCommands}}` row to the placeholder table inside the `awf:section placeholders` block (matching the existing rows' columns/format) — the placeholder is now part of the documented set. (This is template source; a placeholder-table row carries no `ADR-NNNN`, so the ADR-0082 residue guard is unaffected.)
- [ ] In `.awf/domains/parts/rendering/current-state.md`, add a one-line mention that the gated-command list is generated from the command spec via the `{{=awf:gatedCommands}}` placeholder — so the rendering-domain territory change (the new placeholder) has a co-changed current-state part and `awf audit`'s `domain-code-staleness` rule stays quiet.

### Task 5.4 — cover, sync, verify, commit

- [ ] Add `internal/project/gatedcommands_test.go` covering `gatedCommandsDisplay()` (assert the rendered string equals the backticked join of `clispec.GatedCommandNames()`) — this also anchors `gated-commands-generated`.
- [ ] `./x sync` — re-renders `AGENTS.md` (guide invariant), the `tooling` and `rendering` domain docs, and `working-with-awf`, and updates `.awf/awf.lock`. Confirm the rendered `AGENTS.md` binary-version-gate line now reads `Every gated command (\`sync\`, \`check\`, \`invariants\`, \`audit\`, \`list\`, \`config\`, \`context\`, \`new\`, \`enable\`, \`disable\`) … (ADR-0039)` — note the order now follows `clispec` (…`new`, `enable`, `disable`), a cosmetic change from the old hand-order.
- [ ] `./x gate` green; `./x check` clean (no drift; ADR still Proposed so the slug is not yet enforced but the marker is present).
- [ ] Commit (stage the new/edited source + its test + the three `.awf/` files + the two templates + all re-rendered docs + lock together): `internal/project/gatedcommands.go`, `internal/project/gatedcommands_test.go`, `internal/project/placeholders.go`, `internal/project/render.go`, `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`, `.awf/agents-doc.yaml`, `.awf/domains/parts/tooling/current-state.md`, `.awf/domains/parts/rendering/current-state.md`, the re-rendered `AGENTS.md`/`docs/domains/*.md`/`docs/working-with-awf.md`, and `.awf/awf.lock`. `git commit -m "feat(rendering): generate the gated-command list from clispec"`. Scope `rendering`.

---

## Phase 6 — rendering-domain doc currency; flip ADR-0094 to Implemented

### Task 6.1 — rendered-doc currency (rendering-scoped)

- [ ] `.awf/docs/parts/architecture/components.md`: add `internal/clispec` (the command-table leaf) and note the `cmd/awf` parse-once driver; update the ADR-0027 `list`/dispatch line if it references the old `switch`.
- [ ] Glossary (`.awf/docs/glossary.yaml`): add terms `command spec` (a `clispec.Command`) and `gating classification` (the `ungated`/`gated`/`gated-in-handler` enum) if warranted.
- [ ] `./x sync` (re-renders `docs/architecture.md`, `docs/glossary.md`); `./x gate` green; `./x check` clean.
- [ ] Commit (stage the two parts + re-rendered `docs/architecture.md`/`docs/glossary.md` + lock): `git commit -m "docs(rendering): note clispec dispatch in architecture and glossary"`. Scope `rendering`.

### Task 6.2 — changelog + flip status + final sync (adr-scoped)

- [ ] `changelog/CHANGELOG.md` `[Unreleased]`: add an entry under the appropriate heading (Others/Changed) — "CLI dispatch restructured onto an internal command table; resolver vocabulary renamed to enable/disable; the gated-command list is now generated." No adopter-facing behavior change; no schema bump.
- [ ] Flip `docs/decisions/0094-*.md` frontmatter `status: Proposed` → `status: Implemented` (via `awf-adr-lifecycle` semantics; single-line edit).
- [ ] `./x sync` — regenerates `docs/decisions/ACTIVE.md` (0094 now Implemented) and any domain indexes.
- [ ] `./x gate` green; `./x check` — now the two tagged slugs (`cli-command-spec-single-source`, `gated-commands-generated`) are enforced; expect `awf invariants: clean` (markers present since Phases 1 and 5).
- [ ] `./x audit` — expect no blocking findings (rendering + tooling current-state parts both co-changed in Phases 5–6, so `domain-code-staleness` stays quiet).
- [ ] Commit (stage the ADR, ACTIVE.md, changelog, lock): `git commit -m "docs(adr): mark 0094 implemented"`. Scope `adr`.

### Task 6.3 — terminal

- [ ] Invoke `awf-reviewing-impl` on the implementation commits.

## Notes

- **Gate-green-per-phase:** every phase's closing commit passes `./x gate` independently. Phase 1 introduces `clispec` and its first consumer (`main.go`) together; Phase 2's driver uses only Phase 1 types; no forward references.
- **Behavior preservation:** Phases 1–4 are refactors pinned by existing tests. The intended output changes are: (a) the generated per-command `--help` `Usage:`/`Flags:` blocks (Phase 2); (b) a **new capability** — `awf new <child> --help` now prints child-specific help instead of the whole `new` help; and (c) the generated gated-command list membership+order in the docs (Phase 5). Malformed `new <child>` invocations keep their current messages because child arity is left loose (Task 1.1) and `runNew`/`newADR`/`newLocal` still own those messages — verify the `new_test.go` assertions still pass unchanged; if any assert the whole-`new`-help text for `awf new adr --help`, update them to the child help.
- **The exemplar+site-inventory tasks (Phase 4)** are mechanical renames; the grep-zero check is the completeness proof, `./x gate` the correctness proof.
