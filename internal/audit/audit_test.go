package audit

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/currentstate"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

type Commit = awfgit.Commit
type FileChange = awfgit.FileChange
type Action = awfgit.Action

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
	in := Inputs{Settings: Settings{AllowedTypes: []string{"feat", "fix"}, AllowedScopes: []config.ScopeSpec{{Name: "awf"}}, SubjectMaxLength: 20}}
	cases := []struct {
		name    string
		commit  Commit
		wantErr int
	}{
		{"conforming", Commit{Subject: "feat(awf): ok"}, 0},
		{"no scope is fine", Commit{Subject: "fix: also ok"}, 0},
		{"malformed", Commit{Subject: "not a conventional commit"}, 1},
		{"disallowed type", Commit{Subject: "chore(awf): nope"}, 1},
		{"disallowed scope", Commit{Subject: "feat(core): nope"}, 1},
		{"over length", Commit{Subject: "feat(awf): this subject is definitely too long"}, 1},
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
	s := Settings{SubjectMaxLength: 72}
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

func TestRuleConventionalCommitsAcceptAny(t *testing.T) {
	// Empty AllowedTypes/Scopes and 0 max → only the format check applies.
	in := Inputs{}
	got := ruleConventionalCommits([]Commit{{Subject: "anything(weird-scope): super duper extremely long subject line here"}}, in)
	if len(got) != 0 {
		t.Errorf("accept-any config flagged a well-formed commit: %v", got)
	}
}

func TestCheckPlannedSubject(t *testing.T) {
	s := Settings{
		AllowedTypes:     []string{"feat", "fix"},
		AllowedScopes:    []config.ScopeSpec{{Name: "awf"}},
		SubjectMaxLength: 20,
	}
	// A disallowed scope is a warn at plan time (a plan may add the scope).
	if got := CheckPlannedSubject("feat(newscope): x", s); len(got) != 1 || got[0].Severity != severity.Warn {
		t.Fatalf("scope: want 1 warn, got %#v", got)
	}
	// Length, disallowed type, and malformed shape stay error.
	if got := CheckPlannedSubject("feat(awf): this one is definitely over twenty", s); len(got) != 1 || got[0].Severity != severity.Error {
		t.Fatalf("length: want 1 error, got %#v", got)
	}
	if got := CheckPlannedSubject("chore(awf): x", s); len(got) != 1 || got[0].Severity != severity.Error {
		t.Fatalf("type: want 1 error, got %#v", got)
	}
	if got := CheckPlannedSubject("not conventional", s); len(got) != 1 || got[0].Severity != severity.Error {
		t.Fatalf("malformed: want 1 error, got %#v", got)
	}
	// A fully valid subject yields nothing.
	if got := CheckPlannedSubject("feat(awf): ok", s); len(got) != 0 {
		t.Fatalf("valid: want 0, got %#v", got)
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
	in := Inputs{ADRDir: "docs/decisions", Settings: Settings{DependencyManifests: []string{"**/go.mod", "**/*.csproj"}}}
	defaults := Inputs{ADRDir: "docs/decisions", Settings: Settings{DependencyManifests: defaultDependencyManifests()}}
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
		{"manifests disabled", []Commit{{Changes: []FileChange{gomod}}}, Inputs{ADRDir: "docs/decisions"}, 0},
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

// invariant: tooling/audit-and-snapshots:audit-plan-threshold-warn (TestRulePlanForLargeChange)
func TestRulePlanForLargeChange(t *testing.T) {
	gen := map[string]bool{"gen/out.txt": true}
	big := FileChange{Path: "src/a.go", Action: Modified, Added: 300, Deleted: 200}
	genBig := FileChange{Path: "gen/out.txt", Action: Modified, Added: 9000, Deleted: 0}
	plan := FileChange{Path: "docs/plans/2026-01-01-x.md", Action: Added, Added: 10}
	base := Inputs{Settings: Settings{DiffThreshold: 400}, PlansDir: "docs/plans", GeneratedPaths: gen}
	cases := []struct {
		name     string
		commits  []Commit
		in       Inputs
		wantWarn int
	}{
		{"over no plan", []Commit{{Changes: []FileChange{big}}}, base, 1},
		{"over with plan", []Commit{{Changes: []FileChange{big, plan}}}, base, 0},
		{"generated inflates only", []Commit{{Changes: []FileChange{genBig}}}, base, 0},
		{"under threshold", []Commit{{Changes: []FileChange{{Path: "src/a.go", Added: 5, Deleted: 5}}}}, base, 0},
		{"threshold disabled", []Commit{{Changes: []FileChange{big}}}, Inputs{PlansDir: "docs/plans"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(rulePlanForLargeChange(tc.commits, tc.in), "plan-for-large-change", severity.Warn)
			if got != tc.wantWarn {
				t.Errorf("got %d warnings, want %d", got, tc.wantWarn)
			}
		})
	}
}

func TestEvaluateAggregates(t *testing.T) {
	in := Inputs{Settings: Settings{AllowedTypes: []string{"feat"}, DependencyManifests: []string{"go.mod"}}, ADRDir: "docs/decisions"}
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
	if !underDir("docs/plans", "docs/plans") {
		t.Error("exact dir should match")
	}
	if !underDir("docs/plans/x.md", "docs/plans") {
		t.Error("nested path should match")
	}
	if underDir("docs/plansx", "docs/plans") {
		t.Error("sibling prefix should not match")
	}
}

func adrChange(action Action, status string, domains string) FileChange {
	txt := testsupport.ADR(status, testsupport.WithDomains(strings.Split(domains, ", ")...), testsupport.WithBody("body\n"))
	return FileChange{Path: "docs/decisions/0099-x.md", Action: action, NewText: txt}
}

func TestRuleDomainDocStalenessDisabled(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}, DomainsPartsDir: ".awf/domains/parts"}
	if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}}, in); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}
}

