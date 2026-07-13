---
date: 2026-07-14
adrs: [111]
status: Proposed
---
# Plan: Plan-time commit-subject check

## Goal

Implement ADR-0111: a deterministic `awf check` that validates the planned commit subjects a
plan marks with a ` ```commit ` fence — length and type as hard drift, an unknown scope as a
non-failing advisory note, with a per-block `awf-ignore` opt-out. **Non-goal:** changing the
commit-time gate (`CheckConventionalCommit` stays scope=Error), adding any config key, or
migrating existing bare-fence plans.

## Architecture summary

Two commits. **Phase 1** lands the behaviour and teaches the convention, ADR-0111 staying
`Proposed`: refactor `audit.CheckConventionalCommit` into a `scopeSeverity`-parameterised core with
a new plan-time `CheckPlannedSubject` wrapper (scope→Warning); extract ` ```commit ` subjects in
`plan.ParseDir`; route them in `internal/project/check.go` — Error findings as `manifest.Drift`
from `checkPlans`, the scope Warning as a `note:` from a new `planCommitScopeNotes` wired into
`AdvisoryNotes`; update the audit doc comments; teach the fence in the plans template and the
writing-plans skill. **Phase 2** freezes: add `invariant:` proof markers to the Phase 1 tests and
production functions, add the agent-guide bullets, and flip ADR-0111 and this plan to
`Implemented`.

The two phases are separate commits because the invariant proofs must be added in the same commit
that flips the ADR to `Implemented` (a proof for a not-yet-Implemented slug is a dangling-marker
advisory), while the behaviour itself must land as one coupled commit (the dead-code and
100%-coverage gates forbid introducing `CheckPlannedSubject`/`commitSubjects` without their
first production use and tests in the same commit).

## File structure

- **Created:** none.
- **Modified:**
  - `internal/audit/audit.go`, `internal/audit/audit_test.go`
  - `internal/plan/plan.go`, `internal/plan/plan_test.go`
  - `internal/project/check.go`, `internal/project/check_test.go`
  - `templates/plans-template/template.md.tmpl`, `templates/skills/writing-plans/SKILL.md.tmpl`
  - `.awf/agents-doc.yaml`
  - `docs/decisions/0111-plan-time-commit-subject-check-via-a-commit-tagged-fence.md` (status flip, Phase 2)
  - `docs/plans/2026-07-14-plan-time-commit-subject-check.md` (status flip, Phase 2)
  - Rendered outputs from `./x sync` (both phases): `docs/plans/template.md`,
    `examples/sundial/docs/plans/template.md`, `.claude/skills/awf-writing-plans/SKILL.md`,
    `.cursor/…/awf-writing-plans` render, `AGENTS.md`, `docs/decisions/ACTIVE.md`, and both
    `.awf/awf.lock` files.
- **Deleted:** none.

## Phase 1 — Behaviour and convention (one coupled commit)

Coupled group: Tasks 1.1–1.8 share one closing commit (Task 1.9). Reason: `CheckPlannedSubject`,
`commitSubjects`, and `planCommitScopeNotes` are all reachable-and-covered only once their
consumers and tests land together — the dead-code and coverage gates fail on any intermediate
slice. ADR-0111 stays `Proposed`; no `invariant:` proof markers are added in this phase.

