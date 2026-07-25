---
date: 2026-07-25
adrs: [158]
status: Proposed
---
# Plan: Enforce the working-memory citation ban with a gate

## Goal

Make the cited half of the working-memory convention mechanically enforced, per
[ADR-0158](../decisions/0158-enforce-the-working-memory-citation-ban-with-a-gate.md): ship an
`internal/memorycite` detector, an `awf memory-gate` command over the staged decision-record
directories, a commit-message-body scan in `awf commit-gate`, the opt-in `memoryCite` config field
and `memoryGateCmd` var, and enable the gate in this repository with an empty exemptions list.
Non-goals: widening the scan beyond decision records and commit messages, changing the always-on
`.awf/memory/` gitignore half, and re-deciding any of the design (that lives in the ADR).

## Architecture summary

Five commits, each gate-green on its own.

Phase 1 clears the corpus first: three historical plans carry a concrete working-memory reference,
and every later commit is scanned in full once the knob is on, so the authorized rewording must
land before the gate can be enabled.

Phase 2 adds the detector and its first caller together, because the dead-code gate rejects a
package no `main` reaches. It lands `internal/memorycite`, the `memoryCite` config field with its
four configspec entries, the `awf memory-gate` command with its clispec and dispatch wiring, the
commit-gate body scan, and the domain and topic path entries for the new package.

Phase 3 adds the `memoryGateCmd` var and the two wiring points (the `x` runner's gate step and the
pre-commit hook template), plus the config-validation guard entry. This is the ADR's first
application transaction: it applies the three `update` operations with their claim mutations and
moves the ADR to `Implementing`.

Phase 4 lands the prose: the domain narrative, the gate and hook docs, the architecture entries, the
shipped command list, and the changelog.

Phase 5 flips the switch: it enables the knob in this repository, authors the new claim, adds the
two proof markers, and moves the ADR to `Implemented` with its final `add` operation, co-flipping
this plan's status.

### State-changes transaction assignment

ADR-0158 declares four operations. They split across two application transactions:

- **First batch (Phase 3, `Implementing`):** `update rendering/catalog-and-targets:var-descriptor-set-pinned`,
  `update rendering/companion-scripts:hook-payloads-fallback-safe`,
  `update config/validation:hooks-commands-resolvable`. Each becomes true in that same commit, which
  is what makes the batch honest: the var joins the pinned key set, the hook payload gains the line,
  and the guard gains the var.
- **Final batch (Phase 5, `Implemented`):** `add tooling/quality-gates:memory-citation-gate`.

Three applied and one remaining is a nonempty strict subset, so the `Implementing` status is legal.

## File structure

- **Created:**
  - `internal/memorycite/memorycite.go`, `internal/memorycite/memorycite_test.go`
  - `cmd/awf/memorygate.go`, `cmd/awf/memorygate_test.go`
- **Modified:**
  - `internal/config/config.go`, `internal/configspec/spec.go`, `internal/catalog/standard.go`
  - `internal/clispec/clispec.go`, `cmd/awf/dispatch.go`, `cmd/awf/main.go`
  - `cmd/awf/commitgate.go`, `cmd/awf/commitgate_test.go`
  - `internal/project/configreference.go`, `internal/project/configreference_test.go`
  - `internal/project/validate.go`, `internal/project/validate_test.go`
  - `internal/project/hooks_test.go`, `internal/project/descriptor_parity_test.go`,
    `internal/project/example_wiring_test.go`
  - `templates/hooks/pre-commit.sh.tmpl`, `templates/docs/working-with-awf.md.tmpl`
  - `x`, `README.md`, `changelog/CHANGELOG.md`
  - `.awf/config.yaml`, `.awf/domains/tooling.yaml`,
    `.awf/topics/metadata/tooling/quality-gates.yaml`,
    `.awf/topics/parts/tooling/quality-gates/current-state.md`,
    `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`,
    `.awf/topics/parts/rendering/companion-scripts/current-state.md`,
    `.awf/topics/parts/config/validation/current-state.md`,
    `.awf/domains/parts/tooling/current-state.md`, `.awf/agents-doc.yaml`,
    `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/architecture/data-flow.md`,
    `.awf/docs/parts/testing/gate.md`, `.awf/docs/parts/development/command-runner.md`,
    `.awf/parts/workflow/composing-the-gate.md`, `.awf/parts/workflow/local-hooks.md`
  - `docs/plans/2026-07-07-working-memory-convention.md`,
    `docs/plans/2026-07-08-anchored-globs-domain-code-staleness.md`,
    `docs/plans/2026-07-10-closed-config-tree.md`
  - `docs/decisions/0158-enforce-the-working-memory-citation-ban-with-a-gate.md`
  - every file `./x sync` regenerates from the above (AGENTS.md, `docs/config-reference.md`,
    `docs/architecture.md`, `docs/development.md`, `docs/testing.md`, `docs/workflow.md`,
    `docs/working-with-awf.md`, `docs/domains/tooling.md`, `docs/topics/**`,
    `.awf/hooks/pre-commit.sh`, `.awf/lock.yaml`, `examples/sundial/**`)
- **Deleted:** none.

### Authoring constraint for this file

This plan lives under `docs/plans/`, which the gate scans in full on every commit once the knob is
on in Phase 5. It therefore never writes a concrete filename directly after the `.awf/memory/`
prefix. Where a task must identify existing text of that shape, it gives the file and line number
and names the offending segment separately. Where a task must show a Go fixture of that shape, it
builds the string from a `dir` constant. Both are the same dodge `internal/prosegate` already uses
for banned runes, and a future editor of this file must preserve it.

