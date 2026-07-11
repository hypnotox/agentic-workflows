# 2026-07-11 — Read-only context query command (`awf context`)

**Goal:** implement stage (a) of [ADR-0092](../decisions/0092-read-only-context-query-command.md)
— the read-only `awf context <path>...` command: for a set of repo-relative paths it reports
owning domain(s), backed invariant slugs, related ADRs, and each owning domain's rendered
current-state pointer, with human and `--json` output and `--staged`/`--range` git sugar, gated
and degrading exactly like `awf config`. Design rationale lives in the ADR — not duplicated here.
**Stage (b)** of the ADR (rewriting the workflow skills to call `awf context`, and the Implemented
flip) is deliberately **out of scope for this plan**: it waits on a release that exposes the
command to pinned-release adopters like `examples/sundial` (ADR-0092 Decision item 8), and is a
separate later effort.

**Architecture summary:** the assembler is a `*Project` method (`ContextFor`) in a new
`internal/project/context.go`, not a standalone package — this reuses `p.Cfg`, `p.layout()`,
`p.Cfg.Sidecar`, and `p.decisionsDir()` directly and, critically, keeps every new production
function reachable from `main` through the command in the same commit (the dead-code gate,
ADR-0063, fails on any production function unreachable from a `main`). The one genuinely new
join — invariant markers *under a path set* — is a new exported `invariants.MarkersUnder` helper
reusing the existing `slugRe` and marker-scan logic. `cmd/awf/context.go` mirrors `runConfig`
line-for-line for the gate + static-fallback branch, and (because `unparam` is enabled in
`.golangci.yml`) prints human **and** JSON in the same phase, so its `asJSON` parameter is read.
The `--staged`/`--range` git sugar reuses go-git handling extracted from `internal/audit` into a
new shared `internal/git` package. Four phases, each closing on a green `./x gate`:

- **P1** — `awf context <path>...` with human **and** `--json` output (assembler, `MarkersUnder`,
  the command, `main.go` wiring, the gated-command doc-currency line). Backs all three invariants:
  `inv: context-read-only`, `inv: context-static-fallback`, `inv: context-output-parity`.
- **P2** — extract the go-git repo-open handling from `internal/audit` into a shared `internal/git`
  package and refactor `audit` onto it (behavior-preserving; no new command surface).
- **P3** — `--staged` / `--range <a>..<b>` git sugar, resolving paths via `internal/git`.
- **P4** — docs travel + flip ADR-0092 to **Accepted** (design final, command landed; the
  Implemented flip and the AGENTS.md Invariants-section bullets wait for stage (b)).

**Tech stack:** Go 1.26. stdlib `encoding/json` and `sort`; `github.com/go-git/go-git/v5` (already
a direct dependency — `internal/audit` uses it). No new dependencies. Packages touched: new
`internal/project/context.go`; new `internal/git` (P2); `internal/invariants` (`invariants.go`);
`internal/audit` (`git.go`, refactored onto `internal/git` in P2); `cmd/awf` (new `context.go`,
plus `main.go`); `.awf/agents-doc.yaml` (gated-command line in P1); `.awf/domains/parts/tooling/
current-state.md` and `templates/docs/working-with-awf.md.tmpl` (P4); `changelog/`;
`docs/decisions/0092-*.md` (status flip in P4).

**File structure:**

- Created: `internal/project/context.go`, `internal/project/context_test.go`,
  `cmd/awf/context.go`, `cmd/awf/context_test.go`, `internal/git/git.go` (P2),
  `internal/git/git_test.go` (P2), `docs/plans/2026-07-11-cli-context-query-command.md` (this plan)
- Modified: `internal/invariants/invariants.go` (+`MarkersUnder`) and `invariants_test.go`,
  `internal/audit/git.go` (refactor onto `internal/git` in P2) and its test,
  `cmd/awf/main.go` (commandOrder, argSpecs, switch; `--staged`/`--range` added in P3),
  `cmd/awf/help_test.go` (auto-adapts; see fixture-fallout note),
  `.awf/agents-doc.yaml` (gated-command line in P1), `.awf/domains/parts/tooling/current-state.md`
  (P4), `templates/docs/working-with-awf.md.tmpl` (P4), `changelog/CHANGELOG.md`,
  `docs/decisions/0092-read-only-context-query-command.md` (status flip in P4), plus rendered
  files refreshed by `./x sync`
