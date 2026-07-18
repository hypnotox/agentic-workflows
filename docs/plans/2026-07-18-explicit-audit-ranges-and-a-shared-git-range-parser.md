---
date: 2026-07-18
adrs: [127]
status: Proposed
---
# Plan: Explicit Audit Ranges and a Shared Git Range Parser

## Goal

Implement ADR-0127: `awf audit` takes a required range argument, `audit.baseBranch` and
`--base` are removed behind a schema-11 migration, one range parser lives in `internal/git`
with all call sites converged on it, and the audit reports its evaluated scope so a
misplaced call cannot read as success. Non-goals: changing any audit rule's logic, changing
the `uncommitted-changes` rule's home, and adding a range argument to `awf check`.

## Architecture summary

Design and rationale live in ADR-0127; this plan is the execution record and does not
restate them. The execution shape:

- `internal/git` gains `ParseRange`, adopting `cmd/repoaudit`'s guards as the shared
  contract. It is added and immediately consumed in the same phase.
- `internal/audit.Collect` takes an explicit base and head instead of a base-branch name.
- `audit.Settings.BaseBranch`, `config.AuditConfig.BaseBranch`, its `configspec` entry, its
  `configreference` case, and the `--base` flag are removed together, because the coverage
  gate makes a read-less config field unreachable code (see Phase 3's coupling note).
- A schema-11 migration strips the key from adopter configs, needing a nested remover in
  `internal/config` since `RemoveKey` reaches only top-level entries.
- Reporting gains an always-on evaluated-scope line and a distinct empty-range notice.

## File structure

Created:
- `internal/git/parserange.go`, `internal/git/parserange_test.go`
- `internal/migrate/dropauditbase.go`, `internal/migrate/dropauditbase_test.go`

Modified:
- `internal/git/git.go`, `cmd/repoaudit/main.go`, `cmd/awf/changelog.go`
- `internal/audit/git.go`, `internal/audit/settings.go`, `internal/audit/audit.go`
- `internal/config/config.go`, `internal/config/edit.go`, `internal/config/edit_test.go`
- `internal/configspec/spec.go`, `internal/project/configreference.go`
- `internal/project/project.go`, `internal/migrate/migrate.go`
- `internal/clispec/clispec.go`, `cmd/awf/audit.go`, `cmd/awf/dispatch.go`
- `cmd/awf/run_test.go`, `cmd/awf/audit_test.go`, `internal/audit/git_test.go`
- `cmd/repoaudit/main_test.go`, `internal/configspec/spec_test.go`
- `internal/audit/settings_test.go`
- `.awf/docs/parts/development/command-runner.md`, `.awf/docs/parts/releasing/content.md`
- `.awf/agents-doc.yaml`, `templates/skills/reviewing-impl/SKILL.md.tmpl`
- `templates/docs/working-with-awf.md.tmpl`
- `.awf/domains/parts/{tooling,config,rendering}/current-state.md`
- `changelog/CHANGELOG.md`
- Hand-maintained, unreachable by `awf sync`: `x` (the runner comment), `README.md`
- `docs/decisions/0127-explicit-audit-ranges-and-a-single-git-range-parser.md` (status flip)
- Rendered outputs under `docs/`, every enabled target tree, and `examples/sundial/`

Deleted: none.

## Phase 1: One range parser, all call sites converged

- [ ] **Task 1.1: Add the shared parser.** Create
  `/home/hypno/Projects/agentic-workflows/internal/git/parserange.go`:

  ```go
  package git

  import (
  	"errors"
  	"fmt"
  	"strings"
  )

  // ParseRange resolves a range argument to an explicit base and head revision.
  // An argument containing ".." is a two-sided range; otherwise it is a base and
  // head defaults to HEAD, which callers opt into via allowBareBase (ADR-0127
  // Decision 5). Git forbids ".." inside a ref name, so the discrimination is
  // unambiguous. Rejects an empty side, a three-dot range, a multi-".." input,
  // and a "-"-prefixed side: the first three would reach git as a bogus revision
  // and the last as an option-like argument. Dots inside a revision (v0.10.0) are
  // legal, since git forbids "."-leading, ".."-containing, and "-"-leading refs.
  func ParseRange(arg string, allowBareBase bool) (base, head string, err error) {
  	if arg == "" {
  		// errors.New, not fmt.Errorf: perfsprint (.golangci.yml) rejects a
  		// constant-string fmt.Errorf and the gate would fail.
  		return "", "", errors.New("range must not be empty")
  	}
  	if !strings.Contains(arg, "..") {
  		if !allowBareBase {
  			return "", "", fmt.Errorf("range %q must be <a>..<b>", arg)
  		}
  		if strings.HasPrefix(arg, "-") {
  			return "", "", fmt.Errorf("range %q must not start with a dash", arg)
  		}
  		return arg, "HEAD", nil
  	}
  	base, head, _ = strings.Cut(arg, "..")
  	if base == "" || head == "" {
  		return "", "", fmt.Errorf("range %q must be <a>..<b>", arg)
  	}
  	if strings.HasPrefix(head, ".") || strings.Contains(head, "..") {
  		return "", "", fmt.Errorf("range %q must use exactly two dots", arg)
  	}
  	if strings.HasPrefix(base, "-") || strings.HasPrefix(head, "-") {
  		return "", "", fmt.Errorf("range %q must not start with a dash", arg)
  	}
  	return base, head, nil
  }
  ```

- [ ] **Task 1.2: Test the parser to 100%.** Create
  `/home/hypno/Projects/agentic-workflows/internal/git/parserange_test.go` with a table test
  covering, for `allowBareBase` both true and false: `""`; `"HEAD"`; `"-x"`; `"a..b"`;
  `"a..."`; `"a..b..c"`; `"..b"`; `"a.."`; `"-a..b"`; `"a..-b"`; `"v0.10.0..HEAD"`. Assert
  the exact base/head pair or a non-nil error per case. Carry the proof marker
  `// invariant: git-range-rejects-malformed` on the malformed-input test.

- [ ] **Task 1.3: Add the single-definition source scan.** In the same file, add a test
  carrying `// invariant: git-range-parser-single-definition` that fails if any line contains
  both `strings.Cut(` and `".."`. Resolve the module root by walking parent directories from
  the test's working directory until a `go.mod` is found (the test runs with cwd
  `internal/git`, so the repo root is two levels up and must not be hardcoded). Skip
  `internal/git/` (the parser's own package, which legitimately calls `strings.Cut`),
  `examples/` (a separate module with its own sources), and any `_test.go` file. It would
  fail today, so it lands with the conversions in Task 1.4, not before them.

- [ ] **Task 1.4 (batch): Converge the three existing parsers.** Representative site,
  `/home/hypno/Projects/agentic-workflows/cmd/awf/changelog.go`:

  ```diff
  -		from, to, ok := strings.Cut(rng, "..")
  -		if !ok {
  -			return &usageErr{fmt.Sprintf("awf changelog: --range must be <from>..<to>, got %q", rng)}
  -		}
  +		from, to, perr := awfgit.ParseRange(rng, false)
  +		if perr != nil {
  +			return &usageErr{fmt.Sprintf("awf changelog: --range %v", perr)}
  +		}
  ```

  Edge site, `/home/hypno/Projects/agentic-workflows/cmd/repoaudit/main.go`, which also drops
  its default base per ADR-0127 Decision 11. This is the only shape difference: the other two
  sites have no default to remove.

  ```diff
  -	rng := "origin/main..HEAD"
  -	if len(args) >= 2 {
  -		rng = args[1]
  -	}
  -	base, head, ok := strings.Cut(rng, "..")
  -	// Cut mangles a three-dot range (head "."-prefixed) or a multi-".." input
  -	// (head contains ".."); both would reach git as a bogus rev. A "-"-prefixed
  -	// side would reach git as an option-like argument. Dots inside a rev
  -	// (v0.10.0) are fine - git forbids "."-leading, ".."-containing, and
  -	// "-"-leading refs, so no valid rev is rejected.
  -	if !ok || base == "" || head == "" || strings.HasPrefix(head, ".") || strings.Contains(head, "..") ||
  -		strings.HasPrefix(base, "-") || strings.HasPrefix(head, "-") {
  -		fmt.Fprintln(stderr, "usage: repoaudit [<base>..<head>]  (default origin/main..HEAD)")
  +	if len(args) < 2 {
  +		fmt.Fprintln(stderr, "usage: repoaudit <base>..<head>")
  +		return 2
  +	}
  +	base, head, perr := awfgit.ParseRange(args[1], false)
  +	if perr != nil {
  +		fmt.Fprintf(stderr, "repoaudit: %v\n", perr)
   		return 2
   	}
  ```

  The third site is `/home/hypno/Projects/agentic-workflows/internal/git/git.go:56`, an
  in-package call: `from, to, perr := ParseRange(rangeSpec, false)`, with the `!ok` branch
  replaced by returning `perr`. Update `cmd/repoaudit/main_test.go` for the removed default:
  the no-argument case now expects exit 2 and the new usage line. Affected-site set, exactly
  the output of:

  ```
  grep -rln 'strings.Cut(.*"\.\."' --include='*.go' . | grep -v _test | grep -v '^./internal/git/'
  ```

  Post-check: that command must print nothing, and `go test ./internal/git/... ./cmd/...`
  must pass. The `internal/git/` exclusion is load-bearing, not cosmetic: `ParseRange` itself
  calls `strings.Cut(arg, "..")`, so an unfiltered grep can never reach zero.

- [ ] **Task 1.5: Update the runner doc for repoaudit's lost default.** In
  `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/development/command-runner.md`,
  change the `./x audit-local [range]` cell to `./x audit-local <range>` and replace
  ``over `<base>..<head>` (default `origin/main..HEAD`)`` with
  ``over a required `<base>..<head>``.

  Two hand-maintained sources go stale in the same commit and are not reachable by `awf sync`.
  In `/home/hypno/Projects/agentic-workflows/x`, the `audit-local)` case comment (lines
  121-123) reads "Default range origin/main..HEAD; pass `<base>..<head>` to scope it"; rewrite
  it to "Requires an explicit `<base>..<head>` range". awf disables the runner singleton for
  itself, so this file is authored, not rendered. In
  `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md:18`,
  drop the "(default `origin/main..HEAD`)" parenthetical, which this phase falsifies.

  Then run `./x sync`.

- [ ] **Task 1.6: Verify and commit.** Run:

  ```
  ./x gate && ./x check
  ```

  Both must pass. Stage exactly: `internal/git/parserange.go`,
  `internal/git/parserange_test.go`, `internal/git/git.go`, `cmd/awf/changelog.go`,
  `cmd/repoaudit/main.go`, `cmd/repoaudit/main_test.go`,
  `.awf/docs/parts/development/command-runner.md`, `docs/development.md`, `.awf/awf.lock`,
  and any re-rendered `examples/sundial/` output. Commit:

  ```commit
  refactor(tooling): converge git range parsing on internal/git
  ```

## Phase 2: Schema-11 migration removing audit.baseBranch

- [ ] **Task 2.1: Add the nested remover.** In
  `/home/hypno/Projects/agentic-workflows/internal/config/edit.go`, add `RemoveMappingKey`
  beside `SetMappingScalar`, following its nested-lookup shape via `mapValue`: it removes
  `child` from the mapping at top-level `key`, drops the now-empty `key` mapping entirely,
  and returns `src` unchanged when either is absent (idempotent for migration replay).
  Add table tests in `/home/hypno/Projects/agentic-workflows/internal/config/edit_test.go`
  covering: child present among siblings (siblings and comments preserved); child as the only
  entry (parent dropped); child absent; parent absent; malformed YAML (parse error surfaced).

- [ ] **Task 2.2: Add the migration.** Create
  `/home/hypno/Projects/agentic-workflows/internal/migrate/dropauditbase.go` with
  `applyDropAuditBase(root string, w io.Writer) error`, built on the `editConfig` skeleton
  and calling `config.RemoveMappingKey(src, "audit", "baseBranch")`. Unlike `applyDropHooks`
  it prints when it removes something (ADR-0127 Decision 7):
  `drop-audit-base: removed audit.baseBranch`, and prints nothing when the key is absent.
  Register it in `/home/hypno/Projects/agentic-workflows/internal/migrate/migrate.go`:

  ```diff
   	{To: 10, Name: "retirement-tokens", Apply: applyRetirementTokens},
  +	{To: 11, Name: "drop-audit-base", Apply: applyDropAuditBase},
   }
  ```

- [ ] **Task 2.3: Raise the schema floor.** In
  `/home/hypno/Projects/agentic-workflows/internal/project/project.go`:

  ```diff
   	10: "0.17.0",
  +	11: "0.17.0",
   }
  ```

  `Version` is already `0.17.0`, so no const bump is needed; the entry is mandatory or the
  gate fails (ADR-0049 Decision 4).

- [ ] **Task 2.4: Test the migration.** Create
  `/home/hypno/Projects/agentic-workflows/internal/migrate/dropauditbase_test.go`. The key is
  set in neither this repo's config nor `examples/sundial`, so this fixture is its only
  coverage (ADR-0127 Consequences). Cover: an `audit:` block with `baseBranch` plus siblings
  (key removed, siblings preserved, message printed); `baseBranch` as the only `audit:` child
  (whole `audit:` key dropped); no `audit:` block (no-op, nothing printed); re-run on
  already-migrated input (idempotent); absent `config.yaml` (no-op).

- [ ] **Task 2.5: Verify and commit.** Run:

  ```
  ./x gate && ./x check
  ```

  `./x check` reports the lock's `schemaVersion` moving to 11; run `./x sync` and stage the
  result. Stage exactly: `internal/config/edit.go`, `internal/config/edit_test.go`,
  `internal/migrate/dropauditbase.go`, `internal/migrate/dropauditbase_test.go`,
  `internal/migrate/migrate.go`, `internal/project/project.go`, `.awf/awf.lock`, and any
  re-rendered `examples/sundial/` output. Commit:

  ```commit
  feat(config): migrate away the audit.baseBranch key
  ```

## Phase 3: The explicit-range contract (coupled group)

**Why this phase cannot be sliced.** Removing the last read of `Settings.BaseBranch` makes
its resolve branch unreachable, which the 100% coverage gate fails; so the field, its
default, its `configspec` entry, its `configreference` case, and the `--base` flag must all
land in one commit. The `Collect` signature is threaded through `Project.Audit` and
`cmd/awf/audit.go` in the same change. This is the convention's named exception for a
signature threaded through callers plus a struct-field removal, not a convenience.

- [ ] **Task 3.1: Take an explicit range in Collect.** In
  `/home/hypno/Projects/agentic-workflows/internal/audit/git.go`, change the signature to
  `Collect(repoRoot, base, head string) ([]Commit, error)` and resolve `head` via
  `repo.ResolveRevision(plumbing.Revision(head))` instead of `repo.Head()`, keeping the
  merge-base pruning, the unrelated-histories error, and the empty-range `nil, nil` return
  unchanged. Update its doc comment to describe an explicit range.

  The base and head reach `Collect` as new `Run` parameters, not as `Inputs` fields, because
  `Inputs` embeds the `Settings` whose `BaseBranch` Task 3.2 deletes: in
  `/home/hypno/Projects/agentic-workflows/internal/audit/audit.go`, `Run(repoRoot string, in
  Inputs)` becomes `Run(repoRoot, base, head string, in Inputs)`, its line-97 call becomes
  `Collect(repoRoot, base, head)`, and its doc comment at line 76 (which lists `BaseBranch`
  among the resolved knobs the embedded `Settings` carries) drops that mention.

  Three `Run(dir, Inputs{Settings: Settings{BaseBranch: ...}})` call sites in
  `/home/hypno/Projects/agentic-workflows/internal/audit/git_test.go` (lines 178, 190, and
  304) move to the new signature in this task, or Phase 3 will not compile. The line-190 case
  keeps asserting that an unresolvable base errors, now via the explicit base argument rather
  than the configured one.

- [ ] **Task 3.2: Remove the config surface.** Delete `BaseBranch` from the `Settings`
  struct, its `"main"` default, and the `if a.BaseBranch != ""` override in
  `/home/hypno/Projects/agentic-workflows/internal/audit/settings.go`. Delete the
  `BaseBranch  string  yaml:"baseBranch"` field from `AuditConfig` in
  `/home/hypno/Projects/agentic-workflows/internal/config/config.go`. Delete the
  `audit.baseBranch` entry at `/home/hypno/Projects/agentic-workflows/internal/configspec/spec.go:158`.
  Delete the `case "audit.baseBranch":` arm at
  `/home/hypno/Projects/agentic-workflows/internal/project/configreference.go:146-147`.

  `/home/hypno/Projects/agentic-workflows/internal/audit/settings_test.go` asserts the deleted
  field in four places, each a different shape, and will not compile otherwise:

  - Lines 18-19 and 64-65: whole `if s.BaseBranch != ...` / `t.Errorf(...)` blocks. Delete
    both blocks outright.
  - Lines 40-43: a compound `if` condition whose first clause is `s.BaseBranch != "main"`,
    followed by a `t.Errorf` whose format string opens with `base=%q` and passes
    `s.BaseBranch` first. Drop the clause, the `base=%q` verb, and the argument together.
    Dropping the field without the verb fails `go vet`'s printf check, which the gate runs.
  - Line 49: a `BaseBranch: "develop",` entry in the `config.AuditConfig` composite literal.
    Delete the line.

- [ ] **Task 3.3: Change the CLI contract.** In
  `/home/hypno/Projects/agentic-workflows/internal/clispec/clispec.go`, the audit entry
  becomes:

  ```diff
  -		Name: "audit", Summary: "Report workflow-conformance findings over the branch (advisory)",
  -		ValueFlags: []string{"--base"}, MaxPos: 0, Gating: Gated,
  -		HelpBody: `Usage: awf audit [--base <ref>]
  +		Name: "audit", Summary: "Report workflow-conformance findings over a commit range (advisory)",
  +		MaxPos: 1, Gating: Gated,
  +		HelpBody: `Usage: awf audit <base>|<a>..<b>
   
  -Report advisory workflow-conformance findings over the branch's commits; never gates.
  +Report advisory workflow-conformance findings over an explicit commit range; never gates.
  +The range is required: a bare <base> means <base>..HEAD, or give a two-sided <a>..<b>.
  +There is no default range, so an audit never reports over commits nobody named.
   
  -Flags:
  -  --base <ref>   compare against <ref> instead of the configured base branch
   `,
  ```

  In `/home/hypno/Projects/agentic-workflows/cmd/awf/dispatch.go:54`:

  ```diff
  -	"audit":       func(c *cmdCtx) error { return runAudit(c.root, c.inv.values["--base"], c.stdout) },
  +	"audit":       func(c *cmdCtx) error { return runAudit(c.root, firstPos(c.inv.positionals), c.stdout) },
  ```

  In `/home/hypno/Projects/agentic-workflows/cmd/awf/audit.go`, `runAudit` takes the range
  argument, returns a `&usageErr{...}` naming both accepted forms when it is empty, and calls
  `awfgit.ParseRange(arg, true)` (bare base allowed here) before `p.Audit(base, head)`. In
  `/home/hypno/Projects/agentic-workflows/internal/project/project.go:277-282`, `Audit` takes
  `(base, head string)` and drops the `baseOverride` block.

