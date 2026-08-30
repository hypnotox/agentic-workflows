package audit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/severity"
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

func TestSubjectLengthCountsRunes(t *testing.T) {
	s := Settings{}
	subject := "docs: präzisiere " + strings.Repeat("ä", 48) + " zwei"
	if got := CheckConventionalCommit(Commit{Subject: subject}, s); len(got) != 0 {
		t.Errorf("%d-rune subject flagged: %v", utf8.RuneCountInString(subject), got)
	}
	over := "docs: " + strings.Repeat("ä", 70)
	if got := CheckConventionalCommit(Commit{Subject: over}, s); len(got) != 1 {
		t.Errorf("76-rune subject not flagged: %v", got)
	}
}

// invariant: tooling/audit-and-snapshots:audit-advisories-always-run (TestRulePlainPunctuation)
// invariant: tooling/audit-and-snapshots:audit-plain-punctuation (TestRulePlainPunctuation)
func TestRulePlainPunctuation(t *testing.T) {
	in := Inputs{DocsDir: "docs", GeneratedPaths: map[string]bool{"docs/INDEX.md": true}}
	change := func(path, oldText, newText string) []Commit {
		return []Commit{{Hash: "abc1234", Subject: "docs: x", Changes: []FileChange{{Path: path, Action: Modified, OldText: oldText, NewText: newText}}}}
	}
	dash, dots := "\u2014", "\u2026"
	endash, ldq := "\u2013", "\u201c"

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

	got := rulePlainPunctuation(change("docs/x.md", "plain", "a"+dash+"b"+dash+"c"+dash+"d"), in)
	if len(got) != 1 || got[0].Rule != "plain-punctuation" || got[0].Severity != severity.Warn ||
		got[0].Commit != "abc1234" || !strings.Contains(got[0].Detail, "em-dash excess (0 to 1)") {
		t.Fatalf("want one punctuation warning, got %v", got)
	}
	multi := rulePlainPunctuation(change("docs/x.md", "plain", "a"+dash+"b"+dash+"c"+dash+"d "+endash), in)
	if len(multi) != 1 || !strings.Contains(multi[0].Detail, "em-dash excess (0 to 1), en-dash (U+2013) (0 to 1)") {
		t.Fatalf("want stable measures, got %v", multi)
	}
	if f := rulePlainPunctuation(change("docs/x.md", "a"+endash+"b", "plain"), in); len(f) != 0 {
		t.Errorf("a violation removal should be silent, got %v", f)
	}
	if f := rulePlainPunctuation(change("docs/INDEX.md", "", "a"+endash+"b"), in); len(f) != 0 {
		t.Errorf("generated path should be skipped, got %v", f)
	}
	if f := rulePlainPunctuation(change("README.md", "", "a"+endash+"b"), in); len(f) != 0 {
		t.Errorf("path outside docsDir should be skipped, got %v", f)
	}
	if f := rulePlainPunctuation(change("docs/x.txt", "", "a"+endash+"b"), in); len(f) != 0 {
		t.Errorf("non-Markdown path should be skipped, got %v", f)
	}
	deleted := []Commit{{Changes: []FileChange{{Path: "docs/x.md", Action: Deleted, OldText: "a" + endash + "b"}}}}
	if f := rulePlainPunctuation(deleted, in); len(f) != 0 {
		t.Errorf("deleted file should be skipped, got %v", f)
	}
}

func TestEvaluatePreservesCommitAndRuleOrder(t *testing.T) {
	findings := evaluate([]Commit{
		{Hash: "first", Subject: "bad", Changes: []FileChange{{Path: "docs/a.md", Action: Modified, NewText: "a\u2013b"}}},
		{Hash: "second", Subject: "also bad", Changes: []FileChange{{Path: "docs/b.md", Action: Modified, NewText: "a\u2013b"}}},
	}, Inputs{DocsDir: "docs"})
	if got, want := len(findings), 4; got != want {
		t.Fatalf("findings = %v, want %d", findings, want)
	}
	for i, want := range []struct{ rule, commit string }{{"conventional-commits", "first"}, {"conventional-commits", "second"}, {"plain-punctuation", "first"}, {"plain-punctuation", "second"}} {
		if findings[i].Rule != want.rule || findings[i].Commit != want.commit {
			t.Fatalf("finding %d = %#v, want %s for %s", i, findings[i], want.rule, want.commit)
		}
	}
}

func TestUnderDir(t *testing.T) {
	if !underDir("docs/notes", "docs/notes") || !underDir("docs/notes/x.md", "docs/notes") || underDir("docs/notes-extra", "docs/notes") {
		t.Error("underDir did not preserve directory boundaries")
	}
}

// invariant: tooling/audit-and-snapshots:audit-empty-range-clean (TestCollectEmptyRangeIsClean)
func TestCollectEmptyRangeIsClean(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{"a.txt": "x"})
	findings, count, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{})
	if err != nil || count != 0 || len(findings) != 0 {
		t.Fatalf("findings = %#v, count = %d, err = %v", findings, count, err)
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
	findings, count, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{})
	if err != nil || count != 0 || len(findings) != 1 || findings[0].Rule != "uncommitted-changes" {
		t.Fatalf("findings = %#v, count = %d, err = %v", findings, count, err)
	}
}

func TestRunPropagatesRangeErrors(t *testing.T) {
	repo, _ := configuredAuditRepository(t)
	if _, _, err := Run(testContext(t), repo.Root(), "does-not-exist", "HEAD", Inputs{}); err == nil || !strings.Contains(err.Error(), "collect audit range") {
		t.Fatalf("range error = %v, want collect audit range context", err)
	}
}
