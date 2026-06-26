# Plan: Process-conformance audit (`awf audit`)

Implements [ADR-0017](../decisions/0017-process-conformance-audit.md). Design rationale lives in the
ADR; this file is the execution record. Do not duplicate rationale — link.

## Goal

Add `awf audit`: a standalone, advisory subcommand that reads git history (commits reachable from
`HEAD` but not from a base branch) and reports workflow-conformance findings — two `Error` rules
(Conventional Commits; ADR-status-flip co-committed with `ACTIVE.md`) and two `Warning` rules
(dependency-manifest change without an ADR; large change without a plan). Exit non-zero only on an
`Error` finding. The terminal `awf-reviewing-impl` reviewer runs it.

## Architecture summary

- **`internal/audit`** (new package), three seams for testability under the 100% coverage gate:
  - `git.go` — `Collect(repoRoot, baseBranch) ([]Commit, error)`: the only go-git caller. Converts
    go-git commits into neutral `Commit`/`FileChange` structs (per-file action, add/del stats, and
    `OldText`/`NewText` for `.md` files only). Tested with hermetic go-git repos (no `git` binary).
  - `audit.go` — `evaluate(commits []Commit, in Inputs) []Finding`: pure rule engine over the neutral
    structs (no git, no I/O). The four rules live here. Backs the rule invariants directly.
  - `Run(repoRoot string, in Inputs) ([]Finding, error)` = `Collect` then `evaluate`.
- **`internal/config`** — new `Audit *AuditConfig` block + `AuditSettings()` returning effective
  (defaulted) settings, validated in `Validate` (reusing the ADR-0008 basename-glob check).
- **`internal/project`** — `Project.Audit(baseOverride string) ([]audit.Finding, error)` builds
  `audit.Inputs` from `AuditSettings()`, `layout()` (adrDir/activeMd/plansDir), and the lock's
  generated-file set, then calls `audit.Run`.
- **`cmd/awf`** — `audit.go` (`runAudit`) + `main.go` dispatch; `awf audit [--base <ref>]`.
- **Dogfood** — `.awf/config.yaml` `audit:` block (`allowedScopes: [awf]`), `./x audit`, an
  AGENTS.md command row, and a new `run-audit` section in `awf-reviewing-impl`.

## Tech stack

- Go 1.26; new direct dep `github.com/go-git/go-git/v5`; `gopkg.in/yaml.v3`; stdlib `regexp`,
  `strings`, `path/filepath`, `sort`, `fmt`.
- Packages touched: `internal/audit` (new), `internal/config`, `internal/project`, `internal/manifest`
  (read only), `cmd/awf`, `templates`.
- Gate: `./x gate` (`go test ./... -coverpkg=./...` at 100%, `go vet`, golangci-lint) on every
  code-touching commit; `./x check` (drift + invariants) via the pre-commit hook.

## File structure

Created:
- `internal/audit/audit.go` — `Severity`, `Finding`, `Commit`, `FileChange`, `Action`, `Inputs`,
  `Run`, `evaluate`, the four rules + helpers.
- `internal/audit/git.go` — `Collect` (go-git).
- `internal/audit/audit_test.go` — rule tests; backs 4 rule invariants.
- `internal/audit/git_test.go` — `Collect` tests; backs `audit-empty-range-clean`.
- `cmd/awf/audit.go` — `runAudit`.
- `cmd/awf/audit_test.go` — backs `audit-warn-exit-zero`.

Modified (production):
- `internal/config/config.go` — `AuditConfig`, `Audit` field, `AuditSettings()`, `Validate` arm.
- `internal/config/config_test.go` — settings/defaults/validation coverage.
- `internal/project/project.go` — `Project.Audit`.
- `internal/project/audit_test.go` — `Project.Audit` integration coverage.
- `cmd/awf/main.go` — `audit` dispatch + usage line.
- `cmd/awf/main_test.go` — usage/dispatch coverage.

Modified (dogfood/prose):
- `.awf/config.yaml` — `audit:` block.
- `.awf/agents-doc.yaml` — `awf audit` command row.
- `templates/catalog.yaml` — `run-audit` in `reviewing-impl` sections.
- `templates/skills/reviewing-impl/SKILL.md.tmpl` — `run-audit` marker block.
- `x` — `audit` target.
- Re-synced: `.claude/skills/awf-reviewing-impl/SKILL.md`, `AGENTS.md`, `.awf/awf.lock`.