## Phase 1: Clear the corpus

- [ ] **Task 1.1: Reword the three authorized concrete references in historical plans.** ADR-0158
  Context records the owner's explicit authorization to edit these frozen plans. Each edit replaces
  a concrete path segment written directly after the `.awf/memory/` prefix with a placeholder in
  angle brackets, which the detector passes. Preserve each line's illustrative intent.

  1. `docs/plans/2026-07-07-working-memory-convention.md:243`. The line runs a `git check-ignore`
     command against a made-up file whose segment is `somefile`. Replace that one segment with
     `<any-file>`, leaving the rest of the line, including the trailing exit-status clause,
     byte-identical. The point of the line is that the ignore rule covers an arbitrary file, which
     the placeholder states more clearly.
  2. `docs/plans/2026-07-08-anchored-globs-domain-code-staleness.md:725`. The line instructs the
     reader to delete that effort's working-memory file, naming it by the segment
     `domain-code-staleness-audit.md`. Replace that one segment with `<effort-slug>.md`, the
     project's standard placeholder form. The instruction reads identically.
  3. `docs/plans/2026-07-10-closed-config-tree.md:674`. The line lists two example paths that must
     produce no drift, and its intent is that any path under the directory is covered, including a
     nested one and a non-`.md` extension. Replace the first example's segment `anything.md` with
     `<any-file>.md`, and the second example's two segments `deep` and `file.awf-bak` with
     `<nested>` and `<file>.awf-bak` respectively, so the pair still demonstrates nesting and a
     non-`.md` extension.

  Do not touch `docs/plans/2026-07-07-working-memory-convention.md:411`: it names the bare directory
  through a markdown-escaped backtick, so the character after the prefix is a backslash and the
  detector passes it unchanged. Do not touch any line whose segment is `.gitignore`; the detector
  excludes that name, and the corpus carries many of them.

