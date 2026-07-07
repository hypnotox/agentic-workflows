package audit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSeverityString(t *testing.T) {
	if Error.String() != "error" {
		t.Errorf("Error.String() = %q", Error.String())
	}
	if Warning.String() != "warning" {
		t.Errorf("Warning.String() = %q", Warning.String())
	}
}

// countRule returns how many findings of a given rule+severity evaluate emits.
func countRule(findings []Finding, rule string, sev Severity) int {
	n := 0
	for _, f := range findings {
		if f.Rule == rule && f.Severity == sev {
			n++
		}
	}
	return n
}

// invariant: audit-conventional-commits
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
			got := countRule(ruleConventionalCommits([]Commit{tc.commit}, in), "conventional-commits", Error)
			if got != tc.wantErr {
				t.Errorf("got %d errors, want %d", got, tc.wantErr)
			}
		})
	}
}

// The subject-length limit counts characters, not bytes — a multi-byte subject
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

var proposedADR = testsupport.ADR("Proposed")
var acceptedADR = testsupport.ADR("Accepted")

// invariant: audit-adr-status-cochange
func TestRuleADRStatusCochange(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", ActiveMd: "docs/decisions/ACTIVE.md"}
	adr := "docs/decisions/0001-x.md"
	active := "docs/decisions/ACTIVE.md"
	cases := []struct {
		name    string
		commit  Commit
		wantErr int
	}{
		{"added without ACTIVE", Commit{Changes: []FileChange{{Path: adr, Action: Added, NewText: proposedADR}}}, 1},
		{"flip without ACTIVE", Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: proposedADR, NewText: acceptedADR}}}, 1},
		{"flip with ACTIVE", Commit{Changes: []FileChange{
			{Path: adr, Action: Modified, OldText: proposedADR, NewText: acceptedADR},
			{Path: active, Action: Modified},
		}}, 0},
		{"context edit same status", Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: proposedADR, NewText: proposedADR}}}, 0},
		{"non-ADR md", Commit{Changes: []FileChange{{Path: "docs/foo.md", Action: Modified, OldText: proposedADR, NewText: acceptedADR}}}, 0},
		{"deleted ADR", Commit{Changes: []FileChange{{Path: adr, Action: Deleted}}}, 0},
		{"added without frontmatter", Commit{Changes: []FileChange{{Path: adr, Action: Added, NewText: "# no frontmatter"}}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleADRStatusCochange([]Commit{tc.commit}, in), "adr-status-cochange", Error)
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
	in := Inputs{ADRDir: "docs/decisions", ActiveMd: "docs/decisions/ACTIVE.md"}
	adr := "docs/decisions/0001-x.md"
	bad := "---\nstatus: [unclosed\n---\n# X\n"

	newBad := Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: proposedADR, NewText: bad}}}
	fs := evaluate([]Commit{newBad}, in)
	if got := countRule(fs, "adr-frontmatter", Warning); got != 1 {
		t.Errorf("unparseable new frontmatter: got %d adr-frontmatter warnings, want 1 (%v)", got, fs)
	}
	if got := countRule(fs, "adr-status-cochange", Error); got != 0 {
		t.Errorf("unparseable new frontmatter must not fire cochange: %v", fs)
	}

	oldBad := Commit{Changes: []FileChange{{Path: adr, Action: Modified, OldText: bad, NewText: proposedADR}}}
	fs = evaluate([]Commit{oldBad}, in)
	if got := countRule(fs, "adr-status-cochange", Error); got != 0 {
		t.Errorf("unparseable old frontmatter must not read as a status change: %v", fs)
	}
}

// invariant: audit-adr-domain-cochange
func TestRuleADRDomainCochange(t *testing.T) {
	active := FileChange{Path: "docs/decisions/ACTIVE.md", Action: Modified}
	toolingIdx := FileChange{Path: "docs/domains/tooling.md", Action: Modified}
	in := Inputs{ADRDir: "docs/decisions", ActiveMd: "docs/decisions/ACTIVE.md",
		DomainsIndexDir: "docs/domains", ConfiguredDomains: []string{"tooling", "rendering"}}
	noIdxDir := Inputs{ADRDir: "docs/decisions", ActiveMd: "docs/decisions/ACTIVE.md",
		ConfiguredDomains: []string{"tooling"}}
	cases := []struct {
		name    string
		in      Inputs
		commit  Commit
		wantErr int
	}{
		{"added, index missing", in, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "tooling"), active}}, 1},
		{"added, index present", in, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "tooling"), active, toolingIdx}}, 0},
		{"two domains, one missing", in, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "tooling, rendering"), active, toolingIdx}}, 1},
		{"status flip, index missing", in, Commit{Changes: []FileChange{adrChange(Modified, "Implemented", "tooling"), active}}, 1},
		{"unconfigured domain not required", in, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "payments"), active}}, 0},
		{"duplicate domain yields one finding", in, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "tooling, tooling"), active}}, 1},
		{"no DomainsIndexDir inert", noIdxDir, Commit{Changes: []FileChange{adrChange(Added, "Proposed", "tooling"), active}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleADRStatusCochange([]Commit{tc.commit}, tc.in), "adr-domain-cochange", Error)
			if got != tc.wantErr {
				t.Errorf("got %d adr-domain-cochange errors, want %d", got, tc.wantErr)
			}
		})
	}
}

// invariant: audit-dependency-warn
func TestRuleDependencyADR(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", Settings: Settings{DependencyManifests: []string{"go.mod", "*.csproj"}}}
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countRule(ruleDependencyADR(tc.commits, tc.in), "dependency-adr", Warning)
			if got != tc.wantWarn {
				t.Errorf("got %d warnings, want %d", got, tc.wantWarn)
			}
			if countRule(ruleDependencyADR(tc.commits, tc.in), "dependency-adr", Error) != 0 {
				t.Error("dependency-adr must never emit an Error")
			}
		})
	}
}

// invariant: audit-plan-threshold-warn
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
			got := countRule(rulePlanForLargeChange(tc.commits, tc.in), "plan-for-large-change", Warning)
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
	if countRule(got, "conventional-commits", Error) != 1 {
		t.Error("expected a conventional-commits error")
	}
	if countRule(got, "dependency-adr", Warning) != 1 {
		t.Error("expected a dependency-adr warning")
	}
}

func TestStatusOf(t *testing.T) {
	if st, ok := statusOf(""); st != "" || !ok {
		t.Error("empty text should yield empty status, ok")
	}
	if st, ok := statusOf("# no frontmatter"); st != "" || !ok {
		t.Error("no frontmatter should yield empty status, ok")
	}
	if _, ok := statusOf("---\nstatus: [bad yaml\n---\nx"); ok {
		t.Error("malformed frontmatter should yield ok=false")
	}
	if st, ok := statusOf(acceptedADR); st != "Accepted" || !ok {
		t.Errorf("statusOf(acceptedADR) = %q, %v", st, ok)
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
