package coverage

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestAnalyzeDerivesEveryExactSelectorRoot(t *testing.T) {
	files := make(map[string]string)
	var profile strings.Builder
	want := make(map[string][]Identity)
	line := 2
	for _, selector := range selectorPolicy {
		for rootIndex, root := range selector.Roots {
			name := root + "/policy_fixture.go"
			files[name] = "package fixture\nfunc F() {}\n"
			fmtProfileLine(&profile, "example.com/m/"+name, line, rootIndex+1)
			id := identity(name, line, rootIndex+1, line, rootIndex+2, 1)
			want[selector.Name] = append(want[selector.Name], id)
		}
	}
	files["internal/gitextra/not_selected.go"] = "package extra\nfunc F() {}\n"
	fmtProfileLine(&profile, "example.com/m/internal/gitextra/not_selected.go", line, 50)

	analysis := analyzePolicy(t, files, profile.String())
	for index, selector := range analysis.Selectors {
		if selector.Name != selectorPolicy[index].Name || !slices.Equal(selector.Roots, selectorPolicy[index].Roots) {
			t.Fatalf("selector %d = %#v, want %#v", index, selector, selectorPolicy[index])
		}
		if !slices.Equal(selector.Misses, want[selector.Name]) {
			t.Fatalf("selector %q misses = %#v, want %#v", selector.Name, selector.Misses, want[selector.Name])
		}
	}
}

func fmtProfileLine(builder *strings.Builder, file string, line, column int) {
	builder.WriteString(file)
	builder.WriteByte(':')
	builder.WriteString(intString(line))
	builder.WriteByte('.')
	builder.WriteString(intString(column))
	builder.WriteByte(',')
	builder.WriteString(intString(line))
	builder.WriteByte('.')
	builder.WriteString(intString(column + 1))
	builder.WriteString(" 1 0\n")
}

func intString(value int) string { return strconv.Itoa(value) }

func TestAnalyzeSortsExactPositionsAndRejectsInvalidInputs(t *testing.T) {
	files := map[string]string{
		"a.go": "package m\nfunc F() {}\nfunc G() {}\n",
		"b.go": "package m\nfunc H() {}\n",
	}
	profile := "example.com/m/b.go:2.2,3.8 1 0\n" +
		"example.com/m/a.go:3.1,3.9 1 0\n" +
		"example.com/m/a.go:2.4,3.7 1 0\n" +
		"example.com/m/a.go:2.4,2.9 1 0\n" +
		"example.com/m/a.go:2.4,2.8 1 0\n" +
		"example.com/m/a.go:2.1,2.5 2 0\n" +
		"example.com/m/a.go:2.1,2.5 1 0\n"
	analysis := analyzePolicy(t, files, profile)
	for index := 1; index < len(analysis.RawMisses); index++ {
		if compareIdentity(analysis.RawMisses[index-1], analysis.RawMisses[index]) >= 0 {
			t.Fatalf("misses not strictly ordered: %#v", analysis.RawMisses)
		}
	}

	root := t.TempDir()
	profilePath := policyProfile(t, root, "other.example/m/f.go:1.1,1.2 1 0\n")
	if _, err := Analyze(profilePath, root, "example.com/m"); err == nil || !strings.Contains(err.Error(), "outside module") {
		t.Fatalf("outside-module error = %v", err)
	}
	if _, err := Analyze(filepath.Join(root, "missing.out"), root, "example.com/m"); err == nil {
		t.Fatal("expected missing profile error")
	}
	profilePath = policyProfile(t, root, "example.com/m/f.go:1.1,1.2 1 0\n")
	if _, err := Analyze(profilePath, filepath.Join(root, "missing-root"), "example.com/m"); err == nil {
		t.Fatal("expected missing source root error")
	}
	for _, escaped := range []string{"..", "../outside.go", "./f.go", "p/../f.go"} {
		profilePath = policyProfile(t, root, "example.com/m/"+escaped+":1.1,1.2 1 0\n")
		if _, err := Analyze(profilePath, root, "example.com/m"); err == nil || !strings.Contains(err.Error(), "invalid identity") {
			t.Fatalf("noncanonical profile path %q error = %v", escaped, err)
		}
	}
}

func TestSourceDirectivesRejectsMalformedReason(t *testing.T) {
	if _, err := sourceDirectives("p/f.go", []byte("//"+" coverage-ignore\n"), nil); err == nil {
		t.Fatal("expected malformed directive error")
	}
}