- [ ] **Task 1.2: Verify and commit.** Run this command and require empty output, which is the
  terminal state proving the corpus is clear of concrete references outside the excluded
  `.gitignore` name:

  ```
  grep -rnoE '\.awf/memory/[A-Za-z0-9._-][^ \t`/]*' docs/decisions/ docs/plans/ \
    | grep -v '\.gitignore$'
  ```

  Then run `./x check` (expect `awf check: clean`) and `./x gate` (expect green, ending in
  `prose-gate: clean`). Stage exactly the three plan paths and commit:

  ```commit
  docs(plans): reword three frozen working-memory references
  ```

## Phase 2: The detector, the command, and the commit-message scan

- [ ] **Task 2.1: Add the `memoryCite` config field.** In `internal/config/config.go`, add the
  field to the `Config` struct immediately after `ProseGate` (line 59), preserving the existing
  alignment of the struct-tag column:

  ```go
  	MemoryCite           *MemoryCiteConfig       `yaml:"memoryCite"`
  ```

  Then add the two types immediately after `ProseExemption` (which ends at line 304):

  ```go
  // MemoryCiteConfig configures `awf memory-gate` (ADR-0158): a scan of the
  // staged decision-record directories, and of the commit-message body, for a
  // citation of a specific working-memory file. ProseGateConfig semantics: a nil
  // *MemoryCiteConfig (key absent) and Enabled false both mean "the scan does not
  // run". The default is off because the scan blocks a commit, and a corpus that
  // has never been swept would fail it on the day it lands.
  type MemoryCiteConfig struct {
  	Enabled    bool              `yaml:"enabled"`
  	Exemptions []MemoryExemption `yaml:"exemptions"`
  }

  // MemoryExemption permits citations in one path. A nil Count permits any number
  // of them; a non-nil Count pins the expected number, so an added citation in an
  // exempt file still fails.
  type MemoryExemption struct {
  	Path  string `yaml:"path"`
  	Count *int   `yaml:"count"`
  }
  ```

  The ADR number appears here in the Go doc-comment and must not appear in the configspec
  description (invariant `config/configspec-and-reference:configspec-description-residue`).

- [ ] **Task 2.2: Add the four configspec key entries.** In `internal/configspec/spec.go`, insert
  four `Entry` values into the `keys` table immediately after the `proseGate.exemptions[].count`
  entry (which ends at line 288). Invariant
  `config/configspec-and-reference:configspec-key-parity` requires exactly one entry per struct
  leaf, and `configspec-description-residue` forbids an ADR token or a repo-identity literal in any
  description.

  ```go
  	{
  		Path: "memoryCite.enabled", Type: "bool", Default: "false (key absent)",
  		Description:  "Whether `awf memory-gate` scans, and whether `awf commit-gate` scans the commit-message body for the same thing. False, both exit zero without scanning, so a hook or a runner may invoke them unconditionally. Absent and false both mean: do not scan. Default off, because the scan blocks a commit and a corpus that has never been swept would fail it on the day it lands.",
  		Availability: "Always.",
  	},
  	{
  		Path: "memoryCite.exemptions", Type: "list of {path, count} mappings", Default: "empty (nothing is exempt)",
  		Description:  "Decision records permitted to name a specific working-memory file, typically prose that is genuinely about one particular file. An entry exempts one path. Prefer rewording to the placeholder form over adding an entry.",
  		Availability: "While `memoryCite.enabled` is true.",
  	},
  	{
  		Path: "memoryCite.exemptions[].path", Type: "string", Default: "required",
  		Description:  "The repo-relative path the exemption covers. Only a path under the decisions or plans directory can carry a finding, so only such a path is worth exempting.",
  		Availability: "While `memoryCite.enabled` is true.",
  	},
  	{
  		Path: "memoryCite.exemptions[].count", Type: "int", Default: "unset (any number is permitted)",
  		Description:  "The exact number of citations expected. Set, an added citation in an exempt file still fails, which suits a frozen record; unset, any number is permitted, which suits a living file that may gain another mention.",
  		Availability: "While `memoryCite.enabled` is true.",
  	},
  ```

- [ ] **Task 2.3: Add the live-state arms for the new keys.** In
  `internal/project/configreference.go`, add two cases to the same switch that carries the
  `proseGate` arms (lines 223 to 229), immediately after them, so the generated reference shows this
  project's live state rather than `n/a`:

  ```go
  	case "memoryCite.enabled":
  		return strconv.FormatBool(p.Cfg.MemoryCite != nil && p.Cfg.MemoryCite.Enabled)
  	case "memoryCite.exemptions":
  		if p.Cfg.MemoryCite == nil || len(p.Cfg.MemoryCite.Exemptions) == 0 {
  			return "(none)"
  		}
  		return fmt.Sprintf("%d entries", len(p.Cfg.MemoryCite.Exemptions))
  ```

  Both arms need coverage. Extend the fixture in `internal/project/configreference_test.go` (the
  config block starting at line 247, which already carries a `proseGate` block) with a `memoryCite`
  block carrying `enabled: true` and one exemption entry, and add the corresponding live-state
  assertion alongside the existing `"| 1 entries |"` one. Cover the `(none)` branch by asserting it
  for a config with the key absent, using whichever existing key-absent fixture that file builds.

- [ ] **Task 2.4: Add the `internal/memorycite` detector.** Create
  `internal/memorycite/memorycite.go`. The package is deliberately simpler than `internal/prosegate`
  in one respect: the pattern it looks for is pure ASCII, so it needs no UTF-8 validity check and no
  skipped-binary reporting.

  Required behavior, exactly:

  - A package doc-comment naming ADR-0158, stating that the package answers "does this text cite a
    specific working-memory file", and stating why the prefix is held in a constant rather than
    written inline (this file would otherwise carry the very shape the detector flags, and the
    convention is to keep the shipped surfaces free of it).
  - An unexported `const dir = ".awf/memory/"`. Every literal in this file and its test that needs
    the shape is built as `dir + "..."`; no source line writes a concrete segment directly after the
    prefix.
  - `type Reference struct { Path string; Line int; Segment string }`: one citation, with a 1-based
    line number.
  - `type File struct { Path string; Bytes []byte }`: one staged file, mirroring `prosegate.File`
    minus the mode.
  - `type Exemption struct { Path string; Count *int }`.
  - `type Finding struct { Path string; Lines []int; Pinned *int }`: one path's citations. The count
    is `len(Lines)`; `Pinned` is non-nil only when an exemption pinned a count that did not match,
    carrying the pin (which may legitimately be zero), and nil when the path was not exempt at all.
  - `func ScanText(path string, b []byte) []Reference`. Splits `b` on `"\n"`. For each line, finds
    every occurrence of `dir`, and for each occurrence examines the text immediately following it.
    Ordering: references come out in line order, and within a line in left-to-right order. The
    returned slice is nil when there is nothing to report. `path` is used verbatim as
    `Reference.Path`, so a caller with no file may pass a synthetic label.
  - An unexported `func concreteSegment(rest string) (string, bool)` implementing the whole
    discrimination rule. It reads the segment as the run of characters from the start of `rest` up
    to the first `/`, space, tab, carriage return, or backtick, or to the end of `rest`. It reports
    the segment and `true` only when all three hold: the segment's first byte is in
    `[A-Za-z0-9._-]`; the segment contains neither `<` nor `>`; and the segment is not `.gitignore`.
    Otherwise it reports `false`.
  - `func Scan(files []File, exemptions []Exemption) []Finding`. Runs `ScanText` over each file,
    groups the references by path, and applies the exemptions with `prosegate.Scan`'s three-way
    semantics: a path with no exemption and at least one reference is a finding; an exempt path with
    a nil pin is suppressed at any count; an exempt path with a non-nil pin is a finding when the
    pin does not equal the actual count, including the case where the actual count is zero and the
    pin is not (which reports a finding with empty `Lines` and the pin attached). The result is
    sorted by path.
  - `func Format(f Finding) string`. With `f.Pinned` nil, renders the path, the count, the ascending
    comma-separated line numbers, and remediation naming the placeholder and bare-directory forms.
    With `f.Pinned` non-nil, renders the path, the actual count, and the pinned count. Both messages
    must be plain ASCII and, since these are production string literals, must not carry a banned
    typographic codepoint (invariant
    `tooling/quality-gates:emitted-prose-no-typographic-substitutes`).

  Forbidden: reading the filesystem, reading git, taking a config, or exempting anything the
  `Exemption` list does not name. The package is pure: bytes in, findings out.

  Deterministic verification for this task lands in Task 2.5.

- [ ] **Task 2.5: Add the detector's table test.** Create `internal/memorycite/memorycite_test.go`
  in package `memorycite`. Declare `func ptr(n int) *int { return &n }` as `prosegate_test.go` does.
  Build every fixture as `dir + "..."` so no test line writes the flagged shape either.

  A table test over `concreteSegment` through `ScanText` must cover, at minimum, one case per rule
  branch, each asserted by whether a reference is produced and, where produced, its `Segment`:

  - `dir + "effort.md"` flags, segment `effort.md`.
  - `dir + "<effort-slug>.md"` passes: the first byte is an angle bracket, not a segment character.
  - `dir` followed by a space, by a backtick, by a backslash, and at end of input: each passes as the
    bare directory. The backslash case is the markdown-escaped form a historical plan uses.
  - `dir + ".gitignore"` passes by the explicit name exclusion, and `dir + ".gitignored.md"` flags,
    so the exclusion is an exact-name match and not a prefix match.
  - `dir + "nested/file.awf-bak"` flags with segment `nested`: a concrete first segment is a
    citation regardless of what follows it.
  - `dir + "eff<x>.md"` passes by the angle-bracket rule, which is the branch the first-byte rule
    alone does not cover.
  - Two occurrences on one line, and occurrences on different lines, asserting line numbers and
    left-to-right order.

  A second test over `Scan` must cover the exemption semantics: no exemption reports; a nil-count
  exemption suppresses at any count; a matching pin suppresses; a mismatched pin reports with
  `Pinned` set and the actual count; and a pinned path with zero actual citations reports with empty
  `Lines`. A third test asserts both `Format` branches contain the path, the counts, and, for the
  unpinned branch, the line numbers.

  Do not put an invariant proof marker in this file. The ADR places the two proof markers on the
  command-level tests that exercise the blocking path; those land in Phase 5.

- [ ] **Task 2.6: Add the `awf memory-gate` command.** Create `cmd/awf/memorygate.go` in package
  `main`, modelled on `cmd/awf/prosegate.go`. Required behavior, exactly:

  - `func runMemoryGate(root string, stdout io.Writer) error`.
  - Read the staged tree with `snapshot.IndexTree(root)`; on error return a wrapped error whose
    message contains `memory-gate: cannot read staged files`, so the command refuses outside a git
    repository rather than reporting a clean result it could not verify.
  - Look up `.awf/config.yaml` in the tree; a miss is
    `errors.New("memory-gate: staged snapshot has no .awf/config.yaml")`.
  - Parse it with `config.Parse(config.RootDir(root), stagedConfig.Bytes)` and return the parse
    error unwrapped, exactly as `runProseGate` does.
  - Return nil without scanning when `cfg.MemoryCite == nil || !cfg.MemoryCite.Enabled`.
  - Derive the scanned prefixes from the configured docs directory, never from a hardcoded `docs/`:
    take `d := strings.TrimRight(cfg.DocsDir, "/")` and scan exactly the two prefixes
    `d + "/decisions/"` and `d + "/plans/"`. `config.Parse` defaults `DocsDir` to `docs`, so this
    repository gets the ADR's two directories while an adopter with a custom `docsDir` gets theirs.
  - Build the file list from `tree.List()`, keeping a blob only when `blob.Scannable()` is true and
    its path carries one of the two prefixes. The `Scannable` guard drops a staged symlink, whose
    bytes are a target path rather than document text.
  - Map `cfg.MemoryCite.Exemptions` to `memorycite.Exemption` values. There is no fallible
    conversion here, unlike prose-gate's codepoint parse, so this loop returns no error.
  - Print each finding with `memorycite.Format` to `stdout`, one per line.
  - With at least one finding, return an error whose message names the remediation and the
    exemptions key, so a reader knows both ways out.
  - With none, print `memory-gate: clean` and return nil.

  Register the command in the three places prose-gate is registered:

  - `cmd/awf/dispatch.go`, after the `"prose-gate"` entry (line 102):
    ```go
    	"memory-gate": func(c *cmdCtx) error { return runMemoryGate(c.root, c.stdout) },
    ```
  - `cmd/awf/main.go`, extending the bypass list at line 144 to
    `case "version", "changelog", "commit-gate", "prose-gate", "memory-gate":`. A hook-wired check
    must not refuse on binary-version skew, which is why prose-gate and commit-gate are already
    there.
  - `internal/clispec/clispec.go`, a `Command` entry immediately after the `prose-gate` entry (which
    ends at line 173), with `Gating: Ungated` matching that bypass, a `Summary` no longer than its
    neighbours, and a `HelpBody` in the established `Usage: awf memory-gate` shape that states what
    is scanned, that it exits zero without scanning unless `memoryCite.enabled` is true, that
    `memoryCite.exemptions` permits a path, and that awf installs no hook.

- [ ] **Task 2.7: Add the commit-message body scan.** In `cmd/awf/commitgate.go`, inside
  `runCommitGate`, insert the scan after the successful `project.Open` (lines 36 to 40) and before
  the `audit.CheckConventionalCommit` call, so the existing early returns for an empty subject and a
  git-generated subject are untouched and a merge or autosquash message is not scanned.

  Required behavior:

  - Run only when `p.Cfg.MemoryCite != nil && p.Cfg.MemoryCite.Enabled`.
  - Call `memorycite.ScanText` over the full `raw` message bytes, not over the subject, passing a
    synthetic path: the label that will appear in the diagnostic. Use `commit message`.
  - With at least one reference, print one diagnostic line per reference to `stdout`, prefixed
    `commit-gate: ` in the style of the existing finding loop, and return an error naming the
    rejection. The commit-message scan honours no exemptions: an exemption is keyed by path, and a
    commit message has none.
  - With none, fall through to the existing Conventional Commits check unchanged.

  Update the doc-comment on `runCommitGate` so it describes both checks rather than only the
  Conventional Commits rule.

- [ ] **Task 2.8: Add the command's test file.** Create `cmd/awf/memorygate_test.go` in package
  `main`, porting the structure of `cmd/awf/prosegate_test.go`. Add a `memoryGateRepo` helper with
  the same signature and body as `proseGateRepo` but taking the `memoryCite` YAML block. Build every
  fixture body from a `dir` constant declared in this file, for the reason Task 2.5 gives.

  Cover, at minimum: the knob absent and the knob explicitly false (both no-op, nil error); a
  missing staged config and an unparseable staged config (both refuse); a clean staged corpus
  (prints `memory-gate: clean`); a staged decisions file carrying a citation (non-nil error, path
  and line in the output); a staged plans file carrying one (same); a citation in a file outside
  both scanned prefixes, asserting it is ignored, which is the path filter's proof; a custom
  `docsDir` in the staged config, asserting the scan follows it; an exemption that suppresses a
  finding; a staged symlink under the plans prefix, asserting `Scannable` drops it; the staged bytes
  controlling over a differing worktree copy, in both directions, as `prosegate_test.go` does; a
  tree outside a git repository, asserting the refusal names the enumeration failure; and the
  dispatch path through `run([]string{"awf", "memory-gate"}, ...)`.

  Extend `cmd/awf/commitgate_test.go` with cases for the body scan: the knob off leaves a citing
  body accepted; the knob on rejects a citing body and names the reference; the knob on accepts a
  clean body; and a citing body under a merge subject is still accepted, pinning the deliberate
  interaction with the existing exemption.

  Do not add proof markers yet; Phase 5 adds them.

- [ ] **Task 2.9: Give the new package a domain and a topic.** `internal/memorycite/**` must be
  added to both selectors, mirroring `internal/prosegate/**`, because a domain-owned path with no
  claim-bearing scoped topic is an `uncovered` finding and `currentState.topicCoverage` is `error`
  in this repository, which fails a gated command.

  - In `.awf/domains/tooling.yaml`, add `  - internal/memorycite/**` to the `paths` list,
    immediately after the `internal/prosegate/**` entry (line 15).
  - In `.awf/topics/metadata/tooling/quality-gates.yaml`, add `  - internal/memorycite/**` to the
    `paths` list, immediately after the `internal/prosegate/**` entry.

- [ ] **Task 2.10: Name the new package and command in the architecture components list.** In
  `.awf/docs/parts/architecture/components.md`:

  - Extend the `cmd/awf/` verb enumeration (line 4) with `memory-gate`, placed directly after
    `prose-gate`.
  - Add a bullet immediately after the `internal/prosegate/` bullet (lines 101 to 102), in the same
    two-line shape, describing `internal/memorycite/` as the detector for citations of a specific
    working-memory file in a decision record or a commit-message body, powering the opt-in blocking
    `awf memory-gate` and the commit-gate body scan, and citing ADR-0158.

- [ ] **Task 2.11: Verify and commit.** Run `./x sync`, then `./x check` (expect `awf check: clean`)
  and `./x gate` (expect green: 100% statement coverage, `deadcodecheck: no production dead code`,
  and `prose-gate: clean`). The dead-code step is the meaningful one for this phase: it is why the
  detector and its callers share a commit. Stage the exact paths this phase touched plus everything
  `./x sync` regenerated, and commit:

  ```commit
  feat(awf): add the memory-gate command and its citation detector
  ```

## Phase 3: The var, the wiring, and the first application batch

- [ ] **Task 3.1: Add the `memoryGateCmd` catalog var descriptor.** In
  `internal/catalog/standard.go`, add a descriptor immediately after the `proseGateCmd` one
  (line 225), in the identical shape:

  ```go
  		{Key: "memoryGateCmd", Kind: "string", Description: "Command that runs the working-memory citation scan (the pre-commit hook payload calls it). Leave empty to run through the rendered `./awf` wrapper (the generic `awf` when the runner singleton is disabled).", Default: "", Options: []string{"./awf memory-gate"}},
  ```

- [ ] **Task 3.2: Add its availability clause.** In `internal/configspec/spec.go`, add an entry to
  the `varAvailability` map immediately after `proseGateCmd` (line 66), preserving the map's key
  alignment:

  ```go
  	"memoryGateCmd":     "Consumed by the rendered pre-commit hook payload while the hooks singleton is enabled.",
  ```

  Invariant `config/configspec-and-reference:configspec-var-derivation` pins this map's key set to
  the config-var descriptors, so 3.1 and 3.2 must land together.

- [ ] **Task 3.3: Extend the pinned functional var-key set.** In
  `internal/project/descriptor_parity_test.go`, add `"memoryGateCmd"` to `functionalVarKeys`
  (lines 82 to 85). The comment above it (lines 77 to 81) states that extending the list is a
  successor-ADR act; update that comment to name ADR-0158 as the successor that adds this key,
  alongside its existing references to the ADR-0084 set, ADR-0119's `proseGateCmd`, and ADR-0156's
  `awfInvokeCmd`. The comment at lines 91 to 94 raises ADR-0087's seed-on-introduction contract;
  amend it to record that this addition deliberately ships no seed, with the reason ADR-0158
  Context gives: the guard added in Task 3.5 treats a present-but-empty var as unset and refuses
  identically, so a seed would change no behaviour and buy only an advisory the refusal supersedes.

  `TestVarDescriptorSetPinned` fails until this task lands, which is what forces 3.1 and 3.3 into
  one commit.

- [ ] **Task 3.4: Add the pre-commit template line.** In `templates/hooks/pre-commit.sh.tmpl`, add a
  line after the `proseGateCmd` line (line 17), on the identical `with`/`else` idiom so an unset var
  degrades to a runnable invocation with no unresolved-value token:

  ```
  {{ with .vars.memoryGateCmd }}{{ . }}{{ else }}{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} memory-gate{{ end }}
  ```

  Update `internal/project/hooks_test.go` in two places: add `tc.awf + " memory-gate\n"` to the
  `pre-commit` entry of `wantCmds` (line 69), and extend the configured-command fixture and
  expectation (lines 106 and 111) with `memoryGateCmd: ./x memory-gate` and the matching
  `./x memory-gate\n` line.

- [ ] **Task 3.5: Add the var to the hook-command resolvability guard.** In
  `internal/project/validate.go`, extend the loop's var list (line 193) to
  `[]string{"checkCmd", "commitGateCmd", "proseGateCmd", "memoryGateCmd"}`. Update the function's
  doc-comment (lines 176 to 182) so its ADR-0156 reference also names ADR-0158 as the decision that
  added the fourth var.

  Extend `internal/project/validate_test.go`: add a case asserting the new refusal names
  `vars.memoryGateCmd` (mirroring the existing `runner disabled, proseGateCmd third` case at
  line 54), and add `memoryGateCmd: make memory-gate` to the all-vars-set fixture at line 78 so that
  case still passes.

  Add `"memoryGateCmd"` to the dropped-var list in `internal/project/example_wiring_test.go`
  (line 143), keeping the example adopter's assertion that it carries no awf-verb command vars
  exhaustive.

- [ ] **Task 3.6: Set the var and the runner step in this repository.** In `.awf/config.yaml`, add
  `  memoryGateCmd: ./awf memory-gate` to the `vars:` block, keeping the block's alphabetical order
  (it sorts between `invariantTestPath` and `proseGateCmd`). In `x`, add `./awf memory-gate` to the
  `gate)` arm immediately after the `./awf prose-gate` line (line 24). The knob is still off at this
  point, so both call sites are no-ops until Phase 5; that is deliberate, and it means this phase's
  own gate run exercises the wiring without the scan.

- [ ] **Task 3.7: Apply the ADR's first batch: three claim updates.** Each claim gains
  `Revised-by: ADR-0158` (appended after any existing entries, preserving their order) and a changed
  claim text. The claim handshake requires both.

  1. `.awf/topics/parts/rendering/catalog-and-targets/current-state.md`, claim
     `var-descriptor-set-pinned` (its text is at line 98): add `memoryGateCmd` to the enumerated key
     list, in the position matching `functionalVarKeys` after Task 3.3. Append ADR-0158 to its
     `Revised-by` line, adding the line if the claim carries none.
  2. `.awf/topics/parts/rendering/companion-scripts/current-state.md`, claim
     `hook-payloads-fallback-safe` (text at line 38, `Revised-by: ADR-0156` at line 40): add
     `memoryGateCmd` to the enumerated unset-var list, and append ADR-0158 to `Revised-by`.
  3. `.awf/topics/parts/config/validation/current-state.md`, claim `hooks-commands-resolvable`
     (text at line 25): add `vars.memoryGateCmd` to the enumerated required-var list, and append
     ADR-0158 to `Revised-by`, adding the line if absent.

  Then append two entries to the ADR's `## Status history` in
  `docs/decisions/0158-enforce-the-working-memory-citation-ban-with-a-gate.md`, and set its
  frontmatter `status:` to `Implementing`:

  ```
  - 2026-07-25: Implementing; content-sha256: <digest>
  - 2026-07-25: Applied; state-sequence: <n>; operations: update `rendering/catalog-and-targets:var-descriptor-set-pinned`, update `rendering/companion-scripts:hook-payloads-fallback-safe`, update `config/validation:hooks-commands-resolvable`
  ```

  Obtain both values rather than inventing them. For `<n>`, take one more than the highest
  `state-sequence` recorded anywhere in `docs/decisions/`; find it with
  `grep -rhoE 'state-sequence: [0-9]+' docs/decisions/ | grep -oE '[0-9]+' | sort -n | tail -1`.
  For `<digest>`, write a placeholder 64-hex string, run `./x check`, and read the correct value out
  of the resulting `entry content-sha256 ... does not match the computed digest "..."` error; then
  substitute it and re-run until the check is clean. Use today's date on both entries rather than
  the date this plan carries. Do not edit any ADR section other than the frontmatter `status` and
  the appended history entries: the body is frozen the moment the ADR leaves `Proposed`.

- [ ] **Task 3.8: Verify and commit.** Run `./x sync`, then `./x check` and `./x gate`, both green.
  `./x check` is the meaningful one here: it validates the Applied event against exactly the claim
  mutations staged beside it. Stage the exact paths and commit:

  ```commit
  feat(rendering): add the memoryGateCmd var (applies 0158 batch)
  ```

## Phase 4: Documentation

- [ ] **Task 4.1: Extend the tooling domain narrative.** In
  `.awf/domains/parts/tooling/current-state.md`, the paragraph at line 32 describes `awf commit-gate`
  and then `awf prose-gate` as the blocking counterparts to the advisory audit rules. Append prose
  describing `awf memory-gate` in the same register and at the same level of detail: what it scans
  (the staged decision-record directories, derived from the configured docs directory), what the
  detector discriminates (a concrete segment flags; the placeholder and bare-directory forms pass;
  the ignore-file name is excluded), that `commit-gate` runs the same detector over the message body,
  that it is opt-in and default-off and self-gates on `memoryCite.enabled` so a hook may invoke it
  unconditionally, that it is `Ungated` like its two siblings, that it refuses outside a git
  repository, and that this repository enables it and wires it into both `./x gate` and the rendered
  pre-commit payload, scanning twice by the same accepted design prose-gate already carries. Cite
  ADR-0158. Write the concrete-file shape only in placeholder form, per this plan's authoring
  constraint.

- [ ] **Task 4.2: Add the invariant to the agent guide.** In `.awf/agents-doc.yaml`, add an entry to
  the same `Invariants` list that carries the plain-punctuation entry (line 30), placed after it.
  Follow that entry's shape: a bold lead-in naming the rule, the mechanism in one or two sentences,
  the authoring escape (name the file separately from the prefix, or use the placeholder), and a
  trailing ADR reference. State that the ban covers an ADR, a plan, and a commit-message body, and
  that the gate is on in this repo.

- [ ] **Task 4.3: Update the gate and hook prose.** Four parts, each a small addition beside its
  existing prose-gate mention, keeping each file's established sentence shape:

  - `.awf/parts/workflow/composing-the-gate.md` line 7: add the citation scan to the enumerated gate
    steps, after the plain-punctuation scan, with its ADR reference and its opt-in status.
  - `.awf/parts/workflow/local-hooks.md` line 3: add `./awf memory-gate` to the pre-commit payload's
    described command sequence and `memoryGateCmd` to the enumerated var list.
  - `.awf/docs/parts/testing/gate.md` line 9: add the citation scan beside the plain-punctuation
    scan in the gate-tier description.
  - `.awf/docs/parts/development/command-runner.md` line 10: add the citation scan to the `./x gate`
    row's enumerated steps, after the plain-punctuation scan.

- [ ] **Task 4.4: Update the architecture data-flow note.** In
  `.awf/docs/parts/architecture/data-flow.md`, line 54 states that `awf prose-gate` reads the staged
  files it scans through the immutable `internal/snapshot` index. Extend that statement to cover
  `awf memory-gate`, which reads its staged blobs the same way and additionally path-filters them to
  the decision-record directories.

- [ ] **Task 4.5: Add the command to the shipped and adopter-facing command lists.** In
  `templates/docs/working-with-awf.md.tmpl`, add a bullet after the `awf prose-gate` bullet
  (line 42), in the identical shape: what it scans, that it exits non-zero on any finding, that it
  is opt-in via `memoryCite.enabled` and default off, and that a pre-commit hook uses it. In
  `README.md` (hand-written, not rendered), add a row to the command table after the
  `awf prose-gate` row (line 281), matching its two-column shape and terseness.

- [ ] **Task 4.6: Add the changelog entry.** In `changelog/CHANGELOG.md`, under `## [Unreleased]`,
  add a `### Features` section if absent and one entry describing the adopter-facing effect: the new
  `awf memory-gate` command and the commit-gate body scan, both opt-in via the new
  `memoryCite.enabled` knob and both off by default; the new `memoryGateCmd` var and the new
  pre-commit payload line, which re-renders on the adopter's next `awf sync`; and the one breaking
  edge, that a project with the hooks singleton enabled and the runner singleton explicitly disabled
  must now set `memoryGateCmd` before `awf sync` or `awf check` will pass. Categorise by
  adopter-facing effect, not by commit type, per the file's own header.

- [ ] **Task 4.7: Verify and commit.** Run `./x sync`, then `./x check` and `./x gate`, both green.
  Confirm the rendered AGENTS.md, `docs/workflow.md`, `docs/testing.md`, `docs/development.md`,
  `docs/architecture.md`, `docs/working-with-awf.md`, and `docs/domains/tooling.md` all carry the
  new prose, since each is regenerated from a part this phase edited. Stage the exact paths and
  commit:

  ```commit
  docs(awf): document the memory gate
  ```

## Phase 5: Enable the gate and apply the final batch

- [ ] **Task 5.1: Enable the knob in this repository.** In `.awf/config.yaml`, add a `memoryCite`
  block after the `proseGate` block (which ends at line 220) and before `workflowTelemetry`:

  ```yaml
  memoryCite:
    enabled: true
  ```

  Omit `exemptions` entirely: ADR-0158 requires this repository to reach an exemption-free state,
  which Phase 1 established.

- [ ] **Task 5.2: Author the new claim.** In
  `.awf/topics/parts/tooling/quality-gates/current-state.md`, insert a claim block in the file's
  existing alphabetical-by-slug order, which places `memory-citation-gate` between
  `example-zero-notes` and `mutants-timeout-untrusted`:

  ```markdown
  ### `invariant: memory-citation-gate`

  With memoryCite.enabled true, the memory-gate command reports every concrete working-memory file reference in the staged decisions and plans directories and exits non-zero on any finding outside memoryCite.exemptions, and the commit-gate command does the same for the commit-message body; a reference written in the angle-bracket placeholder form, one naming the bare directory, and the ignore-file name all pass.
  Origin: ADR-0158
  Backing: test
  ```

  Match the file's existing claim-block shape exactly: an H3 carrying the backticked
  `invariant: <slug>` form, a blank line, one paragraph of claim text on a single line, then the
  metadata lines in the order `Origin`, `Backing`. Do not write the prefix followed by a concrete
  segment anywhere in this block.

- [ ] **Task 5.3: Add the two proof markers.** ADR-0158 places the backing on the command-level
  tests that exercise the blocking path. Add the comment line
  `// invariant: tooling/quality-gates:memory-citation-gate` immediately above each of:

  - the `cmd/awf/memorygate_test.go` test that asserts a staged decisions or plans blob carrying a
    citation makes `runMemoryGate` return a non-nil error;
  - the `cmd/awf/commitgate_test.go` test that asserts a citing commit-message body with the knob on
    makes `runCommitGate` return a non-nil error.

  Both files match `currentState.testGlobs` (`**/*_test.go`), which is what makes them legal proof
  sites. Add no third marker: `internal/memorycite/memorycite_test.go` carries the discrimination
  table, which the claim describes but does not take backing from.

- [ ] **Task 5.4: Apply the ADR's final batch and flip both statuses.** In
  `docs/decisions/0158-enforce-the-working-memory-citation-ban-with-a-gate.md`, append two entries
  to `## Status history` and set the frontmatter `status:` to `Implemented`:

  ```
  - 2026-07-25: Applied; state-sequence: <n>; operations: add `tooling/quality-gates:memory-citation-gate`
  - 2026-07-25: Implemented; content-sha256: <digest>
  ```

  `<n>` is the next sequence after Phase 3's, obtained the same way. `<digest>` is the same value
  Phase 3 recorded on the `Implementing` entry, because the body has not changed since; verify that
  by running `./x check` and requiring a clean result rather than assuming it. Use today's date on
  both entries. In this plan file, set the frontmatter `status:` to `Implemented`.

