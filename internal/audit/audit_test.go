package audit

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

type Commit = awfgit.Commit
type FileChange = awfgit.FileChange
type Action = awfgit.Action

func ruleFindings(commits []Commit, in Inputs, rule string) []Finding {
	findings := slices.DeleteFunc(evaluate(commits, in), func(f Finding) bool { return f.Rule != rule })
	if len(findings) == 0 {
		return nil
	}
	return findings
}

func ruleConventionalCommits(commits []Commit, in Inputs) []Finding {
	return ruleFindings(commits, in, "conventional-commits")
}
func ruleADRStatusCochange(commits []Commit, in Inputs) []Finding {
	return ruleFindings(commits, in, "adr-status-cochange")
}
func ruleDependencyADR(commits []Commit, in Inputs) []Finding {
	return ruleFindings(commits, in, "dependency-adr")
}
func rulePlanForLargeChange(commits []Commit, in Inputs) []Finding {
	return ruleFindings(commits, in, "plan-for-large-change")
}
func rulePlainPunctuation(commits []Commit, in Inputs) []Finding {
	return ruleFindings(commits, in, "plain-punctuation")
}

func evaluate(commits []Commit, in Inputs) []Finding {
	evaluator := newRangeEvaluator(in)
	for _, commit := range commits {
		evaluator.observe(commit)
	}
	return evaluator.findings()
}

const (
	Added    = awfgit.Added
	Modified = awfgit.Modified
	Deleted  = awfgit.Deleted
)

// countRule returns how many findings of a given rule+rank evaluate emits.
func countRule(findings []Finding, rule string, sev severity.Rank) int {
	n := 0
	for _, f := range findings {
		if f.Rule == rule && f.Severity == sev {
			n++
		}
	}
	return n
}

// invariant: tooling/audit-and-snapshots:audit-conventional-commits (TestRuleConventionalCommits)
func TestRuleConventionalCommits(t *testing.T) {
	in := Inputs{Settings: Settings{AllowedScopes: []config.ScopeSpec{{Name: "awf"}}}}
	cases := []struct {
		name    string
		commit  Commit
		wantErr int
	}{
		{"conforming", Commit{Subject: "feat(awf): ok"}, 0},
		{"no scope is fine", Commit{Subject: "fix: also ok"}, 0},
		{"malformed", Commit{Subject: "not a conventional commit"}, 1},
		{"disallowed type", Commit{Subject: "unknown(awf): nope"}, 1},
		{"disallowed scope", Commit{Subject: "feat(core): nope"}, 1},
		{"over length", Commit{Subject: "feat(awf): " + strings.Repeat("x", 80)}, 1},
		{"merge exempt", Commit{Subject: "Merge branch 'x'", IsMerge: true}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleConventionalCommits([]Commit{tc.commit}, in), "conventional-commits", severity.Error)
			if got != tc.wantErr {
				t.Errorf("got %d errors, want %d", got, tc.wantErr)
			}
		})
	}
}

// The subject-length limit counts characters, not bytes - a multi-byte subject
// within the limit must not be flagged.
func TestSubjectLengthCountsRunes(t *testing.T) {
	s := Settings{}
	// 21 runes before the umlauts + 48 'ä' runes = 69 runes, 117 bytes.
	subject := "docs: präzisiere " + strings.Repeat("ä", 48) + " zwei"
	if got := CheckConventionalCommit(Commit{Subject: subject}, s); len(got) != 0 {
		t.Errorf("%d-rune subject flagged: %v", utf8.RuneCountInString(subject), got)
	}
	over := "docs: " + strings.Repeat("ä", 70)
	if got := CheckConventionalCommit(Commit{Subject: over}, s); len(got) != 1 {
		t.Errorf("76-rune subject not flagged: %v", got)
	}
}

func TestRuleConventionalCommitsUsesFixedPolicy(t *testing.T) {
	got := ruleConventionalCommits([]Commit{{Subject: "anything(weird-scope): " + strings.Repeat("x", 80)}}, Inputs{})
	if len(got) != 2 {
		t.Errorf("fixed policy findings = %v, want type and length errors", got)
	}
}

