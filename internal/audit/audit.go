// Package audit reports workflow-conformance findings over a branch's git
// history. The range rules are advisory (ADR-0017): standalone, never wired into
// the gate. The shared CheckConventionalCommit rule is the exception - it is also
// consumed at commit time by the commit-gate and at plan time by `awf check`
// (ADR-0111). Most rules are pure over the commit range; the uncommitted-changes
// rule (ADR-0025) additionally inspects the live working tree.
package audit

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
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

// Run collects the caller-supplied commit range and evaluates the rules. The
// range arrives as parameters rather than Inputs fields because no config key
// supplies it (ADR-0127 Decision 3).
// It also returns the number of commits the range resolved to, so the caller can
// report the scope it evaluated rather than a bare verdict (ADR-0127 Decision 9).
func Run(repoRoot, base, head string, in Inputs) ([]Finding, int, error) {
	commits, err := Collect(repoRoot, base, head)
	if err != nil {
		return nil, 0, err
	}
	findings := evaluate(commits, in)
	// The clean-working-tree rule reads live state, so it runs here (with the repo
	// root) rather than in the commit-only evaluate.
	findings = append(findings, ruleUncommittedChanges(repoRoot, in)...)
	return findings, len(commits), nil
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
	out = append(out, ruleDomainCodeStaleness(commits, in)...)
	out = append(out, rulePlainPunctuation(commits, in)...)
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
// rule - consumed by the audit range loop above, by the blocking `awf commit-gate`
// command (ADR-0036), and by the plan-time planned-subject check
// (CheckPlannedSubject, ADR-0111) - so none re-implements the regex, the type/scope
// allow-lists, or the subject-length limit. Merge commits are exempt.
// touches-state: tooling/audit-and-snapshots:commit-gate-shared-rule - shared conventional-commit rule consumed by commit-gate; proof in commitgate_test.go
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
			if _, ok := adrRecordOf(ch.Path, ch.NewText); !ok {
				out = append(out, finding(Warning, "adr-frontmatter", c,
					filepath.Base(ch.Path)+" frontmatter does not parse; ADR status rules skipped for it"))
			}
		}
	}
	return out
}

func ruleADRStatusCochange(commits []Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		indexTouched := false
		for _, ch := range c.Changes {
			if ch.Path == in.IndexMd {
				indexTouched = true
			}
		}
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == Deleted {
				continue
			}
			rec, ok := adrRecordOf(ch.Path, ch.NewText)
			if !ok || !rec.HasStatus() || !rec.IsGoverned() {
				continue // malformed ADRs are reported separately; legacy transitions predate INDEX.md
			}
			// An unparseable old side cannot witness a transition - skip rather
			// than read garbage as a status change.
			oldRec, oldOK := adrRecordOf(ch.Path, ch.OldText)
			if ch.Action == Added || (oldOK && oldRec.Status != rec.Status) {
				if !indexTouched {
					out = append(out, finding(Error, "adr-status-cochange", c,
						filepath.Base(ch.Path)+" status set/changed without INDEX.md in the same commit"))
				}
			}
		}
	}
	return out
}

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
			if manifestCommit == nil && matchesAny(in.DependencyManifests, ch.Path) {
				manifestCommit = &commits[i]
			}
		}
	}
	if manifestCommit != nil && !adrTouched {
		return []Finding{finding(Warning, "dependency-adr", *manifestCommit,
			"dependency manifest changed on this branch with no ADR touched: if a dependency was added, confirm an ADR covers it")}
	}
	return nil
}

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

// touches-state: tooling/audit-and-snapshots:audit-domain-doc-staleness - domain-doc-staleness audit rule; proof in audit_test.go
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
			rec, ok := adrRecordOf(ch.Path, ch.NewText)
			if !ok || !rec.IsImplemented() {
				continue
			}
			if oldRec, oldOK := adrRecordOf(ch.Path, ch.OldText); ch.Action != Added && (!oldOK || oldRec.IsImplemented()) {
				continue // already Implemented (or unknowable old side); not a witnessed transition
			}
			for _, d := range rec.Domains {
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

// touches-state: tooling/audit-and-snapshots:audit-undocumented-domain - undocumented-domain audit rule; proof in audit_test.go
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
			// An unparseable record yields no domains, which is what the
			// previous bespoke parser returned too; ruleADRFrontmatter is the
			// rule that reports the parse failure itself.
			rec, _ := adrRecordOf(ch.Path, ch.NewText)
			for _, d := range rec.Domains {
				if !slices.Contains(in.ConfiguredDomains, d) {
					flagged[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range slices.Sorted(maps.Keys(flagged)) {
		out = append(out, Finding{Severity: Warning, Rule: "undocumented-domain",
			Detail: fmt.Sprintf("an ADR is tagged with domain %q, which has no domain doc: add it to config.Domains and author its current-state narrative, or drop the tag", d)})
	}
	return out
}

func ruleDomainCodeStaleness(commits []Commit, in Inputs) []Finding {
	if !in.DomainCodeStaleness || len(in.DomainPaths) == 0 {
		return nil
	}
	refreshed := map[string]bool{} // domains whose source narrative changed in range
	churned := map[string]bool{}   // domains whose declared territory changed in range
	for _, c := range commits {
		for _, ch := range c.Changes {
			if d, ok := domainOfPart(ch.Path, in.DomainsPartsDir); ok {
				refreshed[d] = true
			}
			if in.GeneratedPaths[ch.Path] {
				continue
			}
			for d, globs := range in.DomainPaths {
				if !churned[d] && matchesAny(globs, ch.Path) {
					churned[d] = true
				}
			}
		}
	}
	var out []Finding
	for _, d := range slices.Sorted(maps.Keys(churned)) {
		if !refreshed[d] {
			out = append(out, Finding{Severity: Warning, Rule: "domain-code-staleness",
				Detail: fmt.Sprintf("files in domain %q changed but %s/%s/current-state.md was not refreshed in this range: if anything meaningful changed, document it", d, in.DomainsPartsDir, d)})
		}
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

// touches-state: tooling/audit-and-snapshots:audit-plain-punctuation - plain-punctuation audit rule; proof in audit_test.go
func rulePlainPunctuation(commits []Commit, in Inputs) []Finding {
	if !in.PlainPunctuation || in.DocsDir == "" {
		return nil
	}
	var out []Finding
	for _, c := range commits {
		for _, ch := range c.Changes {
			if ch.Action == Deleted || !strings.HasSuffix(ch.Path, ".md") ||
				!underDir(ch.Path, in.DocsDir) || in.GeneratedPaths[ch.Path] {
				continue
			}
			before, after := countBanned(ch.OldText), countBanned(ch.NewText)
			var risen []string
			for r, name := range bannedProseRunes {
				if after[r] > before[r] {
					risen = append(risen, fmt.Sprintf("%s (%d to %d)", name, before[r], after[r]))
				}
			}
			if len(risen) == 0 {
				continue
			}
			slices.Sort(risen)
			out = append(out, finding(Warning, "plain-punctuation", c,
				fmt.Sprintf("%s adds typographic punctuation: %s; authored prose uses plain punctuation (a colon, semicolon, comma, or parentheses; an ASCII hyphen for a range; three periods for elision)",
					ch.Path, strings.Join(risen, ", "))))
		}
	}
	return out
}

func finding(s Severity, rule string, c Commit, detail string) Finding {
	return Finding{Severity: s, Rule: rule, Commit: c.Hash, Subject: c.Subject, Detail: detail}
}

func isADRFile(path, adrDir string) bool {
	return filepath.Dir(path) == adrDir && adr.FilenameRe.MatchString(filepath.Base(path))
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