- [ ] **Task 1.1 — Parameterise the audit rule by scope severity.** In `internal/audit/audit.go`,
  replace the `CheckConventionalCommit` function (currently the whole body at lines 131–157) with a
  thin wrapper plus a shared core and the plan-time entry point. Also update the package doc
  comment (lines 1–4) so its "never wired into the gate" claim stays accurate.

  Package doc — replace lines 1–4:
  ```
  // Package audit reports workflow-conformance findings over a branch's git
  // history. The range rules are advisory (ADR-0017): standalone, never wired into
  // the gate. The shared CheckConventionalCommit rule is the exception — it is also
  // consumed at commit time by the commit-gate and at plan time by `awf check`
  // (ADR-0111). Most rules are pure over the commit range; the uncommitted-changes
  // rule (ADR-0025) additionally inspects the live working tree.
  ```

  Function — replace lines 131–157 with:
  ```go
  // CheckConventionalCommit validates one commit's subject against the Conventional
  // Commits settings and returns any violations. It is the single definition of the
  // rule — consumed by the audit range loop above, by the blocking `awf commit-gate`
  // command (ADR-0036), and by the plan-time planned-subject check
  // (CheckPlannedSubject, ADR-0111) — so none re-implements the regex, the type/scope
  // allow-lists, or the subject-length limit. Merge commits are exempt.
  // invariant: audit-conventional-commits
  // touches-invariant: commit-gate-shared-rule — shared conventional-commit rule consumed by commit-gate; proof in commitgate_test.go
  func CheckConventionalCommit(c Commit, s Settings) []Finding {
  	return checkConventionalCommit(c, s, Error)
  }

  // CheckPlannedSubject validates a commit subject a plan proposes (not yet
  // committed) against the same rule, but relaxes a disallowed scope to a Warning: a
  // plan may be the change that adds the scope (ADR-0111), so scope conformance is
  // advisory at plan time while length, type, and malformed shape stay hard (Error).
  func CheckPlannedSubject(subject string, s Settings) []Finding {
  	return checkConventionalCommit(Commit{Subject: subject}, s, Warning)
  }

  // checkConventionalCommit is the shared core. scopeSeverity is the severity of a
  // disallowed-scope finding: Error for the commit-time callers, Warning at plan time.
  func checkConventionalCommit(c Commit, s Settings, scopeSeverity Severity) []Finding {
  	if c.IsMerge { // merges exempt (ADR-0017 constraint 2)
  		return nil
  	}
  	m := ccRe.FindStringSubmatch(c.Subject)
  	if m == nil {
  		return []Finding{finding(Error, "conventional-commits", c, "subject is not Conventional Commits (type(scope)?: subject)")}
  	}
  	var out []Finding
  	if len(s.AllowedTypes) > 0 && !containsFold(s.AllowedTypes, m[1]) {
  		out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("disallowed type %q", m[1])))
  	}
  	if scope := m[3]; scope != "" && len(s.AllowedScopes) > 0 && !containsFold(s.ScopeNames(), scope) {
  		out = append(out, finding(scopeSeverity, "conventional-commits", c, fmt.Sprintf("disallowed scope %q", scope)))
  	}
  	if n := utf8.RuneCountInString(c.Subject); s.SubjectMaxLength > 0 && n > s.SubjectMaxLength {
  		out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("subject %d chars > %d", n, s.SubjectMaxLength)))
  	}
  	return out
  }
  ```

  Verify: `go build ./internal/audit/` succeeds.

- [ ] **Task 1.2 — Test the plan-time entry point.** In `internal/audit/audit_test.go`, add a test
  covering `CheckPlannedSubject`'s severity split (`config` is already imported). This backs nothing
  yet (markers land in Phase 2) but is required for the 100%-coverage gate on the new wrapper.
  ```go
  func TestCheckPlannedSubject(t *testing.T) {
  	s := Settings{
  		AllowedTypes:     []string{"feat", "fix"},
  		AllowedScopes:    []config.ScopeSpec{{Name: "awf"}},
  		SubjectMaxLength: 20,
  	}
  	// A disallowed scope is a Warning at plan time (a plan may add the scope).
  	if got := CheckPlannedSubject("feat(newscope): x", s); len(got) != 1 || got[0].Severity != Warning {
  		t.Fatalf("scope: want 1 Warning, got %#v", got)
  	}
  	// Length, disallowed type, and malformed shape stay Error.
  	if got := CheckPlannedSubject("feat(awf): this one is definitely over twenty", s); len(got) != 1 || got[0].Severity != Error {
  		t.Fatalf("length: want 1 Error, got %#v", got)
  	}
  	if got := CheckPlannedSubject("chore(awf): x", s); len(got) != 1 || got[0].Severity != Error {
  		t.Fatalf("type: want 1 Error, got %#v", got)
  	}
  	if got := CheckPlannedSubject("not conventional", s); len(got) != 1 || got[0].Severity != Error {
  		t.Fatalf("malformed: want 1 Error, got %#v", got)
  	}
  	// A fully valid subject yields nothing.
  	if got := CheckPlannedSubject("feat(awf): ok", s); len(got) != 0 {
  		t.Fatalf("valid: want 0, got %#v", got)
  	}
  }
  ```
  Verify: `go test ./internal/audit/` passes.

