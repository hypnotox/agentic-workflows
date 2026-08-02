// Package audit reports workflow-conformance findings over a branch's git
// history. The range rules are advisory (ADR-0017): standalone, never wired into
// the gate. The shared CheckConventionalCommit rule is the exception - it is also
// consumed at commit time by `awf check staged commit` and at plan time by `awf check`
// (ADR-0111). Most rules are pure over the commit range; the uncommitted-changes
// rule (ADR-0025) additionally inspects the live working tree.
package audit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/commitmsg"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
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
	commits, err := repo.RangeCommits(ctx, base, head)
	if err != nil {
		return nil, 0, err
	}
	findings := evaluate(commits, in)
	staleMergeFindings, err := replayStaleMergeAuthorizations(ctx, repoRoot, repo, commits)
	if err != nil {
		return nil, 0, err
	}
	findings = append(findings, staleMergeFindings...)
	// The clean-working-tree rule reads live state, so it runs here rather than
	// in the commit-only evaluate.
	liveFindings, err := ruleUncommittedChanges(ctx, repo, in)
	if err != nil {
		return nil, 0, err
	}
	findings = append(findings, liveFindings...)
	return findings, len(commits), nil
}

var ccRe = regexp.MustCompile(`^([a-zA-Z]+)(\(([^)]+)\))?(!)?: .+`)

// replayStaleMergeAuthorizations applies the live stale-merge evidence policy
// to historical merge commits once their result lock reaches schema generation 31.
func replayStaleMergeAuthorizations(ctx context.Context, repoRoot string, repo *awfgit.Repo, commits []awfgit.Commit) ([]Finding, error) {
	var findings []Finding
	for _, commit := range commits {
		if !commit.IsMerge {
			continue
		}
		resultTree, err := snapshot.CommitTree(ctx, repo, commit.Revision)
		if err != nil {
			return nil, fmt.Errorf("load merge result %s: %w", commit.Hash, err)
		}
		lock, found, err := auditLockFromTree(resultTree)
		if err != nil {
			return nil, fmt.Errorf("load merge result lock %s: %w", commit.Hash, err)
		}
		if !found || lock.SchemaVersion < 31 {
			continue
		}
		current, _ := adr.FormatAtGeneration(lock.SchemaVersion)
		if len(commit.Parents) < 2 {
			return nil, fmt.Errorf("merge %s has fewer than two parents", commit.Hash)
		}
		parentTrees, err := snapshot.CommitTrees(ctx, repo, commit.Parents)
		if err != nil {
			return nil, fmt.Errorf("load merge parents %s: %w", commit.Hash, err)
		}
		result, err := auditUniverse(repoRoot, resultTree)
		if err != nil {
			return nil, fmt.Errorf("load merge result current state %s: %w", commit.Hash, err)
		}
		first, err := auditUniverse(repoRoot, parentTrees[0])
		if err != nil {
			return nil, fmt.Errorf("load merge first parent current state %s: %w", commit.Hash, err)
		}
		incoming := make([]currentstate.Universe, len(parentTrees)-1)
		for i, tree := range parentTrees[1:] {
			incoming[i], err = auditUniverse(repoRoot, tree)
			if err != nil {
				return nil, fmt.Errorf("load merge incoming parent current state %s: %w", commit.Hash, err)
			}
		}
		authorizations, err := commitmsg.ParseAuthorizations(commitmsg.Clean([]byte(commit.Message)), func(value string) bool {
			return value == "legacy" || adr.KnownFormatMarker(value)
		})
		if err != nil {
			syntax, syntaxErr := staleAuthorizationSyntax(err)
			if syntaxErr != nil { // coverage-ignore: ParseAuthorizations returns only *SyntaxError; the checked fallback protects future implementations
				return nil, syntaxErr
			}
			findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
				fmt.Sprintf("malformed reserved trailer at cleaned line %d: %s", syntax.Line, syntax.Reason)))
			continue
		}
		allowed := map[string]bool{}
		for _, authorization := range authorizations {
			allowed[authorization.Version] = true
		}
		for _, qualification := range currentstate.QualifyIncoming(first, result, incoming, current) {
			identity := "ADR-" + qualification.Introduction.Identity
			if !qualification.Qualified {
				findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
					"unqualified incoming-parent record "+identity))
				continue
			}
			version := adr.FormatMarker(qualification.Introduction.Format)
			if version == "" {
				version = "legacy"
			}
			if !allowed[version] {
				findings = append(findings, finding(severity.Error, "stale-merge-authorization", commit,
					"missing authorization version "+version+" for "+identity))
			}
		}
	}
	return findings, nil
}

