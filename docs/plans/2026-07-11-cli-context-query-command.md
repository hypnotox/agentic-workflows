# 2026-07-11 — Read-only context query command (`awf context`)

**Goal:** implement stage (a) of [ADR-0092](../decisions/0092-read-only-context-query-command.md)
— the read-only `awf context <path>...` command: for a set of repo-relative paths it reports
owning domain(s), backed invariant slugs, related ADRs, and each owning domain's rendered
current-state pointer, with human and `--json` output and `--staged`/`--range` git sugar, gated
and degrading exactly like `awf config`. Design rationale lives in the ADR — not duplicated here.
**Stage (b)** of the ADR (rewriting the workflow skills to call `awf context`) is deliberately
**out of scope for this plan**: it waits on a release that exposes the command to pinned-release
adopters like `examples/sundial` (ADR-0092 Decision item 8), and is a separate later effort.

**Architecture summary:** the assembler is a `*Project` method (`ContextFor`) in a new
`internal/project/context.go`, not a standalone package — this reuses `p.Cfg`, `p.layout()`,
`p.Cfg.Sidecar`, and `p.decisionsDir()` directly and, critically, keeps every new production
function reachable from `main` through the command in the same commit (the dead-code gate,
ADR-0063, fails on any production function unreachable from a `main`). The one genuinely new
join — invariant markers *under a path set* — is a new exported `invariants.MarkersUnder`
helper reusing the existing `slugRe` and marker-scan logic. `cmd/awf/context.go` mirrors
`runConfig` line-for-line for the gate + static-fallback branch. Four phases, each closing on a
green `./x gate`:

- **P1** — `awf context <path>...` with human output (the assembler, `MarkersUnder`, the command,
  `main.go` wiring, the gated-command doc-currency line). Backs `inv: context-read-only` and
  `inv: context-static-fallback`.