- Deleted: none

**Phase → ADR Decision map:** P1 → D1 (subcommand) + D2 explicit-paths contract + D3 (`--json`,
parity) + D4 (assembler) + D5 (derived pointer, no-`paths` domain unreachable) + D6 (gate/fallback)
+ the D-Invariants backing; P2 → enabling refactor for D2's git half (no ADR item of its own);
P3 → D2 git-sugar half; P4 → D8 (stage-a close). **ADR Decision item 7** (skills call it directly)
is deferred to stage (b) with the Implemented flip.

**Fixture-fallout rule (every phase):** `cmd/awf/help_test.go` is generic — it iterates
`commandOrder`/`argSpecs`, so adding `context` to both (with `help` text beginning
`Usage: awf context`) satisfies it automatically; there is **no** usage-line pin in
`cmd/awf/main_test.go` (`TestRunNoArgs` only asserts the output contains `usage:`) and no
command-list doc-comment to edit. Editing the bare usage string at `cmd/awf/main.go:59` is
optional (not test-pinned) but do it for accuracy. The `.awf/agents-doc.yaml` gated-command line
edit re-renders `AGENTS.md`; commit the rendered file with the config edit.

---

## Phase 1 — `awf context <path>...` with human + JSON output (D1, D2-paths, D3, D4, D5, D6)

### 1a. The `invariants.MarkersUnder` helper

- [ ] In `internal/invariants/invariants.go`, add an exported helper returning the invariant slugs
      whose backing marker sits in a file *under* one of `paths` (in addition to matching a source
      glob). Reuse `slugRe` and the same "marker must open its line" rule as `scanTags`. Add
      verbatim:

      ```go
      // MarkersUnder returns the sorted, unique invariant slugs whose backing
      // marker comment lies in a file that both matches a source glob and sits
      // under one of paths (a queried path P owns file F when F == P or F is
      // prefixed by P+"/"). paths are slash-separated repo-relative paths. It
      // reads only source files and writes nothing.
      func MarkersUnder(root string, sources []config.InvariantSource, paths []string) ([]string, error) {
      	present := map[string]bool{}
      	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
      		if err != nil {
      			return err
      		}
      		if d.IsDir() {
      			switch d.Name() {
      			case ".git", "vendor", "node_modules":
      				return fs.SkipDir
      			}
      			if path != root {
      				if _, lerr := os.Lstat(filepath.Join(path, ".git")); lerr == nil {
      					return fs.SkipDir
      				}
      			}
      			return nil
      		}
      		rel, rerr := filepath.Rel(root, path)
      		if rerr != nil { // coverage-ignore: WalkDir yields paths under root, so Rel cannot fail
      			return rerr
      		}
      		relSlash := filepath.ToSlash(rel)
      		if !underAny(relSlash, paths) {
      			return nil
      		}
      		var markers []string
      		for _, src := range sources {
      			for _, g := range src.Globs {
      				if pathglob.Match(g, relSlash) {
      					markers = append(markers, src.Marker)
      					break
      				}
      			}
      		}
      		if len(markers) == 0 {
      			return nil
      		}
      		data, err := os.ReadFile(path)
      		if err != nil {
      			return err
      		}
      		for _, line := range strings.Split(string(data), "\n") {
      			trimmed := strings.TrimLeft(line, " \t")
      			for _, marker := range markers {
      				if strings.HasPrefix(trimmed, marker) {
      					if m := slugRe.FindStringSubmatch(trimmed[len(marker):]); m != nil {
      						present[m[1]] = true
      					}
      				}
      			}
      		}
      		return nil
      	})
      	if err != nil {
      		return nil, err
      	}
      	out := make([]string, 0, len(present))
      	for s := range present {
      		out = append(out, s)
      	}
      	sort.Strings(out)
      	return out, nil
      }

      // underAny reports whether rel is one of paths or nested beneath one.
      func underAny(rel string, paths []string) bool {
      	for _, p := range paths {
      		if rel == p || strings.HasPrefix(rel, p+"/") {
      			return true
      		}
      	}
      	return false
      }
      ```

      Add `"sort"` to the import block if absent. (`os`, `io/fs`, `path/filepath`, `strings`, and
      `internal/pathglob` are already imported by `scanTags`.)

