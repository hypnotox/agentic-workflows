package invariants_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestDeclaringADRs exercises the shared slug→declaring-Implemented-ADR join
// directly: an Implemented declarer maps one-to-one, a non-Implemented one is
// ignored, and a retirement drops the slug. The duplicate and dangling error
// paths are covered through Check (TestCheckDuplicateSlug and the retirement
// tests), which now routes through DeclaringADRs.
func TestDeclaringADRs(t *testing.T) {
	dir := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: kept` — x.\n- `invariant: gone` — y.")
	writeADR(t, dir, "0002-b.md", "Proposed", "- `invariant: ignored` — z.")
	// 0003 retires 0001's `gone` slug.
	content := testsupport.ADR("Implemented", testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithRetiresInvariants("gone"), testsupport.WithTitle("X: T"),
		testsupport.WithBody("## Invariants\n- `invariant: fresh` — w.\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(dir, "0003-c.md"), content)

	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := invariants.DeclaringADRs(adrs)
	if err != nil {
		t.Fatalf("DeclaringADRs: %v", err)
	}
	if got["kept"].ADR != "0001-a.md" || got["fresh"].ADR != "0003-c.md" {
		t.Errorf("expected kept→0001, fresh→0003, got %#v", got)
	}
	if got["kept"].Class != invariants.ClassBacked {
		t.Errorf("expected kept classified backed, got %q", got["kept"].Class)
	}
	if _, ok := got["gone"]; ok {
		t.Errorf("retired slug 'gone' must be dropped: %#v", got)
	}
	if _, ok := got["ignored"]; ok {
		t.Errorf("Proposed ADR's slug must be ignored: %#v", got)
	}
}

// TestDeclaringADRsUnbackedClass pins that an `unbacked-invariant:` declaration
// is classified ClassUnbacked, that its Verify text captures a `Verify:` note on
// the bullet (markdown emphasis trimmed), and that a backed declaration ignores
// the note.
func TestDeclaringADRsUnbackedClass(t *testing.T) {
	dir := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented",
		"- `unbacked-invariant: reasoned` — a contract. **Verify:** run it by hand.\n"+
			"- `unbacked-invariant: bare` — a contract with no guidance.\n"+
			"- `invariant: proven` — a backed one.")
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := invariants.DeclaringADRs(adrs)
	if err != nil {
		t.Fatalf("DeclaringADRs: %v", err)
	}
	if got["reasoned"].Class != invariants.ClassUnbacked || got["reasoned"].Verify != "run it by hand." {
		t.Errorf("reasoned: want unbacked + verify text, got %#v", got["reasoned"])
	}
	if got["bare"].Class != invariants.ClassUnbacked || got["bare"].Verify != "" {
		t.Errorf("bare: want unbacked, no verify, got %#v", got["bare"])
	}
	if got["proven"].Class != invariants.ClassBacked {
		t.Errorf("proven: want backed, got %#v", got["proven"])
	}
}

// TestDeclaringADRsVerifyWrapped pins that a `Verify:` note wrapped across a
// bullet's continuation lines is captured whole — not truncated at the first
// physical line and not misread as a missing note (the ADR template wraps it).
func TestDeclaringADRsVerifyWrapped(t *testing.T) {
	dir := t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented",
		"- `unbacked-invariant: wrapped` — a reasoned contract whose property\n"+
			"  description runs long. **Verify:** inspect the assembled result and\n"+
			"  confirm no writer dependency is held.")
	adrs, err := adr.ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := invariants.DeclaringADRs(adrs)
	if err != nil {
		t.Fatalf("DeclaringADRs: %v", err)
	}
	want := "inspect the assembled result and confirm no writer dependency is held."
	if got["wrapped"].Class != invariants.ClassUnbacked || got["wrapped"].Verify != want {
		t.Errorf("wrapped: want unbacked + full note %q, got %#v", want, got["wrapped"])
	}
}

func writeADR(t *testing.T, dir, name, status, invBody string) {
	t.Helper()
	content := testsupport.ADR(status, testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithTitle("X: T"), testsupport.WithBody("## Invariants\n"+invBody+"\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(dir, name), content)
}

func goSrc(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "x.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goSrcConfig is the source config the dogfood tests use: Go files, `//` marker.
func goSrcConfig() *config.InvariantConfig {
	return &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}}}
}