- [ ] **Task 5.5: Verify and commit.** Run `./x sync`, then `./x check` and `./x gate`.

  Both are load-bearing here and neither has run under these conditions before, because this is the
  first run with the scan active. Expect `./x gate` to end with both `prose-gate: clean` and
  `memory-gate: clean`. If `memory-gate` reports a finding, the fix is to reword the offending line
  to the placeholder form, never to add an exemption: ADR-0158 requires this repository to reach an
  exemption-free state. Expect `./x check` to validate the final Applied event against exactly the
  one claim added beside it.

  Stage the exact paths and commit. This commit carries both status flips.

  ```commit
  feat(awf): enforce the working-memory citation ban (implements 0158)
  ```

## Verification

The effort is done when all of the following hold:

- `./x gate` is green and its output ends with both `prose-gate: clean` and `memory-gate: clean`.
- `./x check` reports `awf check: clean`, with ADR-0158 at `Implemented` and all four declared
  operations Applied across the two recorded batches.
- This command returns empty output, proving the corpus carries no concrete reference outside the
  excluded ignore-file name:
  ```
  grep -rnoE '\.awf/memory/[A-Za-z0-9._-][^ \t`/]*' docs/decisions/ docs/plans/ \
    | grep -v '\.gitignore$'
  ```
- `.awf/config.yaml` carries `memoryCite.enabled: true` and no `memoryCite.exemptions` key.
- A deliberate negative check: temporarily add a line naming a concrete working-memory file to any
  file under `docs/plans/`, stage it, and confirm `./awf memory-gate` exits non-zero and names the
  path and line. Revert it before committing anything.
- `git log --oneline` shows five commits carrying the subjects this plan fences, in order.

## Notes

- **Two details the ADR leaves to implementation, resolved here.** First, the ADR names
  `docs/decisions/**` and `docs/plans/**`, but `docsDir` is a configurable key defaulting to `docs`,
  and the decisions and plans directories derive from it throughout `internal/project`. Task 2.6
  derives the prefixes from `cfg.DocsDir`, which yields the ADR's literal paths in this repository
  and the correct ones for an adopter with a custom `docsDir`. Second, the ADR states the knob for
  the `memory-gate` command but is silent on whether the commit-gate body scan honours it; Task 2.7
  gates both on `memoryCite.enabled`, which is the only reading consistent with the field being
  opt-in and with the ADR's own Consequences.
- **The commit-gate body scan sits after the git-generated-subject exemption**, so a merge or
  autosquash message is not scanned. This is a deliberate, minimal-diff choice: it preserves every
  existing early return, including the one that lets `commit-gate` return nil outside an adopted
  tree, and a git-generated body will not carry the pattern. Task 2.8 pins the behaviour with a test
  so it is a decision on the record rather than an accident.
- **`internal/memorycite` deliberately omits prose-gate's UTF-8 validity check** and its
  skipped-binary reporting. The pattern it matches is pure ASCII, so byte-wise scanning of arbitrary
  input is safe, and the `Scannable` filter in the command already drops the one non-document blob
  shape that reaches it.
- **No seeding migration accompanies `memoryGateCmd`**, per ADR-0158 Context. Task 3.3 records the
  reasoning at the exact test comment that raises the contract, so the next person to extend the
  pinned var set finds the decision rather than re-litigating it.
- **Out of scope, worth a later look:** the `awf audit` advisory surface gains no corresponding
  rule, unlike plain punctuation, which has both an advisory net-increase rule and a presence gate.
  ADR-0158 chose blocking-only deliberately. If the asymmetry proves confusing in practice, an
  advisory rule is an additive follow-up, not a change to this decision.