func TestRuleDomainDocStaleness(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling", "rendering"}, DomainsPartsDir: ".awf/domains/parts", Settings: Settings{DomainDocStaleness: true}}
	partChange := func(p string) FileChange { return FileChange{Path: p, Action: Modified} }

	// Implemented in a configured domain, narrative NOT refreshed -> 1 warning.
	got := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}}, in)
	// invariant: tooling/audit-and-snapshots:audit-domain-doc-staleness (TestRuleDomainDocStaleness)
	if len(got) != 1 || got[0].Rule != "domain-doc-staleness" || got[0].Commit != "" {
		t.Fatalf("want 1 branch-level warning, got %v", got)
	}

	// Narrative refreshed in range -> 0. Also exercises domainOfPart valid + invalid-suffix + nested paths.
	clean := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{
		adrChange(Modified, "Implemented", "tooling"),
		partChange(".awf/domains/parts/tooling/current-state.md"),
		partChange(".awf/domains/parts/tooling/notes.md"),     // under partsDir, wrong file
		partChange(".awf/domains/parts/a/b/current-state.md"), // nested -> rejected
	}}}, in)
	if len(clean) != 0 {
		t.Fatalf("refreshed narrative should be clean, got %v", clean)
	}

	// status only Accepted; unconfigured domain; no domains; already Implemented; deleted; non-ADR -> all 0.
	for _, ch := range []FileChange{
		adrChange(Added, "Accepted", "tooling"),
		adrChange(Added, "Implemented", "ghost"),
		{Path: "docs/decisions/0099-x.md", Action: Added, NewText: "---\nstatus: Implemented\n---\n"},
		{Path: "docs/decisions/0099-x.md", Action: Modified, OldText: "---\nstatus: Implemented\ndomains: [tooling]\n---\n", NewText: "---\nstatus: Implemented\ndomains: [tooling]\n---\nedited\n"},
		{Path: "docs/decisions/0099-x.md", Action: Deleted},
		{Path: "README.md", Action: Modified},
	} {
		if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{ch}}}, in); len(f) != 0 {
			t.Errorf("change %+v should be clean, got %v", ch, f)
		}
	}

	// Multi-domain [tooling, rendering], only tooling refreshed -> 1 warning (rendering).
	multi := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{
		adrChange(Added, "Implemented", "tooling, rendering"),
		partChange(".awf/domains/parts/tooling/current-state.md"),
	}}}, in)
	if len(multi) != 1 || multi[0].Detail == "" {
		t.Fatalf("want 1 warning for rendering, got %v", multi)
	}

	// Empty ConfiguredDomains -> inert.
	if f := ruleDomainDocStaleness([]Commit{{Changes: []FileChange{adrChange(Added, "Implemented", "tooling")}}},
		Inputs{ADRDir: "docs/decisions", DomainsPartsDir: ".awf/domains/parts", Settings: Settings{DomainDocStaleness: true}}); len(f) != 0 {
		t.Errorf("no configured domains should be inert, got %v", f)
	}
}

func TestRuleUndocumentedDomain(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}, Settings: Settings{UndocumentedDomain: true}}

	// Disabled.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}},
		Inputs{ADRDir: "docs/decisions", ConfiguredDomains: []string{"tooling"}}); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}
	// No configured domains -> inert.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}},
		Inputs{ADRDir: "docs/decisions", Settings: Settings{UndocumentedDomain: true}}); f != nil {
		t.Errorf("no configured domains returned %v", f)
	}
	// ADR tags an unconfigured domain -> 1 warning.
	got := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "ghost")}}}, in)
	// invariant: tooling/audit-and-snapshots:audit-undocumented-domain (TestRuleUndocumentedDomain)
	if len(got) != 1 || got[0].Rule != "undocumented-domain" {
		t.Fatalf("want 1 warning, got %v", got)
	}
	// Configured domain -> clean.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Modified, "Accepted", "tooling")}}}, in); len(f) != 0 {
		t.Errorf("configured domain should be clean, got %v", f)
	}
	// Deleted ADR -> clean.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{{Path: "docs/decisions/0099-x.md", Action: Deleted}}}}, in); len(f) != 0 {
		t.Errorf("deleted ADR should be clean, got %v", f)
	}
	// ADR file with no parseable frontmatter -> domainsOf hits its not-found branch -> 0.
	if f := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{{Path: "docs/decisions/0099-x.md", Action: Added, NewText: "# no frontmatter"}}}}, in); len(f) != 0 {
		t.Errorf("frontmatter-less ADR should be clean, got %v", f)
	}
	// Multi-domain [tooling, ghost] -> 1 warning (ghost).
	multi := ruleUndocumentedDomain([]Commit{{Changes: []FileChange{adrChange(Added, "Proposed", "tooling, ghost")}}}, in)
	if len(multi) != 1 {
		t.Fatalf("want 1 warning for ghost, got %v", multi)
	}
}