func TestAnalyzePreservesLegalZeroStatementBlocks(t *testing.T) {
	analysis := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.10,2.10 0 0\n")
	if len(analysis.RawMisses) != 1 || analysis.RawMisses[0].Statements != 0 {
		t.Fatalf("zero-statement raw misses = %#v", analysis.RawMisses)
	}
	if _, err := CanonicalBaseline(mustBaseline(t, analysis)); err != nil {
		t.Fatalf("canonical zero-statement baseline: %v", err)
	}
}

func TestAnalyzeProfilePropagatesModuleResolutionErrors(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("working directory failed") })
	if _, err := AnalyzeProfile("missing.out"); err == nil || !strings.Contains(err.Error(), "working directory failed") {
		t.Fatalf("working-directory error = %v", err)
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// no module declaration\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	if _, err := AnalyzeProfile("missing.out"); err == nil || !strings.Contains(err.Error(), "no module line") {
		t.Fatalf("module-path error = %v", err)
	}
}

func TestAnalyzeDirectiveValidationAndSkippedTrees(t *testing.T) {
	markerText := "//" + " coverage-ignore"
	root := t.TempDir()
	policyFile(t, root, "go.mod", "module example.com/m\n")
	policyFile(t, root, "p/f.go", "package p\nfunc F() {}\n")
	policyFile(t, root, ".hidden/bad.go", "package hidden\n"+markerText+"\n")
	policyFile(t, root, "vendor/bad.go", "package vendor\n"+markerText+"\n")
	policyFile(t, root, "nested-repo/.git/config", "fixture")
	policyFile(t, root, "nested-repo/bad.go", "package nested\n"+markerText+"\n")
	policyFile(t, root, "nested-worktree/.git", "gitdir: elsewhere")
	policyFile(t, root, "nested-worktree/bad.go", "package nested\n"+markerText+"\n")
	profilePath := policyProfile(t, root, "example.com/m/p/f.go:2.1,2.5 1 1\n")
	if _, err := Analyze(profilePath, root, "example.com/m"); err != nil {
		t.Fatalf("skipped trees affected inventory: %v", err)
	}
	policyFile(t, root, "p/bad.go", "package p\n"+markerText+"\n")
	if _, err := Analyze(profilePath, root, "example.com/m"); err == nil || !strings.Contains(err.Error(), "requires a non-empty reason") {
		t.Fatalf("reasonless directive error = %v", err)
	}
}

func TestRegeneratePreservesReviewedDirectiveAndOptionalLedgers(t *testing.T) {
	markerText := "//" + " coverage-ignore: impossible"
	files := map[string]string{
		"p/f.go":      "package p\nfunc F() { " + markerText + "\n}\n",
		"p/f_test.go": "package p\n" + markerText + "\nfunc helper() {}\n",
	}
	analysis := analyzePolicy(t, files, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	base := mustBaseline(t, analysis)
	regenerated, err := Regenerate(analysis, &base, Review{})
	if err != nil {
		t.Fatal(err)
	}
	if len(regenerated.ProductionDirectives) != 1 || len(regenerated.TestDirectives) != 1 {
		t.Fatalf("directive inventories = %#v / %#v", regenerated.ProductionDirectives, regenerated.TestDirectives)
	}

	platform := PlatformDirective{
		Directive: Directive{File: "p/platform_darwin.go", Line: 2, TargetLine: 2, Reason: "rollback"},
		Platforms: []string{"windows", "darwin"}, Class: IgnorePlatformOnly, Evidence: "platform test",
	}
	otherPlatform := PlatformDirective{
		Directive: Directive{File: "a/platform_windows.go", Line: 3, TargetLine: 3, Reason: "rollback"},
		Platforms: []string{"windows"}, Class: IgnorePlatformOnly, Evidence: "platform test",
	}
	mutants := []EquivalentMutant{{File: "cmd/covercheck/main.go", Line: 42, Column: 13, Mutator: "ARITHMETIC_BASE", Reason: "equivalent by proof"}}
	withLedgers, err := Regenerate(analysis, &base, Review{PlatformDirectives: []PlatformDirective{platform, otherPlatform}, EquivalentMutants: mutants})
	if err != nil {
		t.Fatal(err)
	}
	if len(withLedgers.PlatformDirectives) != 2 || len(withLedgers.EquivalentMutants) != 1 {
		t.Fatalf("optional ledgers = %#v / %#v", withLedgers.PlatformDirectives, withLedgers.EquivalentMutants)
	}
	canonical, err := CanonicalBaseline(withLedgers)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(canonical), "a/platform_windows.go") > strings.Index(string(canonical), "p/platform_darwin.go") ||
		strings.LastIndex(string(canonical), "darwin") > strings.LastIndex(string(canonical), "windows") {
		t.Fatalf("platform entries are not canonical: %s", canonical)
	}
}

