// Package audit reports workflow-conformance findings over a branch's git
// history. The range rules are advisory (ADR-0017): standalone, never wired into
// the gate. The shared CheckConventionalCommit rule is the exception - it is also
// consumed at commit time by `awf check staged commit` and at plan time by `awf check`
// (ADR-0111). Most rules are pure over the commit range; the uncommitted-changes
// rule (ADR-0025) additionally inspects the live working tree.
package audit

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
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

// Inputs are the resolved audit settings plus the project-derived layout the rules
// need. The embedded Settings carries the resolved knobs (AllowedTypes,
// AllowedScopes, SubjectMaxLength, DependencyManifests, DiffThreshold,
// DomainDocStaleness, DomainCodeStaleness, UndocumentedDomain, UncommittedChanges,
// PlainPunctuation), promoted so the rules read in.AllowedTypes etc. directly.
type Inputs struct {
	Settings
	GeneratedPaths    map[string]bool
	ADRDir            string   // e.g. "docs/decisions"
	DocsDir           string   // e.g. "docs"; the authored-prose root (ADRDir and PlansDir sit under it)
	IndexMd           string   // e.g. "docs/decisions/INDEX.md"
	PlansDir          string   // e.g. "docs/plans"
	ConfiguredDomains []string // config.Domains; staleness limited to these, undocumented-domain fires outside them
	DomainsPartsDir   string   // e.g. ".awf/domains/parts"
	// DomainPaths maps a configured domain to its sidecar-declared anchored
	// path globs (ADR-0077); empty = the domain-code-staleness rule is inert.
	DomainPaths map[string][]string
}