// invariant: tooling/audit-and-snapshots:audit-domain-code-staleness (TestRuleDomainCodeStaleness)
func TestRuleDomainCodeStaleness(t *testing.T) {
	in := Inputs{
		Settings:        Settings{DomainCodeStaleness: true},
		DomainsPartsDir: ".awf/domains/parts",
		GeneratedPaths:  map[string]bool{"docs/domains/tooling.md": true},
		DomainPaths:     map[string][]string{"tooling": {"cmd/**", "internal/audit/**"}},
	}
	churn := FileChange{Path: "cmd/awf/main.go", Action: Modified}
	part := FileChange{Path: ".awf/domains/parts/tooling/current-state.md", Action: Modified}

	// Territory churned, narrative not refreshed -> one branch-level warn.
	got := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{churn}}}, in)
	if len(got) != 1 || got[0].Rule != "domain-code-staleness" || got[0].Severity != severity.Warn || got[0].Commit != "" {
		t.Fatalf("want 1 branch-level warning, got %v", got)
	}

	// Narrative co-changed (any in-range commit) -> silent.
	if f := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{churn}}, {Changes: []FileChange{part}}}, in); len(f) != 0 {
		t.Errorf("refreshed narrative should be clean, got %v", f)
	}

	// Only a generated file matched -> silent.
	if f := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{{Path: "docs/domains/tooling.md", Action: Modified}}}}, in); len(f) != 0 {
		t.Errorf("generated-path churn should be clean, got %v", f)
	}

	// Change outside every territory -> silent.
	if f := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{{Path: "internal/render/render.go", Action: Modified}}}}, in); len(f) != 0 {
		t.Errorf("non-matching churn should be clean, got %v", f)
	}

	// Toggle off -> silent.
	off := in
	off.DomainCodeStaleness = false
	if f := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{churn}}}, off); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}

	// No paths configured -> inert.
	none := in
	none.DomainPaths = nil
	if f := ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{churn}}}, none); f != nil {
		t.Errorf("pathless config returned %v", f)
	}

	// Two domains churned, neither refreshed -> two findings sorted by domain.
	two := in
	two.DomainPaths = map[string][]string{"tooling": {"cmd/**"}, "adr-system": {"internal/adr/**"}}
	got = ruleDomainCodeStaleness([]Commit{{Changes: []FileChange{churn, {Path: "internal/adr/adr.go", Action: Modified}}}}, two)
	if len(got) != 2 || !strings.Contains(got[0].Detail, `"adr-system"`) || !strings.Contains(got[1].Detail, `"tooling"`) {
		t.Fatalf("want 2 warnings sorted by domain, got %v", got)
	}
}

