package coverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func policyFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func policyProfile(t *testing.T, root, body string) string {
	t.Helper()
	return testsupport.WriteProfile(t, root, body)
}

func analyzePolicy(t *testing.T, files map[string]string, profile string) Analysis {
	t.Helper()
	root := t.TempDir()
	const mod = "example.com/m"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+mod+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		policyFile(t, root, name, body)
	}
	a, err := Analyze(policyProfile(t, root, profile), root, mod)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func identity(file string, sl, sc, el, ec, statements int) Identity {
	return Identity{File: file, Start: Position{Line: sl, Column: sc}, End: Position{Line: el, Column: ec}, Statements: statements}
}

func reviewEveryMiss(a Analysis) Review {
	r := Review{}
	for _, id := range a.RawMisses {
		r.Misses = append(r.Misses, MissAdmission{Identity: id, Reason: "reviewed behavior gap"})
	}
	for _, d := range a.ProductionDirectives {
		r.Directives = append(r.Directives, DirectiveAdmission{
			Directive: d,
			Class:     IgnoreImpossibleState,
			Evidence:  "direct invariant test",
		})
	}
	return r
}

func mustBaseline(t *testing.T, a Analysis) Baseline {
	t.Helper()
	b, err := Regenerate(a, nil, reviewEveryMiss(a))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnalyzeUsesExactRawIdentitiesAndWholeProfileSelectors(t *testing.T) {
	files := map[string]string{
		"internal/coverage/a.go": "package coverage\nfunc A() {}\n",
		"cmd/awf/main.go":        "package main\nfunc main() {}\n",
		"internal/git/g.go":      "package git\nfunc G() {}\n",
	}
	profile := "example.com/m/internal/git/g.go:2.2,2.9 3 0\n" +
		"example.com/m/internal/coverage/a.go:2.10,2.15 2 0\n" +
		"example.com/m/internal/coverage/a.go:2.10,2.15 2 1\n" +
		"example.com/m/cmd/awf/main.go:2.1,2.8 1 1\n"
	a := analyzePolicy(t, files, profile)
	if a.Raw != (Report{Covered: 3, Total: 6}) {
		t.Fatalf("raw report = %+v", a.Raw)
	}
	wantMisses := []Identity{identity("internal/git/g.go", 2, 2, 2, 9, 3)}
	if !slices.Equal(a.RawMisses, wantMisses) {
		t.Fatalf("raw misses = %#v, want %#v", a.RawMisses, wantMisses)
	}
	if len(a.UniverseSHA256) != 64 {
		t.Fatalf("universe hash = %q", a.UniverseSHA256)
	}
	if len(a.Selectors) != 6 {
		t.Fatalf("selector count = %d", len(a.Selectors))
	}
	for _, selector := range a.Selectors {
		switch selector.Name {
		case "repository-effort-lifecycle":
			if !slices.Equal(selector.Misses, wantMisses) {
				t.Fatalf("repository selector misses = %#v", selector.Misses)
			}
		case "hard-safety", "state-authority", "migration-recovery", "publication-application", "command-boundary":
			if len(selector.Misses) != 0 {
				t.Fatalf("%s unexpectedly contains %#v", selector.Name, selector.Misses)
			}
		default:
			t.Fatalf("unexpected selector %q", selector.Name)
		}
	}
}

func TestAnalyzeInventoriesProductionTestAndExecutedDirectives(t *testing.T) {
	marker := "//" + " coverage-ignore: "
	files := map[string]string{
		"p/f.go":               "package p\nfunc F() {\nif true { " + marker + "impossible guard\nprintln()\n}\n}\n",
		"p/f_test.go":          "package p\n" + marker + "test process exit\nfunc helper() {}\n",
		"p/platform_darwin.go": "package p\nfunc rollback() { " + marker + "darwin rollback\n}\n",
	}
	profile := "example.com/m/p/f.go:3.11,4.10 1 1\n"
	a := analyzePolicy(t, files, profile)
	if len(a.ProductionDirectives) != 2 || len(a.TestDirectives) != 1 {
		t.Fatalf("directive split: production=%#v test=%#v", a.ProductionDirectives, a.TestDirectives)
	}
	var executed, platform Directive
	for _, d := range a.ProductionDirectives {
		if d.File == "p/f.go" {
			executed = d
		} else {
			platform = d
		}
	}
	if !executed.Mapped || !executed.Executed || executed.TargetLine != 3 {
		t.Fatalf("executed directive = %+v", executed)
	}
	if platform.Mapped || platform.Executed {
		t.Fatalf("platform directive claimed measurement: %+v", platform)
	}
	if a.TestDirectives[0].File != "p/f_test.go" || a.TestDirectives[0].Mapped {
		t.Fatalf("test directive = %+v", a.TestDirectives[0])
	}
}

func TestRegenerateRequiresReviewedAdditionsAndPreservesImprovements(t *testing.T) {
	files := map[string]string{"p/f.go": "package p\nfunc F() {}\nfunc G() {}\n"}
	missA := "example.com/m/p/f.go:2.1,2.5 1 0\n"
	a := analyzePolicy(t, files, missA+"example.com/m/p/f.go:3.1,3.5 1 1\n")
	base := mustBaseline(t, a)
	if got := base.Repository[0].Reason; got != "reviewed behavior gap" {
		t.Fatalf("reason = %q", got)
	}

	covered := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 1\nexample.com/m/p/f.go:3.1,3.5 1 1\n")
	improved, err := Regenerate(covered, &base, Review{})
	if err != nil {
		t.Fatal(err)
	}
	if len(improved.Repository) != 0 {
		t.Fatalf("covered miss retained: %#v", improved.Repository)
	}

	swap := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 1\nexample.com/m/p/f.go:3.1,3.5 1 0\n")
	if _, err := Regenerate(swap, &base, Review{}); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("unreviewed no-net swap error = %v", err)
	}
	add := MissAdmission{Identity: swap.RawMisses[0], Reason: "new uncovered behavior"}
	updated, err := Regenerate(swap, &base, Review{Misses: []MissAdmission{add}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Repository[0].Reason != add.Reason || updated.Repository[0].MovedFrom != nil {
		t.Fatalf("reviewed addition = %+v", updated.Repository[0])
	}

	moved := add
	moved.Reason = "source span moved during refactor"
	moved.MovedFrom = &base.Repository[0].Identity
	movedBaseline, err := Regenerate(swap, &base, Review{Misses: []MissAdmission{moved}})
	if err != nil {
		t.Fatal(err)
	}
	if movedBaseline.Repository[0].MovedFrom == nil || *movedBaseline.Repository[0].MovedFrom != base.Repository[0].Identity {
		t.Fatalf("move provenance = %+v", movedBaseline.Repository[0])
	}
}

func TestCanonicalBaselineRoundTripAndStrictEvidence(t *testing.T) {
	a := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	base := mustBaseline(t, a)
	raw, err := CanonicalBaseline(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("canonical bytes = %q", raw)
	}
	path := filepath.Join(t.TempDir(), "coverage-baseline.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	gotRaw, err := CanonicalBaseline(got)
	if err != nil || string(gotRaw) != string(raw) {
		t.Fatalf("round trip: err=%v\ngot=%s\nwant=%s", err, gotRaw, raw)
	}

	for name, body := range map[string][]byte{
		"missing":   nil,
		"malformed": []byte("{"),
		"unknown":   []byte(`{"version":1,"mystery":true}`),
		"noncanon":  append([]byte(" \n"), raw...),
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), name+".json")
			if name != "missing" {
				if err := os.WriteFile(p, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := LoadBaseline(p); err == nil {
				t.Fatal("expected strict evidence error")
			}
		})
	}
}

func TestEvaluateUsesIdentitySetsNotPercentagesOrLocalProfiles(t *testing.T) {
	files := map[string]string{"p/f.go": "package p\nfunc F() {}\nfunc G() {}\n"}
	a := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 0\nexample.com/m/p/f.go:3.1,3.5 1 1\n")
	base := mustBaseline(t, a)
	if findings := Evaluate(a, base); len(findings) != 0 {
		t.Fatalf("admitted below-100 profile failed: %#v", findings)
	}

	improved := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 1\nexample.com/m/p/f.go:3.1,3.5 1 1\n")
	if findings := Evaluate(improved, base); len(findings) != 0 {
		t.Fatalf("automatic improvement failed: %#v", findings)
	}

	added := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 1\nexample.com/m/p/f.go:3.1,3.5 1 0\n")
	if findings := Evaluate(added, base); !hasFinding(findings, "raw-identity-added") {
		t.Fatalf("identity swap findings = %#v", findings)
	}

	local := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	if findings := Evaluate(local, base); !hasFinding(findings, "profile-universe-mismatch") {
		t.Fatalf("local profile findings = %#v", findings)
	}
}

func TestEvaluateRejectsExecutedIgnoreAndDirectiveDrift(t *testing.T) {
	marker := "//" + " coverage-ignore: "
	files := map[string]string{"p/f.go": "package p\nfunc F() { " + marker + "impossible\n}\n"}
	uncovered := analyzePolicy(t, files, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	base := mustBaseline(t, uncovered)
	executed := analyzePolicy(t, files, "example.com/m/p/f.go:2.12,3.2 1 1\n")
	if findings := Evaluate(executed, base); !hasFinding(findings, "executed-ignore") {
		t.Fatalf("executed ignore findings = %#v", findings)
	}

	changedFiles := map[string]string{"p/f.go": "package p\nfunc F() { " + marker + "different reason\n}\n"}
	changed := analyzePolicy(t, changedFiles, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	if findings := Evaluate(changed, base); !hasFinding(findings, "production-directive-changed") {
		t.Fatalf("directive drift findings = %#v", findings)
	}
}

func TestBaselineValidationRejectsInvalidPolicyEvidence(t *testing.T) {
	a := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	valid := mustBaseline(t, a)
	tests := map[string]func(*Baseline){
		"version":         func(b *Baseline) { b.Version = 0 },
		"universe":        func(b *Baseline) { b.UniverseSHA256 = "bad" },
		"miss reason":     func(b *Baseline) { b.Repository[0].Reason = "" },
		"duplicate miss":  func(b *Baseline) { b.Repository = append(b.Repository, b.Repository[0]) },
		"selector roots":  func(b *Baseline) { b.Selectors[0].Roots = []string{"wrong"} },
		"selector misses": func(b *Baseline) { b.Selectors[0].Misses = []Identity{identity("wrong.go", 1, 1, 1, 2, 1)} },
		"directive class": func(b *Baseline) {
			b.ProductionDirectives = []DirectiveAdmission{{Directive: Directive{File: "p/f.go", Line: 2, TargetLine: 2, Reason: "x"}, Class: "wrong", Evidence: "x"}}
		},
		"platform mapped": func(b *Baseline) {
			b.PlatformDirectives = []PlatformDirective{{Directive: Directive{File: "p/f.go", Line: 2, TargetLine: 2, Reason: "x", Mapped: true}, Platforms: []string{"darwin"}, Class: IgnorePlatformOnly, Evidence: "x"}}
		},
		"equivalent reason": func(b *Baseline) {
			b.EquivalentMutants = []EquivalentMutant{{File: "p/f.go", Line: 2, Column: 1, Mutator: "ARITHMETIC_BASE"}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			b := valid
			b.Repository = slices.Clone(valid.Repository)
			b.Selectors = append([]SelectorBaseline(nil), valid.Selectors...)
			for i := range b.Selectors {
				b.Selectors[i].Roots = slices.Clone(valid.Selectors[i].Roots)
				b.Selectors[i].Misses = slices.Clone(valid.Selectors[i].Misses)
			}
			mutate(&b)
			if _, err := CanonicalBaseline(b); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadReviewRejectsMalformedAndUnknownFields(t *testing.T) {
	for name, body := range map[string]string{"malformed": "{", "unknown": `{"mystery":true}`} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "review.json")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadReview(p); err == nil {
				t.Fatal("expected review parse error")
			}
		})
	}
	if _, err := LoadReview(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing review error")
	}
	p := filepath.Join(t.TempDir(), "review.json")
	raw, _ := json.Marshal(Review{Misses: []MissAdmission{{Identity: identity("p/f.go", 1, 1, 1, 2, 1), Reason: "reviewed"}}})
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReview(p); err != nil {
		t.Fatal(err)
	}
}

func hasFinding(findings []Finding, code string) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return f.Code == code })
}