- [ ] In `internal/invariants/invariants_test.go`, add table cases for `MarkersUnder`:
      (1) a marker file under a queried dir → slug returned; (2) a marker file *outside* the
      queried paths → excluded; (3) a queried file path exactly matching a marker file → returned;
      (4) a marker in a file matching no source glob → excluded; (5) a mid-line `invariant:` token
      (not opening its line) → excluded; (6) empty `paths` → empty result; (7) a `.git`-bearing
      nested dir → skipped. Assert sorted, de-duplicated output.

- [ ] Verify: `go test ./internal/invariants/... -run MarkersUnder` → `ok`.

### 1b. The `ContextFor` assembler

- [ ] Create `internal/project/context.go` verbatim:

      ```go
      package project

      import (
      	"path/filepath"
      	"sort"
      	"strings"

      	"github.com/hypnotox/agentic-workflows/internal/adr"
      	"github.com/hypnotox/agentic-workflows/internal/invariants"
      	"github.com/hypnotox/agentic-workflows/internal/pathglob"
      )

      // ContextResult is the read-only context awf holds for a set of repo-relative
      // paths: their owning domains (each with the rendered current-state pointer),
      // the invariant slugs backed under those paths, the ADRs related via the
      // owning domains, and any queried path matching no configured domain.
      type ContextResult struct {
      	Paths      []string    `json:"paths"`
      	Domains    []DomainRef `json:"domains"`
      	Invariants []string    `json:"invariants"`
      	ADRs       []ADRRef    `json:"adrs"`
      	Unowned    []string    `json:"unowned"`
      }

      // DomainRef is an owning domain and its rendered current-state doc path,
      // derived by convention (never a sidecar field — ADR-0086).
      type DomainRef struct {
      	Name         string `json:"name"`
      	CurrentState string `json:"currentState"`
      }

      // ADRRef is an ADR related to the query via an owning domain. Title is the
      // human title with the "ADR-NNNN: " prefix stripped (Number carries it).
      type ADRRef struct {
      	Number string `json:"number"`
      	Title  string `json:"title"`
      	Status string `json:"status"`
      	Path   string `json:"path"`
      }

      // ContextFor assembles the read-only context for paths. It reads only
      // committed state (domain sidecars, ADR files, source markers) and writes
      // nothing.
      // invariant: context-read-only
      func (p *Project) ContextFor(paths []string) (ContextResult, error) {
      	clean := normalizeContextPaths(paths)
      	lay := p.layout()
      	res := ContextResult{Paths: clean}

      	owners := map[string]bool{}
      	matched := map[string]bool{}
      	for _, d := range p.Cfg.Domains {
      		sc, err := p.Cfg.Sidecar("domains", d)
      		if err != nil {
      			return ContextResult{}, err
      		}
      		for _, g := range sc.Paths {
      			for _, path := range clean {
      				if pathglob.Match(g, path) {
      					owners[d] = true
      					matched[path] = true
      				}
      			}
      		}
      	}
      	for d := range owners {
      		res.Domains = append(res.Domains, DomainRef{
      			Name:         d,
      			CurrentState: lay.DocsDir + "/domains/" + d + ".md",
      		})
      	}
      	sort.Slice(res.Domains, func(i, j int) bool { return res.Domains[i].Name < res.Domains[j].Name })
      	for _, path := range clean {
      		if !matched[path] {
      			res.Unowned = append(res.Unowned, path)
      		}
      	}

      	if p.Cfg.Invariants != nil && !p.Cfg.Invariants.Disabled {
      		slugs, err := invariants.MarkersUnder(p.Root, p.Cfg.Invariants.Sources, clean)
      		if err != nil {
      			return ContextResult{}, err
      		}
      		res.Invariants = slugs
      	}

      	adrs, err := adr.ParseDir(p.decisionsDir())
      	if err != nil {
      		return ContextResult{}, err
      	}
      	for _, a := range adrs {
      		for _, dm := range a.Domains {
      			if owners[dm] {
      				res.ADRs = append(res.ADRs, ADRRef{
      					Number: a.Number,
      					Title:  strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "),
      					Status: a.Status,
      					Path:   lay.DocsDir + "/decisions/" + a.Filename,
      				})
      				break
      			}
      		}
      	}
      	sort.Slice(res.ADRs, func(i, j int) bool { return res.ADRs[i].Number < res.ADRs[j].Number })
      	return res, nil
      }

      // normalizeContextPaths slash-normalizes, path-cleans, de-duplicates, and
      // sorts the queried paths so the assembly is deterministic.
      func normalizeContextPaths(paths []string) []string {
      	seen := map[string]bool{}
      	var out []string
      	for _, p := range paths {
      		c := filepath.ToSlash(filepath.Clean(p))
      		if c == "" || c == "." || seen[c] {
      			continue
      		}
      		seen[c] = true
      		out = append(out, c)
      	}
      	sort.Strings(out)
      	return out
      }
      ```