- [ ] **Task 3.4: Update the audit command tests.**
  `/home/hypno/Projects/agentic-workflows/cmd/awf/audit_test.go` exercises all three surfaces
  this phase changes and must move with them:

  - Line 150, inside `TestRunAuditDispatch`, calls
    `run([]string{"awf", "audit", "--base", base.String()}, ...)` and expects exit 0. The flag
    is gone, so it becomes `run([]string{"awf", "audit", base.String()}, ...)`; update the
    function's doc comment at line 139-140, which says it covers "the `--base` flag plumbing",
    to say it covers the positional range argument.
  - Line 121, `runAudit(t.TempDir(), "", out(t))`, expects an error. It still errors, but now
    for a different reason: the empty range is refused before the project is opened, so it no
    longer reaches the not-a-project path. Split it into two cases, one asserting the
    empty-argument usage error (this is the natural home for the
    `// invariant: audit-requires-explicit-range` marker) and one passing a valid range
    against a non-project directory to keep the original path covered.
  - Line 129, `runAudit(root, "no-such-ref", out(t))`, still asserts an unresolvable base
    errors. A bare ref is a legal bare base under `ParseRange(arg, true)`, so it now fails at
    revision resolution rather than parsing; the assertion holds unchanged.

  The remaining `runAudit(root, base.String(), ...)` call sites need no edit: a bare commit
  hash is a valid bare base, resolving to `<hash>..HEAD`.