// The tests below preserve the ADR-0007 invariant slugs (their only backing
// comments live in this file, which this task rewrote to the two-marker model).
// They keep the source proof markers so the dogfood `*.go`/`//` scan (source-only
// fallback, no testGlobs) keeps finding `invariants-implemented-only`,
// `invariants-unbacked-detected`, and `invariants-duplicate-slug`.

// invariant: invariants-implemented-only
func TestCheckImplementedOnly(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-impl` — x.")
	writeADR(t, dir, "0002-b.md", "Proposed", "- `invariant: fixture-prop` — x.")
	writeADR(t, dir, "0003-c.md", "Accepted", "- `invariant: fixture-acc` — x.")
	writeADR(t, dir, "0004-d.md", "Superseded by ADR-0001", "- `invariant: fixture-sup` — x.")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-impl" {
		t.Errorf("expected only fixture-impl required, got %#v", f)
	}
}

// invariant: invariants-unbacked-detected
func TestCheckUnbackedAndBacked(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-backed` — x.\n- `invariant: fixture-missing` — y.")
	goSrc(t, root, "package x\n// invariant: fixture-backed\nfunc T() {}\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", f)
	}
}

// TestCheckBackedProofInTestFile: with testGlobs configured, a proof marker in a
// test file backs the slug (structural teeth, ADR-0105).
func TestCheckBackedProofInTestFile(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: t-backed` — x.")
	testsupport.WriteFile(t, filepath.Join(root, "x_test.go"), "package x\n// invariant: t-backed\n")
	cfg := goSrcConfig()
	cfg.TestGlobs = []string{"**/*_test.go"}
	f, notes, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 || len(notes) != 0 {
		t.Errorf("proof in a test file must back the slug cleanly, got findings=%#v notes=%#v", f, notes)
	}
}

// TestCheckBackedNoProof: a backed declaration with no proof marker anywhere
// fails (backed-requires-proof).
func TestCheckBackedNoProof(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: b-missing` — x.")
	goSrc(t, root, "package x\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	// invariant: backed-requires-proof
	if len(f) != 1 || f[0].Slug != "b-missing" || f[0].Status != invariants.Unbacked {
		t.Errorf("backed-without-proof must fail unbacked, got %#v", f)
	}
}

// TestCheckProofInNonTestFileScoped: with testGlobs set, a proof marker only in a
// non-test source file does NOT back the slug (proof-marker-test-scoped); the
// identical marker backs it with testGlobs empty (absent-testglobs-source-fallback).
func TestCheckProofInNonTestFileScoped(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: b-prod` — x.")
	goSrc(t, root, "package x\n// invariant: b-prod\n") // x.go: a non-test source file

	scoped := goSrcConfig()
	scoped.TestGlobs = []string{"**/*_test.go"}
	f, notes, err := invariants.Check(dir, root, scoped)
	if err != nil {
		t.Fatal(err)
	}
	// invariant: proof-marker-test-scoped
	if len(f) != 1 || f[0].Slug != "b-prod" || f[0].Status != invariants.Unbacked {
		t.Fatalf("testGlobs set: non-test proof must not back, got %#v", f)
	}
	if len(notes) != 0 {
		t.Errorf("a declared slug's proof marker is not dangling, got notes=%#v", notes)
	}

	// Same source, no testGlobs → source-only fallback backs it.
	f, _, err = invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	// invariant: absent-testglobs-source-fallback
	if len(f) != 0 {
		t.Errorf("testGlobs empty: source-only fallback must back the slug, got %#v", f)
	}
}

// TestCheckUnbackedWithVerify: an unbacked declaration carrying a Verify: note
// and no proof marker is clean.
func TestCheckUnbackedWithVerify(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `unbacked-invariant: u-ok` — a contract. **Verify:** inspect by hand.")
	goSrc(t, root, "package x\n// touches-invariant: u-ok — the reasoned site.\n")
	f, notes, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Errorf("unbacked-with-verify (no proof) must be clean, got %#v", f)
	}
	if len(notes) != 0 {
		t.Errorf("a touches marker with a note is silent, got notes=%#v", notes)
	}
}

// TestCheckUnbackedWithoutVerify: an unbacked declaration missing its Verify:
// note fails (unbacked-requires-verify-note).
func TestCheckUnbackedWithoutVerify(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `unbacked-invariant: u-noverify` — a contract with no guidance.")
	goSrc(t, root, "package x\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	// invariant: unbacked-requires-verify-note
	if len(f) != 1 || f[0].Slug != "u-noverify" || f[0].Status != invariants.MissingVerify {
		t.Errorf("unbacked-without-verify must fail MissingVerify, got %#v", f)
	}
}

// TestCheckUnbackedWithProof: an unbacked declaration for which a proof marker
// exists in scope fails (unbacked-refuses-proof).
func TestCheckUnbackedWithProof(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `unbacked-invariant: u-proof` — a contract. **Verify:** by hand.")
	goSrc(t, root, "package x\n// invariant: u-proof\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	// invariant: unbacked-refuses-proof
	if len(f) != 1 || f[0].Slug != "u-proof" || f[0].Status != invariants.UnbackedHasProof {
		t.Errorf("unbacked-with-proof must fail UnbackedHasProof, got %#v", f)
	}
}

// TestCheckUnbackedProofAndMissingVerify: an unbacked declaration that both has
// a proof marker in scope and lacks a Verify: note raises two findings for the
// one slug (UnbackedHasProof then MissingVerify), exercising the same-slug
// finding tie-break.
func TestCheckUnbackedProofAndMissingVerify(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `unbacked-invariant: u-both` — a contract with no guidance.")
	goSrc(t, root, "package x\n// invariant: u-both\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].Slug != "u-both" || f[1].Slug != "u-both" {
		t.Fatalf("want two findings for u-both, got %#v", f)
	}
	if f[0].Status != invariants.MissingVerify || f[1].Status != invariants.UnbackedHasProof {
		t.Errorf("want MissingVerify then UnbackedHasProof (status-sorted), got %q,%q", f[0].Status, f[1].Status)
	}
}

// TestCheckDanglingMarkerNote: a proof or touches marker naming a slug no
// Implemented ADR declares yields a note, never a finding (dangling-marker-advisory);
// a slug named by both a proof and a touches marker is noted once.
func TestCheckDanglingMarkerNote(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: real` — x.")
	goSrc(t, root, "package x\n"+
		"// invariant: real\n"+ // backs the declared slug
		"// invariant: ghost\n"+ // undeclared proof marker → dangling
		"// touches-invariant: ghost\n"+ // same ghost via touches → deduped
		"// touches-invariant: phantom — a note\n") // undeclared touches → dangling
	f, notes, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("dangling markers must not produce findings, got %#v", f)
	}
	// invariant: dangling-marker-advisory
	if len(notes) != 2 || notes[0].Slug != "ghost" || notes[1].Slug != "phantom" {
		t.Errorf("want one note each for ghost, phantom (sorted), got %#v", notes)
	}
	if !strings.Contains(notes[0].Line(), "ghost") {
		t.Errorf("note line should name the slug, got %q", notes[0].Line())
	}
}