- [ ] Create `internal/project/context_test.go` (mirror `configreference_test.go`'s fixture
      setup: scaffold a tree with `.awf/config.yaml`, domain sidecars with `paths:`, an ADR
      carrying `domains:`, and a source file with a backing marker). Cases:
      - a path under a domain's `paths:` glob → that domain in `Domains` with
        `CurrentState == "docs/domains/<d>.md"`;
      - a path under two domains → both, sorted;
      - a path under no domain → in `Unowned`, absent from `Domains`;
      - a marker file under the queried path → its slug in `Invariants`;
      - an ADR whose `domains:` includes an owning domain → in `ADRs` with `Title` **stripped** of
        the `ADR-NNNN: ` prefix; an ADR touching no owning domain → excluded;
      - a domain configured **without** `paths:` → never appears (D5 unreachable-by-path);
      - `Invariants` disabled (`invariants.disabled: true`) → empty `Invariants`, no error;
      - duplicate + unclean input paths (`./cmd/`, `cmd`, `.`) collapse to one cleaned entry.

- [ ] Verify: `go test ./internal/project/... -run Context` → `ok`.

### 1c. The `context` command (human + JSON) + gate/static-fallback

- [ ] Create `cmd/awf/context.go` verbatim:

      ```go
      package main

      import (
      	"encoding/json"
      	"errors"
      	"fmt"
      	"io"
      	"io/fs"
      	"os"

      	"github.com/hypnotox/agentic-workflows/internal/config"
      	"github.com/hypnotox/agentic-workflows/internal/project"
      )

      // runContext prints the read-only context for the given repo-relative paths:
      // owning domains, backed invariants, related ADRs, and each domain's rendered
      // current-state pointer. It mirrors runConfig's gate + static-fallback shape:
      // a genuinely absent config prints the pre-adoption notice; any other stat
      // fault is an error; inside a tree the binary-version gate runs before Open.
      func runContext(cwd string, paths []string, asJSON bool, stdout io.Writer) error {
      	if _, err := os.Stat(config.ConfigPath(cwd)); err != nil {
      		if !errors.Is(err, fs.ErrNotExist) {
      			return err
      		}
      		// invariant: context-static-fallback
      		return printContext(stdout, project.ContextResult{Paths: paths}, asJSON,
      			"context (static — not inside an awf project; live context appears inside one)")
      	}
      	if err := gate(cwd); err != nil {
      		return err
      	}
      	p, err := project.Open(cwd)
      	if err != nil {
      		return err
      	}
      	res, err := p.ContextFor(paths)
      	if err != nil {
      		return err
      	}
      	return printContext(stdout, res, asJSON, "context — live state for this project")
      }

      // printContext renders res as JSON or human-readable text. Both modes read the
      // same assembled res, so they cannot diverge.
      // invariant: context-output-parity
      func printContext(stdout io.Writer, res project.ContextResult, asJSON bool, header string) error {
      	if asJSON {
      		enc := json.NewEncoder(stdout)
      		enc.SetIndent("", "  ")
      		return enc.Encode(res)
      	}
      	fmt.Fprintln(stdout, header)
      	fmt.Fprintf(stdout, "\npaths: %v\n", res.Paths)
      	fmt.Fprintln(stdout, "\n## Domains")
      	for _, d := range res.Domains {
      		fmt.Fprintf(stdout, "  %s — %s\n", d.Name, d.CurrentState)
      	}
      	fmt.Fprintln(stdout, "\n## Invariants")
      	for _, s := range res.Invariants {
      		fmt.Fprintf(stdout, "  %s\n", s)
      	}
      	fmt.Fprintln(stdout, "\n## Related ADRs")
      	for _, a := range res.ADRs {
      		fmt.Fprintf(stdout, "  ADR-%s (%s) %s — %s\n", a.Number, a.Status, a.Title, a.Path)
      	}
      	if len(res.Unowned) > 0 {
      		fmt.Fprintln(stdout, "\n## Unowned paths (no configured domain)")
      		for _, u := range res.Unowned {
      			fmt.Fprintf(stdout, "  %s\n", u)
      		}
      	}
      	return nil
      }
      ```