var proposedADR = testsupport.ADR("Proposed")
var acceptedADR = testsupport.ADR("Accepted")

func auditV1(t *testing.T, status string) string {
	t.Helper()
	doc := func(history string) string {
		return "---\nformat: current-state-v1\nstatus: " + status + "\ndate: 2026-07-20\n---\n" +
			"# ADR-0137: Audit\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n" +
			"## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n" +
			"## Alternatives Considered\n\nNone.\n\n## Status history\n\n" + history + "\n"
	}
	if status == "Proposed" {
		return doc("- 2026-07-20: Proposed")
	}
	proposedText := strings.Replace(doc("- 2026-07-20: Proposed"), "status: "+status, "status: Proposed", 1)
	proposed, err := adr.ParseV1("0137-audit.md", []byte(proposedText))
	if err != nil {
		t.Fatal(err)
	}
	return doc("- 2026-07-20: Proposed\n- 2026-07-20: Accepted; content-sha256: " + adr.ContentDigest(proposed.Sections))
}

// invariant: tooling/audit-and-snapshots:audit-adr-status-cochange (TestRuleADRStatusCochange)
func TestRuleADRStatusCochange(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", IndexMd: "docs/decisions/INDEX.md"}
	adrPath := "docs/decisions/0137-x.md"
	index := "docs/decisions/INDEX.md"
	proposedV1, acceptedV1 := auditV1(t, "Proposed"), auditV1(t, "Accepted")
	proposedV2 := strings.Replace(proposedV1, adr.V1FormatMarker, adr.V2FormatMarker, 1)
	implementingV2 := strings.Replace(proposedV2, "status: Proposed", "status: Implementing", 1)
	cases := []struct {
		name    string
		commit  Commit
		wantErr int
	}{
		{"v1 added without INDEX", Commit{Changes: []FileChange{{Path: adrPath, Action: Added, NewText: proposedV1}}}, 1},
		{"v1 flip without INDEX", Commit{Changes: []FileChange{{Path: adrPath, Action: Modified, OldText: proposedV1, NewText: acceptedV1}}}, 1},
		{"v1 flip with INDEX", Commit{Changes: []FileChange{{Path: adrPath, Action: Modified, OldText: proposedV1, NewText: acceptedV1}, {Path: index, Action: Modified}}}, 0},
		{"v2 Implementing without INDEX", Commit{Changes: []FileChange{{Path: adrPath, Action: Modified, OldText: proposedV2, NewText: implementingV2}}}, 1},
		{"v2 Implementing with INDEX", Commit{Changes: []FileChange{{Path: adrPath, Action: Modified, OldText: proposedV2, NewText: implementingV2}, {Path: index, Action: Modified}}}, 0},
		{"legacy added is outside current-state rule", Commit{Changes: []FileChange{{Path: "docs/decisions/0001-x.md", Action: Added, NewText: proposedADR}}}, 0},
		{"context edit same status", Commit{Changes: []FileChange{{Path: adrPath, Action: Modified, OldText: proposedV1, NewText: proposedV1}}}, 0},
		{"non-ADR md", Commit{Changes: []FileChange{{Path: "docs/foo.md", Action: Modified, OldText: proposedV1, NewText: acceptedV1}}}, 0},
		{"deleted ADR", Commit{Changes: []FileChange{{Path: adrPath, Action: Deleted}}}, 0},
		{"added without frontmatter", Commit{Changes: []FileChange{{Path: adrPath, Action: Added, NewText: "# no frontmatter"}}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleADRStatusCochange([]Commit{tc.commit}, in), "adr-status-cochange", severity.Error)
			if got != tc.wantErr {
				t.Errorf("got %d errors, want %d", got, tc.wantErr)
			}
		})
	}
}

