// Package audit reports workflow-conformance findings over a branch's git
// history. It is advisory (ADR-0017): standalone, never wired into the gate.
package audit

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

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
						filepath.Base(ch.Path)+" status set/changed without ACTIVE.md in the same commit"))
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
	var meta struct {
		Status string `yaml:"status"`
	}
	if _, found, err := frontmatter.Parse([]byte(text), &meta); err != nil || !found {
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