- [ ] Wire `main.go`:
      - add `"context"` to `commandOrder` (after `"config"`);
      - update the bare usage string at `main.go:59` to include `context` (optional, for accuracy);
      - add the `argSpecs["context"]` entry (P1 declares only `--json`; `--staged`/`--range` are
        added in P3 with their implementation, so `checkArgs` rejects them until then):

        ```go
        "context": {
        	boolFlags: []string{"--json"}, maxPos: -1,
        	summary: "Report owning domains, invariants, and ADRs for paths",
        	help: `Usage: awf context <path>... [--json]

Report the committed context awf holds for a set of repo-relative paths: owning
domain(s), the invariant slugs backed under those paths, related ADRs, and each
domain's current-state doc. Read-only. Inside an awf project the output reflects
live config; outside one, a static pre-adoption notice prints.

Flags:
  --json    emit the context as JSON
` ,
        },
        ```

      - add the switch case:

        ```go
        case "context":
        	spec := argSpecs["context"]
        	pos := positionals(args[2:], spec.boolFlags, spec.valueFlags)
        	if len(pos) == 0 {
        		cmdErr = &usageErr{"usage: awf context <path>... [--json]"}
        	} else {
        		cmdErr = runContext(cwd, pos, hasFlag(args, "--json"), stdout)
        	}
        ```

- [ ] Create `cmd/awf/context_test.go`. Cases (drive `run(...)` with injected writers, mirroring
      `config_test.go`):
      - inside a scaffolded tree, `awf context <path>` → exit 0, human output contains the owning
        domain and its `docs/domains/<d>.md` pointer;
      - `awf context <path> --json` → exit 0, output parses as JSON and unmarshals to a
        `project.ContextResult` equal to the human run's underlying set (the parity check);
      - `awf context` with no paths → exit 2, usage error on stderr;
      - outside a tree (no `.awf/config.yaml`) → exit 0, static pre-adoption header (and the
        `--json` outside-tree variant → JSON of the paths-only result);
      - a **non-`ErrNotExist`** stat fault → exit 1: use the `TestRunConfigStatFault` pattern —
        `os.WriteFile(filepath.Join(root, ".awf"), []byte("not a dir"), 0o644)` so
        `os.Stat(".awf/config.yaml")` returns ENOTDIR;
      - a behind-version tree → the gate error surfaces (exit 1) — reuse the version-gate fixture
        pattern from `config_test.go`;
      - `awf context --help` → prints the help text, exit 0;
      - a path with an unowned segment → the `## Unowned paths` section prints.