- **P2** — `--json` output. Backs `inv: context-output-parity`.
- **P3** — `--staged` / `--range <a>..<b>` git sugar (go-git, mirroring `audit`'s collection).
- **P4** — docs travel + flip ADR-0092 to **Accepted** (design final, command landed; the
  Implemented flip waits for stage (b)).

**Tech stack:** Go 1.26. stdlib `encoding/json` (P2) and `sort`; `github.com/go-git/go-git/v5`
(P3, already a direct dependency — `internal/audit` uses it). No new dependencies. Packages
touched: `internal/project` (new `context.go`), `internal/invariants` (`invariants.go`),
`cmd/awf` (new `context.go`, plus `main.go`), `.awf/agents-doc.yaml` (gated-command line +
ADR bullet), `.awf/domains/parts/tooling/current-state.md`, `templates/docs/working-with-awf.md.tmpl`,
`changelog/`, `docs/decisions/0092-*.md` (status flip).

**File structure:**

- Created: `internal/project/context.go`, `internal/project/context_test.go`,
  `cmd/awf/context.go`, `cmd/awf/context_test.go`,
  `docs/plans/2026-07-11-cli-context-query-command.md` (this plan)
- Modified: `internal/invariants/invariants.go` (+`MarkersUnder`, +`invariants_test.go` cases),
  `cmd/awf/main.go` (commandOrder, usage line, argSpecs, switch, `pathsFor` helper in P3),
  `cmd/awf/help_test.go` (global-help pin), `.awf/agents-doc.yaml` (gated-command line in P1,
  ADR-0092 bullet in P4), `.awf/domains/parts/tooling/current-state.md` (P4),
  `templates/docs/working-with-awf.md.tmpl` (P4), `changelog/CHANGELOG.md`,
  `docs/decisions/0092-read-only-context-query-command.md` (status flip in P4), plus rendered
  files refreshed by `./x sync`
- Deleted: none

**Phase → ADR Decision map:** P1 → D1 (paths core), D4 (assembler), D5 (derived pointer +
no-paths domain unreachable), D6 (gate/fallback), D7 (invariant backing); P2 → D3 (`--json`,
parity); P3 → D2 (git sugar); P4 → D8 (rollout stage-a close) + doc-currency obligations.

**Fixture-fallout rule (every phase):** a new gated command moves any test that pins the command
set or help text. Update pins only by *adding* `context` (never weaken an assertion). Known
candidates: `cmd/awf/help_test.go` (global-help string, per-command help), `cmd/awf/main_test.go`
(usage line), and any `awf help`/`list` golden. The `.awf/agents-doc.yaml` gated-command line edit
re-renders `AGENTS.md`; commit the rendered file with the config edit.

---

## Phase 1 — `awf context <path>...` with human output (D1, D4, D5, D6, D7)

### 1a. The `invariants.MarkersUnder` helper

- [ ] In `internal/invariants/invariants.go`, add an exported helper that returns the invariant
      slugs whose backing marker sits in a file *under* one of `paths` (in addition to matching a
      source glob). Reuse `slugRe` and the same "marker must open its line" rule as `scanTags`.
      Add verbatim:

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

      Add `"sort"` to the import block if absent. (`os`, `io/fs`, `path/filepath`, `strings`,
      and `internal/pathglob` are already imported by `scanTags`.)

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
      	"sort"

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

      // ADRRef is an ADR related to the query via an owning domain.
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
      					Number: a.Number, Title: a.Title, Status: a.Status,
      					Path: lay.DocsDir + "/decisions/" + a.Filename,
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
      		if c == "" || seen[c] {
      			continue
      		}
      		seen[c] = true
      		out = append(out, c)
      	}
      	sort.Strings(out)
      	return out
      }
      ```

      Add `"path/filepath"` to the import block (used by `normalizeContextPaths`). The `json`
      struct tags are inert in P1 (no encoder yet) and consumed in P2 — they carry no behavior,
      so they add no unreachable statement.

- [ ] Create `internal/project/context_test.go`. Use the existing project test fixture helper
      (mirror `configreference_test.go`'s setup: scaffold a tree with `.awf/config.yaml`, domain
      sidecars with `paths:`, an ADR carrying `domains:`, and a source file with a backing
      marker). Cases:
      - a path under a domain's `paths:` glob → that domain in `Domains` with
        `CurrentState == "docs/domains/<d>.md"`;
      - a path under two domains → both, sorted;
      - a path under no domain → in `Unowned`, absent from `Domains`;
      - a marker file under the queried path → its slug in `Invariants`;
      - an ADR whose `domains:` includes an owning domain → in `ADRs` (number/title/status/path);
        an ADR touching no owning domain → excluded;
      - a domain configured **without** `paths:` → never appears (D5 unreachable-by-path);
      - `Invariants` disabled (`invariants.disabled: true`) → empty `Invariants`, no error;
      - duplicate + unclean input paths (`./cmd/`, `cmd`) collapse to one cleaned entry.

- [ ] Verify: `go test ./internal/project/... -run Context` → `ok`.

### 1c. The `context` command (human output) + gate/static-fallback

- [ ] Create `cmd/awf/context.go` verbatim:

      ```go
      package main

      import (
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

      // printContext renders res as human-readable text (asJSON is wired in P2).
      func printContext(stdout io.Writer, res project.ContextResult, asJSON bool, header string) error {
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

      The `asJSON` parameter is accepted but unused in P1; P2 adds its branch. To keep P1
      statement-coverage clean, the P1 command wiring (next task) always passes `false`, and the
      `asJSON` param carries no P1 branch — it is a plain unread parameter, not a dead statement.
      (If the linter flags the unread param, keep it: the next commit reads it. Confirm the
      `./x gate` lint tier is silent on unread params before committing; if not, fold P2's JSON
      branch into P1 instead — see P2 note.)

- [ ] Wire `main.go`:
      - add `"context"` to `commandOrder` (after `"config"`);
      - add `context` to the two usage-line command lists (the `len(args) < 2` branch and the
        top-of-file doc comment enumerations that list commands);
      - add the `argSpecs["context"]` entry:

        ```go
        "context": {
        	boolFlags: []string{"--json", "--staged"}, valueFlags: []string{"--range"}, minPos: 0, maxPos: -1,
        	summary: "Report owning domains, invariants, and ADRs for paths",
        	help: `Usage: awf context <path>... [--json] [--staged] [--range <a>..<b>]

Report the committed context awf holds for a set of repo-relative paths: owning
domain(s), the invariant slugs backed under those paths, related ADRs, and each
domain's current-state doc. Read-only. Inside an awf project the output reflects
live config; outside one, a static pre-adoption notice prints.

Provide paths explicitly, or resolve them from git with --staged (staged changes)
or --range <a>..<b> (the diff between two revisions).

Flags:
  --json               emit the context as JSON
  --staged             use the staged changed paths
  --range <a>..<b>     use the paths changed between revisions a and b
` ,
        },
        ```

        (`--staged`/`--range` are declared in the spec now so `checkArgs` accepts them; their
        behavior lands in P3. Until then, passing them resolves to an empty path set, which the
        no-path guard below rejects with the same hint — acceptable and covered.)

      - add the switch case:

        ```go
        case "context":
        	spec := argSpecs["context"]
        	pos := positionals(args[2:], spec.boolFlags, spec.valueFlags)
        	if len(pos) == 0 && !hasFlag(args, "--staged") && valueFlag(args, "--range") == "" {
        		cmdErr = &usageErr{"usage: awf context <path>... [--json] [--staged] [--range <a>..<b>]"}
        	} else {
        		cmdErr = runContext(cwd, pos, hasFlag(args, "--json"), stdout)
        	}
        ```

- [ ] Update `cmd/awf/help_test.go` and `cmd/awf/main_test.go` pins: add the `context` line to the
      expected global-help output and the usage string. Add `context` to any exhaustive
      command-list assertion.

- [ ] Create `cmd/awf/context_test.go`. Cases (drive `run(...)` with injected writers, mirroring
      `config_test.go`):
      - inside a scaffolded tree, `awf context <path>` → exit 0, human output contains the owning
        domain and its `docs/domains/<d>.md` pointer;
      - `awf context` with no paths and no git flag → exit 2, usage error on stderr;
      - outside a tree (no `.awf/config.yaml`) → exit 0, static pre-adoption header, no gate/Open;
      - a stat fault that is **not** `ErrNotExist` (e.g. `.awf/config.yaml` is a directory, or a
        permission-denied parent) → exit 1 (covers the non-`ErrNotExist` branch);
      - a behind-version tree → the gate error surfaces (exit 1) — reuse the version-gate fixture
        pattern from `config_test.go`;
      - `awf context --help` → prints the help text, exit 0;
      - a path with an unowned segment → the `## Unowned paths` section prints.

- [ ] Back the invariants: `inv: context-read-only` (marker already on `ContextFor`) — add a
      `context_test.go` (or `context_readonly_test.go`) case that snapshots the working-tree file
      mtimes and `.awf/awf.lock` bytes before and after running `awf context` across its branches
      and asserts byte-identity (the ADR's named backing check). `inv: context-static-fallback`
      (marker on the fallback branch in `runContext`) — the outside-tree test case above is its
      backing.

- [ ] Doc-currency (gated line): edit `.awf/agents-doc.yaml`'s gated-command invariant data to add
      `context` to **both** the enumerated command list **and** the outside-tree degrade clause of
      the "Binary-version gate" line (context degrades like config). Run `./x sync`; commit the
      re-rendered `AGENTS.md` with the config edit.

- [ ] Verify: `./x gate` → passes (100% coverage, 0 lint, deadcode clean). Then
      `go run ./cmd/awf context cmd/awf/main.go` → prints the `tooling` domain, tooling-backed
      invariants, and ADRs tagged `tooling` (a live smoke check in this repo).

- [ ] Commit: `feat(tooling): add read-only awf context command (human output)`.

---

## Phase 2 — `--json` output (D3, parity)

### 2a. JSON rendering

- [ ] In `cmd/awf/context.go`, add the JSON branch at the top of `printContext`:

      ```go
      if asJSON {
      	enc := json.NewEncoder(stdout)
      	enc.SetIndent("", "  ")
      	return enc.Encode(res)
      }
      ```

      Add `"encoding/json"` to the imports. Both renderings now read the **same** `res` — the
      structural backing of `inv: context-output-parity`.

- [ ] Add the invariant marker `// invariant: context-output-parity` on `printContext` (the
      single function both output modes flow through — one assembled `res` feeds both).

- [ ] `context_test.go` cases: `awf context <path> --json` → exit 0, output parses as JSON and
      unmarshals to a `project.ContextResult` whose `Domains`/`Invariants`/`ADRs`/`Unowned` equal
      the human run's underlying set (decode both from one `ContextFor` call, or assert the JSON
      decodes to the same struct the human path printed). Cover the `enc.Encode` path and the
      `--json` outside-tree fallback (static header replaced by JSON of the paths-only result).

- [ ] Verify: `./x gate` → passes. `go run ./cmd/awf context cmd/awf/main.go --json` → valid JSON.

- [ ] Commit: `feat(tooling): add --json output to awf context`.

> **Note (P1↔P2 fold):** if the P1 `./x gate` lint tier rejects the unread `asJSON` parameter,
> merge 2a into P1 (land human + JSON together) and back all three invariants in the single
> commit. The phase split is for reviewability, not a hard dependency — the deadcode gate does not
> force it, since `printContext` is reachable from `main` either way.

---

## Phase 3 — `--staged` / `--range` git sugar (D2)

### 3a. Path resolution from git

- [ ] In `cmd/awf/context.go` (or a small `cmd/awf/gitpaths.go`), add a resolver that turns
      `--staged` / `--range <a>..<b>` into a repo-relative path set using `go-git/go-git/v5`,
      mirroring how `internal/audit` opens the repo (`git.PlainOpen(cwd)`). Signature:

      ```go
      // resolveGitPaths returns the repo-relative paths changed either in the
      // staged index (staged) or between the two revisions of rangeSpec ("a..b").
      // Exactly one selector is honored; an empty selector returns nil.
      func resolveGitPaths(cwd string, staged bool, rangeSpec string) ([]string, error)
      ```

      For `--staged`: open the repo, diff `HEAD`'s tree against the index, collect changed file
      paths. For `--range a..b`: resolve both revisions, diff their trees. Return a clear error on
      a shallow/detached checkout where a revision cannot resolve (the ADR's edge case). Follow the
      exact go-git call shape `internal/audit` already uses — read `internal/audit/collect.go`
      (or wherever `audit.Collect` lives) for the established pattern rather than inventing one.

- [ ] Wire the switch case: when `pos` is empty and `--staged`/`--range` is present, call
      `resolveGitPaths` and pass its result to `runContext`; when it resolves to an empty set,
      keep the existing usage error (nothing to report). Explicit `<path>...` args take precedence
      over the git flags if both are given (document this in the help text; simplest: if `pos`
      is non-empty, ignore the git flags).

- [ ] `context_test.go` cases (use the repo's git test fixture helper —
      `internal/testsupport/gitfixture`): `--staged` with a staged change → its path resolves and
      reports context; `--range a..b` across two commits → the changed paths resolve; a bad range
      (unknown revision) → exit 1 with a clear error; `--staged` with nothing staged → empty set →
      usage error.

- [ ] Verify: `./x gate` → passes.

- [ ] Commit: `feat(tooling): resolve awf context paths from --staged/--range`.

---

## Phase 4 — Docs travel + status flip (D8 stage-a close)

- [ ] `templates/docs/working-with-awf.md.tmpl`: add `awf context <path>...` to the commands
      reference (alongside the other read-only query commands), one line describing it as the
      read-only context oracle. Run `./x sync`.

- [ ] `.awf/domains/parts/tooling/current-state.md`: add a sentence noting the new read-only
      `awf context` query command. Run `./x sync`.

- [ ] `.awf/agents-doc.yaml`: add the ADR-0092 invariants (the three `inv:` slugs) to the agent
      guide's Invariants data, and a one-line entry for `context` if the guide enumerates commands.
      Run `./x sync`; commit rendered `AGENTS.md`.

- [ ] `changelog/CHANGELOG.md`: add an Unreleased bullet — "Add read-only `awf context` query
      command (owning domains, backed invariants, related ADRs; `--json`, `--staged`, `--range`)."

- [ ] Flip `docs/decisions/0092-read-only-context-query-command.md` `status:` to **Accepted**
      (design final, command implemented; the **Implemented** flip waits for stage (b) skill
      adoption). Run `./x sync` to regenerate `ACTIVE.md` + domain indexes. Do not hand-edit
      `ACTIVE.md`.

- [ ] Verify: `./x gate` → passes; `./x check` → clean (no drift; `awf invariants` clean — all
      three slugs are backed from P1/P2).

- [ ] Commit: `docs(tooling): document awf context and accept ADR-0092`.

---

## Out of scope (stage b, later effort)

- Rewriting the workflow skills (`awf-brainstorming` step 1, subagent dispatch briefs, the impl
  reviewers) to call `awf context` / `awf context --json` instead of grep-and-read prose. This is
  ADR-0092 Decision item 7 + the Implemented flip, and waits for a release that exposes the
  command to pinned-release adopters (`examples/sundial`). Track as a follow-up once stage (a)
  ships in a release.
