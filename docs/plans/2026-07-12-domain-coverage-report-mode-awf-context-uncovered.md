---
date: 2026-07-12
adrs: [102]
status: Proposed
---
# Plan: Domain-Coverage Report Mode: awf context --uncovered

Implements ADR-0102: an `--uncovered` mode on `awf context` that reports git-tracked
paths owned by no configured domain glob, so adopters get an on-demand signal for
where domains are missing. The design lives in `docs/decisions/0102-domain-coverage-report-mode-via-awf-context-uncovered.md`; this plan is the execution record.

## Goal

Add `awf context --uncovered [<scan-root>...]`: list git-tracked-at-HEAD paths matched
by no domain `paths` glob, collapsing a fully-uncovered directory to its topmost node,
with human and `--json` output from one assembled result — reusing `awf context`'s
read-only / static-fallback contracts.

## Architecture summary

- `internal/git` gains a read-only `TrackedPaths(repoRoot)` that walks the HEAD tree.
- `internal/project` gains `Uncovered(tracked, scanRoots)` returning an `UncoveredResult`
  — a pure function of the tracked set and the project's domain config (no git import),
  matching how `ContextFor` already takes its paths as input. It filters to the scan
  roots (path-segment boundaries), drops domain-owned paths, and collapses
  fully-uncovered directories.
- `cmd/awf` adds an `--uncovered` branch to `runContext` (resolving the tracked set via
  `git.TrackedPaths`, calling `Uncovered`, rendering via a new `printUncovered`) and
  registers the flag in `internal/clispec`.
- The three invariants are marked in source; the ADR and this plan flip to Implemented.

Coupling note: the dead-code gate (ADR-0063) requires every production function be
reachable from a `main`, so `git.TrackedPaths`, `project.Uncovered`, and the `runContext`
wiring cannot land in separate commits — Phase 1 is one commit by necessity. Phase 2 is
the status-flip.

## Tech stack

Go 1.26. Packages touched: `internal/git` (go-git tree walk), `internal/project`
(assembly + `internal/pathglob`), `cmd/awf` (dispatch + rendering), `internal/clispec`
(command spec), and `docs/working-with-awf.md`. Tests use `internal/testsupport/gitfixture`.
Gate: `./x gate` before every commit (100% coverage, deadcode, pincheck).

## File structure

- **Created:** `.awf/parts/working-with-awf/commands.md` (repo-local doc-section
  override documenting the mode).
- **Modified:** `internal/git/git.go`, `internal/git/git_test.go`,
  `internal/project/context.go`, `internal/project/context_test.go`,
  `cmd/awf/context.go`, `cmd/awf/dispatch.go`, `cmd/awf/context_test.go`,
  `internal/clispec/clispec.go`, `internal/clispec/clispec_test.go` (if flag-set assertions exist),
  `docs/working-with-awf.md`, `docs/decisions/0102-domain-coverage-report-mode-via-awf-context-uncovered.md`,
  `docs/decisions/ACTIVE.md`, this plan, `.awf/awf.lock`.
- **Deleted:** none.

## Phase 1 — Implement `--uncovered` end-to-end (one commit; deadcode-coupled)

- [ ] **Task 1.1 — Add `git.TrackedPaths`.** Append to `internal/git/git.go` (after
  `ChangedPaths`, before `treeAt`). `object` is already imported.

  ```go
  // TrackedPaths returns the sorted, unique repo-relative slash paths tracked at
  // HEAD. It reads the repository only.
  func TrackedPaths(repoRoot string) ([]string, error) {
  	repo, err := OpenRepo(repoRoot)
  	if err != nil {
  		return nil, fmt.Errorf("open repo: %w", err)
  	}
  	ref, err := repo.Head()
  	if err != nil {
  		return nil, fmt.Errorf("resolve HEAD: %w", err)
  	}
  	c, err := repo.CommitObject(ref.Hash())
  	if err != nil { // coverage-ignore: HEAD resolved above points at a real commit
  		return nil, err
  	}
  	tree, err := c.Tree()
  	if err != nil { // coverage-ignore: a resolved commit always yields its tree
  		return nil, err
  	}
  	var out []string
  	if ferr := tree.Files().ForEach(func(f *object.File) error {
  		out = append(out, f.Name)
  		return nil
  	}); ferr != nil { // coverage-ignore: the collector callback never returns an error
  		return nil, ferr
  	}
  	sort.Strings(out)
  	return out, nil
  }
  ```

