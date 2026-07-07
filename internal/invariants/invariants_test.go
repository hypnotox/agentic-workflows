package invariants_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

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

// The three tests below preserve the ADR-0007 invariant slugs (their only
// backing comments live in this file, which this task rewrites). They are
// retained, updated to the new three-arg Check signature, so the dogfood
// `*.go`/`//` scan keeps finding `invariants-implemented-only`,
// `invariants-unbacked-detected`, and `invariants-duplicate-slug`.

// invariant: invariants-implemented-only
func TestCheckImplementedOnly(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-impl` — x.")
	writeADR(t, dir, "0002-b.md", "Proposed", "- `inv: fixture-prop` — x.")
	writeADR(t, dir, "0003-c.md", "Accepted", "- `inv: fixture-acc` — x.")
	writeADR(t, dir, "0004-d.md", "Superseded by ADR-0001", "- `inv: fixture-sup` — x.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-backed` — x.\n- `inv: fixture-missing` — y.")
	goSrc(t, root, "package x\n// invariant: fixture-backed\nfunc T() {}\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", f)
	}
}

// A marker inside a string literal (e.g. a test fixture's source-code string)
// must not back a slug: only a marker opening its line, after optional
// indentation, counts as a backing comment.
func TestCheckMarkerMustOpenLine(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-literal` — x.\n- `inv: fixture-indented` — y.")
	goSrc(t, root, "package x\n"+
		"var s = \"src\\n// invariant: fixture-literal\\n\"\n"+ // mid-line, inside a literal
		"func T() {\n\t// invariant: fixture-indented\n}\n") // indented comment — still backs
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-dup` — x.")
	writeADR(t, dir, "0002-b.md", "Implemented", "- `inv: fixture-dup` — y.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	if _, err := invariants.Check(dir, root, cfg); err == nil {
		t.Error("expected error for duplicate slug")
	}
}