- [ ] Back `inv: context-read-only`: add a `context_test.go` case that snapshots the working-tree
      file mtimes and `.awf/awf.lock` bytes before and after running `awf context` across its
      branches (human, `--json`, unowned) and asserts byte-identity — the ADR's named backing
      check. (`inv: context-static-fallback` is backed by the outside-tree case;
      `inv: context-output-parity` by the human-vs-JSON parity case.)

- [ ] Doc-currency (gated line): edit `.awf/agents-doc.yaml`'s gated-command invariant data to add
      `context` to **both** the enumerated command list **and** the outside-tree degrade clause of
      the "Binary-version gate" line — and make the degrade verb plural (`config` and `context`
      degrade …), since two commands now degrade. Run `./x sync`; commit the re-rendered
      `AGENTS.md` with the config edit.

- [ ] Verify: `./x gate` → passes (100% coverage, 0 lint, deadcode clean). Then
      `go run ./cmd/awf context cmd/awf/main.go` → prints the `tooling` domain, tooling-backed
      invariants, and ADRs tagged `tooling`; `go run ./cmd/awf context cmd/awf/main.go --json` →
      valid JSON (a live smoke check in this repo).

- [ ] Commit: `feat(tooling): add read-only awf context command`.

---

## Phase 2 — extract the shared `internal/git` package (enabling refactor)

Behavior-preserving: `internal/audit`'s go-git repo-open handling moves to a shared package so the
context command's git sugar (P3) can reuse it. No command or output changes; the audit test suite
is the regression pin.

- [ ] Read `internal/audit/git.go`. Move the repo-open helpers — `openRepo` and the worktree/
      submodule filesystem handling it depends on (`dotGitFs`, `gitfileFs`, and the
      `noExtensionsStorer` type + its `Config` method) — into a new `internal/git/git.go`, exported
      as `git.OpenRepo(repoRoot string) (*gogit.Repository, error)` (alias the go-git import to
      `gogit` to avoid the package-name clash). Keep audit's commit-log logic (`Collect`,
      `toCommit`, `ruleUncommittedChanges`, etc.) in `internal/audit`; only the open/FS plumbing
      moves.

- [ ] Refactor `internal/audit/git.go` to call `git.OpenRepo` wherever it called `openRepo`. Move
      the corresponding open/FS unit tests from the audit test file into `internal/git/git_test.go`
      (the moved functions must keep their coverage; `internal/git` needs its own 100%). Leave the
      audit tests that exercise `Collect`/rules in place — they now exercise the refactored path.

- [ ] Verify: `./x gate` → passes (audit behavior unchanged; `internal/git` fully covered;
      deadcode clean — `git.OpenRepo` is reachable from `main` via `awf audit`).

- [ ] Commit: `refactor(tooling): extract shared internal/git repo-open helpers`.

---

## Phase 3 — `--staged` / `--range` git sugar (D2 git half)

### 3a. `internal/git` changed-paths resolution

- [ ] In `internal/git/git.go`, add `ChangedPaths` (reachable from `main` via P3b's command
      wiring, landing in the same commit):

      ```go
      // ChangedPaths returns the repo-relative paths changed either in the staged
      // index (staged) or between the two revisions of rangeSpec ("a..b"). Exactly
      // one selector is honored (staged takes precedence); an empty selector
      // returns nil. A revision that cannot resolve (shallow/detached checkout) is
      // a clear error.
      func ChangedPaths(repoRoot string, staged bool, rangeSpec string) ([]string, error)
      ```

      For `staged`: open via `OpenRepo`, diff `HEAD`'s tree against the index worktree/staging,
      collect changed file paths. For `rangeSpec`: split on `..`, `ResolveRevision` both ends,
      diff their trees (`object.Tree.Diff`), collect paths. Return sorted, de-duplicated,
      slash-separated repo-relative paths. Error clearly on a malformed range or an unresolvable
      revision.

