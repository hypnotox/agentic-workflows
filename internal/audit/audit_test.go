package audit

import "testing"

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
	in := Inputs{AllowedTypes: []string{"feat", "fix"}, AllowedScopes: []string{"awf"}, SubjectMaxLength: 20}
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

func TestRuleConventionalCommitsAcceptAny(t *testing.T) {
	// Empty AllowedTypes/Scopes and 0 max → only the format check applies.
	in := Inputs{}
	got := ruleConventionalCommits([]Commit{{Subject: "anything(weird-scope): super duper extremely long subject line here"}}, in)
	if len(got) != 0 {
		t.Errorf("accept-any config flagged a well-formed commit: %v", got)
	}
}

const proposedADR = "---\nstatus: Proposed\n---\n# ADR\n"
const acceptedADR = "---\nstatus: Accepted\n---\n# ADR\n"

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

// invariant: audit-dependency-warn
func TestRuleDependencyADR(t *testing.T) {
	in := Inputs{ADRDir: "docs/decisions", DependencyManifests: []string{"go.mod", "*.csproj"}}
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
	base := Inputs{DiffThreshold: 400, PlansDir: "docs/plans", GeneratedPaths: gen}
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
	in := Inputs{AllowedTypes: []string{"feat"}, ADRDir: "docs/decisions", DependencyManifests: []string{"go.mod"}}
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
	if statusOf("") != "" {
		t.Error("empty text should yield empty status")
	}
	if statusOf("# no frontmatter") != "" {
		t.Error("no frontmatter should yield empty status")
	}
	if statusOf("---\nstatus: [bad yaml\n---\nx") != "" {
		t.Error("malformed frontmatter should yield empty status")
	}
	if statusOf(acceptedADR) != "Accepted" {
		t.Errorf("statusOf(acceptedADR) = %q", statusOf(acceptedADR))
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