Modified (status, final phase):
- `docs/decisions/0017-process-conformance-audit.md` (Accepted → Implemented), regenerated
  `ACTIVE.md` + `docs/domains/{tooling,config}.md`.

---

## Phase 1 — Dependency + git range collection

- [ ] **Add the dependency.** Run:
  ```
  go get github.com/go-git/go-git/v5@latest
  ```
  Expected: `go.mod` gains a `require github.com/go-git/go-git/v5 vX.Y.Z` line (direct).

- [ ] **Create `internal/audit/audit.go`** with the neutral types (rules come in Phase 3; this
  compiles standalone):
  ```go
  // Package audit reports workflow-conformance findings over a branch's git
  // history. It is advisory (ADR-0017): standalone, never wired into the gate.
  package audit

  // Severity ranks a finding. Only Error findings make the command exit non-zero.
  type Severity int

  const (
  	Warning Severity = iota
  	Error
  )

  func (s Severity) String() string {
  	if s == Error {
  		return "error"
  	}
  	return "warning"
  }

  // Action is how a file changed in a commit.
  type Action int

  const (
  	Added Action = iota
  	Modified
  	Deleted
  )

  // FileChange is one file touched by a commit. OldText/NewText are populated
  // only for ".md" files (cheap; the rules need ADR frontmatter), empty otherwise.
  type FileChange struct {
  	Path             string // repo-relative path (the new path; old path for a delete)
  	OldPath          string // repo-relative pre-image path (differs only on rename)
  	Action           Action
  	Added, Deleted   int
  	OldText, NewText string
  }

  // Commit is a neutral view of one range commit. The rule engine reads only this.
  type Commit struct {
  	Hash    string
  	Subject string
  	Body    string
  	IsMerge bool
  	Changes []FileChange
  }

  // Finding is one reported conformance issue.
  type Finding struct {
  	Severity Severity
  	Rule     string
  	Commit   string // short hash, "" for a branch-level finding
  	Subject  string
  	Detail   string
  }

  // Inputs are the resolved settings + layout the rules need.
  type Inputs struct {
  	BaseBranch          string
  	AllowedTypes        []string // empty = accept any
  	AllowedScopes       []string // empty = accept any
  	SubjectMaxLength    int      // 0 = skip the length sub-check
  	DependencyManifests []string // empty = dependency-adr off
  	DiffThreshold       int      // 0 = plan-for-large-change off
  	GeneratedPaths      map[string]bool
  	ADRDir              string // e.g. "docs/decisions"
  	ActiveMd            string // e.g. "docs/decisions/ACTIVE.md"
  	PlansDir            string // e.g. "docs/plans"
  }

  // Run collects the branch range and evaluates the rules.
  func Run(repoRoot string, in Inputs) ([]Finding, error) {
  	commits, err := Collect(repoRoot, in.BaseBranch)
  	if err != nil {
  		return nil, err
  	}
  	return evaluate(commits, in), nil
  }
  ```

