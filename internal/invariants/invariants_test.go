package invariants_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/invariants"
)

func writeADR(t *testing.T, dir, name, status, invBody string) {
	t.Helper()
	content := "---\nstatus: " + status + "\ndate: 2026-06-25\ntags: [x]\n---\n# ADR-X: T\n## Invariants\n" + invBody + "\n## Consequences\nc\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func goSrc(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
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
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-backed\nfunc T() {}\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, err := invariants.Check(dir, root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 1 || f[0].Slug != "fixture-missing" {
		t.Errorf("expected only fixture-missing unbacked, got %#v", f)
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
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-one\n")
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
	goSrc(t, root, "x.go", "package x\n// invariant: fixture-a\n//invariant: fixture-b\n")
	cfg := &config.InvariantConfig{Sources: []config.InvariantSource{{Globs: []string{"*.go"}, Marker: "//"}}}
	f, _ := invariants.Check(dir, root, cfg)
	if len(f) != 0 {
		t.Errorf("whitespace-tolerant marker should match both, got %#v", f)
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
