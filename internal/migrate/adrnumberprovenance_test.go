package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProvenanceFixture lays out a minimal adopted tree with a decisions
// directory and topic parts, returning the root.
func writeProvenanceFixture(t *testing.T, decisions, parts map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".awf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("docsDir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decisionsDir := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range decisions {
		if err := os.WriteFile(filepath.Join(decisionsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, content := range parts {
		path := filepath.Join(root, ".awf", "topics", "parts", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const provenanceDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// TestADRNumberProvenanceMigration covers the generation-27 retrofit: Applied
// and terminal state-sequence segments stripped with digest text untouched,
// Revised-by lists canonicalized to duplicate-free ascending ADR number, a
// mention of the retired term outside the Status history section preserved,
// and a second run finding nothing to rewrite.
func TestADRNumberProvenanceMigration(t *testing.T) {
	explicit := "---\nstatus: Implemented\ndate: 2026-01-02\n---\n" +
		"# ADR-0002: Explicit\n\n## Context\n\nThe old state-sequence namespace is discussed here.\n\n" +
		"## Status history\n\n- 2026-01-01: Proposed\n" +
		"- 2026-01-02: Implementing; content-sha256: " + provenanceDigest + "\n" +
		"- 2026-01-02: Applied; state-sequence: 1; operations: add `d/t:c`\n" +
		"- 2026-01-03: Applied; state-sequence: 2; operations: update `d/t:c`\n" +
		"- 2026-01-03: Implemented; content-sha256: " + provenanceDigest + "\n"
	implicit := "---\nstatus: Implemented\ndate: 2026-01-04\n---\n" +
		"# ADR-0003: Implicit\n\n## Status history\n\n- 2026-01-03: Proposed\n" +
		"- 2026-01-04: Implemented; content-sha256: " + provenanceDigest + "; state-sequence: 3\n"
	headless := "---\nstatus: Proposed\ndate: 2026-01-06\n---\n# ADR-0005: Headless\n\n## Context\n\nNo history section here.\n"
	abandoned := "---\nstatus: Abandoned\ndate: 2026-01-05\n---\n" +
		"# ADR-0004: Abandoned\n\n## Status history\n\n- 2026-01-04: Proposed\n" +
		"- 2026-01-05: Abandoned; content-sha256: " + provenanceDigest + "; rationale: kept; reason text\n"
	mentioning := "---\nstatus: Abandoned\ndate: 2026-01-07\n---\n" +
		"# ADR-0006: Mentioning\n\n## Status history\n\n- 2026-01-06: Proposed\n" +
		"- 2026-01-07: Abandoned; content-sha256: " + provenanceDigest + "; state-sequence: 4; rationale: dropped with the state-sequence namespace\n"
	root := writeProvenanceFixture(t, map[string]string{
		"0002-explicit.md": explicit,
		"0003-implicit.md": implicit,
		"0004-kept.md":     abandoned,
		"0005-headless.md": headless,
		"0006-mention.md":  mentioning,
	}, map[string]string{
		"rendering/pi/current-state.md":  "### `invariant: x`\n\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0167, ADR-0166, ADR-0166\nBacking: test\n",
		"tooling/cli/current-state.md":   "### `invariant: y`\n\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0100, ADR-0101\nBacking: test\n",
		"tooling/other/current-state.md": "### `invariant: z`\n\nProse.\nOrigin: ADR-0001\nBacking: test\n",
	})

	var out Changes
	if err := applyADRNumberProvenance(root, &out); err != nil {
		t.Fatalf("applyADRNumberProvenance: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	wantExplicit := "---\nstatus: Implemented\ndate: 2026-01-02\n---\n" +
		"# ADR-0002: Explicit\n\n## Context\n\nThe old state-sequence namespace is discussed here.\n\n" +
		"## Status history\n\n- 2026-01-01: Proposed\n" +
		"- 2026-01-02: Implementing; content-sha256: " + provenanceDigest + "\n" +
		"- 2026-01-02: Applied; operations: add `d/t:c`\n" +
		"- 2026-01-03: Applied; operations: update `d/t:c`\n" +
		"- 2026-01-03: Implemented; content-sha256: " + provenanceDigest + "\n"
	if got := read("docs/decisions/0002-explicit.md"); got != wantExplicit {
		t.Errorf("explicit ADR mismatch:\n got: %q\nwant: %q", got, wantExplicit)
	}
	wantImplicit := "---\nstatus: Implemented\ndate: 2026-01-04\n---\n" +
		"# ADR-0003: Implicit\n\n## Status history\n\n- 2026-01-03: Proposed\n" +
		"- 2026-01-04: Implemented; content-sha256: " + provenanceDigest + "\n"
	if got := read("docs/decisions/0003-implicit.md"); got != wantImplicit {
		t.Errorf("implicit ADR mismatch:\n got: %q\nwant: %q", got, wantImplicit)
	}
	if got := read("docs/decisions/0004-kept.md"); got != abandoned {
		t.Errorf("abandoned ADR without a sequence must stay byte-identical:\n got: %q", got)
	}
	if got := read("docs/decisions/0005-headless.md"); got != headless {
		t.Errorf("record without a Status history section must stay byte-identical:\n got: %q", got)
	}
	wantMentioning := "---\nstatus: Abandoned\ndate: 2026-01-07\n---\n" +
		"# ADR-0006: Mentioning\n\n## Status history\n\n- 2026-01-06: Proposed\n" +
		"- 2026-01-07: Abandoned; content-sha256: " + provenanceDigest + "; rationale: dropped with the state-sequence namespace\n"
	if got := read("docs/decisions/0006-mention.md"); got != wantMentioning {
		t.Errorf("a rationale mentioning the term must not abort the rewrite:\n got: %q\nwant: %q", got, wantMentioning)
	}
	wantPart := "### `invariant: x`\n\nProse.\nOrigin: ADR-0001\nRevised-by: ADR-0166, ADR-0167\nBacking: test\n"
	if got := read(".awf/topics/parts/rendering/pi/current-state.md"); got != wantPart {
		t.Errorf("Revised-by canonicalization mismatch:\n got: %q\nwant: %q", got, wantPart)
	}
	if got := read(".awf/topics/parts/tooling/cli/current-state.md"); !strings.Contains(got, "Revised-by: ADR-0100, ADR-0101\n") {
		t.Errorf("already-ascending Revised-by must stay unchanged: %q", got)
	}
	for _, want := range []string{
		"adr-number-provenance: 0002-explicit.md: stripped state-sequence segment(s)",
		"adr-number-provenance: 0003-implicit.md: stripped state-sequence segment(s)",
		"adr-number-provenance: .awf/topics/parts/rendering/pi/current-state.md: Revised-by canonicalized to ascending ADR number",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q in:\n%s", want, out.String())
		}
	}

	var second Changes
	if err := applyADRNumberProvenance(root, &second); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.String() != "" {
		t.Errorf("second run must rewrite nothing, announced:\n%s", second.String())
	}
}

// TestADRNumberProvenanceMigrationRefusesResidual proves the hard stop: a
// status-history line carrying state-sequence text neither rewrite consumes
// names the file instead of being guessed at.
func TestADRNumberProvenanceMigrationRefusesResidual(t *testing.T) {
	root := writeProvenanceFixture(t, map[string]string{
		"0002-bad.md": "---\nstatus: Implemented\ndate: 2026-01-02\n---\n" +
			"# ADR-0002: Bad\n\n## Status history\n\n- 2026-01-01: Proposed\n" +
			"- 2026-01-02: Bogus; state-sequence: 3\n",
	}, nil)
	err := applyADRNumberProvenance(root, &Changes{})
	if err == nil || !strings.Contains(err.Error(), "0002-bad.md") || !strings.Contains(err.Error(), "cannot rewrite status-history line") {
		t.Fatalf("want a residual-line error naming the file, got %v", err)
	}
}

// TestADRNumberProvenanceMigrationSkipsBareTrees proves the no-config and
// no-decisions guards leave a tree untouched.
func TestADRNumberProvenanceMigrationSkipsBareTrees(t *testing.T) {
	if err := applyADRNumberProvenance(t.TempDir(), &Changes{}); err != nil {
		t.Fatalf("no config: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("docsDir: docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyADRNumberProvenance(root, &Changes{}); err != nil {
		t.Fatalf("no decisions dir: %v", err)
	}
}

// TestADRNumberProvenanceMigrationSurfacesLoadErrors proves the migration
// propagates a config it cannot parse and a corpus it cannot load.
func TestADRNumberProvenanceMigrationSurfacesLoadErrors(t *testing.T) {
	badConfig := t.TempDir()
	if err := os.MkdirAll(filepath.Join(badConfig, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badConfig, ".awf", "config.yaml"), []byte("docsDir: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyADRNumberProvenance(badConfig, &Changes{}); err == nil {
		t.Fatal("an unparseable config must be an error")
	}

	badCorpus := writeProvenanceFixture(t, map[string]string{
		"0001-bad.md": "---\nstatus: [\ndate: 2026-01-01\n---\n# ADR-0001: Bad\n",
	}, nil)
	if err := applyADRNumberProvenance(badCorpus, &Changes{}); err == nil {
		t.Fatal("an unloadable corpus must be an error")
	}
}