- [ ] **Create `internal/audit/git.go`** (the sole go-git caller):
  ```go
  package audit

  import (
  	"fmt"
  	"strings"

  	"github.com/go-git/go-git/v5"
  	"github.com/go-git/go-git/v5/plumbing"
  	"github.com/go-git/go-git/v5/plumbing/object"
  )

  // Collect returns the commits reachable from HEAD but not from baseBranch,
  // as neutral Commit values. Empty range -> nil. Not-a-repo, an unresolvable
  // base, and unrelated histories are errors.
  func Collect(repoRoot, baseBranch string) ([]Commit, error) {
  	repo, err := git.PlainOpen(repoRoot)
  	if err != nil {
  		return nil, fmt.Errorf("open repo: %w", err)
  	}
  	headRef, err := repo.Head()
  	if err != nil {
  		return nil, fmt.Errorf("resolve HEAD: %w", err)
  	}
  	headCommit, err := repo.CommitObject(headRef.Hash())
  	if err != nil {
  		return nil, err
  	}
  	baseHash, err := repo.ResolveRevision(plumbing.Revision(baseBranch))
  	if err != nil {
  		return nil, fmt.Errorf("resolve base %q: %w", baseBranch, err)
  	}
  	baseCommit, err := repo.CommitObject(*baseHash)
  	if err != nil {
  		return nil, err
  	}
  	bases, err := headCommit.MergeBase(baseCommit)
  	if err != nil {
  		return nil, err
  	}
  	if len(bases) == 0 {
  		return nil, fmt.Errorf("HEAD and base %q have unrelated histories", baseBranch)
  	}
  	// Prune the HEAD walk by everything reachable from base.
  	seen := map[plumbing.Hash]bool{}
  	if ferr := object.NewCommitPreorderIter(baseCommit, nil, nil).
  		ForEach(func(c *object.Commit) error { seen[c.Hash] = true; return nil }); ferr != nil {
  		return nil, ferr
  	}
  	if seen[headCommit.Hash] {
  		return nil, nil // HEAD already in base: empty range
  	}
  	var commits []Commit
  	err = object.NewCommitPreorderIter(headCommit, seen, nil).ForEach(func(c *object.Commit) error {
  		nc, cerr := toCommit(c)
  		if cerr != nil {
  			return cerr
  		}
  		commits = append(commits, nc)
  		return nil
  	})
  	if err != nil {
  		return nil, err
  	}
  	return commits, nil
  }

  func toCommit(c *object.Commit) (Commit, error) {
  	subject, body := splitMessage(c.Message)
  	nc := Commit{
  		Hash:    c.Hash.String()[:8],
  		Subject: subject,
  		Body:    body,
  		IsMerge: c.NumParents() > 1,
  	}
  	curTree, err := c.Tree()
  	if err != nil {
  		return Commit{}, err
  	}
  	var parentTree *object.Tree
  	if c.NumParents() > 0 {
  		parent, perr := c.Parent(0)
  		if perr != nil {
  			return Commit{}, perr
  		}
  		if parentTree, perr = parent.Tree(); perr != nil {
  			return Commit{}, perr
  		}
  	}
  	changes, err := object.DiffTree(parentTree, curTree)
  	if err != nil {
  		return Commit{}, err
  	}
  	patch, err := changes.Patch()
  	if err != nil {
  		return Commit{}, err
  	}
  	stats := map[string]object.FileStat{}
  	for _, s := range patch.Stats() {
  		stats[s.Name] = s
  	}
  	for _, ch := range changes {
  		fc, ferr := toFileChange(ch, parentTree, curTree, stats)
  		if ferr != nil {
  			return Commit{}, ferr
  		}
  		nc.Changes = append(nc.Changes, fc)
  	}
  	return nc, nil
  }

  func toFileChange(ch *object.Change, parentTree, curTree *object.Tree, stats map[string]object.FileStat) (FileChange, error) {
  	action, err := ch.Action()
  	if err != nil {
  		return FileChange{}, err
  	}
  	fc := FileChange{OldPath: ch.From.Name, Path: ch.To.Name}
  	switch action.String() {
  	case "Insert":
  		fc.Action = Added
  		fc.Path = ch.To.Name
  	case "Delete":
  		fc.Action = Deleted
  		fc.Path = ch.From.Name
  	default:
  		fc.Action = Modified
  	}
  	if s, ok := stats[fc.Path]; ok {
  		fc.Added, fc.Deleted = s.Addition, s.Deletion
  	}
  	if strings.HasSuffix(fc.Path, ".md") {
  		if fc.Action != Added && parentTree != nil {
  			fc.OldText = fileText(parentTree, ch.From.Name)
  		}
  		if fc.Action != Deleted {
  			fc.NewText = fileText(curTree, ch.To.Name)
  		}
  	}
  	return fc, nil
  }

  func fileText(tree *object.Tree, name string) string {
  	f, err := tree.File(name)
  	if err != nil {
  		return ""
  	}
  	s, err := f.Contents()
  	if err != nil {
  		return ""
  	}
  	return s
  }

  func splitMessage(msg string) (subject, body string) {
  	msg = strings.ReplaceAll(msg, "\r\n", "\n")
  	if i := strings.IndexByte(msg, '\n'); i >= 0 {
  		return strings.TrimRight(msg[:i], " "), strings.TrimSpace(msg[i+1:])
  	}
  	return strings.TrimRight(msg, " "), ""
  }
  ```