func TestRegenerateRejectsInvalidMoveAndUnreviewedDirective(t *testing.T) {
	files := map[string]string{"p/f.go": "package p\nfunc F() {}\nfunc G() {}\n"}
	first := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 0\nexample.com/m/p/f.go:3.1,3.5 1 1\n")
	base := mustBaseline(t, first)
	swap := analyzePolicy(t, files, "example.com/m/p/f.go:2.1,2.5 1 1\nexample.com/m/p/f.go:3.1,3.5 1 0\n")
	badPrevious := identity("p/f.go", 99, 1, 99, 2, 1)
	if _, err := Regenerate(swap, &base, Review{Misses: []MissAdmission{{Identity: swap.RawMisses[0], Reason: "moved", MovedFrom: &badPrevious}}}); err == nil || !strings.Contains(err.Error(), "invalid previous identity") {
		t.Fatalf("invalid move error = %v", err)
	}
	stillPresent := base.Repository[0].Identity
	if _, err := Regenerate(first, &base, Review{Misses: []MissAdmission{{Identity: first.RawMisses[0], Reason: "moved", MovedFrom: &stillPresent}}}); err != nil {
		// The unchanged prior admission wins and intentionally ignores irrelevant review evidence.
		t.Fatalf("unchanged admission should remain authoritative: %v", err)
	}

	markerText := "//" + " coverage-ignore: impossible"
	directiveAnalysis := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() { " + markerText + "\n}\n"}, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	if _, err := Regenerate(directiveAnalysis, nil, Review{Misses: []MissAdmission{{Identity: directiveAnalysis.RawMisses[0], Reason: "reviewed"}}}); err == nil || !strings.Contains(err.Error(), "directive") {
		t.Fatalf("unreviewed directive error = %v", err)
	}
	badReview := reviewEveryMiss(directiveAnalysis)
	badReview.EquivalentMutants = []EquivalentMutant{{File: "p/f.go", Line: 2, Column: 1, Mutator: "A"}}
	if _, err := Regenerate(directiveAnalysis, nil, badReview); err == nil || !strings.Contains(err.Error(), "equivalent mutant") {
		t.Fatalf("regeneration validation error = %v", err)
	}
}

func TestCanonicalBaselineRequiresCompleteSelectorProjection(t *testing.T) {
	analysis := analyzePolicy(t, map[string]string{"internal/git/f.go": "package git\nfunc F() {}\n"}, "example.com/m/internal/git/f.go:2.1,2.5 1 0\n")
	base := mustBaseline(t, analysis)
	for index := range base.Selectors {
		if base.Selectors[index].Name == "repository-effort-lifecycle" {
			base.Selectors[index].Misses = nil
		}
	}
	if _, err := CanonicalBaseline(base); err == nil || !strings.Contains(err.Error(), "omits applicable miss") {
		t.Fatalf("selector omission error = %v", err)
	}
}

func TestEvaluateReportsSelectorAndRemovedDirectiveDrift(t *testing.T) {
	files := map[string]string{"internal/git/f.go": "package git\nfunc F() {}\nfunc G() {}\n"}
	first := analyzePolicy(t, files, "example.com/m/internal/git/f.go:2.1,2.5 1 0\nexample.com/m/internal/git/f.go:3.1,3.5 1 1\n")
	base := mustBaseline(t, first)
	swap := analyzePolicy(t, files, "example.com/m/internal/git/f.go:2.1,2.5 1 1\nexample.com/m/internal/git/f.go:3.1,3.5 1 0\n")
	findings := Evaluate(swap, base)
	if !hasFinding(findings, "selector-identity-added") {
		t.Fatalf("selector drift findings = %#v", findings)
	}

	markerText := "//" + " coverage-ignore: impossible"
	withDirective := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() { " + markerText + "\n}\n"}, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	directiveBase := mustBaseline(t, withDirective)
	withoutDirective := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	if findings := Evaluate(withoutDirective, directiveBase); !hasFinding(findings, "production-directive-removed") {
		t.Fatalf("removed directive findings = %#v", findings)
	}
}