- [ ] **Task 1.2 — Add `project.Uncovered` + `UncoveredResult`.** Append to
  `internal/project/context.go` (after `ContextFor`, before `normalizeContextPaths`).
  `sort`, `strings`, `path` semantics via `strings`, and `pathglob` are available;
  add `"strings"` usage (already imported) — no new imports.

  ```go
  // UncoveredResult is the read-only domain-coverage report for a set of scan roots:
  // the git-tracked paths matched by no configured domain glob, with a fully-uncovered
  // directory collapsed to its topmost node (a trailing-slash entry). ScanRoots echoes
  // the requested roots (empty = whole repository).
  type UncoveredResult struct {
  	ScanRoots []string `json:"scanRoots"`
  	Entries   []string `json:"entries"`
  }

  // Uncovered assembles the domain-coverage report over the tracked paths. It writes
  // nothing and reads only the domain sidecars. scanRoots restrict the report to
  // tracked paths at or beneath them, matched on slash-separated segment boundaries
  // (a directory subtree), not raw string prefixes; empty scanRoots scans everything.
  // invariant: uncovered-lists-unowned-only
  // invariant: uncovered-collapses-directories
  func (p *Project) Uncovered(tracked, scanRoots []string) (UncoveredResult, error) {
  	roots := NormalizeContextPaths(scanRoots)
  	res := UncoveredResult{ScanRoots: roots}

  	// Domain glob set, once.
  	var globs []string
  	for _, d := range p.Cfg.Domains {
  		sc, err := p.Cfg.Sidecar("domains", d)
  		if err != nil {
  			return UncoveredResult{}, err
  		}
  		globs = append(globs, sc.Paths...)
  	}

  	inScope := func(path string) bool {
  		if len(roots) == 0 {
  			return true
  		}
  		for _, r := range roots {
  			if path == r || strings.HasPrefix(path, r+"/") {
  				return true
  			}
  		}
  		return false
  	}
  	covered := func(path string) bool {
  		for _, g := range globs {
  			if pathglob.Match(g, path) {
  				return true
  			}
  		}
  		return false
  	}

  	// coveredDirs: every ancestor directory (including "." root) of an in-scope
  	// covered tracked path, plus each scan root's strict ancestors so a collapse
  	// never climbs above a requested root. A directory absent here has no covered
  	// descendant within scope.
  	coveredDirs := map[string]bool{}
  	for _, r := range roots {
  		for _, a := range ancestors(r) {
  			coveredDirs[a] = true
  		}
  	}
  	var uncovered []string
  	for _, path := range tracked {
  		clean := filepath.ToSlash(filepath.Clean(path))
  		if !inScope(clean) {
  			continue
  		}
  		if covered(clean) {
  			for _, a := range ancestors(clean) {
  				coveredDirs[a] = true
  			}
  			continue
  		}
  		uncovered = append(uncovered, clean)
  	}

  	// Collapse each uncovered path to its topmost ancestor that has no covered
  	// descendant; a path all of whose ancestors are covered-adjacent reports itself.
  	entries := map[string]bool{}
  	for _, u := range uncovered {
  		pick := u
  		for _, a := range ancestors(u) {
  			if !coveredDirs[a] {
  				if a == "." {
  					pick = "."
  				} else {
  					pick = a + "/"
  				}
  				break
  			}
  		}
  		entries[pick] = true
  	}
  	for e := range entries {
  		res.Entries = append(res.Entries, e)
  	}
  	sort.Strings(res.Entries)
  	return res, nil
  }

  // ancestors returns path's directory ancestors from the top down — "." then each
  // strict directory prefix — excluding path itself.
  func ancestors(path string) []string {
  	out := []string{"."}
  	segs := strings.Split(path, "/")
  	for i := 1; i < len(segs); i++ {
  		out = append(out, strings.Join(segs[:i], "/"))
  	}
  	return out
  }
  ```

  Also **export** the existing `normalizeContextPaths` by renaming it to
  `NormalizeContextPaths` (unchanged body), and update its two callers —
  `ContextFor` (line ~69, `clean := NormalizeContextPaths(paths)`) and the new
  `Uncovered` above. The CLI static fallback (Task 1.3) also calls it.