// TestCheckBareTouchesNote: a touches marker on a declared slug carrying no note
// yields an advisory note (bare-touches-note), deduplicated per slug; a touches
// marker with a note stays silent.
func TestCheckBareTouchesNote(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: real` — x.")
	goSrc(t, root, "package x\n"+
		"// invariant: real\n"+
		"// touches-invariant: real\n"+ // bare touches on a declared slug
		"// touches-invariant: real\n") // second bare touches → deduped
	f, notes, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("a bare touches marker must not produce a finding, got %#v", f)
	}
	// invariant: bare-touches-note
	if len(notes) != 1 || notes[0].Slug != "real" || !strings.Contains(notes[0].Line(), "no note") {
		t.Errorf("want one bare-touches note for real, got %#v", notes)
	}
}

// TestCheckPlainCommentIgnored: a marker-opening line that is neither a proof nor
// a touches marker (an ordinary comment) is ignored without a note or finding.
func TestCheckPlainCommentIgnored(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: plain` — x.")
	goSrc(t, root, "package x\n// invariant: plain\n// just an ordinary comment\n")
	f, notes, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 || len(notes) != 0 {
		t.Errorf("an ordinary comment must be inert, got findings=%#v notes=%#v", f, notes)
	}
}

// Anchored scope (ADR-0077): a slashed glob confines the scan to its subtree —
// a tag outside it does not back the slug, with no basename fallback.
func TestCheckAnchoredGlobScope(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-scoped` — x.\n- `invariant: fixture-outside` — y.")
	testsupport.WriteFile(t, filepath.Join(root, "sub", "x.go"), "package x\n// invariant: fixture-scoped\n")
	goSrc(t, root, "package y\n// invariant: fixture-outside\n") // top-level y.go: outside sub/**
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"sub/**"}, Marker: "//"}}}
	f, _, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-outside" {
		t.Errorf("expected only fixture-outside unbacked under sub/** scope, got %#v", f)
	}
}

// A marker inside a string literal (e.g. a test fixture's source-code string)
// must not back a slug: only a marker opening its line, after optional
// indentation, counts as a backing comment.
func TestCheckMarkerMustOpenLine(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-literal` — x.\n- `invariant: fixture-indented` — y.")
	goSrc(t, root, "package x\n"+
		"var s = \"src\\n// invariant: fixture-literal\\n\"\n"+ // mid-line, inside a literal
		"func T() {\n\t// invariant: fixture-indented\n}\n") // indented comment — still backs
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-literal" {
		t.Errorf("want only fixture-literal unbacked (literal must not back, indentation must), got %#v", f)
	}
}