func staleAuthorizationSyntax(err error) (*commitmsg.SyntaxError, error) {
	var syntax *commitmsg.SyntaxError
	if !errors.As(err, &syntax) {
		return nil, fmt.Errorf("parse stale merge authorizations: %w", err)
	}
	return syntax, nil
}

func auditLockFromTree(tree *snapshot.Tree) (*manifest.Lock, bool, error) {
	file, ok := tree.Lookup(config.DirName + "/awf.lock")
	if !ok {
		return nil, false, nil
	}
	if !file.Scannable() {
		return nil, true, fmt.Errorf("%s/awf.lock is not a scannable file", config.DirName)
	}
	lock, err := manifest.Parse(file.Bytes)
	return lock, true, err
}

func auditUniverse(root string, tree *snapshot.Tree) (currentstate.Universe, error) {
	file, ok := tree.Lookup(config.DirName + "/config.yaml")
	if !ok {
		return currentstate.Universe{}, nil
	}
	if !file.Scannable() {
		return currentstate.Universe{}, fmt.Errorf("%s/config.yaml is not a scannable file", config.DirName)
	}
	lock, _, err := auditLockFromTree(tree)
	if err != nil {
		return currentstate.Universe{}, err
	}
	schema := migrate.Current()
	if lock != nil {
		schema = lock.SchemaVersion
	}
	data, err := migrate.ConfigForCurrentSchema(file.Bytes, schema)
	if err != nil {
		return currentstate.Universe{}, err
	}
	cfg, err := config.ParseTree(config.RootDir(root), data, auditSnapshotReader{tree})
	if err != nil {
		return currentstate.Universe{}, err
	}
	if err := cfg.Validate(); err != nil {
		return currentstate.Universe{}, err
	}
	loaded, err := currentstate.LoadFromTree(tree, cfg)
	if err != nil {
		return currentstate.Universe{}, err
	}
	return loaded.Universe(), nil
}

type auditSnapshotReader struct{ tree *snapshot.Tree }

func (r auditSnapshotReader) ReadFile(path string) ([]byte, bool) {
	file, ok := r.tree.Lookup(config.DirName + "/" + filepath.ToSlash(path))
	if !ok || !file.Scannable() {
		return nil, false
	}
	return slices.Clone(file.Bytes), true
}

func (r auditSnapshotReader) Paths(prefix string) []string {
	full := config.DirName + "/" + filepath.ToSlash(prefix)
	var paths []string
	for _, file := range r.tree.List() {
		if file.Scannable() && strings.HasPrefix(file.Path, full) {
			paths = append(paths, strings.TrimPrefix(file.Path, config.DirName+"/"))
		}
	}
	return paths
}

// evaluate applies every rule to the range and returns all findings.
func evaluate(commits []awfgit.Commit, in Inputs) []Finding {
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
func ruleConventionalCommits(commits []awfgit.Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		out = append(out, CheckConventionalCommit(c, in.Settings)...)
	}
	return out
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

// CheckPlannedSubject validates a commit subject a plan proposes (not yet
// committed) against the same rule, but relaxes a disallowed scope to a warn rank: a
// plan may be the change that adds the scope (ADR-0111), so scope conformance is
// advisory at plan time while length, type, and malformed shape stay hard (error).
func CheckPlannedSubject(subject string, s Settings) []Finding {
	return checkConventionalCommit(awfgit.Commit{Subject: subject}, s, severity.Warn)
}

// checkConventionalCommit is the shared core. scopeSeverity is the rank of a
// disallowed-scope finding: error for the commit-time callers, warn at plan time.
func checkConventionalCommit(c awfgit.Commit, s Settings, scopeSeverity severity.Rank) []Finding {
	if c.IsMerge { // merges exempt (ADR-0017 constraint 2)
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

// ruleADRFrontmatter surfaces an ADR change whose new frontmatter does not
// parse: the status-cochange and staleness rules cannot evaluate such a change,
// so the breakage is reported instead of silently skipped.
func ruleADRFrontmatter(commits []awfgit.Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == awfgit.Deleted {
				continue
			}
			if _, ok := adrRecordOf(ch.Path, ch.NewText); !ok {
				out = append(out, finding(severity.Warn, "adr-frontmatter", c,
					filepath.Base(ch.Path)+" frontmatter does not parse; ADR status rules skipped for it"))
			}
		}
	}
	return out
}

func ruleADRStatusCochange(commits []awfgit.Commit, in Inputs) []Finding {
	var out []Finding
	for _, c := range commits {
		indexTouched := false
		for _, ch := range c.Changes {
			if ch.Path == in.IndexMd {
				indexTouched = true
			}
		}
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == awfgit.Deleted {
				continue
			}
			rec, ok := adrRecordOf(ch.Path, ch.NewText)
			if !ok || !rec.HasStatus() || !rec.IsGoverned() {
				continue // malformed ADRs are reported separately; legacy transitions predate INDEX.md
			}
			// An unparseable old side cannot witness a transition - skip rather
			// than read garbage as a status change.
			oldRec, oldOK := adrRecordOf(ch.Path, ch.OldText)
			if ch.Action == awfgit.Added || (oldOK && oldRec.Status != rec.Status) {
				if !indexTouched {
					out = append(out, finding(severity.Error, "adr-status-cochange", c,
						filepath.Base(ch.Path)+" status set/changed without INDEX.md in the same commit"))
				}
			}
		}
	}
	return out
}