- [ ] **Task 1.3 — Wire the `--uncovered` mode into `runContext` + add `printUncovered`.**
  In `cmd/awf/context.go`: add `uncovered bool` to `runContext`'s signature (after
  `asJSON`), branch on it before the existing path/selector logic, and add the
  renderer. Exact new signature line:

  ```go
  func runContext(cwd string, paths []string, staged bool, rng string, asJSON, uncovered bool, stdout io.Writer) error {
  ```

  Immediately after the `func runContext(...) {` line, insert the mode branch:

  ```go
  	if uncovered {
  		return runUncovered(cwd, paths, staged, rng, asJSON, stdout)
  	}
  ```

  Add these functions after `runContext` (before `printContext`):

  ```go
  // runUncovered serves `awf context --uncovered`: the whole-tree inverse of the
  // domain-ownership resolution. Positional args are optional scan roots; --staged and
  // --range are rejected. It mirrors runContext's read-only + static-fallback shape.
  // invariant: context-read-only
  func runUncovered(cwd string, scanRoots []string, staged bool, rng string, asJSON bool, stdout io.Writer) error {
  	if staged || rng != "" {
  		return &usageErr{"awf context --uncovered takes optional scan-root paths, not --staged/--range"}
  	}
  	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
  		if !errors.Is(err, fs.ErrNotExist) {
  			return err
  		}
  		// invariant: context-static-fallback
  		return printUncovered(stdout, project.UncoveredResult{ScanRoots: project.NormalizeContextPaths(scanRoots)}, asJSON,
  			"context --uncovered (static — not inside an awf project; live coverage appears inside one)")
  	}
  	if err := gate(cwd); err != nil {
  		return err
  	}
  	p, err := project.Open(cwd)
  	if err != nil {
  		return err
  	}
  	tracked, err := awfgit.TrackedPaths(cwd)
  	if err != nil {
  		return err
  	}
  	res, err := p.Uncovered(tracked, scanRoots)
  	if err != nil {
  		return err
  	}
  	return printUncovered(stdout, res, asJSON, "context --uncovered — tracked paths owned by no domain")
  }

  // printUncovered renders res as JSON or human-readable text. Both modes read the same
  // assembled res, so they cannot diverge.
  // invariant: uncovered-output-parity
  func printUncovered(stdout io.Writer, res project.UncoveredResult, asJSON bool, header string) error {
  	if asJSON {
  		enc := json.NewEncoder(stdout)
  		enc.SetIndent("", "  ")
  		return enc.Encode(res)
  	}
  	fmt.Fprintln(stdout, header)
  	if len(res.ScanRoots) > 0 {
  		fmt.Fprintf(stdout, "\nscan roots: %v\n", res.ScanRoots)
  	}
  	if len(res.Entries) == 0 {
  		fmt.Fprintln(stdout, "\nall scanned tracked paths are owned by a domain")
  		return nil
  	}
  	fmt.Fprintln(stdout, "\n## Uncovered (configure a domain to own these)")
  	for _, e := range res.Entries {
  		fmt.Fprintf(stdout, "  %s\n", e)
  	}
  	return nil
  }
  ```

- [ ] **Task 1.4 — Pass the flag from dispatch.** In `cmd/awf/dispatch.go`, update the
  `context` handler (line ~58) to pass the new flag:

  ```go
  	"context": func(c *cmdCtx) error {
  		return runContext(c.root, c.inv.positionals, c.inv.bools["--staged"], c.inv.values["--range"], c.inv.bools["--json"], c.inv.bools["--uncovered"], c.stdout)
  	},
  ```

- [ ] **Task 1.5 — Register `--uncovered` in the clispec + refresh help.** In
  `internal/clispec/clispec.go`, edit the `context` command: add `--uncovered` to
  `BoolFlags` and extend `HelpBody`. Exact `BoolFlags` change:

  ```go
  		BoolFlags: []string{"--json", "--staged", "--uncovered"}, ValueFlags: []string{"--range"}, MaxPos: -1, Gating: GatedInHandler,
  ```

  Update the `HelpBody` `Usage:` line to advertise the flag:

  ```
  Usage: awf context <path>... [--json] [--staged] [--range <a>..<b>] [--uncovered]
  ```

  Extend the `HelpBody` string: after the "Explicit paths take precedence…" paragraph
  and before the `Flags:` block, insert a paragraph, and add the flag to the list:

  ```
  With --uncovered, ignore the path lookup and instead report git-tracked-at-HEAD
  paths matched by no configured domain glob — the coverage-gap signal for where to
  add a domain. Positional args become optional scan roots (a directory subtree);
  --staged/--range are not accepted in this mode.
  ```

  and in the `Flags:` list add:

  ```
    --uncovered          report tracked paths owned by no domain (scan roots optional)
  ```