// invariant: invariants-duplicate-slug
func TestCheckDuplicateSlug(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-dup` — x.")
	writeADR(t, dir, "0002-b.md", "Implemented", "- `invariant: fixture-dup` — y.")
	if _, _, err := invariants.Check(dir, root, goSrcConfig()); err == nil {
		t.Error("expected error for duplicate slug")
	}
}

// invariant: invariants-three-state
func TestCheckThreeState(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-one` — x.")
	src := []config.InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}}

	// nil config -> unchecked
	f, _, err := invariants.Check(dir, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Status != invariants.Unchecked {
		t.Fatalf("nil cfg: want 1 unchecked, got %#v", f)
	}
	// source-less config -> unchecked
	f, _, _ = invariants.Check(dir, root, &config.InvariantConfig{})
	if len(f) != 1 || f[0].Status != invariants.Unchecked {
		t.Fatalf("source-less cfg: want 1 unchecked, got %#v", f)
	}
	// disabled -> clean
	f, _, _ = invariants.Check(dir, root, &config.InvariantConfig{Disabled: true, Sources: src})
	if len(f) != 0 {
		t.Errorf("disabled: want clean, got %#v", f)
	}
	// sources, unbacked -> unbacked
	f, _, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 1 || f[0].Status != invariants.Unbacked {
		t.Fatalf("sources unbacked: want 1 unbacked, got %#v", f)
	}
	// sources, backed -> clean
	goSrc(t, root, "package x\n// invariant: fixture-one\n")
	f, _, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 0 {
		t.Errorf("sources backed: want clean, got %#v", f)
	}
}