func TestEvaluateComparesMeasuredAndTestDirectiveState(t *testing.T) {
	markerText := "//" + " coverage-ignore: impossible"
	files := map[string]string{
		"p/f.go":      "package p\nfunc F() { " + markerText + "\n}\n",
		"p/f_test.go": "package p\n" + markerText + "\nfunc helper() {}\n",
	}
	mapped := analyzePolicy(t, files, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	base := mustBaseline(t, mapped)
	unmapped := analyzePolicy(t, files, "example.com/m/p/f.go:3.1,3.2 1 0\n")
	if findings := Evaluate(unmapped, base); !hasFinding(findings, "production-directive-changed") {
		t.Fatalf("mapping drift findings = %#v", findings)
	}
	changedFiles := map[string]string{
		"p/f.go":      files["p/f.go"],
		"p/f_test.go": "package p\n//" + " coverage-ignore: changed test reason\nfunc helper() {}\n",
	}
	changed := analyzePolicy(t, changedFiles, "example.com/m/p/f.go:2.12,3.2 1 0\n")
	if findings := Evaluate(changed, base); !hasFinding(findings, "test-directive-changed") || !hasFinding(findings, "test-directive-removed") {
		t.Fatalf("test directive drift findings = %#v", findings)
	}
}

func TestCanonicalBaselineSortsAllExactIdentityAndMutantAxes(t *testing.T) {
	analysis := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	base := mustBaseline(t, analysis)
	base.EquivalentMutants = []EquivalentMutant{
		{File: "b.go", Line: 2, Column: 1, Mutator: "B", Reason: "r"},
		{File: "a.go", Line: 3, Column: 1, Mutator: "B", Reason: "r"},
		{File: "a.go", Line: 2, Column: 3, Mutator: "B", Reason: "r"},
		{File: "a.go", Line: 2, Column: 1, Mutator: "C", Reason: "r"},
		{File: "a.go", Line: 2, Column: 1, Mutator: "A", Reason: "r"},
	}
	raw, err := CanonicalBaseline(base)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index < len(loaded.EquivalentMutants); index++ {
		if compareEquivalentMutant(loaded.EquivalentMutants[index-1], loaded.EquivalentMutants[index]) >= 0 {
			t.Fatalf("mutants not sorted: %#v", loaded.EquivalentMutants)
		}
	}
}

func TestStrictJSONRejectsMultipleValuesAndTrailingGarbage(t *testing.T) {
	for name, body := range map[string]string{
		"multiple": `{} {}`,
		"trailing": `{} x`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "review.json")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadReview(path); err == nil {
				t.Fatal("expected strict JSON error")
			}
		})
	}
}

func TestAWFPlatformLedgerIsExactAndRelatedToProductionInventory(t *testing.T) {
	analysis := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	base := mustBaseline(t, analysis)
	base.ModulePath = awfModulePath
	for _, spec := range []struct {
		file     string
		platform string
		lines    []int
	}{
		{file: "internal/effort/publication_darwin.go", platform: "darwin", lines: []int{73, 94}},
		{file: "internal/effort/publication_windows.go", platform: "windows", lines: []int{73, 94}},
	} {
		for _, line := range spec.lines {
			directive := Directive{File: spec.file, Line: line, TargetLine: line, Reason: "rollback", Mapped: false}
			base.ProductionDirectives = append(base.ProductionDirectives, DirectiveAdmission{Directive: directive, Class: IgnorePlatformOnly, Evidence: "platform test"})
			base.PlatformDirectives = append(base.PlatformDirectives, PlatformDirective{Directive: directive, Platforms: []string{spec.platform}, Class: IgnorePlatformOnly, Evidence: "platform test"})
		}
	}
	if _, err := CanonicalBaseline(base); err != nil {
		t.Fatalf("valid awf ledger: %v", err)
	}
	mutations := map[string]func(*Baseline){
		"missing":        func(b *Baseline) { b.PlatformDirectives = b.PlatformDirectives[:3] },
		"extra":          func(b *Baseline) { b.PlatformDirectives = append(b.PlatformDirectives, b.PlatformDirectives[0]) },
		"duplicate":      func(b *Baseline) { b.PlatformDirectives[1] = b.PlatformDirectives[0] },
		"wrong platform": func(b *Baseline) { b.PlatformDirectives[0].Platforms = []string{"linux"} },
		"unrelated path": func(b *Baseline) { b.PlatformDirectives[0].Directive.File = "internal/other/file.go" },
		"not admitted":   func(b *Baseline) { b.ProductionDirectives = b.ProductionDirectives[:len(b.ProductionDirectives)-1] },
		"imbalanced paths": func(b *Baseline) {
			old := b.PlatformDirectives[0].Directive
			replacement := old
			replacement.File = "internal/effort/publication_windows.go"
			replacement.Line = 75
			replacement.TargetLine = 75
			b.PlatformDirectives[0].Directive = replacement
			b.PlatformDirectives[0].Platforms = []string{"windows"}
			for index := range b.ProductionDirectives {
				if b.ProductionDirectives[index].Directive == old {
					b.ProductionDirectives[index].Directive = replacement
				}
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := base
			copy.ProductionDirectives = slices.Clone(base.ProductionDirectives)
			copy.PlatformDirectives = append([]PlatformDirective(nil), base.PlatformDirectives...)
			for index := range copy.PlatformDirectives {
				copy.PlatformDirectives[index].Platforms = slices.Clone(base.PlatformDirectives[index].Platforms)
			}
			mutate(&copy)
			if _, err := CanonicalBaseline(copy); err == nil {
				t.Fatal("expected exact awf ledger error")
			}
		})
	}
}