- [ ] **Verify it builds.** Run `go build ./...` — expected: no output (success). If a go-git
  signature differs from the above (`MergeBase`, `DiffTree`, `FileStat`, `Action().String()`),
  adjust the call to the actual v5 API; the neutral types and the rest of the plan are unaffected.

- [ ] **Commit Phase 1:**
  ```
  git add go.mod go.sum internal/audit/audit.go internal/audit/git.go
  git commit -m "feat(awf): add internal/audit git range collection (ADR-0017)"
  ```
  (`./x gate` runs via the hook; coverage for `git.go`/`Run` lands in Phase 3–4 tests. If the hook's
  coverage gate fails on this intermediate commit, fold Phase 1 into the Phase 3 commit instead —
  prefer one green commit over a red intermediate.)

> Note: because the 100% coverage gate is per-commit, Phases 1–4 may need to land as **one** commit
> if intermediate commits cannot reach 100%. Author the code phase-by-phase but stage/commit at the
> first point the gate is green (typically after Phase 4 tests). Each "Commit Phase N" step below is
> a checkpoint; collapse adjacent ones when the gate demands it.

## Phase 2 — Config block

- [ ] **Edit `internal/config/config.go`** — add the block, field, and accessor. Add near the
  existing `InvariantConfig`:
  ```go
  // AuditConfig tunes `awf audit` (ADR-0017). A nil *AuditConfig means all
  // defaults; within it, a nil slice means "use the default", an explicit empty
  // slice means "accept any / disabled" per field (see AuditSettings).
  type AuditConfig struct {
  	BaseBranch          string   `yaml:"baseBranch"`
  	AllowedTypes        []string `yaml:"allowedTypes"`
  	AllowedScopes       []string `yaml:"allowedScopes"`
  	SubjectMaxLength    *int     `yaml:"subjectMaxLength"`
  	DependencyManifests []string `yaml:"dependencyManifests"`
  	DiffThreshold       *int     `yaml:"diffThreshold"`
  }

  // AuditSettings resolves the effective audit settings, applying defaults.
  // Returned slices/ints are ready for internal/audit to consume directly.
  func (c *Config) AuditSettings() (baseBranch string, allowedTypes, allowedScopes, dependencyManifests []string, subjectMax, diffThreshold int) {
  	a := c.Audit
  	baseBranch = "main"
  	allowedTypes = defaultAllowedTypes()
  	dependencyManifests = defaultDependencyManifests()
  	subjectMax, diffThreshold = 72, 400
  	if a == nil {
  		return
  	}
  	if a.BaseBranch != "" {
  		baseBranch = a.BaseBranch
  	}
  	if a.AllowedTypes != nil { // explicit (incl. empty = accept any)
  		allowedTypes = a.AllowedTypes
  	}
  	allowedScopes = a.AllowedScopes // nil default = accept any
  	if a.DependencyManifests != nil {
  		dependencyManifests = a.DependencyManifests
  	}
  	if a.SubjectMaxLength != nil {
  		subjectMax = *a.SubjectMaxLength
  	}
  	if a.DiffThreshold != nil {
  		diffThreshold = *a.DiffThreshold
  	}
  	return
  }

  func defaultAllowedTypes() []string {
  	return []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}
  }

  func defaultDependencyManifests() []string {
  	return []string{
  		"go.mod", "package.json", "pyproject.toml", "setup.py", "requirements*.txt",
  		"Cargo.toml", "Gemfile", "*.gemspec", "composer.json", "pom.xml", "build.gradle",
  		"build.gradle.kts", "*.csproj", "Directory.Packages.props", "mix.exs",
  		"Package.swift", "pubspec.yaml", "*.cabal", "package.yaml",
  	}
  }
  ```
  Add `Audit *AuditConfig \`yaml:"audit"\`` to the `Config` struct.

- [ ] **Add a `Validate` arm** for the dependency-manifest globs, reusing the existing basename-glob
  check. In `Config.Validate`, after the `invariants` validation, add:
  ```go
  if c.Audit != nil {
  	for _, g := range c.Audit.DependencyManifests {
  		if err := validateBasenameGlob(g); err != nil {
  			return fmt.Errorf("audit.dependencyManifests: %w", err)
  		}
  	}
  }
  ```
  If the existing glob check is inline (not a `validateBasenameGlob` helper), extract it into that
  helper first (mechanical) so both `invariants` and `audit` call it — note this in the commit.