// invariant: invariants-multilang-scan
func TestCheckMultiLangScan(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-py` — x.")
	if err := os.WriteFile(filepath.Join(root, "t.py"), []byte("# invariant: fixture-py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"**/*.py"}, Marker: "#"}}}
	f, _, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Errorf("python-backed slug should be clean, got %#v", f)
	}
}

// invariant: invariants-marker-literal
func TestCheckMarkerLiteral(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-lit` — x.")
	// marker contains regex metacharacters; must be matched literally.
	if err := os.WriteFile(filepath.Join(root, "t.txt"), []byte("[x] invariant: fixture-lit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"**/*.txt"}, Marker: "[x]"}}}
	f, _, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("literal marker should match, got %#v", f)
	}
}

// invariant: invariants-marker-whitespace
func TestCheckMarkerWhitespace(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-a` — x.\n- `invariant: fixture-b` — y.")
	// one with a space after the marker, one without.
	goSrc(t, root, "package x\n// invariant: fixture-a\n//invariant: fixture-b\n")
	f, _, _ := invariants.Check(dir, root, goSrcConfig())
	if len(f) != 0 {
		t.Errorf("whitespace-tolerant marker should match both, got %#v", f)
	}
}

// TestCheckIgnoresProseCrossReference pins that only a slug leading an invariant
// list item is a declaration: an `invariant: <slug>` token in mid-prose (e.g. a
// parenthetical cross-reference to another ADR's slug) is not. Regression for the
// false duplicate ADR-0009/0010 hit when one ADR's Invariants section referenced
// the other's slug in backticks.
func TestCheckIgnoresProseCrossReference(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	// ADR-1 declares real-slug and, in prose, cross-references ADR-2's slug.
	writeADR(t, dir, "0001-a.md", "Implemented",
		"- `invariant: real-slug` — the real one (co-owned with ADR-2 `invariant: shared-slug`).")
	// ADR-2 legitimately declares shared-slug.
	writeADR(t, dir, "0002-b.md", "Implemented", "- `invariant: shared-slug` — the real declaration.")
	goSrc(t, root, "package x\n// invariant: real-slug\n// invariant: shared-slug\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatalf("a prose cross-reference must not register as a declaration (no duplicate): %v", err)
	}
	if len(f) != 0 {
		t.Errorf("both slugs are declared once and backed; got %#v", f)
	}
}

// TestCheckRecognisesDoubleBacktickDeclaration pins that the double-backtick
// declaration form ADR-0007 uses (a bullet whose tag is wrapped in double
// backticks so the inner single backticks render literally) is still recognised
// as a declaration — see the fixture body for the exact shape.
func TestCheckRecognisesDoubleBacktickDeclaration(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `` `invariant: dbl-slug` `` — declared with double backticks.")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "dbl-slug" {
		t.Fatalf("double-backtick declaration must be required (and unbacked here); got %#v", f)
	}
}

// TestFindingDetail pins the human remedy line for each Status: Unchecked points
// at the invariants config; Unbacked names the backed-but-unproven slug;
// UnbackedHasProof and MissingVerify name their reclassify/verify remedies.
func TestFindingDetail(t *testing.T) {
	unchecked := invariants.Finding{Slug: "fixture-x", Status: invariants.Unchecked}.Detail()
	if !strings.Contains(unchecked, "unchecked") || !strings.Contains(unchecked, "invariants.sources") {
		t.Errorf("unchecked detail unexpected: %q", unchecked)
	}
	unbacked := invariants.Finding{Slug: "fixture-y", Status: invariants.Unbacked}.Detail()
	if !strings.Contains(unbacked, "unbacked") || !strings.Contains(unbacked, "fixture-y") {
		t.Errorf("unbacked detail unexpected: %q", unbacked)
	}
	hasProof := invariants.Finding{Slug: "fixture-z", Status: invariants.UnbackedHasProof}.Detail()
	if !strings.Contains(hasProof, "reclassify") || !strings.Contains(hasProof, "fixture-z") {
		t.Errorf("unbacked-has-proof detail unexpected: %q", hasProof)
	}
	missing := invariants.Finding{Slug: "fixture-w", Status: invariants.MissingVerify}.Detail()
	if !strings.Contains(missing, "Verify:") {
		t.Errorf("missing-verify detail unexpected: %q", missing)
	}
	// Line() wraps Detail with the ADR and slug.
	line := invariants.Finding{Slug: "fixture-y", ADR: "0001-a.md", Status: invariants.Unbacked}.Line()
	if !strings.Contains(line, "0001-a.md") || !strings.Contains(line, "fixture-y") {
		t.Errorf("line unexpected: %q", line)
	}
}

// TestCheckParseDirError pins that a decisions dir holding a malformed ADR (here,
// unparseable YAML frontmatter) surfaces the parse error rather than being
// silently skipped.
func TestCheckParseDirError(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	bad := "---\nstatus: \"unterminated\n---\n# ADR-X: T\n"
	if err := os.WriteFile(filepath.Join(dir, "0001-fixture-bad.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := invariants.Check(dir, root, nil); err == nil {
		t.Error("expected error for malformed ADR frontmatter")
	}
}

// TestCheckSortsMultipleFindings pins deterministic slug-sorted output for both
// the Unchecked (nil cfg) and Unbacked (sources configured, nothing backing)
// paths when more than one slug is required.
func TestCheckSortsMultipleFindings(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-zeta` — z.\n- `invariant: fixture-alpha` — a.")
	src := []config.InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}}

	f, _, err := invariants.Check(dir, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].Slug != "fixture-alpha" || f[1].Slug != "fixture-zeta" {
		t.Fatalf("nil cfg: want alpha,zeta unchecked in order, got %#v", f)
	}

	f, _, err = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].Slug != "fixture-alpha" || f[1].Slug != "fixture-zeta" {
		t.Fatalf("sources: want alpha,zeta unbacked in order, got %#v", f)
	}
}

// TestCheckSkipsVCSDirsAndNonMatchingFiles pins that backing comments living
// inside .git/vendor/node_modules are not honoured (those dirs are skipped) and
// that a file not matching any source glob is ignored without error.
func TestCheckSkipsVCSDirsAndNonMatchingFiles(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-skip` — x.")
	for _, skip := range []string{".git", "vendor", "node_modules"} {
		sub := filepath.Join(root, skip)
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		// A backing comment here must be ignored because the dir is skipped.
		if err := os.WriteFile(filepath.Join(sub, "x.go"), []byte("// invariant: fixture-skip\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A non-matching file in root must be walked past without matching a marker.
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("// invariant: fixture-skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-skip" || f[0].Status != invariants.Unbacked {
		t.Fatalf("backings in skipped dirs / non-matching files must not count, want fixture-skip unbacked, got %#v", f)
	}
}

// A nested checkout — a subdirectory carrying its own .git entry, a directory
// in a primary clone or a gitdir-pointer file in a linked worktree/submodule —
// is another repository's working tree: a marker inside it must not back this
// project's invariants (stale session worktrees under .claude/worktrees/ were
// silently keeping deleted markers "backed"). The root's own .git entry must
// not suppress the scan.
func TestCheckSkipsNestedCheckouts(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-nested` — x.\n- `invariant: fixture-own` — y.")
	// The scanned project is itself a checkout; its root must still scan.
	testsupport.WriteFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	goSrc(t, root, "package x\n// invariant: fixture-own\n")
	// A nested primary clone: .git is a directory.
	testsupport.WriteFile(t, filepath.Join(root, "clone", ".git", "HEAD"), "ref: refs/heads/main\n")
	testsupport.WriteFile(t, filepath.Join(root, "clone", "x.go"), "// invariant: fixture-nested\n")
	// A linked worktree or submodule: .git is a gitdir-pointer file.
	testsupport.WriteFile(t, filepath.Join(root, "wt", ".git"), "gitdir: /elsewhere/.git/worktrees/wt\n")
	testsupport.WriteFile(t, filepath.Join(root, "wt", "x.go"), "// invariant: fixture-nested\n")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-nested" || f[0].Status != invariants.Unbacked {
		t.Fatalf("markers in nested checkouts must not back, want only fixture-nested unbacked, got %#v", f)
	}
}

// A nested awf adopter — a subdirectory carrying its own .awf tree (e.g. an
// embedded example project) — is another awf project: a marker inside it backs
// its own ADRs and must not back this project's invariants, nor surface as a
// dangling advisory here.
func TestCheckSkipsNestedAwfProject(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-own` — y.")
	goSrc(t, root, "package x\n// invariant: fixture-own\n")
	// A nested adopter tree: its .awf/ marks it as a separate awf project.
	testsupport.WriteFile(t, filepath.Join(root, "example", ".awf", "config.yaml"), "prefix: ex\n")
	testsupport.WriteFile(t, filepath.Join(root, "example", "x.go"),
		"// invariant: fixture-own\n// invariant: example-only\n") // neither must be honoured
	f, notes, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("the nested adopter's marker must still leave fixture-own backed by the root marker, got %#v", f)
	}
	if len(notes) != 0 {
		t.Fatalf("a nested adopter's markers must not surface as dangling advisories, got %#v", notes)
	}
}

// TestCheckScanWalkError pins that a WalkDir error during the source scan (here,
// a non-existent root) propagates out of Check rather than being swallowed.
func TestCheckScanWalkError(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-walk` — x.")
	if _, _, err := invariants.Check(dir, filepath.Join(root, "does-not-exist"), goSrcConfig()); err == nil {
		t.Error("expected WalkDir error for non-existent root")
	}
}

// TestCheckScanReadError pins that an unreadable matched source file surfaces the
// read error. A dangling symlink whose name matches a glob is used so the scan
// matches the path but os.ReadFile (which follows the link) fails — no chmod
// fixtures, which are unportable and root-fragile.
func TestCheckScanReadError(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-sym` — x.")
	if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "bad.go")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := invariants.Check(dir, root, goSrcConfig()); err == nil {
		t.Error("expected read error for dangling symlink source file")
	}
}

func writeRetiringADR(t *testing.T, dir, name, status, retires, invBody string) {
	t.Helper()
	content := testsupport.ADR(status, testsupport.WithDate("2026-06-25"), testsupport.WithTags("x"),
		testsupport.WithRetiresInvariants(retires), testsupport.WithTitle("X: T"),
		testsupport.WithBody("## Invariants\n"+invBody+"\n## Consequences\nc\n"))
	testsupport.WriteFile(t, filepath.Join(dir, name), content)
}

// invariant: inv-retirement-drops-slug
func TestCheckRetirementDropsSlug(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-retired` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Implemented", "fixture-retired", "- a textual invariant with no slug.")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Errorf("a slug retired by an Implemented ADR must be dropped from enforcement (unbacked is fine), got %#v", f)
	}
}

// invariant: inv-retirement-implemented-only
func TestCheckRetirementImplementedOnly(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-live` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Proposed", "fixture-live", "- a textual invariant with no slug.")
	f, _, err := invariants.Check(dir, root, goSrcConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-live" || f[0].Status != invariants.Unbacked {
		t.Errorf("a Proposed ADR's retirement must not drop the slug, got %#v", f)
	}
}

// invariant: inv-retirement-dangling-errors
func TestCheckRetirementDangling(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `invariant: fixture-real` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Implemented", "fixture-ghost", "- a textual invariant with no slug.")
	if _, _, err := invariants.Check(dir, root, goSrcConfig()); err == nil || !strings.Contains(err.Error(), "fixture-ghost") {
		t.Errorf("a dangling retirement must error mentioning the slug, got %v", err)
	}
}

// invariant: invariants-zero-slugs-clean
func TestCheckZeroSlugsClean(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- a textual invariant with no slug.")
	for _, cfg := range []*config.InvariantConfig{nil, {}, {Sources: []config.InvariantSource{{Globs: []string{"**/*.go"}, Marker: "//"}}}} {
		f, _, err := invariants.Check(dir, root, cfg)
		if err != nil || len(f) != 0 {
			t.Errorf("zero slugs must be clean (cfg=%#v): got %#v err=%v", cfg, f, err)
		}
	}
}

// hitSlugs joins the slugs of a MarkerHit slice for compact case assertions.
func hitSlugs(hits []invariants.MarkerHit) string {
	s := make([]string, len(hits))
	for i, h := range hits {
		s[i] = h.Slug
	}
	return strings.Join(s, ",")
}

func TestMarkersUnder(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "a.go"), "// invariant: alpha\n")
	testsupport.WriteFile(t, filepath.Join(root, "internal", "b.go"), "// invariant: beta\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "notes.txt"), "// invariant: untracked\n")   // no glob match
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "c.go"), "x := \"// invariant: midline\"\n") // marker not opening the line
	testsupport.WriteFile(t, filepath.Join(root, "vendor", "v.go"), "// invariant: vendored\n")      // skipped by name
	testsupport.WriteFile(t, filepath.Join(root, "nested", ".git"), "gitdir: elsewhere\n")           // nested checkout
	testsupport.WriteFile(t, filepath.Join(root, "nested", "n.go"), "// invariant: nested\n")
	testsupport.WriteFile(t, filepath.Join(root, "adopter", ".awf", "config.yaml"), "prefix: ex\n") // nested adopter
	testsupport.WriteFile(t, filepath.Join(root, "adopter", "z.go"), "// invariant: adopted\n")

	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		{"under dir", []string{"cmd"}, "alpha"},                     // beta out of scope, midline/txt excluded
		{"exact file", []string{"internal/b.go"}, "beta"},           // a queried file that is itself a marker file
		{"union sorted", []string{"cmd", "internal"}, "alpha,beta"}, // sorted, de-duplicated
		{"nested checkout skipped", []string{"nested"}, ""},         // .git-bearing dir is another repo's tree
		{"nested adopter skipped", []string{"adopter"}, ""},         // .awf-bearing dir is another awf project
		{"vendor skipped", []string{"vendor"}, ""},                  // vendor is skipped by name
		{"empty paths", nil, ""},                                    // nothing queried
	}
	for _, c := range cases {
		got, err := invariants.MarkersUnder(root, goSrcConfig(), c.paths)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if hitSlugs(got) != c.want {
			t.Errorf("%s: got %q want %q", c.name, hitSlugs(got), c.want)
		}
	}
}