func TestBaselineValidationAdditionalInvalidEvidence(t *testing.T) {
	analysis := analyzePolicy(t, map[string]string{"p/f.go": "package p\nfunc F() {}\n"}, "example.com/m/p/f.go:2.1,2.5 1 0\n")
	valid := mustBaseline(t, analysis)
	tests := map[string]func(*Baseline){
		"module path":    func(b *Baseline) { b.ModulePath = "" },
		"selector count": func(b *Baseline) { b.Selectors = b.Selectors[:len(b.Selectors)-1] },
		"selector name":  func(b *Baseline) { b.Selectors[0].Name = "unknown" },
		"selector duplicate": func(b *Baseline) {
			b.Selectors[1].Name = b.Selectors[0].Name
			b.Selectors[1].Roots = slices.Clone(b.Selectors[0].Roots)
		},
		"duplicate directive": func(b *Baseline) {
			d := DirectiveAdmission{Directive: Directive{File: "p/f.go", Line: 2, TargetLine: 2, Reason: "x"}, Class: IgnoreImpossibleState, Evidence: "x"}
			b.ProductionDirectives = []DirectiveAdmission{d, d}
		},
		"invalid test directive": func(b *Baseline) {
			b.TestDirectives = []Directive{{File: "../test.go", Line: 2, TargetLine: 2, Reason: "x"}}
		},
		"duplicate test directive": func(b *Baseline) {
			d := Directive{File: "p/f_test.go", Line: 2, TargetLine: 2, Reason: "x"}
			b.TestDirectives = []Directive{d, d}
		},
		"platform class": func(b *Baseline) {
			b.PlatformDirectives = []PlatformDirective{{Directive: Directive{File: "p/f.go", Line: 2, TargetLine: 2, Reason: "x"}, Platforms: []string{"darwin"}, Class: IgnoreImpossibleState, Evidence: "x"}}
		},
		"duplicate mutant": func(b *Baseline) {
			m := EquivalentMutant{File: "p/f.go", Line: 2, Column: 1, Mutator: "A", Reason: "x"}
			b.EquivalentMutants = []EquivalentMutant{m, m}
		},
		"bad identity":     func(b *Baseline) { b.Repository[0].Identity.File = "/absolute.go" },
		"dot-dot identity": func(b *Baseline) { b.Repository[0].Identity.File = ".." },
		"parent identity":  func(b *Baseline) { b.Repository[0].Identity.File = "../outside.go" },
		"unclean identity": func(b *Baseline) { b.Repository[0].Identity.File = "p/../f.go" },
		"dot identity":     func(b *Baseline) { b.Repository[0].Identity.File = "./p/f.go" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			copy := valid
			copy.Repository = slices.Clone(valid.Repository)
			copy.Selectors = append([]SelectorBaseline(nil), valid.Selectors...)
			for index := range copy.Selectors {
				copy.Selectors[index].Roots = slices.Clone(valid.Selectors[index].Roots)
				copy.Selectors[index].Misses = slices.Clone(valid.Selectors[index].Misses)
			}
			mutate(&copy)
			if _, err := CanonicalBaseline(copy); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