- [ ] **Task 3.5: Move the value-flag fixture and add the refusal tests.** In
  `/home/hypno/Projects/agentic-workflows/cmd/awf/run_test.go:145`, the "value flag without
  value" case uses `audit --base`; switch it to `context --range`, which still carries a
  ValueFlag. Add a case asserting bare `awf audit` exits non-zero with a message naming both
  forms. The proof marker for that refusal lives on the Task 3.4 case in
  `cmd/awf/audit_test.go`, so do not repeat it here. In
  `/home/hypno/Projects/agentic-workflows/internal/configspec/spec_test.go`, add a test
  asserting no spec entry, config field, or resolved setting supplies an audit base, carrying
  `// invariant: audit-no-base-branch-config`. In
  `/home/hypno/Projects/agentic-workflows/cmd/repoaudit/main_test.go`, add
  `// invariant: repoaudit-requires-explicit-range` to the no-argument case.

- [ ] **Task 3.6: Update the rendered sources.** Per ADR-0127 Decision 8. In
  `/home/hypno/Projects/agentic-workflows/templates/skills/reviewing-impl/SKILL.md.tmpl:57`,
  replace ``run `awf audit` (or this project's runner alias for it) over the branch`` with
  ``run `awf audit ${baseSha}..${headSha}` (or this project's runner alias for it) over the
  session range``. In `/home/hypno/Projects/agentic-workflows/.awf/agents-doc.yaml`, change
  the audit command description from "over the branch's commits" to "over an explicit commit
  range". In `/home/hypno/Projects/agentic-workflows/.awf/docs/parts/releasing/content.md:34`,
  change `./x gate && ./x check && ./x audit` to
  `./x gate && ./x check && ./x audit <previous-tag>..HEAD`, and state in the surrounding
  prose that the previous release tag is the base. In
  `/home/hypno/Projects/agentic-workflows/templates/docs/working-with-awf.md.tmpl:25`, replace
  "report Conventional-Commits / workflow-conformance findings over the branch (advisory)"
  with wording naming the required range, and check line 138's staleness-rule prose in the
  same pass. In
  `/home/hypno/Projects/agentic-workflows/.awf/domains/parts/tooling/current-state.md:22`,
  replace "over a branch's git history" with "over an explicit commit range".

  `/home/hypno/Projects/agentic-workflows/README.md` is hand-maintained (absent from
  `.awf/awf.lock`, so neither sync nor check reaches it): rewrite its `awf audit` row at line
  178 from `` `awf audit [--base <ref>]` `` to `` `awf audit <base>|<a>..<b>` `` and replace
  "over the branch's commits" with the explicit-range description.

  Then run `./x sync`.

- [ ] **Task 3.7: Verify and commit.** Run:

  ```
  ./x gate && ./x check
  ```

  Both must pass. Stage every file touched in Tasks 3.1-3.6 plus `.awf/awf.lock`, the
  re-rendered `docs/`, every enabled target tree, and `examples/sundial/`. Commit:

  ```commit
  feat(tooling): require an explicit range for awf audit
  ```

## Phase 4: Fail-safe reporting, and freeze

- [ ] **Task 4.1: Report the evaluated scope.** The count originates where `Collect` is
  called, inside `audit.Run`, not inside `Project.Audit`, so it is threaded out along the
  whole chain: `/home/hypno/Projects/agentic-workflows/internal/audit/audit.go`'s
  `Run(repoRoot, base, head string, in Inputs)` returns `([]Finding, int, error)`;
  `/home/hypno/Projects/agentic-workflows/internal/project/project.go`'s
  `Audit(base, head string)` returns `([]audit.Finding, int, error)`; and
  `/home/hypno/Projects/agentic-workflows/cmd/awf/audit.go` consumes both. Every terminal line
  names the
  resolved range and count: `awf audit: clean over 12 commit(s) in <base>..<head>`, and the
  warning and error lines gain the same ` over N commit(s) in <base>..<head>` suffix.
  ADR-0127 Decision 9.

- [ ] **Task 4.2: Announce an empty range.** In the same file, when the count is zero print
  `awf audit: <base>..<head> resolved to 0 commit(s); no history rule evaluated` instead of
  the clean line, then still report any range-independent findings and still return nil.
  ADR-0127 Decision 10. Add tests in
  `/home/hypno/Projects/agentic-workflows/cmd/awf/audit_test.go` asserting the exact strings
  and a zero exit, carrying `// invariant: audit-reports-evaluated-scope` and
  `// invariant: audit-empty-range-announced`.

- [ ] **Task 4.3: Fix the mis-anchored proof marker.** In
  `/home/hypno/Projects/agentic-workflows/internal/audit/git_test.go`, move
  `// invariant: audit-empty-range-clean` off `TestCollectMergeCommitCarriesNoChanges`
  (line 115) onto `TestCollectEmptyRangeIsClean` (line 165), which is the test that actually
  proves it. Pre-existing defect in a file this effort edits.

- [ ] **Task 4.4: Record the breaking change in the changelog.** This effort touches
  `cmd/awf/`, `internal/config/`, and `templates/`, all adopter-facing prefixes under
  `cmd/repoaudit`'s changelog rule, so an absent entry makes `./x audit-local` warn and leaves
  `releasecheck` nothing to promote at release time. Under the standing `## [Unreleased]`
  heading in `/home/hypno/Projects/agentic-workflows/changelog/CHANGELOG.md`, add to
  **Breaking changes**: the removed `--base` flag, the removed `audit.baseBranch` config key
  (migrated at schema 11), `awf audit`'s now-required range argument, and `./x audit-local`'s
  now-required range. Add to **Features**: the audit's evaluated-scope reporting and the
  empty-range notice. Group by adopter-facing effect per ADR-0041.

- [ ] **Task 4.5: Freeze and commit.** Flip
  `docs/decisions/0127-explicit-audit-ranges-and-a-single-git-range-parser.md` frontmatter to
  `status: Implemented`, and this plan's frontmatter to `status: Implemented`. Record
  ADR-0127 in the `tooling`, `config`, and `rendering` current-state narratives under
  `.awf/domains/parts/<domain>/current-state.md`, then run `./x sync` so `ACTIVE.md` and the
  domain indexes regenerate. Run:

  ```
  ./x gate && ./x check
  ```

  Both must pass. Commit:

  ```commit
  feat(tooling): report the audit's evaluated scope
  ```

  Then, and only then, dogfood the new contract against the now-clean tree:

  ```
  go run ./cmd/awf audit <first-commit-of-this-effort>~1..HEAD
  ```

  It must report over this effort's own commits with a scope line. This runs *after* the
  commit deliberately: `uncommitted-changes` (ADR-0025) is range-independent and always on
  (ADR-0127 Decision 4), so running it against the staged freeze would emit a branch-level
  Error and exit non-zero no matter how healthy the range is.

## Verification

- `grep -rln 'strings.Cut(.*"\.\."' --include='*.go' . | grep -v _test | grep -v '^./internal/git/'`
  prints nothing (the parser's own package is excluded; it legitimately calls `strings.Cut`).
- `go run ./cmd/awf audit` exits non-zero and names both accepted forms.
- `go run ./cmd/awf audit HEAD` exits zero and prints the 0-commit notice, not `clean`.
- `go run ./cmd/awf audit HEAD~5` prints a scope line naming the range and the count.
- `grep -rn baseBranch --include='*.go' .` returns only migration and test-fixture hits.
- `./x check` reports no drift and no invariant issues with ADR-0127 `Implemented`, proving
  all six backed slugs resolve.

## Notes

- The migration lands in Phase 2 while `AuditConfig.BaseBranch` still exists (Phase 3 removes
  it). An adopter upgrading between those two commits would lose a configured value while the
  binary still reads the field, silently falling back to `main`. Both land before any
  release, so no shipped version exhibits it.
- `awf context --range` inherits the stricter guards from Phase 1 (ADR-0127 Consequences): a
  three-dot or `-`-prefixed range that parses today begins to error there.
- `cmd/repoaudit`'s `origin/main..HEAD` default is removed in Phase 1 rather than deferred;
  its only caller, the `reviewing-impl` convention part, already passes an explicit range.