- [ ] **Task 1.6 — Document the mode via a source part (never the generated doc).**
  `docs/working-with-awf.md` is generated (`GENERATED by awf` banner) — hand-editing it
  fails the drift oracle. Its `commands` section is overridable (marker at line ~18:
  `create .awf/parts/working-with-awf/commands.md to override`). Create
  `.awf/parts/working-with-awf/commands.md` that re-injects the section default and
  appends the mode note:

  ```markdown
  {{=awf:sectionDefault}}

  `awf context --uncovered [<scan-root>...]` reports git-tracked paths owned by no
  domain — the signal for where to configure a new domain.
  ```

  Then `./x sync` re-renders `docs/working-with-awf.md` (awf's own tree only — the part
  is repo-local, so `examples/sundial` is untouched). Commit the part and the
  re-rendered doc together in Task 1.10. (Note for the ADR resync: ADR-0102 says "update
  the `awf context` entry"; this realizes it as an appended mode note rather than an
  in-place edit of the existing bullet, to stay repo-local and off the shared template.)

- [ ] **Task 1.7 — Tests: `git.TrackedPaths`.** Add to `internal/git/git_test.go`:
  - `TestTrackedPathsListsHeadTreeSorted`: `gitfixture.InitRepo`; `Commit` writing
    `{"b.txt":"1","a/c.txt":"2","a/d.txt":"3"}`; assert `TrackedPaths(dir)` returns
    exactly `["a/c.txt","a/d.txt","b.txt"]` (sorted, slash-separated).
  - `TestTrackedPathsNoHeadErrors`: `InitRepo` with no commit; assert `TrackedPaths(dir)`
    returns an error mentioning `resolve HEAD`.
  - `TestTrackedPathsBadRepoErrors`: call `TrackedPaths(t.TempDir())` (not a repo); assert
    an error mentioning `open repo`.

- [ ] **Task 1.8 — Tests: `project.Uncovered`.** Add `TestUncovered` to
  `internal/project/context_test.go`, a table over one project fixture whose domains
  own `internal/render/**` and the single file `x`. Build the `Project` with the
  existing test-project helper used by `TestContextFor` (mirror its construction).
  Cases (tracked set → expected `Entries`):
  1. **collapse fully-uncovered subtree:** tracked `internal/render/r.go`,
     `internal/plan/p.go`, `internal/plan/q.go` → `["internal/plan/"]` (render covered
     and absent from output; plan subtree collapsed).
  2. **stray top-level file reported individually:** tracked `x`, `README.md` →
     `["README.md"]` (`x` covered; `README.md` a top-level file, not collapsed to `.`).
  3. **mixed dir reports files individually:** domains own the single file
     `cmd/awf/main.go`; tracked `cmd/awf/main.go`, `cmd/awf/other.go` →
     `["cmd/awf/other.go"]`.
  4. **zero covered → collapse to root:** a project with no domains; tracked
     `a/b.go`, `c.go` → `["."]`.
  5. **scan-root segment boundary:** domains own nothing; tracked `internal/git/g.go`,
     `internal/gitlab/h.go`; `scanRoots=["internal/git"]` → `["internal/git/"]` (the
     `internal/gitlab` sibling is out of scope, proving segment-boundary matching, not
     raw-prefix).
  6. **sidecar fault (error branch):** mirroring `TestContextForReaderFaults`, write a
     malformed `domains/<x>.yaml` (`paths: [\n`) for a configured domain, then assert
     `p.Uncovered(tracked, nil)` returns an error — covering the `Sidecar` error return
     in the domain loop.
  Assert `res.ScanRoots` echoes the normalized roots for case 5.

