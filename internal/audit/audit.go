// Package audit reports workflow-conformance findings over a branch's git
// history. It is advisory (ADR-0017): standalone, never wired into the gate.
// Most rules are pure over the commit range; the uncommitted-changes rule
// (ADR-0025) additionally inspects the live working tree.
package audit

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
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

// Inputs are the resolved audit settings plus the project-derived layout the rules
// need. The embedded Settings carries the resolved knobs (BaseBranch, AllowedTypes,
// AllowedScopes, SubjectMaxLength, DependencyManifests, DiffThreshold,
// DomainDocStaleness, UndocumentedDomain, UncommittedChanges), promoted so the rules
// read in.AllowedTypes etc. directly.
type Inputs struct {
	Settings
	GeneratedPaths    map[string]bool
	ADRDir            string   // e.g. "docs/decisions"
	ActiveMd          string   // e.g. "docs/decisions/ACTIVE.md"
	PlansDir          string   // e.g. "docs/plans"
	ConfiguredDomains []string // config.Domains; staleness limited to these, undocumented-domain fires outside them
	DomainsPartsDir   string   // e.g. ".awf/domains/parts"
	DomainsIndexDir   string   // e.g. "docs/domains"; rendered per-domain index dir (adr-domain-cochange)
}

// Run collects the branch range and evaluates the rules.
func Run(repoRoot string, in Inputs) ([]Finding, error) {
	commits, err := Collect(repoRoot, in.BaseBranch)
	if err != nil {
		return nil, err
	}
	findings := evaluate(commits, in)
	// The clean-working-tree rule reads live state, so it runs here (with the repo
	// root) rather than in the commit-only evaluate.
	findings = append(findings, ruleUncommittedChanges(repoRoot, in)...)
	return findings, nil
}

var ccRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?: .+`)

// evaluate applies every rule to the range and returns all findings.
func evaluate(commits []Commit, in Inputs) []Finding {
	var out []Finding
	out = append(out, ruleConventionalCommits(commits, in)...)
	out = append(out, ruleADRStatusCochange(commits, in)...)
	out = append(out, ruleADRFrontmatter(commits, in)...)
	out = append(out, ruleDependencyADR(commits, in)...)
	out = append(out, rulePlanForLargeChange(commits, in)...)
	out = append(out, ruleDomainDocStaleness(commits, in)...)
	out = append(out, ruleUndocumentedDomain(commits, in)...)
	return out
}

// ruleConventionalCommits applies the shared Conventional Commits check to every
// commit in the range.
func ruleConventionalCommits(commits []Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		out = append(out, CheckConventionalCommit(c, in.Settings)...)
	}
	return out
}

// CheckConventionalCommit validates one commit's subject against the Conventional
// Commits settings and returns any violations. It is the single definition of the
// rule — consumed by the audit range loop above and by the blocking `awf
// commit-gate` command (ADR-0036), so neither re-implements the regex, the
// type/scope allow-lists, or the subject-length limit. Merge commits are exempt.
// invariant: audit-conventional-commits
// invariant: commit-gate-shared-rule
func CheckConventionalCommit(c Commit, s Settings) []Finding {
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
		out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("disallowed scope %q", scope)))
	}
	if s.SubjectMaxLength > 0 && len(c.Subject) > s.SubjectMaxLength {
		out = append(out, finding(Error, "conventional-commits", c, fmt.Sprintf("subject %d chars > %d", len(c.Subject), s.SubjectMaxLength)))
	}
	return out
}

// ruleADRFrontmatter surfaces an ADR change whose new frontmatter does not
// parse: the status-cochange and staleness rules cannot evaluate such a change,
// so the breakage is reported instead of silently skipped.
func ruleADRFrontmatter(commits []Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			if _, ok := statusOf(ch.NewText); !ok {
				out = append(out, finding(Warning, "adr-frontmatter", c,
					filepath.Base(ch.Path)+" frontmatter does not parse; ADR status rules skipped for it"))
			}
		}
	}
	return out
}

