package currentstate_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/templates"
)

// legacyAuthorityIdents are the identifiers of the deleted ADR-derived authority
// engines: the anchor-supersession/coverage model, the ADR-projected indexes,
// and the invariant-declaration scanner. ADR-0133/0135 make canonical topic
// claims the sole active authority, so none of these may reappear in shipped Go
// or in an embedded runtime template. Each is a CamelCase Go identifier, so a
// whole-word match never trips ordinary prose.
var legacyAuthorityIdents = []string{
	"SupersessionRef", "AnnotatedAnchors", "Chains", "Retirers",
	"StateCovered", "PartiallySuperseded", "DeclaringADRs",
	"RenderActiveMD", "RenderDomainIndex",
}

// legacyContextFields are the old ContextResult expansion fields: the
// ADR-derived governing/related/background context that ADR-0134's topic-centric
// context replaced. They are scoped to internal/project/context.go rather than
// banned tree-wide, because Related collides with the live adr.ADR.Related
// frontmatter field and Background/Plans are ordinary words elsewhere; context.go
// is the one file the legacy result lived in, so their absence there proves the
// expansion is gone without a false positive.
var legacyContextFields = []string{"Governing", "Related", "Background", "Pitfalls", "Plans"}

// migrationApprovalSeams are the only production files that may spell out the
// migration approval file's path literal. internal/upgrade/digest.go binds it to
// the approvalPath const that recomputes the sealed digest before the journaled
// cutover deletion (the rest of internal/upgrade reaches it through that const,
// never the raw string), and the closed-tree sweep protects the file while it
// still exists. A new permanent parser, claim consumer, or runtime path for it is
// exactly what current-state authority forbids after cutover.
var migrationApprovalSeams = []string{
	"internal/upgrade/digest.go",
	"internal/project/sweep.go",
}

// bridgeImportPath is the deleted cross-schema bridge package; no production file
// may import it (its inventory, readiness, snapshot, and approval parsers went
// with it, ADR-0136).
const bridgeImportPath = `"github.com/hypnotox/agentic-workflows/internal/bridge"`

// contextGoSuffix identifies the rewritten context producer among the walked
// files without depending on the test's working directory.
const contextGoSuffix = "internal/project/context.go"

// bannedWholeWords returns which banned identifiers occur in body as whole words.
// The pure matcher is unit-tested directly so the tree scan cannot pass vacuously.
func bannedWholeWords(body string, banned []string) []string {
	var hit []string
	for _, w := range banned {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`).MatchString(body) {
			hit = append(hit, w)
		}
	}
	return hit
}

// productionGoSources walks the shipped Go tree (internal/ and cmd/, no tests)
// and hands each file's slash path and contents to fn, returning the count. It
// deliberately never descends docs/decisions, docs/plans, or the changelog: a
// historical ADR that discusses the retired supersession model in its prose stays
// legal, because it is history, not shipped authority.
func productionGoSources(t *testing.T, fn func(path, body string)) int {
	t.Helper()
	seen := 0
	for _, root := range []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			seen++
			fn(filepath.ToSlash(path), string(b))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return seen
}

// TestLegacyAuthorityAbsent is the deterministic denylist that keeps the deleted
// ADR-derived authority from creeping back after the current-state cutover
// (ADR-0133/0134/0135). It scans shipped Go and runtime templates for the retired
// identifiers, confines the legacy context fields to the file they were removed
// from, forbids the deleted bridge import, and holds the migration approval file
// to its enumerated transient seams. The companion behavioral assertion that the
// ADR-projected ACTIVE.md index is no longer a planned output lives in
// internal/project, where the output plan is reachable without an import cycle.
func TestLegacyAuthorityAbsent(t *testing.T) {
	seams := map[string]bool{}
	sawContext := false
	goSeen := productionGoSources(t, func(path, body string) {
		for _, w := range bannedWholeWords(body, legacyAuthorityIdents) {
			t.Errorf("%s reintroduces the retired authority identifier %q; current-state authority reads topic claims, not the ADR supersession model (ADR-0133)", path, w)
		}
		if strings.Contains(body, bridgeImportPath) {
			t.Errorf("%s imports the deleted internal/bridge package; the final binary carries no cross-schema inventory, readiness, or approval parser (ADR-0136)", path)
		}
		if strings.HasSuffix(path, contextGoSuffix) {
			sawContext = true
			for _, w := range bannedWholeWords(body, legacyContextFields) {
				t.Errorf("context.go carries the retired ADR-derived context field %q; ADR-0134 context selects topic claims, not tag/relation/pitfall/plan expansion", w)
			}
		}
		if strings.Contains(body, "current-state-migration.yaml") {
			for _, seam := range migrationApprovalSeams {
				if strings.HasSuffix(path, seam) {
					seams[seam] = true
					return
				}
			}
			t.Errorf("%s names the migration approval file; only the enumerated upgrade/sweep seams may - a permanent consumer or runtime-path owner is what cutover forbids (ADR-0136)", path)
		}
	})
	if goSeen < 60 {
		t.Fatalf("inspected only %d production Go file(s) under internal/ and cmd/; the scan is not reaching the tree", goSeen)
	}
	if !sawContext {
		t.Fatal("internal/project/context.go was not scanned - has the context producer moved?")
	}
	for _, seam := range migrationApprovalSeams {
		if !seams[seam] {
			t.Errorf("%s no longer names the migration approval file - has the enumerated seam set changed?", seam)
		}
	}

	tmplSeen := 0
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := fs.ReadFile(templates.FS, path)
		if rerr != nil {
			return rerr
		}
		tmplSeen++
		for _, w := range bannedWholeWords(string(b), legacyAuthorityIdents) {
			t.Errorf("template %s reintroduces the retired authority identifier %q (ADR-0133)", path, w)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmplSeen < 40 {
		t.Fatalf("inspected only %d embedded template file(s); the scan is not reaching the FS", tmplSeen)
	}
}

// TestLegacyAuthorityScannerFires proves the pure matcher the tree scan relies on
// actually catches a planted token and does not fire on ordinary prose, so
// TestLegacyAuthorityAbsent cannot pass vacuously.
func TestLegacyAuthorityScannerFires(t *testing.T) {
	if got := bannedWholeWords("x := adr.SupersessionRef{}\ncorpus.Chains()", legacyAuthorityIdents); len(got) != 2 {
		t.Errorf("planted tokens = %v, want SupersessionRef and Chains", got)
	}
	// Whole-word matching does not fire on a substring or a differently-cased
	// word: the live adr.ADR.Related field and lowercase prose stay legal.
	all := append(append([]string{}, legacyAuthorityIdents...), legacyContextFields...)
	for _, clean := range []string{"the retirers list", "unRelated code", "chainsaw", "// background material"} {
		if got := bannedWholeWords(clean, all); len(got) != 0 {
			t.Errorf("%q wrongly flagged %v", clean, got)
		}
	}
	// A historical ADR path is never a scan root: the retired model is discussed
	// in docs/decisions prose, which stays legal because it is history.
	for _, root := range []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")} {
		if strings.Contains(root, "decisions") || strings.Contains(root, "plans") {
			t.Errorf("scan root %q would sweep historical ADRs or plans", root)
		}
	}
}