- [ ] **Task 1.9 — Tests: `runContext --uncovered` CLI.** Add to `cmd/awf/context_test.go`,
  mirroring the existing `runContext` tests' harness (adopted-tree temp project +
  git init):
  - `TestRunContextUncoveredHuman`: in an adopted tree with a domain owning
    `internal/render/**`, a committed uncovered file `internal/plan/p.go`; run
    `runContext(dir, nil, false, "", false, true, buf)`; assert stdout contains
    `## Uncovered` and `internal/plan/`.
  - `TestRunContextUncoveredJSONParity`: same setup, `asJSON=true`; decode stdout into
    `project.UncoveredResult`; assert its `Entries` equals the human run's listed
    entries (parity).
  - `TestRunContextUncoveredRejectsSelectors`: `runContext(dir, nil, true, "", false, true, buf)`
    returns a `*usageErr` mentioning `--staged/--range`.
  - `TestRunContextUncoveredStaticFallback`: a non-adopted `t.TempDir()`;
    `runContext(dir, nil, false, "", false, true, buf)` prints the static header and
    returns nil.
  - **Error-branch coverage** (`runUncovered` has five reachable error returns; each
    needs a test or the 100% gate drops — mirror the existing `runContext` fault tests):
    - stat non-ErrNotExist fault: point `cwd` at a path whose `.awf` stat errors
      non-`ErrNotExist` (mirror `TestRunContextStatFault`), `uncovered=true` → error.
    - gate refusal: an adopted tree whose lock fails the version gate (mirror
      `TestRunContextGated`), `uncovered=true` → error.
    - `project.Open` error: an adopted tree with a corrupt config (mirror
      `TestRunContextOpenError`), `uncovered=true` → error.
    - `TrackedPaths` fault: an adopted `.awf/` tree that is **not a git repo** (no
      `.git`) → `awfgit.TrackedPaths` errors → `runContext(...,uncovered=true)` returns
      it.
    - `p.Uncovered` fault: an adopted tree with a configured domain whose sidecar is
      malformed (mirror the Task 1.8 case 6 setup at the CLI layer) → error.
  Update every existing `runContext(...)` call in this test file to pass the new
  `uncovered` arg (`false`) — post-check: `grep -c "runContext(" cmd/awf/context_test.go`
  equals the count of updated call sites (no arity mismatch; `go build` is the real gate).

- [ ] **Task 1.10 — Verify and commit.** Run `./x sync` (regenerates nothing yet —
  ADR text unchanged — but refresh in case help text feeds a doc), then `./x gate`
  (expect `coverage: 100.0%`, `0 issues.`, `deadcodecheck: no production dead code`,
  `pincheck: all workflow references pinned`). `git add` the modified source, test,
  and clispec files, the new `.awf/parts/working-with-awf/commands.md`, the re-rendered
  `docs/working-with-awf.md`, and `.awf/awf.lock`. Commit:
  `feat(tooling): awf context --uncovered domain-coverage report`. Body notes the
  deadcode coupling that forces the single commit and lists the three marked invariants.

## Phase 2 — Flip ADR-0102 and this plan to Implemented (final commit)

- [ ] **Task 2.1 — Flip statuses + regenerate.** In
  `docs/decisions/0102-domain-coverage-report-mode-via-awf-context-uncovered.md` set
  `status: Implemented`. In this plan set `status: Implemented` and record any
  implementation deltas in Notes. Run `./x sync` to regenerate
  `docs/decisions/ACTIVE.md`. Run `./x gate` and `./x check` — check now enforces the
  three slugs are backed (they were marked in Phase 1): expect `awf check: clean`,
  `awf invariants: clean`.

- [ ] **Task 2.2 — Verify and commit.** `git add` the ADR, this plan, `ACTIVE.md`, and
  `.awf/awf.lock`. Commit: `docs(adr): implement 0102 — flip status`. This is the
  final commit; the plan freezes here.

## Verification

- `awf context --uncovered` in this repo lists the currently-unowned `internal/*`
  packages (e.g. `internal/project/`, `internal/plan/`, `internal/git/`), confirming
  the dogfood value that motivates the follow-on domain-coverage work.
- `awf context --uncovered internal/git` lists only entries under `internal/git`,
  never `internal/gitlab`-style siblings (segment-boundary scan-root).
- `awf context --uncovered --json` decodes to an `UncoveredResult` whose `Entries`
  match the human rendering.
- `./x gate` and `./x check` are green at every commit; the ADR's three invariant
  slugs are backed once it is Implemented.

## Notes

- Out of scope (per ADR-0102): promoting `--uncovered` to an audit rule; adding the
  domains for the unowned awf packages the report surfaces (that config work follows,
  informed by this tool); the tag-vocabulary and context-tiering ADRs.