// TestMarkersUnderTwoMarkers pins that both marker kinds surface a slug under a
// queried path (ADR-0106): a proof-only marker, a touches-only marker, and a slug
// carrying both, with the touches site notes deduped and a bare touches marker
// contributing no note.
func TestMarkersUnderTwoMarkers(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "proof.go"), "package x\n// invariant: proof-only\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "touch.go"), "package x\n// touches-invariant: touch-only — a site note.\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "both.go"), "package x\n"+
		"// invariant: both\n"+
		"// touches-invariant: both — first note.\n"+
		"// touches-invariant: both — first note.\n"+ // duplicate note → deduped
		"// touches-invariant: both\n") // bare touches → no note
	hits, err := invariants.MarkersUnder(root, goSrcConfig(), []string{"cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if hitSlugs(hits) != "both,proof-only,touch-only" { // slug-sorted
		t.Fatalf("slugs: got %q", hitSlugs(hits))
	}
	both, proof, touch := hits[0], hits[1], hits[2]
	if !both.Proof || !both.Touches || len(both.Notes) != 1 || !strings.Contains(both.Notes[0], "first note") {
		t.Errorf("both: want proof+touches with one deduped note, got %#v", both)
	}
	if !proof.Proof || proof.Touches || len(proof.Notes) != 0 {
		t.Errorf("proof-only: want proof, no touches/notes, got %#v", proof)
	}
	// invariant: touches-marker-advisory
	if touch.Proof || !touch.Touches || len(touch.Notes) != 1 || !strings.Contains(touch.Notes[0], "site note") {
		t.Errorf("touch-only: want touches with a note, no proof, got %#v", touch)
	}
}