// Malformed ADR frontmatter must surface as a warning instead of silently
// disabling the status rules (unparseable new text) or falsely firing them
// (unparseable old text reading as a status change).
func TestRuleADRFrontmatterUnparseable(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", IndexMd: "docs/decisions/INDEX.md"}
	adr := "docs/decisions/0001-x.md"
	bad := "---\nstatus: [unclosed\n---\n# X\n"

	newBad := Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: proposedADR, NewText: bad}}}
	fs := evaluate([]Commit{newBad}, in)
	if got := countRule(fs, "adr-frontmatter", severity.Warn); got != 1 {
		t.Errorf("unparseable new frontmatter: got %d adr-frontmatter warnings, want 1 (%v)", got, fs)
	}
	if got := countRule(fs, "adr-status-cochange", severity.Error); got != 0 {
		t.Errorf("unparseable new frontmatter must not fire cochange: %v", fs)
	}

	oldBad := Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: bad, NewText: proposedADR}}}
	fs = evaluate([]Commit{oldBad}, in)
	if got := countRule(fs, "adr-status-cochange", severity.Error); got != 0 {
		t.Errorf("unparseable old frontmatter must not read as a status change: %v", fs)
	}
}

// invariant: tooling/audit-and-snapshots:audit-dependency-warn (TestRuleDependencyADR)
func TestRuleDependencyADR(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions"}
	defaults := Inputs{ADRDir: "docs/decisions"}
	adr := FileChange{Path: "docs/decisions/0001-x.md", Action: Added, NewText: proposedADR}
	gomod := FileChange{Path: "go.mod", Action: Modified}
	cases := []struct {
		name     string
		commits  []Commit
		in       Inputs
		wantWarn int
	}{
		{"manifest no ADR", []Commit{{Changes: []FileChange{gomod}}}, in, 1},
		{"manifest with ADR", []Commit{{Changes: []FileChange{gomod, adr}}}, in, 0},
		{"no manifest", []Commit{{Changes: []FileChange{{Path: "main.go", Action: Modified}}}}, in, 0},
		{"manifest rule is unconditional", []Commit{{Changes: []FileChange{gomod}}}, Inputs{ADRDir: "docs/decisions"}, 1},
		{"nested manifest under defaults", []Commit{{Changes: []FileChange{{Path: "sub/go.mod", Action: Modified}}}}, defaults, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleDependencyADR(tc.commits, tc.in), "dependency-adr", severity.Warn)
			if got != tc.wantWarn {
				t.Errorf("got %d warnings, want %d", got, tc.wantWarn)
			}
			if countRule(ruleDependencyADR(tc.commits, tc.in), "dependency-adr", severity.Error) != 0 {
				t.Error("dependency-adr must never emit an error rank")
			}
		})
	}
}
func TestEvaluateAggregates(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions"}
	commits := []Commit{
		{Subject: "bad subject", Changes: []FileChange{{Path: "go.mod", Action: Modified}}},
	}
	got := evaluate(commits, in)
	if countRule(got, "conventional-commits", severity.Error) != 1 {
		t.Error("expected a conventional-commits error")
	}
	if countRule(got, "dependency-adr", severity.Warn) != 1 {
		t.Error("expected a dependency-adr warning")
	}
}

// TestADRRecordOf pins the tri-state the shared bytes seam must preserve
// (ADR-0130 item 5): absent frontmatter is a legitimate statusless record,
// present-but-unparseable is not ok, and a real status parses through.
func TestADRRecordOf(t *testing.T) {
	const path = "docs/decisions/0001-x.md"
	if rec, ok := adrRecordOf(path, ""); rec.HasStatus() || !ok {
		t.Error("empty text should yield a statusless record, ok")
	}
	if rec, ok := adrRecordOf(path, "# no frontmatter"); rec.HasStatus() || !ok {
		t.Error("no frontmatter should yield a statusless record, ok")
	}
	if _, ok := adrRecordOf(path, "---\nstatus: [bad yaml\n---\nx"); ok {
		t.Error("malformed frontmatter should yield ok=false")
	}
	rec, ok := adrRecordOf(path, acceptedADR)
	if rec.Status != "Accepted" || !ok {
		t.Errorf("adrRecordOf(acceptedADR) = %q, %v", rec.Status, ok)
	}
	// The seam carries the whole record, not just a status: the domain
	// co-change rules read Domains off the same parse.
	if rec.Number != "0001" {
		t.Errorf("Number = %q, want 0001 (derived from the filename)", rec.Number)
	}
}