func TestRulePlainPunctuation(t *testing.T) {
	in := Inputs{DocsDir: "docs", Settings: Settings{PlainPunctuation: true},
		GeneratedPaths: map[string]bool{"docs/decisions/INDEX.md": true}}
	change := func(path, oldText, newText string) []Commit {
		return []Commit{{Hash: "abc1234", Subject: "docs(adr): x",
			Changes: []FileChange{{Path: path, Action: Modified, OldText: oldText, NewText: newText}}}}
	}
	dash, dots := "\u2014", "\u2026"
	endash, ldq := "\u2013", "\u201c"

	// A rising count warns, naming the file, the codepoint, and the commit.
	got := rulePlainPunctuation(change("docs/decisions/0001-x.md", "plain", "an "+dash+" dash"), in)
	// invariant: tooling/audit-and-snapshots:audit-plain-punctuation (TestRulePlainPunctuation)
	if len(got) != 1 || got[0].Rule != "plain-punctuation" || got[0].Severity != severity.Warn ||
		got[0].Commit != "abc1234" || !strings.Contains(got[0].Detail, "em-dash (U+2014)") ||
		!strings.Contains(got[0].Detail, "docs/decisions/0001-x.md") {
		t.Fatalf("want 1 warning naming the file and the em-dash, got %v", got)
	}
	// Risen runes are named in sorted order: map iteration order is not
	// deterministic, so the rule sorts before joining and this pins that. Four
	// runes, not two: two entries fall in sorted order by chance often enough
	// that a two-rune case lets an unsorted join pass about a quarter of runs.
	multi := rulePlainPunctuation(change("docs/x.md", "plain", "a"+dash+"b"+dots+"c"+endash+"d"+ldq+"e"), in)
	if len(multi) != 1 || !strings.Contains(multi[0].Detail,
		"ellipsis (U+2026) (0 to 1), em-dash (U+2014) (0 to 1), en-dash (U+2013) (0 to 1), left double quote (U+201C) (0 to 1)") {
		t.Fatalf("want all four risen runes named in sorted order, got %v", multi)
	}
	// An unchanged count is silent: grandfathering is emergent, not configured.
	if f := rulePlainPunctuation(change("docs/plans/p.md", "a"+dash+"b", "c"+dash+"d"), in); len(f) != 0 {
		t.Errorf("net-zero swap should be silent, got %v", f)
	}
	// A falling count is silent.
	if f := rulePlainPunctuation(change("docs/plans/p.md", "a"+dash+"b"+dash+"c", "a"+dash+"b"), in); len(f) != 0 {
		t.Errorf("a removal should be silent, got %v", f)
	}
	// A new file has empty OldText, so every occurrence in it is new.
	added := []Commit{{Hash: "d", Changes: []FileChange{{Path: "docs/x.md", Action: Added, NewText: "a " + dots + " b"}}}}
	if f := rulePlainPunctuation(added, in); len(f) != 1 || !strings.Contains(f[0].Detail, "ellipsis (U+2026)") {
		t.Fatalf("want 1 warning naming the ellipsis on an added file, got %v", f)
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
	// Disabled.
	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+dash+"b"), Inputs{DocsDir: "docs"}); f != nil {
		t.Errorf("disabled rule returned %v", f)
	}
	// Unset DocsDir is inert.
	if f := rulePlainPunctuation(change("docs/x.md", "", "a"+dash+"b"),
		Inputs{Settings: Settings{PlainPunctuation: true}}); f != nil {
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

func TestRunEmptyRangeStillEvaluatesLiveCleanliness(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{"a.txt": "clean\n"})
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, count, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{Settings: Settings{UncommittedChanges: true}})
	if err != nil || count != 0 || len(findings) != 1 || findings[0].Rule != "uncommitted-changes" {
		t.Fatalf("findings = %#v, count = %d, err = %v", findings, count, err)
	}
}