func ruleDependencyADR(commits []awfgit.Commit, in Inputs) []Finding {
	if len(in.DependencyManifests) == 0 {
		return nil
	}
	var manifestCommit *awfgit.Commit
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
		return []Finding{finding(severity.Warn, "dependency-adr", *manifestCommit,
			"dependency manifest changed on this branch with no ADR touched: if a dependency was added, confirm an ADR covers it")}
	}
	return nil
}

func rulePlanForLargeChange(commits []awfgit.Commit, in Inputs) []Finding {
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
		return []Finding{{Severity: severity.Warn, Rule: "plan-for-large-change",
			Detail: fmt.Sprintf("branch changes %d non-generated lines (> %d) with no plan under %s", total, in.DiffThreshold, in.PlansDir)}}
	}
	return nil
}

// touches-state: tooling/audit-and-snapshots:audit-domain-doc-staleness - domain-doc-staleness audit rule; proof in audit_test.go
func ruleDomainDocStaleness(commits []awfgit.Commit, in Inputs) []Finding {
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
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == awfgit.Deleted {
				continue
			}
			rec, ok := adrRecordOf(ch.Path, ch.NewText)
			if !ok || !rec.IsImplemented() {
				continue
			}
			if oldRec, oldOK := adrRecordOf(ch.Path, ch.OldText); ch.Action != awfgit.Added && (!oldOK || oldRec.IsImplemented()) {
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
			out = append(out, Finding{Severity: severity.Warn, Rule: "domain-doc-staleness",
				Detail: fmt.Sprintf("an ADR in domain %q reached Implemented but %s/%s/current-state.md was not refreshed in this range", d, in.DomainsPartsDir, d)})
		}
	}
	return out
}

// touches-state: tooling/audit-and-snapshots:audit-undocumented-domain - undocumented-domain audit rule; proof in audit_test.go
func ruleUndocumentedDomain(commits []awfgit.Commit, in Inputs) []Finding {
	if !in.UndocumentedDomain || len(in.ConfiguredDomains) == 0 {
		return nil
	}
	flagged := map[string]bool{}
	for _, c := range commits {
		for _, ch := range c.Changes {
			if !isADRFile(ch.Path, in.ADRDir) || ch.Action == awfgit.Deleted {
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
		out = append(out, Finding{Severity: severity.Warn, Rule: "undocumented-domain",
			Detail: fmt.Sprintf("an ADR is tagged with domain %q, which has no domain doc: add it to config.Domains and author its current-state narrative, or drop the tag", d)})
	}
	return out
}

func ruleDomainCodeStaleness(commits []awfgit.Commit, in Inputs) []Finding {
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
			out = append(out, Finding{Severity: severity.Warn, Rule: "domain-code-staleness",
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
func rulePlainPunctuation(commits []awfgit.Commit, in Inputs) []Finding {
	if !in.PlainPunctuation || in.DocsDir == "" {
		return nil
	}
	var out []Finding
	for _, c := range commits {
		for _, ch := range c.Changes {
			if ch.Action == awfgit.Deleted || !strings.HasSuffix(ch.Path, ".md") ||
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
			out = append(out, finding(severity.Warn, "plain-punctuation", c,
				fmt.Sprintf("%s adds typographic punctuation: %s; authored prose uses plain punctuation (a colon, semicolon, comma, or parentheses; an ASCII hyphen for a range; three periods for elision)",
					ch.Path, strings.Join(risen, ", "))))
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