- [ ] **Add `internal/config/config_test.go` cases:** nil-Audit defaults; explicit-empty
  `allowedTypes`/`allowedScopes` (accept-any); explicit `baseBranch`/manifests/`*int` overrides
  including `0`; a `dependencyManifests` glob with a path separator rejected by `Validate`.

- [ ] **Commit Phase 2:**
  ```
  git add internal/config/config.go internal/config/config_test.go
  git commit -m "feat(awf): add audit config block and AuditSettings (ADR-0017)"
  ```

## Phase 3 — Rule engine (backs four rule invariants)

- [ ] **Append the rules + helpers to `internal/audit/audit.go`:**
  ```go
  import (
  	"fmt"
  	"path/filepath"
  	"regexp"
  	"strings"

  	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
  )

  var ccRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?: .+`)
  var adrNameRe = regexp.MustCompile(`^\d{4}-.+\.md$`)

  // evaluate applies every rule to the range and returns all findings.
  func evaluate(commits []Commit, in Inputs) []Finding {
  	var out []Finding
  	out = append(out, ruleConventionalCommits(commits, in)...)
  	out = append(out, ruleADRStatusCochange(commits, in)...)
  	out = append(out, ruleDependencyADR(commits, in)...)
  	out = append(out, rulePlanForLargeChange(commits, in)...)
  	return out
  }

  // invariant: audit-conventional-commits
  func ruleConventionalCommits(commits []Commit, in Inputs) []Finding {
  	var out []Finding
  	for _, c := range commits {
  		if c.IsMerge { // merges exempt (ADR-0017 constraint 2)
  			continue
  		}
  		m := ccRe.FindStringSubmatch(c.Subject)
  		if m == nil {
  			out = append(out, finding(Error, "conventional-commits", c, "subject is not Conventional Commits (type(scope)?: subject)"))
  			continue
  		}
  		if len(in.AllowedTypes) > 0 && !containsFold(in.AllowedTypes, m[1]) {
  			out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("disallowed type %q", m[1])))
  		}
  		if scope := m[3]; scope != "" && len(in.AllowedScopes) > 0 && !containsFold(in.AllowedScopes, scope) {
  			out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("disallowed scope %q", scope)))
  		}
  		if in.SubjectMaxLength > 0 && len(c.Subject) > in.SubjectMaxLength {
  			out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("subject %d chars > %d", len(c.Subject), in.SubjectMaxLength)))
  		}
  	}
  	return out
  }

  // invariant: audit-adr-status-cochange
  func ruleADRStatusCochange(commits []Commit, in Inputs) []Finding {
  	var out []Finding
  	for _, c := range commits {
  		activeTouched := false
  		for _, ch := range c.Changes {
  			if ch.Path == in.ActiveMd {
  				activeTouched = true
  			}
  		}
  		for _, ch := range c.Changes {
  			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
  				continue
  			}
  			if statusOf(ch.NewText) == "" {
  				continue
  			}
  			if ch.Action == Added || statusOf(ch.OldText) != statusOf(ch.NewText) {
  				if !activeTouched {
  					out = append(out, finding(Error, "adr-status-cochange", c,
  						fmt.Sprintf("%s status set/changed without ACTIVE.md in the same commit", filepath.Base(ch.Path))))
  				}
  			}
  		}
  	}
  	return out
  }

  // invariant: audit-dependency-warn
  func ruleDependencyADR(commits []Commit, in Inputs) []Finding {
  	if len(in.DependencyManifests) == 0 {
  		return nil
  	}
  	var manifestCommit *Commit
  	adrTouched := false
  	for i := range commits {
  		for _, ch := range commits[i].Changes {
  			if isADRFile(ch.Path, in.ADRDir) {
  				adrTouched = true
  			}
  			if manifestCommit == nil && matchesAny(in.DependencyManifests, filepath.Base(ch.Path)) {
  				manifestCommit = &commits[i]
  			}
  		}
  	}
  	if manifestCommit != nil && !adrTouched {
  		return []Finding{finding(Warning, "dependency-adr", *manifestCommit,
  			"dependency manifest changed on this branch with no ADR touched — if a dependency was added, confirm an ADR covers it")}
  	}
  	return nil
  }

  // invariant: audit-plan-threshold-warn
  func rulePlanForLargeChange(commits []Commit, in Inputs) []Finding {
  	if in.DiffThreshold <= 0 {
  		return nil
  	}
  	total, planTouched := 0, false
  	for _, c := range commits {
  		for _, ch := range c.Changes {
  			if in.PlansDir != "" && underDir(ch.Path, in.PlansDir) {
  				planTouched = true
  			}
  			if in.GeneratedPaths[ch.Path] {
  				continue
  			}
  			total += ch.Added + ch.Deleted
  		}
  	}
  	if total > in.DiffThreshold && !planTouched {
  		return []Finding{{Severity: Warning, Rule: "plan-for-large-change",
  			Detail: fmt.Sprintf("branch changes %d non-generated lines (> %d) with no plan under %s", total, in.DiffThreshold, in.PlansDir)}}
  	}
  	return nil
  }

  func finding(s Severity, rule string, c Commit, detail string) Finding {
  	return Finding{Severity: s, Rule: rule, Commit: c.Hash, Subject: c.Subject, Detail: detail}
  }

  func isADRFile(path, adrDir string) bool {
  	return filepath.Dir(path) == adrDir && adrNameRe.MatchString(filepath.Base(path))
  }

  func statusOf(text string) string {
  	if text == "" {
  		return ""
  	}
  	fm, _, err := frontmatter.Split(text)
  	if err != nil || fm == "" {
  		return ""
  	}
  	var meta struct {
  		Status string `yaml:"status"`
  	}
  	if frontmatter.Parse(fm, &meta) != nil {
  		return ""
  	}
  	return meta.Status
  }

  func underDir(path, dir string) bool {
  	return path == dir || strings.HasPrefix(path, dir+"/")
  }

  func containsFold(list []string, v string) bool {
  	for _, x := range list {
  		if strings.EqualFold(x, v) {
  			return true
  		}
  	}
  	return false
  }

  func matchesAny(globs []string, base string) bool {
  	for _, g := range globs {
  		if ok, _ := filepath.Match(g, base); ok {
  			return true
  		}
  	}
  	return false
  }
  ```
  Confirm the actual `internal/frontmatter` API (`Split`, `Parse`) against `frontmatter.go`; adjust
  the `statusOf` calls to the real signatures if they differ (memory: `Split` returns the frontmatter
  block + body, `Parse` unmarshals into a struct).

- [ ] **Create `internal/audit/audit_test.go`** — table-driven `evaluate` tests constructing
  `[]Commit` directly (no git), one tag-comment per rule invariant on the relevant test:
  - `// invariant: audit-conventional-commits` — a malformed subject, a disallowed type, a disallowed
    scope, and an over-length subject each yield an `Error`; a conforming `feat(awf): ...` yields
    none; a merge commit with a non-CC subject yields none.
  - `// invariant: audit-adr-status-cochange` — a `Modified` ADR whose `status` differs across
    `OldText`/`NewText` without an `ACTIVE.md` change yields an `Error`; same flip with `ACTIVE.md`
    in `Changes` yields none; an ADR `Modified` with unchanged status (a Context edit) yields none;
    a non-ADR `.md` change yields none.
  - `// invariant: audit-dependency-warn` — a `go.mod` change with no ADR yields one `Warning`
    (severity `Warning`, not `Error`); the same with an ADR file also changed yields none; empty
    `DependencyManifests` yields none.
  - `// invariant: audit-plan-threshold-warn` — a branch over `DiffThreshold` (excluding
    `GeneratedPaths`) with no `PlansDir` file yields a `Warning`; with a plan file touched yields
    none; generated lines above the threshold but real lines below yield none; `DiffThreshold` 0
    yields none.