func TestUnderDir(t *testing.T) {
	if !underDir("docs/notes", "docs/notes") {
		t.Error("exact dir should match")
	}
	if !underDir("docs/notes/x.md", "docs/notes") {
		t.Error("nested path should match")
	}
	if underDir("docs/notes-extra", "docs/notes") {
		t.Error("sibling prefix should not match")
	}
}

func adrChange(action Action, status string, domains string) FileChange {
	txt := testsupport.ADR(status, testsupport.WithDomains(strings.Split(domains, ", ")...), testsupport.WithBody("body\n"))
	return FileChange{Path: "docs/decisions/0099-x.md", Action: action, NewText: txt}
}

// invariant: tooling/audit-and-snapshots:audit-advisories-always-run (TestRulePlainPunctuation)
func TestRulePlainPunctuation(t *testing.T) {
	in := Inputs{DocsDir: "docs", Settings: Settings{},
		GeneratedPaths: map[string]bool{"docs/decisions/INDEX.md": true}}
	change := func(path, oldText, newText string) []Commit {
		return []Commit{{Hash: "abc1234", Subject: "docs(adr): x",
			Changes: []FileChange{{Path: path, Action: Modified, OldText: oldText, NewText: newText}}}}
	}
	dash, dots := "\u2014", "\u2026"
	endash, ldq := "\u2013", "\u201c"

	// One or two em dashes in a paragraph, ellipses, and curly quotes are silent.
	for name, text := range map[string]string{
		"one em dash":   "a" + dash + "b",
		"two em dashes": "a" + dash + "b" + dash + "c",
		"ellipsis":      "wait" + dots,
		"curly quote":   ldq + "quoted\u201d",
	} {
		t.Run(name, func(t *testing.T) {
			if f := rulePlainPunctuation(change("docs/x.md", "plain", text), in); len(f) != 0 {
				t.Fatalf("permitted punctuation must be silent, got %v", f)
			}
		})
	}

	// Rising en-dash count or paragraph em-dash excess warns, naming the file,
	// violation measure, and commit.
	got := rulePlainPunctuation(change("docs/decisions/0001-x.md", "plain", "a"+dash+"b"+dash+"c"+dash+"d"), in)
	// invariant: tooling/audit-and-snapshots:audit-plain-punctuation (TestRulePlainPunctuation)
	if len(got) != 1 || got[0].Rule != "plain-punctuation" || got[0].Severity != severity.Warn ||
		got[0].Commit != "abc1234" || !strings.Contains(got[0].Detail, "em-dash excess (0 to 1)") ||
		!strings.Contains(got[0].Detail, "docs/decisions/0001-x.md") {
		t.Fatalf("want 1 warning naming the file and em-dash excess, got %v", got)
	}
	multi := rulePlainPunctuation(change("docs/x.md", "plain", "a"+dash+"b"+dash+"c"+dash+"d "+endash), in)
	if len(multi) != 1 || !strings.Contains(multi[0].Detail,
		"em-dash excess (0 to 1), en-dash (U+2013) (0 to 1)") {
		t.Fatalf("want both risen measures named in stable order, got %v", multi)
	}
	// Three em dashes split across paragraphs remain restrained.
	if f := rulePlainPunctuation(change("docs/notes/p.md", "plain", "a"+dash+"b\n\nc"+dash+"d\n\ne"+dash+"f"), in); len(f) != 0 {
		t.Errorf("paragraph-local restraint should be silent, got %v", f)
	}
	// Excess is summed across paragraphs: one paragraph contributes one and a
	// second contributes two, rather than the later paragraph replacing the first.
	summed := "a" + dash + "b" + dash + "c" + dash + "d\n\n" +
		"e" + dash + "f" + dash + "g" + dash + "h" + dash + "i"
	if f := rulePlainPunctuation(change("docs/summed.md", "plain", summed), in); len(f) != 1 || !strings.Contains(f[0].Detail, "em-dash excess (0 to 3)") {
		t.Errorf("multiple violating paragraphs must contribute summed excess, got %v", f)
	}
	// Unchanged or falling violation measures are silent.
	if f := rulePlainPunctuation(change("docs/notes/p.md", "a"+dash+"b"+dash+"c"+dash+"d", "w"+dash+"x"+dash+"y"+dash+"z"), in); len(f) != 0 {
		t.Errorf("unchanged excess should be silent, got %v", f)
	}
	if f := rulePlainPunctuation(change("docs/notes/p.md", "a"+endash+"b", "plain"), in); len(f) != 0 {
		t.Errorf("a violation removal should be silent, got %v", f)
	}
	// A new file has empty OldText, so a violation in it is new.
	added := []Commit{{Hash: "d", Changes: []FileChange{{Path: "docs/x.md", Action: Added, NewText: "a " + endash + " b"}}}}
	if f := rulePlainPunctuation(added, in); len(f) != 1 || !strings.Contains(f[0].Detail, "en-dash (U+2013)") {
		t.Fatalf("want 1 warning naming the en dash on an added file, got %v", f)
	}
	// A generated path is skipped: its glyphs are its source's fault.
	if f := rulePlainPunctuation(change("docs/decisions/INDEX.md", "", "a"+dash+"b"), in); len(f) != 0 {
		t.Errorf("generated path should be skipped, got %v", f)
	}
	// Outside docsDir is skipped.
	if f := rulePlainPunctuation(change("README.md", "", "a"+dash+"b"), in); len(f) != 0 {
		t.Errorf("path outside docsDir should be skipped, got %v", f)
	}
	// A non-markdown path under docsDir is skipped: FileChange loads text only for .md.
	if f := rulePlainPunctuation(change("docs/x.txt", "", "a"+dash+"b"), in); len(f) != 0 {
		t.Errorf("non-markdown path should be skipped, got %v", f)
	}
	// A deleted file is skipped.
	deleted := []Commit{{Hash: "e", Changes: []FileChange{{Path: "docs/x.md", Action: Deleted, OldText: "a" + dash + "b"}}}}
	if f := rulePlainPunctuation(deleted, in); len(f) != 0 {
		t.Errorf("deleted file should be skipped, got %v", f)
	}
	// No audit setting suppresses this rule.
	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+endash+"b"), Inputs{DocsDir: "docs"}); len(f) != 1 {
		t.Errorf("unconditional rule returned %v", f)
	}
	// Unset DocsDir is inert.
	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+dash+"b"),
		Inputs{Settings: Settings{}}); f != nil {
		t.Errorf("unset DocsDir should be inert, got %v", f)
	}
}