// TestMarkersUnderUnionScan pins the ADR-0106 union: a proof marker in a test
// file under a queried production path surfaces (the file matches both the source
// glob and testGlobs → the same marker is added once), and a file matched ONLY by
// a testGlobs glob (not by any source glob) is still scanned with the source
// markers.
func TestMarkersUnderUnionScan(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal", "foo", "x_test.go"), "package x\n// invariant: test-proof\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "x.spec"), "// invariant: spec-slug\n") // only testGlobs matches
	cfg := goSrcConfig()
	cfg.TestGlobs = []string{"**/*_test.go", "**/*.spec"}
	hits, err := invariants.MarkersUnder(root, cfg, []string{"internal/foo", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if hitSlugs(hits) != "spec-slug,test-proof" {
		t.Fatalf("union scan: got %q want spec-slug,test-proof", hitSlugs(hits))
	}
	if !hits[1].Proof {
		t.Errorf("test-proof should surface via its proof marker, got %#v", hits[1])
	}
}

// A WalkDir error during the marker scan (a non-existent root) propagates out.
func TestMarkersUnderWalkError(t *testing.T) {
	if _, err := invariants.MarkersUnder(filepath.Join(t.TempDir(), "does-not-exist"), goSrcConfig(), []string{"cmd"}); err == nil {
		t.Error("expected WalkDir error for non-existent root")
	}
}

// An unreadable matched source file under a queried path surfaces the read error.
// A dangling symlink whose name matches a glob keeps the fixture portable (no
// chmod, root-fragile).
func TestMarkersUnderReadError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "cmd", "bad.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := invariants.MarkersUnder(root, goSrcConfig(), []string{"cmd"}); err == nil {
		t.Error("expected read error for dangling symlink source file")
	}
}