// invariant: tooling/audit-and-snapshots:stale-merge-trailer-replay (TestAuditReplaysStaleMergeTrailers)
func TestAuditReplaysStaleMergeTrailers(t *testing.T) {
	cases := []struct {
		name    string
		format  adr.Format
		message string
		evil    bool
		schema  int
		want    string
	}{
		{"valid V1", adr.CurrentStateV1, allow(adr.V1FormatMarker), false, 31, ""},
		{"valid V2", adr.CurrentStateV2, allow(adr.V2FormatMarker), false, 31, ""},
		{"valid legacy", adr.Legacy, allow("legacy"), false, 31, ""},
		{"missing", adr.CurrentStateV1, "Merge feature", false, 31, "missing authorization version"},
		{"malformed", adr.CurrentStateV1, "Merge feature\n\nAWF-Allow-Version: current-state-v1", false, 31, "malformed reserved trailer"},
		{"wrong version", adr.CurrentStateV1, allow(adr.V2FormatMarker), false, 31, "missing authorization version"},
		{"duplicate", adr.CurrentStateV1, allow(adr.V1FormatMarker) + "\nAWF-Allow-Version: current-state-v1\nAWF-Allow-Reason: repeated", false, 31, ""},
		{"redundant", adr.CurrentStateV1, allow(adr.V1FormatMarker) + "\nAWF-Allow-Version: current-state-v2\nAWF-Allow-Reason: unrelated", false, 31, ""},
		{"evil mutation", adr.CurrentStateV1, allow(adr.V1FormatMarker), true, 31, "unqualified incoming-parent record"},
		{"pre generation 31", adr.CurrentStateV1, "Merge feature", false, 30, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, base := staleAuditRepo(t, tc.schema)
			main := gitfixture.Commit(t, repo, "feat(awf): main", map[string]string{"main.txt": "main\n"})
			gitfixture.CheckoutNewBranch(t, repo, "feature", base)
			path, record := "docs/decisions/0001-old.md", staleADR(tc.format, "0001")
			feature := gitfixture.Commit(t, repo, "feat(awf): feature", map[string]string{path: record})
			result := record
			if tc.evil {
				result = strings.Replace(result, "Context.", "Evil mutation.", 1)
			}
			gitfixture.Stage(t, repo, map[string]string{"main.txt": "main\n", path: result})
			gitfixture.Merge(t, repo, tc.message, main, feature)

			findings, _, err := Run(testContext(t), repo.Root(), "master", "HEAD", Inputs{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(findings) != 0 {
					t.Fatalf("findings = %#v", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Rule != "stale-merge-authorization" || findings[0].Severity != severity.Error || !strings.Contains(findings[0].Detail, tc.want) {
				t.Fatalf("findings = %#v, want %q", findings, tc.want)
			}
		})
	}

	t.Run("octopus qualification", func(t *testing.T) {
		repo, base := staleAuditRepo(t, 31)
		main := gitfixture.Commit(t, repo, "feat(awf): main", map[string]string{"main.txt": "main\n"})
		gitfixture.CheckoutNewBranch(t, repo, "one", base)
		one := gitfixture.Commit(t, repo, "feat(awf): one", map[string]string{"docs/decisions/0001-one.md": staleADR(adr.CurrentStateV1, "0001")})
		gitfixture.CheckoutNewBranch(t, repo, "two", base)
		two := gitfixture.Commit(t, repo, "feat(awf): two", map[string]string{"docs/decisions/0002-two.md": staleADR(adr.CurrentStateV2, "0002")})
		gitfixture.Stage(t, repo, map[string]string{
			"main.txt":                   "main\n",
			"docs/decisions/0001-one.md": staleADR(adr.CurrentStateV1, "0001"),
			"docs/decisions/0002-two.md": staleADR(adr.CurrentStateV2, "0002"),
		})
		gitfixture.Merge(t, repo, allow(adr.V1FormatMarker)+"\nAWF-Allow-Version: current-state-v2\nAWF-Allow-Reason: second parent", main, one, two)
		findings, _, err := Run(testContext(t), repo.Root(), "master", "HEAD", Inputs{})
		if err != nil || len(findings) != 0 {
			t.Fatalf("findings = %#v, %v", findings, err)
		}
	})

	t.Run("nested adopter", func(t *testing.T) {
		repo := gitfixture.InitRepo(t)
		base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
			"nested/.awf/awf.lock":    `{"awfVersion":"v0.18.0","schemaVersion":31,"files":{}}`,
			"nested/.awf/config.yaml": "prefix: test\nintegrationBranch: master\ntargets: [claude]\n",
		})
		main := gitfixture.Commit(t, repo, "feat(awf): main", map[string]string{"nested/main.txt": "main\n"})
		gitfixture.CheckoutNewBranch(t, repo, "feature", base)
		path := "nested/docs/decisions/0001-old.md"
		record := staleADR(adr.CurrentStateV1, "0001")
		feature := gitfixture.Commit(t, repo, "feat(awf): feature", map[string]string{path: record})
		gitfixture.Stage(t, repo, map[string]string{"nested/main.txt": "main\n", path: record})
		gitfixture.Merge(t, repo, "Merge feature", main, feature)

		findings, _, err := Run(testContext(t), filepath.Join(repo.Root(), "nested"), "master", "HEAD", Inputs{})
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || findings[0].Rule != "stale-merge-authorization" || !strings.Contains(findings[0].Detail, "missing authorization version") {
			t.Fatalf("nested findings = %#v", findings)
		}
	})

	t.Run("generation 31 non merge and fast forward", func(t *testing.T) {
		repo, base := staleAuditRepo(t, 31)
		gitfixture.Commit(t, repo, "feat(awf): old record", map[string]string{"docs/decisions/0001-old.md": staleADR(adr.CurrentStateV1, "0001")})
		findings, _, err := Run(testContext(t), repo.Root(), base, "HEAD", Inputs{})
		if err != nil || len(findings) != 0 {
			t.Fatalf("non-merge findings = %#v, %v", findings, err)
		}
		findings, _, err = Run(testContext(t), repo.Root(), "HEAD", "HEAD", Inputs{})
		if err != nil || len(findings) != 0 {
			t.Fatalf("fast-forward findings = %#v, %v", findings, err)
		}
	})
}

func staleAuditRepo(t *testing.T, schema int) (gitfixture.Fixture, string) {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	lock := `{"awfVersion":"v0.18.0","schemaVersion":` + strconv.Itoa(schema) + `,"files":{}}`
	base := gitfixture.Commit(t, repo, "feat(awf): base", map[string]string{
		".awf/awf.lock":    lock,
		".awf/config.yaml": "prefix: test\nintegrationBranch: master\ntargets: [claude]\n",
	})
	return repo, base
}

func allow(version string) string {
	return "Merge feature\n\nAWF-Allow-Version: " + version + "\nAWF-Allow-Reason: carried from a stale branch"
}

func TestAuditSnapshotReadersAndErrors(t *testing.T) {
	if _, err := staleAuthorizationSyntax(os.ErrInvalid); err == nil {
		t.Fatal("non-syntax authorization error accepted")
	}
	tree := auditTree(t, []snapshot.File{
		{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte(`{"awfVersion":"v0.18.0","schemaVersion":31,"files":{}}`)},
		{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: master\ntargets: [claude]\n")},
		{Path: ".awf/parts/a.md", Mode: snapshot.Regular, Bytes: []byte("part")},
		{Path: ".awf/parts/link.md", Mode: snapshot.Symlink, Bytes: []byte("part")},
	})
	reader := auditSelectionReader{auditSelection(t, tree)}
	got, ok := reader.ReadFile("parts/a.md")
	if !ok || string(got) != "part" {
		t.Fatalf("ReadFile = %q, %v", got, ok)
	}
	got[0] = 'x'
	if again, _ := reader.ReadFile("parts/a.md"); string(again) != "part" {
		t.Fatalf("ReadFile did not clone: %q", again)
	}
	if _, ok := reader.ReadFile("parts/link.md"); ok {
		t.Fatal("ReadFile accepted a symlink")
	}
	if _, ok := reader.ReadFile("missing"); ok {
		t.Fatal("ReadFile accepted a missing file")
	}
	if paths := reader.Paths("parts"); len(paths) != 1 || paths[0] != "parts/a.md" {
		t.Fatalf("Paths = %v", paths)
	}
	if paths := reader.Paths("missing"); len(paths) != 0 {
		t.Fatalf("missing Paths = %v", paths)
	}
	if lock, found, err := auditLockFromSelection(auditSelection(t, tree)); err != nil || !found || lock.SchemaVersion != 31 {
		t.Fatalf("lock = %#v, %v, %v", lock, found, err)
	}
	if _, found, err := auditLockFromSelection(auditSelection(t, auditTree(t, nil))); err != nil || found {
		t.Fatalf("missing lock found=%v err=%v", found, err)
	}
	for _, file := range []snapshot.File{
		{Path: ".awf/awf.lock", Mode: snapshot.Symlink, Bytes: []byte("lock")},
		{Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte("{")},
	} {
		if _, _, err := auditLockFromSelection(auditSelection(t, auditTree(t, []snapshot.File{file}))); err == nil {
			t.Fatalf("bad lock %#v accepted", file)
		}
	}
	if universe, err := auditUniverseFromTree(t.TempDir(), auditTree(t, nil)); err != nil || len(universe.ADRs) != 0 {
		t.Fatalf("absent config = %#v, %v", universe, err)
	}
	for _, files := range [][]snapshot.File{
		{{Path: ".awf/config.yaml", Mode: snapshot.Symlink, Bytes: []byte("config")}},
		{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: master\ntargets: 1\n")}},
		{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: master\ntargets: [claude]\n")}, {Path: "docs/decisions/0001-bad.md", Mode: snapshot.Regular, Bytes: []byte("---\nformat: unknown\n---\n")}},
		{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: bad/test\nintegrationBranch: master\ntargets: [claude]\n")}},
		{{Path: ".awf/config.yaml", Mode: snapshot.Regular, Bytes: []byte("prefix: test\nintegrationBranch: master\ntargets: [claude]\n")}, {Path: ".awf/awf.lock", Mode: snapshot.Regular, Bytes: []byte("{")}},
	} {
		if _, err := auditUniverseFromTree(t.TempDir(), auditTree(t, files)); err == nil {
			t.Fatalf("invalid audit universe accepted: %#v", files)
		}
	}
	if universe, err := auditUniverseFromTree(t.TempDir(), tree); err != nil || len(universe.ADRs) != 0 {
		t.Fatalf("full audit universe = %#v, %v", universe, err)
	}
}

func loadUniverseFromTree(tree *snapshot.Tree, cfg *config.Config) (currentstate.Universe, error) {
	selection, err := snapshot.NewSelection(tree.List())
	if err != nil {
		return currentstate.Universe{}, err
	}
	return currentstate.LoadUniverseFromSelection(selection, cfg)
}

func auditTree(t *testing.T, files []snapshot.File) *snapshot.Tree {
	t.Helper()
	tree, err := snapshot.NewTree(files)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func auditSelection(t *testing.T, tree *snapshot.Tree) *snapshot.Selection {
	t.Helper()
	selection, err := snapshot.NewSelection(tree.List())
	if err != nil {
		t.Fatal(err)
	}
	return selection
}

func auditUniverseFromTree(root string, tree *snapshot.Tree) (currentstate.Universe, error) {
	selection, err := snapshot.NewSelection(tree.List())
	if err != nil {
		return currentstate.Universe{}, err
	}
	lock, _, err := auditLockFromSelection(selection)
	if err != nil {
		return currentstate.Universe{}, err
	}
	cfg, err := auditConfig(root, selection, lock)
	if err != nil || cfg == nil {
		return currentstate.Universe{}, err
	}
	return loadUniverseFromTree(tree, cfg)
}

func staleMergeFindingsForTest(t *testing.T, root string, repo *awfgit.Repo, commits []Commit) error {
	t.Helper()
	op, err := newHistoryOperation(testContext(t), "base", "head", Inputs{},
		func(context.Context, string, string) ([]Commit, error) { return commits, nil },
		func(ctx context.Context, revision string) (*revisionState, error) {
			return loadSelectedRevision(ctx, root, revision, repo.CommitEntries, repo.CommitBlobsAt)
		},
		nil,
		func(context.Context) ([]Finding, error) { return nil, nil })
	if err != nil {
		return err
	}
	_, err = op.staleMergeFindings(testContext(t))
	return err
}

func TestReplayStaleMergeAuthorizationErrorPaths(t *testing.T) {
	repo, base := staleAuditRepo(t, 31)
	main := gitfixture.Commit(t, repo, "feat(awf): main", map[string]string{"main.txt": "main\n"})
	gitfixture.CheckoutNewBranch(t, repo, "feature", base)
	feature := gitfixture.Commit(t, repo, "feat(awf): feature", map[string]string{"docs/decisions/0001-old.md": staleADR(adr.CurrentStateV1, "0001")})
	gitfixture.Stage(t, repo, map[string]string{"main.txt": "main\n", "docs/decisions/0001-old.md": staleADR(adr.CurrentStateV1, "0001")})
	merge := gitfixture.Merge(t, repo, allow(adr.V1FormatMarker), main, feature)
	handle, _, err := awfgit.OpenContaining(repo.Root())
	if err != nil {
		t.Fatal(err)
	}
	commit := Commit{Hash: merge[:8], Revision: merge, IsMerge: true, Parents: []string{main, feature}}
	for _, tc := range []struct {
		name   string
		commit Commit
	}{
		{"missing result", Commit{Hash: "missing", Revision: "missing", IsMerge: true}},
		{"one parent", Commit{Hash: merge[:8], Revision: merge, IsMerge: true, Parents: []string{main}}},
		{"missing parent", Commit{Hash: merge[:8], Revision: merge, IsMerge: true, Parents: []string{main, "missing"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := staleMergeFindingsForTest(t, repo.Root(), handle, []Commit{tc.commit}); err == nil {
				t.Fatal("replay accepted broken commit evidence")
			}
		})
	}
	bad := gitfixture.Commit(t, repo, "feat(awf): bad config", map[string]string{".awf/config.yaml": "prefix: [\n"})
	for _, tc := range []struct {
		name    string
		parents []string
	}{
		{"first parent config", []string{bad, feature}},
		{"incoming parent config", []string{main, bad}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broken := commit
			broken.Parents = tc.parents
			if err := staleMergeFindingsForTest(t, repo.Root(), handle, []Commit{broken}); err == nil {
				t.Fatal("replay accepted malformed parent config")
			}
		})
	}

	badRepo := gitfixture.InitRepo(t)
	badBase := gitfixture.Commit(t, badRepo, "feat(awf): base", map[string]string{
		".awf/awf.lock":    `{"awfVersion":"v0.18.0","schemaVersion":31,"files":{}}`,
		".awf/config.yaml": "prefix: [\n",
	})
	badMain := gitfixture.Commit(t, badRepo, "feat(awf): main", map[string]string{"main.txt": "main\n"})
	gitfixture.CheckoutNewBranch(t, badRepo, "feature", badBase)
	badFeature := gitfixture.Commit(t, badRepo, "feat(awf): feature", map[string]string{"feature.txt": "feature\n"})
	gitfixture.Stage(t, badRepo, map[string]string{"main.txt": "main\n"})
	gitfixture.Merge(t, badRepo, "Merge feature", badMain, badFeature)
	if _, _, err := Run(testContext(t), badRepo.Root(), "master", "HEAD", Inputs{}); err == nil {
		t.Fatal("Run accepted malformed merge result config")
	}

	gitfixture.Stage(t, repo, map[string]string{".awf/awf.lock": "{"})
	badLock := gitfixture.Merge(t, repo, "Merge broken lock", main, feature)
	if err := staleMergeFindingsForTest(t, repo.Root(), handle, []Commit{{Hash: badLock[:8], Revision: badLock, IsMerge: true, Parents: []string{main, feature}}}); err == nil {
		t.Fatal("replay accepted malformed merge result lock")
	}
}

func staleADR(format adr.Format, number string) string {
	if format == adr.Legacy {
		return "---\nstatus: Proposed\ndate: 2026-01-01\n---\n# ADR-" + number + ": Old\n"
	}
	marker := adr.FormatMarker(format)
	return "---\nformat: " + marker + "\nstatus: Proposed\ndate: 2026-01-01\n---\n" +
		"# ADR-" + number + ": Old\n\n## Context\n\nContext.\n\n## Decision\n\n1. Decide.\n\n## State changes\n\nNone.\n\n## Consequences\n\nConsequence.\n\n## Alternatives Considered\n\nNone.\n\n## Status history\n\n- 2026-01-01: Proposed\n"
}

func TestRuleUncommittedChanges(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "init", map[string]string{"a.txt": "a"})
	handle, _, err := awfgit.OpenContaining(dir)
	if err != nil {
		t.Fatal(err)
	}
	if findings, err := ruleUncommittedChanges(testContext(t), handle, Inputs{Settings: Settings{UncommittedChanges: true}}); err != nil || len(findings) != 0 {
		t.Fatalf("clean = %#v, %v", findings, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := ruleUncommittedChanges(testContext(t), handle, Inputs{Settings: Settings{UncommittedChanges: true}})
	// invariant: tooling/audit-and-snapshots:audit-uncommitted-changes (TestRuleUncommittedChanges)
	if err != nil || len(findings) != 1 || findings[0].Rule != "uncommitted-changes" || findings[0].Severity != severity.Error || findings[0].Commit != "" {
		t.Fatalf("dirty = %#v, %v", findings, err)
	}
	wantDetail := "working tree not clean: 1 tracked change(s), 1 untracked file(s); commit or discard before concluding the implementation"
	if findings[0].Detail != wantDetail {
		t.Errorf("Detail mismatch:\n got %q\nwant %q", findings[0].Detail, wantDetail)
	}
	if disabled, err := ruleUncommittedChanges(testContext(t), handle, Inputs{}); err != nil || disabled != nil {
		t.Fatalf("disabled dirty = %#v, %v", disabled, err)
	}
}

func TestRunIncludesUncommittedChanges(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "init", map[string]string{"a.txt": "a"})
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, _, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{Settings: Settings{UncommittedChanges: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != "uncommitted-changes" || findings[0].Commit != "" {
		t.Fatalf("Run findings = %#v", findings)
	}
}

func TestRuleUncommittedChangesDisabled(t *testing.T) {
	if findings, err := ruleUncommittedChanges(testContext(t), nil, Inputs{}); err != nil || findings != nil {
		t.Fatalf("disabled = %#v, %v", findings, err)
	}
}

func TestRunNestedAdopterFiltersAndReroots(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	if err := os.MkdirAll(filepath.Join(dir, "nested", "docs", "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := gitfixture.Commit(t, repo, "base", map[string]string{"nested/README.md": "base\n", "outside.txt": "old\n"})
	gitfixture.Commit(t, repo, "not a conventional commit", map[string]string{"outside.txt": "new\n"})
	gitfixture.Commit(t, repo, "feat: nested", map[string]string{"nested/docs/decisions/0001-x.md": "---\nstatus: [unclosed\n---\n"})

	findings, _, err := Run(testContext(t), filepath.Join(dir, "nested"), base, "HEAD", Inputs{ADRDir: "docs/decisions"})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != "adr-frontmatter" {
		t.Fatalf("nested findings = %#v", findings)
	}
}

func TestRunPropagatesOpenAndWalkErrors(t *testing.T) {
	if _, _, err := Run(testContext(t), t.TempDir(), "base", "HEAD", Inputs{}); err == nil {
		t.Fatal("non-repository accepted")
	}
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "base", map[string]string{"a.txt": "a"})
	if _, _, err := Run(testContext(t), dir, "missing", "HEAD", Inputs{}); err == nil {
		t.Fatal("missing revision accepted")
	}
}

func TestRuleUncommittedChangesPropagatesStatusError(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "init", map[string]string{"a.txt": "a"})
	handle, _, err := awfgit.OpenContaining(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := ruleUncommittedChanges(testContext(t), handle, Inputs{Settings: Settings{UncommittedChanges: true}}); err == nil {
		t.Fatal("unavailable native git accepted")
	}
}

func TestRunPropagatesUncommittedStatusError(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "init", map[string]string{"a.txt": "a"})
	t.Setenv("PATH", t.TempDir())
	if _, _, err := Run(testContext(t), dir, "HEAD", "HEAD", Inputs{Settings: Settings{UncommittedChanges: true}}); err == nil {
		t.Fatal("unavailable native git accepted")
	}
}

// invariant: tooling/audit-and-snapshots:audit-uncommitted-changes (TestRuleUncommittedChangesIgnoresManagedWorktreeResidents)
func TestRuleUncommittedChangesIgnoresManagedWorktreeResidents(t *testing.T) {
	repo := gitfixture.InitRepo(t)
	dir := repo.Root()
	gitfixture.Commit(t, repo, "init", map[string]string{".gitignore": ".awf/worktrees/\n", "a.txt": "a"})
	resident := filepath.Join(dir, ".awf", "worktrees", "other", ".awf", "memory", ".gitignore")
	if err := os.MkdirAll(filepath.Dir(resident), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resident, []byte("*\n!.gitignore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handle, _, err := awfgit.OpenContaining(dir)
	if err != nil {
		t.Fatal(err)
	}
	findings, err := ruleUncommittedChanges(testContext(t), handle, Inputs{Settings: Settings{UncommittedChanges: true}})
	if err != nil || len(findings) != 0 {
		t.Fatalf("ignored managed-worktree residents = %#v, %v", findings, err)
	}
}