// A range with no commits beyond the base yields zero findings and exits clean.
// ADR-0127 keeps this contract intact: an empty range is still reachable (a..a),
// and the new notice reports it without turning it into a finding.
// invariant: tooling/audit-and-snapshots:audit-empty-range-clean (TestCollectEmptyRangeIsClean)
func TestCollectEmptyRangeIsClean(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{"a.txt": "x"})
	findings, _, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{})
	if err != nil || len(findings) != 0 {
		t.Fatalf("findings = %#v, %v", findings, err)
	}
}

// invariant: tooling/audit-and-snapshots:audit-uncommitted-changes (TestRunEmptyRangeStillEvaluatesLiveCleanliness)
func TestRunEmptyRangeStillEvaluatesLiveCleanliness(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{"a.txt": "clean\n"})
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, count, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{Settings: Settings{}})
	if err != nil || count != 0 || len(findings) != 1 || findings[0].Rule != "uncommitted-changes" {
		t.Fatalf("findings = %#v, count = %d, err = %v", findings, count, err)
	}
}

func staleAuditRepo(t *testing.T, schema int) (gitfixture.Fixture, string) {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	lock := `{"awfVersion":"v0.18.0","schemaVersion":` + strconv.Itoa(schema) + `,"files":{}}`
	base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
		".awf/awf.lock":    lock,
		".awf/config.yaml": "prefix: test\nprofile: full\nintegrationBranch: master\ntargets: [claude]\n",
	})
	return repo, base
}

func allow(version string) string {
	return "Merge feature\n\nAWF-Allow-Version: " + version + "\nAWF-Allow-Reason: carried from a stale branch"
}