// ruleUncommittedChanges flags a non-clean working tree as a branch-level Error
// (ADR-0025). It reads live worktree state from native Git porcelain so the
// audit uses Git's own repository, global, and system ignore semantics.
// touches-state: tooling/audit-and-snapshots:audit-uncommitted-changes - uncommitted-changes live-state rule; proof in audit_test.go
func ruleUncommittedChanges(ctx context.Context, repo *awfgit.Repo, in Inputs) ([]Finding, error) {
	if !in.UncommittedChanges {
		return nil, nil
	}
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

// Run collects the caller-supplied commit range and evaluates the rules. The
// range arrives as parameters rather than Inputs fields because no config key
// supplies it (ADR-0127 Decision 3).
// It also returns the number of commits the range resolved to, so the caller can
// report the scope it evaluated rather than a bare verdict (ADR-0127 Decision 9).
func Run(ctx context.Context, repoRoot, base, head string, in Inputs) ([]Finding, int, error) {
	repo, _, err := awfgit.OpenContaining(repoRoot)
	if err != nil {
		return nil, 0, fmt.Errorf("open repo: %w", err)
	}
	op, err := newStreamingHistoryOperation(ctx, base, head, in,
		repo.WalkRangeCommits,
		func(ctx context.Context, revision string) (*revisionState, error) {
			return loadSelectedRevision(ctx, repoRoot, revision, repo.CommitEntries, repo.CommitBlobsAt)
		},
		repo.FirstParentChangedPaths,
		func(ctx context.Context) ([]Finding, error) {
			return ruleUncommittedChanges(ctx, repo, in)
		})
	if err != nil {
		return nil, 0, err
	}
	findings, err := op.run(ctx)
	if err != nil {
		return nil, 0, err
	}
	return findings, op.visited, nil
}

var ccRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?: .+`)

// rangeEvaluator owns only the summaries each ordinary range rule needs. A
// rich commit is consumed synchronously by observe and is never retained.
type rangeEvaluator struct {
	in                          Inputs
	conventional, status, front []Finding
	punctuation                 []Finding
	manifest                    *Finding
	adrTouched, planTouched     bool
	lines                       int
	refreshed, docFlagged       map[string]bool
	undocumented, codeChurned   map[string]bool
}

func newRangeEvaluator(in Inputs) *rangeEvaluator {
	return &rangeEvaluator{in: in, refreshed: map[string]bool{}, docFlagged: map[string]bool{}, undocumented: map[string]bool{}, codeChurned: map[string]bool{}}
}

func (e *rangeEvaluator) observe(c awfgit.Commit) {
	e.conventional = append(e.conventional, CheckConventionalCommit(c, e.in.Settings)...)
	indexTouched := false
	for _, ch := range c.Changes {
		indexTouched = indexTouched || ch.Path == e.in.IndexMd
	}
	for _, ch := range c.Changes {
		if isADRFile(ch.Path, e.in.ADRDir) {
			e.adrTouched = true
		}
		if e.manifest == nil && matchesAny(e.in.DependencyManifests, ch.Path) {
			f := finding(severity.Warn, "dependency-adr", c, "dependency manifest changed on this branch with no ADR touched: if a dependency was added, confirm an ADR covers it")
			e.manifest = &f
		}
		if e.in.PlansDir != "" && underDir(ch.Path, e.in.PlansDir) {
			e.planTouched = true
		}
		if !e.in.GeneratedPaths[ch.Path] {
			e.lines += ch.Added + ch.Deleted
		}
		if d, ok := domainOfPart(ch.Path, e.in.DomainsPartsDir); ok {
			e.refreshed[d] = true
		}
		if !e.in.GeneratedPaths[ch.Path] {
			for d, globs := range e.in.DomainPaths {
				if matchesAny(globs, ch.Path) {
					e.codeChurned[d] = true
				}
			}
		}
		if isADRFile(ch.Path, e.in.ADRDir) && ch.Action != awfgit.Deleted {
			rec, ok := adrRecordOf(ch.Path, ch.NewText)
			if !ok {
				e.front = append(e.front, finding(severity.Warn, "adr-frontmatter", c, filepath.Base(ch.Path)+" frontmatter does not parse; ADR status rules skipped for it"))
			}
			if ok && rec.HasStatus() && rec.IsGoverned() {
				old, oldOK := adrRecordOf(ch.Path, ch.OldText)
				if (ch.Action == awfgit.Added || (oldOK && old.Status != rec.Status)) && !indexTouched {
					e.status = append(e.status, finding(severity.Error, "adr-status-cochange", c, filepath.Base(ch.Path)+" status set/changed without INDEX.md in the same commit"))
				}
			}
			if ok && rec.IsImplemented() {
				old, oldOK := adrRecordOf(ch.Path, ch.OldText)
				if ch.Action == awfgit.Added || (oldOK && !old.IsImplemented()) {
					for _, d := range rec.Domains {
						if slices.Contains(e.in.ConfiguredDomains, d) {
							e.docFlagged[d] = true
						}
					}
				}
			}
			if ok {
				for _, d := range rec.Domains {
					if !slices.Contains(e.in.ConfiguredDomains, d) {
						e.undocumented[d] = true
					}
				}
			}
		}
		if ch.Action != awfgit.Deleted && strings.HasSuffix(ch.Path, ".md") && underDir(ch.Path, e.in.DocsDir) && !e.in.GeneratedPaths[ch.Path] && e.in.PlainPunctuation {
			before, after := countBanned(ch.OldText), countBanned(ch.NewText)
			var risen []string
			for r, name := range bannedProseRunes {
				if after[r] > before[r] {
					risen = append(risen, fmt.Sprintf("%s (%d to %d)", name, before[r], after[r]))
				}
			}
			if len(risen) != 0 {
				slices.Sort(risen)
				e.punctuation = append(e.punctuation, finding(severity.Warn, "plain-punctuation", c, fmt.Sprintf("%s adds typographic punctuation: %s; authored prose uses plain punctuation (a colon, semicolon, comma, or parentheses; an ASCII hyphen for a range; three periods for elision)", ch.Path, strings.Join(risen, ", "))))
			}
		}
	}
}

func (e *rangeEvaluator) findings() []Finding {
	var out []Finding
	out = append(out, e.conventional...)
	out = append(out, e.status...)
	out = append(out, e.front...)
	if e.manifest != nil && !e.adrTouched {
		out = append(out, *e.manifest)
	}
	if e.in.DiffThreshold > 0 && e.lines > e.in.DiffThreshold && !e.planTouched {
		out = append(out, Finding{Severity: severity.Warn, Rule: "plan-for-large-change", Detail: fmt.Sprintf("branch changes %d non-generated lines (> %d) with no plan under %s", e.lines, e.in.DiffThreshold, e.in.PlansDir)})
	}
	if e.in.DomainDocStaleness {
		for _, d := range slices.Sorted(maps.Keys(e.docFlagged)) {
			if !e.refreshed[d] {
				out = append(out, Finding{Severity: severity.Warn, Rule: "domain-doc-staleness", Detail: fmt.Sprintf("an ADR in domain %q reached Implemented but %s/%s/current-state.md was not refreshed in this range", d, e.in.DomainsPartsDir, d)})
			}
		}
	}
	if e.in.UndocumentedDomain && len(e.in.ConfiguredDomains) > 0 {
		for _, d := range slices.Sorted(maps.Keys(e.undocumented)) {
			out = append(out, Finding{Severity: severity.Warn, Rule: "undocumented-domain", Detail: fmt.Sprintf("an ADR is tagged with domain %q, which has no domain doc: add it to config.Domains and author its current-state narrative, or drop the tag", d)})
		}
	}
	if e.in.DomainCodeStaleness {
		for _, d := range slices.Sorted(maps.Keys(e.codeChurned)) {
			if !e.refreshed[d] {
				out = append(out, Finding{Severity: severity.Warn, Rule: "domain-code-staleness", Detail: fmt.Sprintf("files in domain %q changed but %s/%s/current-state.md was not refreshed in this range: if anything meaningful changed, document it", d, e.in.DomainsPartsDir, d)})
			}
		}
	}
	return append(out, e.punctuation...)
}

// CheckConventionalCommit validates one commit's subject against the Conventional
// Commits settings and returns any violations. It is the single definition of the
// rule - consumed by the audit range loop above, by the blocking `awf check staged commit`
// command (ADR-0036), and by the plan-time planned-subject check
// (CheckPlannedSubject, ADR-0111) - so none re-implements the regex, the type/scope
// allow-lists, or the subject-length limit. Merge commits are exempt.
// touches-state: tooling/audit-and-snapshots:commit-gate-shared-rule - shared conventional-commit rule consumed by check staged commit; proof in commitgate_test.go
func CheckConventionalCommit(c awfgit.Commit, s Settings) []Finding {
	return checkConventionalCommit(c, s, severity.Error)
}

// CheckPlannedSubject validates a commit subject a plan proposes with the shared policy.
func CheckPlannedSubject(subject string, s Settings) []Finding {
	return checkConventionalCommit(awfgit.Commit{Subject: subject}, s, severity.Warn)
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
	if len(s.AllowedTypes) > 0 && !containsFold(s.AllowedTypes, m[1]) {
		out = append(out, finding(severity.Error, "conventional-commits", c, fmt.Sprintf("disallowed type %q", m[1])))
	}
	if scope := m[3]; scope != "" && len(s.AllowedScopes) > 0 && !containsFold(s.ScopeNames(), scope) {
		out = append(out, finding(scopeSeverity, "conventional-commits", c, fmt.Sprintf("disallowed scope %q", scope)))
	}
	if n := utf8.RuneCountInString(c.Subject); s.SubjectMaxLength > 0 && n > s.SubjectMaxLength {
		out = append(out, finding(severity.Error, "conventional-commits", c, fmt.Sprintf("subject %d chars > %d", n, s.SubjectMaxLength)))
	}
	return out
}

func domainOfPart(path, partsDir string) (string, bool) {
	const suffix = "/current-state.md"
	rest, ok := strings.CutPrefix(path, partsDir+"/")
	if !ok || !strings.HasSuffix(rest, suffix) {
		return "", false
	}
	domain := strings.TrimSuffix(rest, suffix)
	if domain == "" || strings.Contains(domain, "/") {
		return "", false
	}
	return domain, true
}

// bannedProseRunes are the typographic punctuation substitutes the documentation
// standard bans. Each is written as an escape so this file states the rule
// without typing the glyphs it bans.
var bannedProseRunes = map[rune]string{
	'\u2014': "em-dash (U+2014)",
	'\u2013': "en-dash (U+2013)",
	'\u2026': "ellipsis (U+2026)",
	'\u2018': "left single quote (U+2018)",
	'\u2019': "right single quote (U+2019)",
	'\u201c': "left double quote (U+201C)",
	'\u201d': "right double quote (U+201D)",
}

// countBanned tallies each banned rune in s.
func countBanned(s string) map[rune]int {
	out := map[rune]int{}
	for _, r := range s {
		if _, bad := bannedProseRunes[r]; bad {
			out[r]++
		}
	}
	return out
}

func finding(s severity.Rank, rule string, c awfgit.Commit, detail string) Finding {
	return Finding{Severity: s, Rule: rule, Commit: c.Hash, Subject: c.Subject, Detail: detail}
}

// isADRFile reports whether path is a decision record directly under adrDir.
// Every non-reserved Markdown file there is a record, numbered or pending
// (ADR-0202 item 4).
func isADRFile(path, adrDir string) bool {
	return filepath.Dir(path) == adrDir && adr.FileIdentity(filepath.Base(path)) != ""
}

// adrRecordOf parses an ADR from blob text through internal/adr's bytes seam
// (ADR-0130 item 5). ok is false only when frontmatter is present but does not
// parse; absent frontmatter is a legitimate empty record, which is the
// distinction ruleADRFrontmatter reports on.
func adrRecordOf(path, text string) (adr.ADR, bool) {
	rec, _, err := adr.ParseBytes(filepath.Base(path), []byte(text))
	if err != nil {
		return adr.ADR{}, false
	}
	return rec, true
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

// matchesAny reports whether the repo-relative path matches any anchored glob.
func matchesAny(globs []string, path string) bool {
	for _, g := range globs {
		if pathglob.Match(g, path) {
			return true
		}
	}
	return false
}