- [ ] **Create `internal/audit/git_test.go`** — build hermetic repos with go-git (`git.PlainInit`
  in `t.TempDir()`, `worktree.Add`/`Commit` with a fixed `object.Signature`; no `git` binary).
  Cover: a normal range (Collect returns the HEAD-only commits with correct subjects/changes/stats);
  the unrelated-histories error; a not-a-repo error; and, tagged
  `// invariant: audit-empty-range-clean`, a branch equal to base → `Collect` returns nil and
  `evaluate(nil, Inputs{})` returns no findings.

- [ ] **Run** `go test ./internal/audit/... -coverpkg=./internal/audit/...` — expected: `ok` with
  `coverage: 100.0%` for the package (add cases for any uncovered branch).

- [ ] **Commit Phase 3** (collapse with Phases 1–2 if their commits were deferred for the gate):
  ```
  git add internal/audit/
  git commit -m "feat(awf): add audit rule engine with four conformance rules (ADR-0017)"
  ```

## Phase 4 — Project method + CLI command (backs `audit-warn-exit-zero`)

- [ ] **Add `Project.Audit` to `internal/project/project.go`:**
  ```go
  // Audit runs the process-conformance audit (ADR-0017) over the branch range.
  // baseOverride wins over the configured base branch when non-empty.
  func (p *Project) Audit(baseOverride string) ([]audit.Finding, error) {
  	base, types, scopes, manifests, subjectMax, threshold := p.cfg.AuditSettings()
  	if baseOverride != "" {
  		base = baseOverride
  	}
  	lay := p.layout()
  	generated := map[string]bool{}
  	for path := range p.lock.Files {
  		generated[path] = true
  	}
  	return audit.Run(p.root, audit.Inputs{
  		BaseBranch:          base,
  		AllowedTypes:        types,
  		AllowedScopes:       scopes,
  		SubjectMaxLength:    subjectMax,
  		DependencyManifests: manifests,
  		DiffThreshold:       threshold,
  		GeneratedPaths:      generated,
  		ADRDir:              lay["adrDir"],
  		ActiveMd:            lay["activeMd"],
  		PlansDir:            lay["plansDir"],
  	})
  }
  ```
  Confirm the real field names: `p.cfg` (config), `p.lock` (`manifest.Lock` with `Files`), `p.root`,
  and that `layout()` returns a `map[string]string` with those keys (project.go ~155-165). Adjust
  accessors to match; add the `internal/audit` import.