// invariant: audit-adr-status-cochange
func ruleADRStatusCochange(commits []Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		activeTouched := false
		touched := make(map[string]bool, len(c.Changes))
		for _, ch := range c.Changes {
			if ch.Path == in.ActiveMd {
				activeTouched = true
			}
			touched[ch.Path] = true
		}
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			st, ok := statusOf(ch.NewText)
			if !ok || st == "" {
				continue // unparseable new frontmatter is ruleADRFrontmatter's finding
			}
			// An unparseable old side cannot witness a transition — skip rather
			// than read garbage as a status change.
			oldSt, oldOK := statusOf(ch.OldText)
			if ch.Action == Added || (oldOK && oldSt != st) {
				if !activeTouched {
					out = append(out, finding(Error, "adr-status-cochange", c,
						filepath.Base(ch.Path)+" status set/changed without ACTIVE.md in the same commit"))
				}
				// The same ADR frontmatter regenerates each configured domain's index;
				// require it co-changed in the same commit (ADR-0033). seen dedupes a
				// repeated domain so a missing index yields exactly one finding.
				if in.DomainsIndexDir != "" {
					seen := map[string]bool{}
					for _, d := range domainsOf(ch.NewText) {
						if !slices.Contains(in.ConfiguredDomains, d) {
							continue
						}
						idx := in.DomainsIndexDir + "/" + d + ".md"
						if seen[idx] {
							continue
						}
						seen[idx] = true
						if !touched[idx] {
							out = append(out, finding(Error, "adr-domain-cochange", c,
								filepath.Base(ch.Path)+" status set/changed without "+idx+" in the same commit"))
						}
					}
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

// invariant: audit-domain-doc-staleness
func ruleDomainDocStaleness(commits []Commit, in Inputs) []Finding {
	if !in.DomainDocStaleness {
		return nil
	}
	refreshed := map[string]bool{} // domains whose source narrative changed in range
	flagged := map[string]bool{}   // configured domains brought to Implemented in range
	for _, c := range commits {
		for _, ch := range c.Changes {
			if d, ok := domainOfPart(ch.Path, in.DomainsPartsDir); ok {
				refreshed[d] = true
			}
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			if st, ok := statusOf(ch.NewText); !ok || st != "Implemented" {
				continue
			}
			if oldSt, oldOK := statusOf(ch.OldText); ch.Action != Added && (!oldOK || oldSt == "Implemented") {
				continue // already Implemented (or unknowable old side); not a witnessed transition
			}
			for _, d := range domainsOf(ch.NewText) {
				if slices.Contains(in.ConfiguredDomains, d) {
					flagged[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range slices.Sorted(maps.Keys(flagged)) {
		if !refreshed[d] {
			out = append(out, Finding{Severity: Warning, Rule: "domain-doc-staleness",
				Detail: fmt.Sprintf("an ADR in domain %q reached Implemented but %s/%s/current-state.md was not refreshed in this range", d, in.DomainsPartsDir, d)})
		}
	}
	return out
}

// invariant: audit-undocumented-domain
func ruleUndocumentedDomain(commits []Commit, in Inputs) []Finding {
	if !in.UndocumentedDomain || len(in.ConfiguredDomains) == 0 {
		return nil
	}
	flagged := map[string]bool{}
	for _, c := range commits {
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			for _, d := range domainsOf(ch.NewText) {
				if !slices.Contains(in.ConfiguredDomains, d) {
					flagged[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range slices.Sorted(maps.Keys(flagged)) {
		out = append(out, Finding{Severity: Warning, Rule: "undocumented-domain",
			Detail: fmt.Sprintf("an ADR is tagged with domain %q, which has no domain doc — add it to config.Domains and author its current-state narrative, or drop the tag", d)})
	}
	return out
}

func domainsOf(text string) []string {
	var meta struct {
		Domains []string `yaml:"domains"`
	}
	if _, found, err := frontmatter.Parse([]byte(text), &meta); err != nil || !found {
		return nil
	}
	return meta.Domains
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

func finding(s Severity, rule string, c Commit, detail string) Finding {
	return Finding{Severity: s, Rule: rule, Commit: c.Hash, Subject: c.Subject, Detail: detail}
}

func isADRFile(path, adrDir string) bool {
	return filepath.Dir(path) == adrDir && adr.FilenameRe.MatchString(filepath.Base(path))
}

// statusOf extracts the frontmatter status; ok is false only when frontmatter
// is present but does not parse — absent frontmatter is a legitimate ("", true).
func statusOf(text string) (string, bool) {
	if text == "" {
		return "", true
	}
	var meta struct {
		Status string `yaml:"status"`
	}
	_, found, err := frontmatter.Parse([]byte(text), &meta)
	if err != nil {
		return "", false
	}
	if !found {
		return "", true
	}
	return meta.Status, true
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