// invariant: invariants-three-state
func TestCheckThreeState(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-one` — x.")
	src := []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}

	// nil config -> unchecked
	f, err := invariants.Check(dir, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Status != invariants.Unchecked {
		t.Fatalf("nil cfg: want 1 unchecked, got %#v", f)
	}
	// disabled -> clean
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Disabled: true, Sources: src})
	if len(f) != 0 {
		t.Errorf("disabled: want clean, got %#v", f)
	}
	// sources, unbacked -> unbacked
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 1 || f[0].Status != invariants.Unbacked {
		t.Fatalf("sources unbacked: want 1 unbacked, got %#v", f)
	}
	// sources, backed -> clean
	goSrc(t, root, "package x\n// invariant: fixture-one\n")
	f, _ = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
	if len(f) != 0 {
		t.Errorf("sources backed: want clean, got %#v", f)
	}
}

// invariant: invariants-multilang-scan
func TestCheckMultiLangScan(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-py` — x.")
	if err := os.WriteFile(filepath.Join(root, "t.py"), []byte("# invariant: fixture-py\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.py"}, Marker: "#"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-lit` — x.")
	// marker contains regex metacharacters; must be matched literally.
	if err := os.WriteFile(filepath.Join(root, "t.txt"), []byte("[x] invariant: fixture-lit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.txt"}, Marker: "[x]"}}}
	f, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("literal marker should match, got %#v", f)
	}
}

// invariant: invariants-marker-whitespace
func TestCheckMarkerWhitespace(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-a` — x.\n- `inv: fixture-b` — y.")
	// one with a space after the marker, one without.
	goSrc(t, root, "package x\n// invariant: fixture-a\n//invariant: fixture-b\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("whitespace-tolerant marker should match both, got %#v", f)
	}
}

// TestCheckIgnoresProseCrossReference pins that only a slug leading an invariant
// list item is a declaration: an `inv: <slug>` token in mid-prose (e.g. a
// parenthetical cross-reference to another ADR's slug) is not. Regression for the
// false duplicate ADR-0009/0010 hit when one ADR's Invariants section referenced
// the other's slug in backticks.
func TestCheckIgnoresProseCrossReference(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	// ADR-1 declares real-slug and, in prose, cross-references ADR-2's slug.
	writeADR(t, dir, "0001-a.md", "Implemented",
		"- `inv: real-slug` — the real one (co-owned with ADR-2 `inv: shared-slug`).")
	// ADR-2 legitimately declares shared-slug.
	writeADR(t, dir, "0002-b.md", "Implemented", "- `inv: shared-slug` — the real declaration.")
	goSrc(t, root, "package x\n// invariant: real-slug\n// invariant: shared-slug\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `` `inv: dbl-slug` `` — declared with double backticks.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "dbl-slug" {
		t.Fatalf("double-backtick declaration must be required (and unbacked here); got %#v", f)
	}
}

// TestFindingDetail pins the human remedy line for each Status: the Unchecked
// branch points at the invariants config, the Unbacked branch names the missing
// slug and the comment to add.
func TestFindingDetail(t *testing.T) {
	unchecked := invariants.Finding{Slug: "fixture-x", Status: invariants.Unchecked}.Detail()
	if !strings.Contains(unchecked, "unchecked") || !strings.Contains(unchecked, "invariants.sources") {
		t.Errorf("unchecked detail unexpected: %q", unchecked)
	}
	unbacked := invariants.Finding{Slug: "fixture-y", Status: invariants.Unbacked}.Detail()
	if !strings.Contains(unbacked, "unbacked") || !strings.Contains(unbacked, "fixture-y") {
		t.Errorf("unbacked detail unexpected: %q", unbacked)
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
	if _, err := invariants.Check(dir, root, nil); err == nil {
		t.Error("expected error for malformed ADR frontmatter")
	}
}

// TestCheckSortsMultipleFindings pins deterministic slug-sorted output for both
// the Unchecked (nil cfg) and Unbacked (sources configured, nothing backing)
// paths when more than one slug is required.
func TestCheckSortsMultipleFindings(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-zeta` — z.\n- `inv: fixture-alpha` — a.")
	src := []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}

	f, err := invariants.Check(dir, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0].Slug != "fixture-alpha" || f[1].Slug != "fixture-zeta" {
		t.Fatalf("nil cfg: want alpha,zeta unchecked in order, got %#v", f)
	}

	f, err = invariants.Check(dir, root, &config.InvariantConfig{Sources: src})
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-skip` — x.")
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
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-skip" || f[0].Status != invariants.Unbacked {
		t.Fatalf("backings in skipped dirs / non-matching files must not count, want fixture-skip unbacked, got %#v", f)
	}
}

// TestCheckScanWalkError pins that a WalkDir error during the source scan (here,
// a non-existent root) propagates out of Check rather than being swallowed.
func TestCheckScanWalkError(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-walk` — x.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	if _, err := invariants.Check(dir, filepath.Join(root, "does-not-exist"), cfg); err == nil {
		t.Error("expected WalkDir error for non-existent root")
	}
}

// TestCheckScanReadError pins that an unreadable matched source file surfaces the
// read error. A dangling symlink whose name matches a glob is used so the scan
// matches the path but os.ReadFile (which follows the link) fails — no chmod
// fixtures, which are unportable and root-fragile.
func TestCheckScanReadError(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-sym` — x.")
	if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "bad.go")); err != nil {
		t.Fatal(err)
	}
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	if _, err := invariants.Check(dir, root, cfg); err == nil {
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-retired` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Implemented", "fixture-retired", "- a textual invariant with no slug.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-live` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Proposed", "fixture-live", "- a textual invariant with no slug.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
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
	writeADR(t, dir, "0001-a.md", "Implemented", "- `inv: fixture-real` — x.")
	writeRetiringADR(t, dir, "0002-b.md", "Implemented", "fixture-ghost", "- a textual invariant with no slug.")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	if _, err := invariants.Check(dir, root, cfg); err == nil || !strings.Contains(err.Error(), "fixture-ghost") {
		t.Errorf("a dangling retirement must error mentioning the slug, got %v", err)
	}
}

// invariant: invariants-zero-slugs-clean
func TestCheckZeroSlugsClean(t *testing.T) {
	dir, root := t.TempDir(), t.TempDir()
	writeADR(t, dir, "0001-a.md", "Implemented", "- a textual invariant with no slug.")
	for _, cfg := range []*config.InvariantConfig{nil, {}, {Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}} {
		f, err := invariants.Check(dir, root, cfg)
		if err != nil || len(f) != 0 {
			t.Errorf("zero slugs must be clean (cfg=%#v): got %#v err=%v", cfg, f, err)
		}
	}
}
