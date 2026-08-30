// Package audit reports conventional-commit and prose-restraint findings over a
// branch's git history. Range rules are advisory; the shared
// CheckConventionalCommit rule is also consumed at commit time by the repository
// commit hook. The uncommitted-changes rule additionally inspects the live
// working tree.
package audit

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/prosegate"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// Finding is one reported conformance issue.
type Finding struct {
	Severity severity.Rank
	Rule     string
	Commit   string // short hash, "" for a branch-level finding
	Subject  string
	Detail   string
}

// Inputs are the resolved audit settings plus the generated and prose paths the
// retained range rules need.
type Inputs struct {
	Settings
	GeneratedPaths map[string]bool
	DocsDir        string // e.g. "docs", the authored-prose root
}

// ruleUncommittedChanges flags a non-clean working tree as a branch-level Error.
// It reads live worktree state from native Git porcelain so the audit uses Git's
// own repository, global, and system ignore semantics.
func ruleUncommittedChanges(ctx context.Context, repo *awfgit.Repo) ([]Finding, error) {
	tracked, untracked, err := repo.ChangeCounts(ctx)
	if err != nil {
		return nil, err
	}
	if tracked == 0 && untracked == 0 {
		return nil, nil
	}
	return []Finding{{
		Severity: severity.Error,
		Rule:     "uncommitted-changes",
		Detail:   fmt.Sprintf("working tree not clean: %d tracked change(s), %d untracked file(s); commit or discard before concluding the implementation", tracked, untracked),
	}}, nil
}

// Run collects the caller-supplied commit range and evaluates retained range
// rules. It also returns the resolved commit count so callers can report the
// evaluated scope.
func Run(ctx context.Context, repoRoot, base, head string, in Inputs) ([]Finding, int, error) {
	repo, _, err := awfgit.OpenContaining(repoRoot)
	if err != nil {
		return nil, 0, fmt.Errorf("open repo: %w", err)
	}
	evaluator := newRangeEvaluator(in)
	count, err := repo.WalkRangeCommits(ctx, base, head, func(commit awfgit.Commit) error {
		evaluator.observe(commit)
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("collect audit range: %w", err)
	}
	live, err := ruleUncommittedChanges(ctx, repo)
	if err != nil {
		return nil, 0, fmt.Errorf("evaluate live cleanliness: %w", err)
	}
	return append(evaluator.findings(), live...), count, nil
}

var ccRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?: .+`)

// rangeEvaluator owns only the findings each retained range rule needs.
type rangeEvaluator struct {
	in                        Inputs
	conventional, punctuation []Finding
}

func newRangeEvaluator(in Inputs) *rangeEvaluator { return &rangeEvaluator{in: in} }

func (e *rangeEvaluator) observe(c awfgit.Commit) {
	e.conventional = append(e.conventional, CheckConventionalCommit(c, e.in.Settings)...)
	for _, ch := range c.Changes {
		if ch.Action == awfgit.Deleted || !strings.HasSuffix(ch.Path, ".md") || !underDir(ch.Path, e.in.DocsDir) || e.in.GeneratedPaths[ch.Path] {
			continue
		}
		before := prosegate.CountViolations(ch.OldText)
		after := prosegate.CountViolations(ch.NewText)
		var risen []string
		if after.EmDashExcess > before.EmDashExcess {
			risen = append(risen, fmt.Sprintf("em-dash excess (%d to %d)", before.EmDashExcess, after.EmDashExcess))
		}
		if after.EnDashes > before.EnDashes {
			risen = append(risen, fmt.Sprintf("en-dash (U+2013) (%d to %d)", before.EnDashes, after.EnDashes))
		}
		if len(risen) != 0 {
			slices.Sort(risen)
			e.punctuation = append(e.punctuation, finding(severity.Warn, "plain-punctuation", c, fmt.Sprintf("%s adds punctuation-restraint violations: %s; prefer ordinary punctuation and use at most two em dashes per paragraph", ch.Path, strings.Join(risen, ", "))))
		}
	}
}

func (e *rangeEvaluator) findings() []Finding {
	return append(e.conventional, e.punctuation...)
}

// CheckConventionalCommit validates one commit's subject against the Conventional
// Commits settings and returns any violations. Merge commits are exempt.
func CheckConventionalCommit(c awfgit.Commit, s Settings) []Finding {
	return checkConventionalCommit(c, s, severity.Error)
}

func checkConventionalCommit(c awfgit.Commit, s Settings, scopeSeverity severity.Rank) []Finding {
	if c.IsMerge {
		return nil
	}
	m := ccRe.FindStringSubmatch(c.Subject)
	if m == nil {
		return []Finding{finding(severity.Error, "conventional-commits", c, "subject is not Conventional Commits (type(scope)?: subject)")}
	}
	var out []Finding
	if !containsFold(defaultAllowedTypes(), m[1]) {
		out = append(out, finding(severity.Error, "conventional-commits", c, fmt.Sprintf("disallowed type %q", m[1])))
	}
	if scope := m[3]; scope != "" && len(s.AllowedScopes) > 0 && !containsFold(s.ScopeNames(), scope) {
		out = append(out, finding(scopeSeverity, "conventional-commits", c, fmt.Sprintf("disallowed scope %q", scope)))
	}
	if n := utf8.RuneCountInString(c.Subject); n > subjectMaxLength {
		out = append(out, finding(severity.Error, "conventional-commits", c, fmt.Sprintf("subject %d chars > %d", n, subjectMaxLength)))
	}
	return out
}

func finding(s severity.Rank, rule string, c awfgit.Commit, detail string) Finding {
	return Finding{Severity: s, Rule: rule, Commit: c.Hash, Subject: strings.Clone(c.Subject), Detail: detail}
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