- [ ] **Create `cmd/awf/audit.go`:**
  ```go
  package main

  import (
  	"fmt"
  	"io"

  	"github.com/hypnotox/agentic-workflows/internal/audit"
  	"github.com/hypnotox/agentic-workflows/internal/project"
  )

  func runAudit(root, base string, stdout io.Writer) error {
  	p, err := project.Open(root)
  	if err != nil {
  		return err
  	}
  	findings, err := p.Audit(base)
  	if err != nil {
  		return err
  	}
  	errors := 0
  	for _, f := range findings {
  		if f.Severity == audit.Error {
  			errors++
  		}
  		loc := f.Commit
  		if loc == "" {
  			loc = "branch"
  		}
  		fmt.Fprintf(stdout, "  %-7s %-22s %s — %s\n", f.Severity, f.Rule, loc, f.Detail)
  	}
  	if len(findings) == 0 {
  		fmt.Fprintln(stdout, "awf audit: clean")
  		return nil
  	}
  	warns := len(findings) - errors
  	if errors == 0 {
  		fmt.Fprintf(stdout, "awf audit: %d warning(s), 0 errors\n", warns)
  		return nil // warnings never set non-zero exit
  	}
  	return fmt.Errorf("awf audit: %d error(s), %d warning(s)", errors, warns)
  }
  ```

- [ ] **Wire dispatch in `cmd/awf/main.go`** — add an `audit` case alongside `check`/`invariants`,
  parsing an optional `--base <ref>` flag, and add `audit` to the usage text. Match the file's
  existing arg-parsing style (read `main.go` first).

- [ ] **Create `cmd/awf/audit_test.go`** — tagged `// invariant: audit-warn-exit-zero`: build a
  temp project + go-git repo where the range yields a warnings-only result and assert `runAudit`
  returns `nil`; build one with an `Error` finding and assert it returns non-nil. Add a clean-range
  case ("awf audit: clean", nil) and the `project.Open` error path.

- [ ] **Add `cmd/awf/main_test.go` cases** for the `audit` dispatch + usage line.

- [ ] **Run** `./x gate` — expected: `coverage: 100.0%`, `0 issues`.