- [ ] `internal/git/git_test.go` cases (use `internal/testsupport/gitfixture`): a staged change →
      its path; a two-commit range → the changed paths; a malformed `rangeSpec` (no `..`) → error;
      an unknown revision → error; nothing staged → empty slice.

### 3b. Wire the flags into the command

- [ ] In `cmd/awf/main.go`, extend `argSpecs["context"]`: add `--staged` to `boolFlags`, `--range`
      to `valueFlags`, and append the two flags + a "provide paths explicitly, or resolve from git"
      paragraph to the `help` text. Update the switch case so that when `pos` is empty, it resolves
      git paths before erroring:

      ```go
      case "context":
      	spec := argSpecs["context"]
      	pos := positionals(args[2:], spec.boolFlags, spec.valueFlags)
      	if len(pos) == 0 {
      		staged, rng := hasFlag(args, "--staged"), valueFlag(args, "--range")
      		if !staged && rng == "" {
      			cmdErr = &usageErr{"usage: awf context <path>... [--json] [--staged] [--range <a>..<b>]"}
      			break
      		}
      		var gerr error
      		if pos, gerr = git.ChangedPaths(cwd, staged, rng); gerr != nil {
      			cmdErr = gerr
      			break
      		}
      		if len(pos) == 0 {
      			cmdErr = &usageErr{"awf context: no changed paths for the given selector"}
      			break
      		}
      	}
      	cmdErr = runContext(cwd, pos, hasFlag(args, "--json"), stdout)
      ```

      (Explicit `<path>...` args take precedence — if `pos` is non-empty, the git flags are
      ignored; state this in the help text.) Add the `internal/git` import.

- [ ] `cmd/awf/context_test.go` cases: `--staged` with a staged change → reports its context;
      `--range a..b` → the changed paths' context; a bad range → exit 1 with the git error;
      `--staged` with nothing staged → exit 2 usage error ("no changed paths").

- [ ] Verify: `./x gate` → passes.

- [ ] Commit: `feat(tooling): resolve awf context paths from --staged/--range`.

---

## Phase 4 — Docs travel + status flip (D8 stage-a close)

- [ ] `templates/docs/working-with-awf.md.tmpl`: add `awf context <path>...` to the commands
      reference, one line describing it as the read-only context oracle. Run `./x sync`.

- [ ] `.awf/domains/parts/tooling/current-state.md`: add a sentence noting the new read-only
      `awf context` query command and the shared `internal/git` package. Run `./x sync`.

- [ ] `changelog/CHANGELOG.md`: add an Unreleased bullet — "Add read-only `awf context` query
      command (owning domains, backed invariants, related ADRs; `--json`, `--staged`, `--range`)."

- [ ] Flip `docs/decisions/0092-read-only-context-query-command.md` `status:` to **Accepted**
      (design final, command implemented; the **Implemented** flip — and adding the three `inv:`
      slugs to the AGENTS.md Invariants section — waits for stage (b) skill adoption, per the
      Implemented-only convention of that section). Run `./x sync` to regenerate `ACTIVE.md` +
      domain indexes. Do not hand-edit `ACTIVE.md`.

- [ ] Verify: `./x gate` → passes; `./x check` → clean. Note: `awf invariants` reports clean
      because it requires backing only for **Implemented**-ADR slugs (ADR-0092 is `Accepted`), so
      the three markers land now but are not yet gate-*required* — they become required at the
      stage-(b) Implemented flip.

- [ ] Commit: `docs(tooling): document awf context and accept ADR-0092`.

---

## Out of scope (stage b, later effort)

- Rewriting the workflow skills (`awf-brainstorming` step 1, subagent dispatch briefs, the impl
  reviewers) to call `awf context` / `awf context --json` instead of grep-and-read prose — ADR-0092
  Decision item 7.
- Flipping ADR-0092 to **Implemented** and adding its three `inv:` slugs to the AGENTS.md
  Invariants section (the section cites Implemented ADRs only).

Both wait for a release that exposes the command to pinned-release adopters (`examples/sundial`).
Track as a follow-up once stage (a) ships in a release.