- [ ] **Task 1.3 — Extract ` ```commit ` subjects in the plan parser.** In `internal/plan/plan.go`:
  add the `CommitSubjects` field to `Plan`, populate it in `ParseDir`, and add the extraction
  helpers. `strings` is already imported.

  Add to the `Plan` struct (after the `HasFrontmatter bool` field):
  ```go
  	// CommitSubjects are the planned commit subjects a plan marks with ```commit
  	// fences (ADR-0111): the first non-empty line of each fenced block whose info
  	// string's first token is `commit` and which carries no `awf-ignore` opt-out.
  	CommitSubjects []string
  ```

  In `ParseDir`, extend the appended `Plan` literal to populate the field:
  ```go
  		plans = append(plans, Plan{
  			Filename: base, Path: path, Date: fm.Date, ADRs: fm.ADRs,
  			Status: fm.Status, HasFrontmatter: found,
  			CommitSubjects: commitSubjects(string(data)),
  		})
  ```

  Add the helpers (place after `ParseDir`):
  ```go
  // commitSubjects returns the planned commit subjects a plan marks with ```commit
  // fences (ADR-0111): for every ``` fenced block whose info string's first
  // whitespace-delimited token is `commit` and which carries no `awf-ignore` opt-out
  // token, the block's first non-empty line. An empty/whitespace-only block yields
  // nothing. Every line beginning with ``` toggles the fenced state, mirroring the
  // fence tracking in refs.WithoutFences.
  func commitSubjects(content string) []string {
  	var subjects []string
  	inFence := false
  	checked := false // the open fence is a checkable ```commit block
  	var first string // first non-empty line inside the open block
  	for _, line := range strings.Split(content, "\n") {
  		trimmed := strings.TrimSpace(line)
  		if !strings.HasPrefix(trimmed, "```") {
  			if inFence && checked && first == "" && trimmed != "" {
  				first = trimmed
  			}
  			continue
  		}
  		if inFence {
  			if checked && first != "" {
  				subjects = append(subjects, first)
  			}
  			inFence, checked, first = false, false, ""
  			continue
  		}
  		inFence = true
  		checked = isCommitInfo(trimmed[3:])
  	}
  	return subjects
  }

  // isCommitInfo reports whether a fence info string marks a checkable planned-commit
  // block: its first token is `commit` and no token is the `awf-ignore` opt-out.
  func isCommitInfo(info string) bool {
  	fields := strings.Fields(info)
  	if len(fields) == 0 || fields[0] != "commit" {
  		return false
  	}
  	for _, f := range fields[1:] {
  		if f == "awf-ignore" {
  			return false
  		}
  	}
  	return true
  }
  ```
  Verify: `go build ./internal/plan/` succeeds.

- [ ] **Task 1.4 — Test the extraction.** In `internal/plan/plan_test.go`, add a test that writes one
  plan file exercising every extraction branch and asserts `Plan.CommitSubjects`. This drives 100%
  coverage of `commitSubjects`/`isCommitInfo`; proof markers land in Phase 2.
  ```go
  // TestParseDirExtractsCommitSubjects covers commit-subject extraction: a ```commit
  // block is captured (first non-empty line, multi-line body ignored); a bare fence, a
  // language fence, a ```commit awf-ignore opt-out, and an empty ```commit block are
  // all skipped.
  func TestParseDirExtractsCommitSubjects(t *testing.T) {
  	dir := t.TempDir()
  	body := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: X\n\n" +
  		"```commit\nfeat(awf): real subject\nbody line ignored\n```\n\n" +
  		"```\nfeat(awf): bare fence not captured\n```\n\n" +
  		"```go\nfmt.Println()\n```\n\n" +
  		"```commit awf-ignore\nfeat(awf): opted-out example\n```\n\n" +
  		"```commit\n\n```\n"
  	if err := os.WriteFile(filepath.Join(dir, "2026-07-14-x.md"), []byte(body), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	plans, err := plan.ParseDir(dir)
  	if err != nil {
  		t.Fatalf("ParseDir: %v", err)
  	}
  	if len(plans) != 1 {
  		t.Fatalf("want 1 plan, got %d", len(plans))
  	}
  	got := plans[0].CommitSubjects
  	want := []string{"feat(awf): real subject"}
  	if len(got) != len(want) || got[0] != want[0] {
  		t.Fatalf("CommitSubjects = %#v, want %#v", got, want)
  	}
  }
  ```
  (Confirm `os`, `path/filepath`, and the `plan` import are present — they are, per the existing
  `TestParseDirParsesFrontmatterAndSkipsNonPlans`.)
  Verify: `go test ./internal/plan/` passes.

- [ ] **Task 1.5 — Emit length/type/shape drift from `checkPlans`.** In `internal/project/check.go`,
  add `"github.com/hypnotox/agentic-workflows/internal/audit"` to the import block. In `checkPlans`
  (starts line 719), resolve audit settings once and add the subject-drift loop.

  After the `known` map is built and before `var drift []manifest.Drift`, add:
  ```go
  	aset := audit.Resolve(p.Cfg.Audit)
  ```
  Inside `for _, pl := range plans`, after the existing `for _, n := range pl.ADRs { … }` loop and
  before the closing brace of the plan loop, add:
  ```go
  		for _, sub := range pl.CommitSubjects {
  			for _, f := range audit.CheckPlannedSubject(sub, aset) {
  				if f.Severity == audit.Error {
  					drift = append(drift, manifest.Drift{Path: path, Kind: "plan-commit-subject", Detail: f.Detail})
  				}
  			}
  		}
  ```
  Update the `checkPlans` doc comment to mention the new check (leave the two existing
  `// invariant:` markers untouched; the two new ones land in Phase 2):
  ```
  // checkPlans validates plan frontmatter, plan→ADR links, and planned commit
  // subjects over docs/plans/, scanning the YYYY-MM-DD-*.md set only (excluding
  // template.md and README.md). Frontmatter-less plans (the grandfathered corpus,
  // ADR-0098) are skipped. A ```commit subject's length/type/shape violation is
  // drift; an unknown scope is advisory (planCommitScopeNotes), not drift (ADR-0111).
  ```
  Verify: `go build ./internal/project/` succeeds.

- [ ] **Task 1.6 — Emit the scope advisory from `AdvisoryNotes`.** In `internal/project/check.go`,
  add the `planCommitScopeNotes` method and wire it into `AdvisoryNotes`.

  Replace the tail of `AdvisoryNotes` (currently `th, err := p.tagHealthNotes() … return append(notes, th...), nil`) with:
  ```go
  	th, err := p.tagHealthNotes()
  	if err != nil {
  		return nil, err
  	}
  	notes = append(notes, th...)
  	pcs, err := p.planCommitScopeNotes()
  	if err != nil {
  		return nil, err
  	}
  	return append(notes, pcs...), nil
  ```
  Add the method (place near `checkPlans`):
  ```go
  // planCommitScopeNotes returns advisory (non-failing) notes for a plan's ```commit
  // subject naming a scope outside the configured allow-list. Unlike an over-length or
  // mistyped subject (hard drift in checkPlans), an unknown scope is advisory: a plan
  // may be the change that adds the scope (ADR-0111). Mirrors checkPlans' scan; a
  // frontmatter-less plan is skipped.
  func (p *Project) planCommitScopeNotes() ([]string, error) {
  	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
  	if err != nil {
  		return nil, err
  	}
  	aset := audit.Resolve(p.Cfg.Audit)
  	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "plans"))
  	var notes []string
  	for _, pl := range plans {
  		if !pl.HasFrontmatter {
  			continue
  		}
  		for _, sub := range pl.CommitSubjects {
  			for _, f := range audit.CheckPlannedSubject(sub, aset) {
  				if f.Severity == audit.Warning {
  					notes = append(notes, fmt.Sprintf("%s/%s: planned commit %s", rel, pl.Filename, f.Detail))
  				}
  			}
  		}
  	}
  	return notes, nil
  }
  ```
  Verify: `go build ./internal/project/` succeeds.

- [ ] **Task 1.7 — Test the check wiring.** In `internal/project/check_test.go`, add three tests. The
  first drives `checkPlans` drift (and asserts a bad-scope subject produces no drift — covering the
  `Severity == Error` false branch); the second drives `planCommitScopeNotes` (a note for a bad
  scope, none for an over-length subject — covering its false branch — a frontmatter-less plan
  skipped — covering the `!HasFrontmatter` continue — and the `ParseDir` error branch); the third
  drives `AdvisoryNotes` to a `planCommitScopeNotes` error (covering its new error-propagation
  branch). Follow the fixture style of `TestCheckPlansValidatesFrontmatterAndLinks`
  (`scaffold(t, sampleYAML)`, `testsupport.WriteFile`) and, for the wiring test,
  `TestAdvisoryNotesSurfacesTagHealthError` in `notes_test.go`. Proof markers land in Phase 2.

  For a non-empty scope allow-list, use a config with an audit scope. Add this fixture constant near
  the top of the test file:
  ```go
  const commitSubjectCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: []\ndomains: []\naudit:\n  allowedScopes:\n    - name: awf\n"
  ```
  ```go
  // TestCheckPlansCommitSubjectDrift covers the ```commit length/type/shape drift and
  // confirms an unknown scope is NOT drift (it is an advisory note instead).
  func TestCheckPlansCommitSubjectDrift(t *testing.T) {
  	root := scaffold(t, commitSubjectCfg)
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	write := func(name, content string) {
  		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
  	}
  	fm := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: P\n\n"
  	long := "feat(awf): " + strings.Repeat("x", 80)
  	write("2026-07-14-long.md", fm+"```commit\n"+long+"\n```\n")
  	write("2026-07-14-type.md", fm+"```commit\nzzz(awf): bad type\n```\n")
  	write("2026-07-14-shape.md", fm+"```commit\nno conventional shape here\n```\n")
  	write("2026-07-14-scope.md", fm+"```commit\nfeat(nope): unknown scope\n```\n")
  	write("2026-07-14-ok.md", fm+"```commit\nfeat(awf): fine\n```\n")

  	drift, err := p.checkPlans()
  	if err != nil {
  		t.Fatalf("checkPlans: %v", err)
  	}
  	got := map[string]bool{}
  	for _, d := range drift {
  		if d.Kind == "plan-commit-subject" {
  			got[filepath.Base(d.Path)] = true
  		}
  	}
  	for _, name := range []string{"2026-07-14-long.md", "2026-07-14-type.md", "2026-07-14-shape.md"} {
  		if !got[name] {
  			t.Errorf("expected plan-commit-subject drift for %s, got %#v", name, drift)
  		}
  	}
  	if got["2026-07-14-scope.md"] {
  		t.Errorf("unknown scope must be advisory, not drift: %#v", drift)
  	}
  	if got["2026-07-14-ok.md"] {
  		t.Errorf("valid subject must not drift: %#v", drift)
  	}
  }

  // TestPlanCommitScopeNotes covers the scope advisory: a note for an unknown scope,
  // none for an over-length subject (Error, not Warning), and the ParseDir error branch.
  func TestPlanCommitScopeNotes(t *testing.T) {
  	root := scaffold(t, commitSubjectCfg)
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	write := func(name, content string) {
  		testsupport.WriteFile(t, filepath.Join(root, "docs/plans", name), content)
  	}
  	fm := "---\ndate: 2026-07-14\nadrs: []\nstatus: Proposed\n---\n# Plan: P\n\n"
  	write("2026-07-14-scope.md", fm+"```commit\nfeat(nope): unknown scope\n```\n")
  	write("2026-07-14-long.md", fm+"```commit\nfeat(awf): "+strings.Repeat("x", 80)+"\n```\n")
  	// A frontmatter-less plan is skipped (covers the !HasFrontmatter continue); the
  	// note count stays 1.
  	write("2026-06-24-legacy.md", "# Plan: Legacy\n\nNo frontmatter, grandfathered.\n")

  	notes, err := p.planCommitScopeNotes()
  	if err != nil {
  		t.Fatalf("planCommitScopeNotes: %v", err)
  	}
  	if len(notes) != 1 || !strings.Contains(notes[0], "2026-07-14-scope.md") || !strings.Contains(notes[0], "disallowed scope") {
  		t.Fatalf("want one scope note, got %#v", notes)
  	}

  	// A malformed plan makes ParseDir fail.
  	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
  		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
  	if _, err := p.planCommitScopeNotes(); err == nil {
  		t.Fatal("expected ParseDir error for malformed frontmatter, got nil")
  	}
  }

  // TestAdvisoryNotesSurfacesPlanCommitError covers the planCommitScopeNotes error
  // propagation wired into AdvisoryNotes. Empty tags keep tagHealthNotes inert (so it
  // does not error first); a malformed plan makes planCommitScopeNotes' ParseDir fail.
  func TestAdvisoryNotesSurfacesPlanCommitError(t *testing.T) {
  	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: []\n")
  	testsupport.WriteFile(t, filepath.Join(root, "docs/plans/2026-07-14-broken.md"),
  		"---\nstatus: [unterminated\n---\n# Plan: Broken\n")
  	p, err := Open(root)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if _, err := p.AdvisoryNotes(); err == nil {
  		t.Fatal("expected AdvisoryNotes to surface the plan-commit ParseDir error")
  	}
  }
  ```
  (Confirm `strings`, `path/filepath`, and `testsupport` are imported in `check_test.go` — they are.)
  Verify: `go test ./internal/project/` passes.

- [ ] **Task 1.8 — Teach the ` ```commit ` fence in the template and skill.** Two edits, both in
  prose using inline code (no literal ` ```commit ` fenced block, so a scaffolded plan carries no
  checkable block and the `plans-template-taxonomy` golden guard — no `{{`/`}}`, no bare "the gate" —
  stays green).

  `templates/plans-template/template.md.tmpl`, Task 1.2 line (the "Verify and commit" task) — replace
  its first sentence so it reads:
  ```
  - [ ] **Task 1.2 — Verify and commit.** Run {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the project's gate{{ end }}; `git add` the exact paths; commit with a
    Conventional-Commits subject, placed in a fenced code block tagged `commit` so `awf check`
    validates its length and type before you commit (tag a display-only example `commit awf-ignore`
    to skip it). Every phase's closing commit passes {{ with .vars.gateCmd }}`{{ . }}`{{ else }}the gate{{ end }} on its own — unless the
    change genuinely cannot be sliced, in which case mark the coupled phases and share one closing
    commit, stating why.
  ```

  `templates/skills/writing-plans/SKILL.md.tmpl`, in the `conventions-tasks` section — insert a new
  bullet immediately after the `- **Tasks:** …` bullet (line 29):
  ```
  - **Commit subjects in a `commit` fence:** write each phase's closing-commit subject in a fenced code block tagged `commit`, so `awf check` validates its length and type at plan time — a subject too long is caught before implementation, not at review. Tag a display-only commit example `commit awf-ignore` to exclude it from the check.
  ```
  Verify: `go test ./internal/project/ -run TestSyncGolden` passes.

- [ ] **Task 1.9 — Sync, verify, and commit Phase 1.** Run `./x sync` (re-renders
  `docs/plans/template.md`, `examples/sundial/docs/plans/template.md`, the `.claude`/`.cursor`
  `awf-writing-plans` skill renders, and both `awf.lock` files). Run `./x gate` — expect
  `0 issues.` and `coverage: 100.0%`. Run `./x check` — expect `awf check: clean`. Stage exactly the
  eight edited sources plus every file `./x sync` changed (`git status --short` to confirm nothing
  unrelated), then commit:

  ```commit
  feat(plans): add plan-time commit-subject check
  ```

  Commit body: name the `scopeSeverity` refactor, the `commitSubjects` extraction, the
  drift/advisory split, and the template/skill guidance; note ADR-0111 stays Proposed until Phase 2.

## Phase 2 — Back the invariants and freeze

- [ ] **Task 2.1 — Add the invariant proof markers.** Add a `// invariant: <slug>` marker to each
  Phase 1 test and to its production function, matching the existing check pattern (marker on both
  the function and its backing test, e.g. `checkPlans` at `check.go:717`).
  - `internal/plan/plan.go`: above `commitSubjects`, add
    `// invariant: plan-commit-subject-marker-scoped` and
    `// invariant: plan-commit-subject-optout-honored`.
  - `internal/plan/plan_test.go`: above `TestParseDirExtractsCommitSubjects`, add the same two
    markers.
  - `internal/project/check.go`: in the `checkPlans` doc comment, add
    `// invariant: plan-commit-subject-length-checked` and
    `// invariant: plan-commit-subject-shape-checked` (after the two existing plan markers); above
    `planCommitScopeNotes`, add `// invariant: plan-commit-subject-scope-advisory`.
  - `internal/project/check_test.go`: above `TestCheckPlansCommitSubjectDrift`, add
    `// invariant: plan-commit-subject-length-checked` and
    `// invariant: plan-commit-subject-shape-checked`; above `TestPlanCommitScopeNotes`, add
    `// invariant: plan-commit-subject-scope-advisory`.

  Verify: `go build ./...` succeeds (markers are comments; still `Proposed`, so not yet enforced).

- [ ] **Task 2.2 — Add the agent-guide invariant bullets.** In `.awf/agents-doc.yaml`, after the
  `plan-adr-link-resolved` bullet (line 136), insert:
  ```yaml
        - ref: ADR-0111
          text: '**Planned commit subject checked.** `awf check` fails a plan whose ` ```commit ` fence first non-empty line is over the resolved `audit.SubjectMaxLength` or is malformed / a disallowed type; an unknown scope is a non-failing `note:` instead, since a plan may add the scope (`invariant: plan-commit-subject-length-checked`, `invariant: plan-commit-subject-shape-checked`, `invariant: plan-commit-subject-scope-advisory`).'
        - ref: ADR-0111
          text: '**Commit-fence marker scoped.** Only a fence whose info string''s first token is `commit` and which lacks the `awf-ignore` opt-out is read as a planned subject; bare fences, other info strings, empty blocks, and `template.md` are never validated (`invariant: plan-commit-subject-marker-scoped`, `invariant: plan-commit-subject-optout-honored`).'
  ```

- [ ] **Task 2.3 — Flip ADR-0111 and this plan to `Implemented`, sync, verify, and commit.** In
  `docs/decisions/0111-plan-time-commit-subject-check-via-a-commit-tagged-fence.md`, change
  `status: Proposed` → `status: Implemented`. In this plan file, change `status: Proposed` →
  `status: Implemented` and append any implementation findings to a `## Notes` section. Run
  `./x sync` (regenerates `AGENTS.md`, `docs/decisions/ACTIVE.md`, and both `awf.lock` files). Run
  `./x gate` — expect `0 issues.` and `coverage: 100.0%`. Run `./x check` — expect `awf check: clean`
  and `awf invariants: clean` (the five slugs now enforced and backed). Stage the marker edits, the
  agents-doc edit, both status flips, and every synced file, then commit:

  ```commit
  docs(invariants): back and freeze ADR-0111 commit-subject invariants
  ```

## Verification

- `./x check` prints `awf check: clean` and `awf invariants: clean`; `./x gate` prints
  `coverage: 100.0%` and `0 issues.`
- A plan under `docs/plans/` with a ` ```commit ` fence over 72 chars fails `awf check` with a
  `plan-commit-subject` drift; the same subject under `commit awf-ignore` produces no finding.
- A ` ```commit ` subject with an unknown scope surfaces one `note:` line and does not fail the
  check.
- `git grep -n "CheckPlannedSubject"` shows the commit gate (`cmd/awf/commitgate.go`) still calls
  `CheckConventionalCommit`, unchanged (scope=Error).
- The rendered `docs/plans/template.md` mentions the `commit`-tagged fence in prose and contains no
  literal ` ``` ` fenced block, no `{{`/`}}`, and no bare "the gate".

## Notes

Out of scope: migrating the existing bare-fence plan corpus (grandfathered by the presence trigger),
adding a config key (the check reuses `audit.SubjectMaxLength`/`allowedScopes`/`allowedTypes`), and
supporting `~~~` fences (commit subjects use ` ``` ` only — dropping `~~~` avoids an uncovered
branch). This plan's own Task 1.9 / 2.3 commit tasks use ` ```commit ` fences, so once Phase 1 lands
they are self-validated by the new check.