- [ ] **Commit Phase 4:**
  ```
  git add internal/project/project.go internal/project/audit_test.go cmd/awf/audit.go cmd/awf/audit_test.go cmd/awf/main.go cmd/awf/main_test.go
  git commit -m "feat(awf): add awf audit command and Project.Audit (ADR-0017)"
  ```

## Phase 5 — Dogfood wiring + reviewer integration

- [ ] **Add the `audit:` block to `.awf/config.yaml`** (this repo restricts the commit scope):
  ```yaml
  audit:
      allowedScopes:
          - awf
  ```
  (All other settings keep their defaults: base `main`, the conventional type set, max 72, the broad
  manifest set, threshold 400.)

- [ ] **Add the command row to `.awf/agents-doc.yaml`** under `commands:`, after the `awf upgrade`
  row:
  ```yaml
        - cmd: awf audit
          desc: report workflow-conformance findings over the branch's commits (advisory)
  ```

- [ ] **Add the `run-audit` section to `templates/skills/reviewing-impl/SKILL.md.tmpl`**, before the
  `re-review-loop` section:
  ```
  <!-- awf:section run-audit -->
  1. **Run the process-conformance audit.** After the code-review findings are routed, run
     `{{ .prefix }}-audit` (here `{{ .vars.checkCmd }}`'s sibling `awf audit`) over the branch. Treat
     `Error` findings as blocking and `Warning` findings as advisory — surface both in the digest.
     The audit is advisory and never gates; it does not replace the gate or drift check.
  <!-- awf:end -->
  ```
  Then add `- run-audit` to the `reviewing-impl` `sections:` list in `templates/catalog.yaml`,
  immediately before `re-review-loop`.

- [ ] **Add the `audit` target to `x`** (hand-maintained, mirror the `invariants` target):
  ```
  audit) shift; exec go run ./cmd/awf audit "$@" ;;
  ```
  Match the existing `case` style in `x`; add `audit` to its usage/help string.

- [ ] **Re-render and verify:**
  ```
  ./x sync && ./x check
  ```
  Expected: `awf sync: done`, then `awf check: clean`. Confirm `.claude/skills/awf-reviewing-impl/SKILL.md`
  and `AGENTS.md` regenerated with the new section/command.

- [ ] **Smoke-test the command** on this very branch:
  ```
  ./x audit
  ```
  Expected: findings print (or "clean"); the command must not error on this branch's own history.
  If it flags a real conformance issue in this branch's commits, fix the issue (it is a true
  positive), not the rule.

- [ ] **Commit Phase 5:**
  ```
  git add .awf/config.yaml .awf/agents-doc.yaml templates/catalog.yaml templates/skills/reviewing-impl/SKILL.md.tmpl x .claude/skills/awf-reviewing-impl/SKILL.md AGENTS.md .awf/awf.lock
  git commit -m "feat(awf): dogfood awf audit and run it from the impl reviewer (ADR-0017)"
  ```

## Phase 6 — Flip ADR to Implemented

- [ ] **Confirm all six invariant slugs are backed.** Run `./x invariants` — expected:
  `awf invariants: clean` (with ADR-0017 now about to be Implemented, every tagged slug must resolve:
  `audit-conventional-commits`, `audit-adr-status-cochange`, `audit-dependency-warn`,
  `audit-plan-threshold-warn` in `internal/audit/audit_test.go`; `audit-empty-range-clean` in
  `internal/audit/git_test.go`; `audit-warn-exit-zero` in `cmd/awf/audit_test.go`).

- [ ] **Flip the status** in `docs/decisions/0017-process-conformance-audit.md`:
  `status: Accepted` → `status: Implemented`.

- [ ] **Regenerate and verify:**
  ```
  ./x sync && ./x check
  ```
  Expected: `awf check: clean` (ACTIVE.md + `docs/domains/{tooling,config}.md` regenerated;
  invariants resolve for the now-Implemented ADR).

- [ ] **Commit Phase 6:**
  ```
  git add docs/decisions/0017-process-conformance-audit.md docs/decisions/ACTIVE.md docs/domains/tooling.md docs/domains/config.md .awf/awf.lock
  git commit -m "feat(awf): mark ADR-0017 Implemented; enforce audit invariants"
  ```

- [ ] **Terminal step:** invoke `awf-reviewing-impl` against the implementation commit range.
