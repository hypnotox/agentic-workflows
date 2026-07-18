package adr_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// goSources walks the repo's production Go files (no tests, no vendored trees)
// and hands each one's path and contents to fn. Shared by the corpus source
// scans, which enforce structural rules no unit test can reach.
func goSources(t *testing.T, fn func(path, body string)) {
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
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			seen++
			fn(filepath.ToSlash(path), string(data))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if seen < 20 {
		t.Fatalf("inspected only %d production source file(s); the scan is not reaching the tree", seen)
	}
}

// codeLines yields the file's lines with whole-line comments dropped, so a rule
// about call sites is not tripped by prose that merely names the function.
func codeLines(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// TestCorpusParsedOnce enforces ADR-0130 item 1: one parse per invocation.
// adr.ParseDir has no production caller outside internal/adr - every consumer
// enters through Corpus construction - and inside internal/adr only that seam
// and NextNumber call it. NextNumber is the enumerated exception: it runs on
// the awf new adr path, which holds no corpus.
// invariant: corpus-parsed-once
func TestCorpusParsedOnce(t *testing.T) {
	var outside []string
	goSources(t, func(path, body string) {
		if strings.HasPrefix(path, "../../internal/adr/") {
			return
		}
		for i, line := range codeLines(body) {
			if strings.Contains(line, "adr.ParseDir(") {
				outside = append(outside, fromLine(path, i, line))
			}
		}
	})
	if len(outside) != 0 {
		t.Errorf("adr.ParseDir must have no production call site outside internal/adr; construct a Corpus instead (ADR-0130 item 1):\n\t%s",
			strings.Join(outside, "\n\t"))
	}

	// In-package: only the construction seam and the enumerated NextNumber.
	callers := map[string]bool{}
	for _, f := range []string{"corpus.go", "adr.go", "domain.go", "status.go", "declarations.go"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range codeLines(string(data)) {
			if strings.Contains(line, "ParseDir(") && !strings.Contains(line, "func ParseDir") {
				callers[f] = true
			}
		}
	}
	for f := range callers {
		if f != "corpus.go" && f != "adr.go" {
			t.Errorf("%s calls ParseDir; only Corpus construction (corpus.go) and NextNumber (adr.go) may", f)
		}
	}
	if !callers["corpus.go"] {
		t.Error("corpus.go no longer calls ParseDir - has the construction seam moved?")
	}

	// The render entry points take a Corpus rather than parsing.
	for _, f := range []string{"adr.go", "domain.go"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"func RenderActiveMD(corpus Corpus)", "func RenderDomainIndex(corpus Corpus,"} {
			if strings.Contains(string(data), strings.Split(want, "(")[0]+"(") && !strings.Contains(string(data), want) {
				t.Errorf("%s: %s must take a Corpus rather than parsing (ADR-0130 item 1)", f, strings.Split(want, "(")[0])
			}
		}
	}
}

// TestCorpusOwnsFieldReads enforces ADR-0130's corpus-owns-field-reads: the
// supersession and declared-invariant questions ADR.Refs and ADR.Sections
// answer are asked of the view, not re-derived from the fields.
// invariant: corpus-owns-field-reads
func TestCorpusOwnsFieldReads(t *testing.T) {
	// A field read, not a Corpus method call: RefsOf/DeclaredSlugs are the
	// sanctioned way to ask, and they live on the view.
	refsRead := regexp.MustCompile(`\.Refs\b`)
	var bad []string
	goSources(t, func(path, body string) {
		if strings.HasPrefix(path, "../../internal/adr/") {
			return
		}
		for i, line := range codeLines(body) {
			if strings.Contains(line, ".Sections[") || refsRead.MatchString(line) {
				bad = append(bad, fromLine(path, i, line))
			}
		}
	})
	if len(bad) != 0 {
		t.Errorf("ADR.Refs or ADR.Sections is read outside internal/adr; ask the view instead (ADR-0130 item 2):\n\t%s",
			strings.Join(bad, "\n\t"))
	}
}

// TestCorpusRawAccessEnumerated enforces ADR-0130 item 6: exactly two consumers
// work below the semantic layer - the migration's offset surgery and the
// retired-key frontmatter scan - and both go through the view's named
// accessor rather than re-reading the file.
// invariant: corpus-raw-access-enumerated
func TestCorpusRawAccessEnumerated(t *testing.T) {
	raw := map[string]bool{}
	var pathReads []string
	goSources(t, func(path, body string) {
		if strings.HasPrefix(path, "../../internal/adr/") {
			return
		}
		importsADR := strings.Contains(body, `"github.com/hypnotox/agentic-workflows/internal/adr"`)
		for i, line := range codeLines(body) {
			if strings.Contains(line, ".Raw(") {
				raw[path] = true
			}
			// Scoped to files that actually handle ADR records: a .Path read in
			// a package that does not import internal/adr is some other type's.
			if importsADR && strings.Contains(line, "os.ReadFile(") && strings.Contains(line, ".Path") {
				pathReads = append(pathReads, fromLine(path, i, line))
			}
		}
	})
	want := map[string]bool{
		"../../internal/migrate/retirementtokens.go": true,
		"../../internal/project/supersession.go":     true,
	}
	for path := range raw {
		if !want[path] {
			t.Errorf("%s calls the raw-bytes accessor; only the migration and the retired-key scan may (ADR-0130 item 6). A third consumer means the view is missing a question.", path)
		}
	}
	for path := range want {
		if !raw[path] {
			t.Errorf("%s no longer calls the raw-bytes accessor - has the enumerated set changed?", path)
		}
	}
	if len(pathReads) != 0 {
		t.Errorf("an ADR file is read directly rather than through the view's accessor (ADR-0130 item 6):\n\t%s",
			strings.Join(pathReads, "\n\t"))
	}
}

func fromLine(path string, i int, line string) string {
	return path + ":" + strconv.Itoa(i+1) + ": " + strings.TrimSpace(line)
}

// TestCorpusAbsentADR covers the view's absent-ADR guards. Every lookup takes a
// number that may not resolve: a token can cite an ADR that does not exist, and
// the check that reports that is a different one from the accessor. The
// accessors answer emptily rather than panicking, leaving the missing-target
// finding to its owner.
func TestCorpusAbsentADR(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "0001-only.md"),
		testsupport.ADR("Accepted", testsupport.WithTitle("0001: Only"),
			testsupport.WithBody("## Decision\n\n1. x.\n\n## Invariants\n\n- `invariant: only-slug` - x.\n")))
	c, err := adr.LoadCorpus(dir)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}

	if _, ok := c.ByNumber("9999"); ok {
		t.Error("ByNumber resolved an absent ADR")
	}
	if c.Has("9999") {
		t.Error("Has reported an absent ADR present")
	}
	if got := c.DecisionItems("9999"); got != nil {
		t.Errorf("DecisionItems on an absent ADR = %v, want nil", got)
	}
	if got := c.DeclaredSlugs("9999"); got != nil {
		t.Errorf("DeclaredSlugs on an absent ADR = %v, want nil", got)
	}
	if got := c.RefsOf("9999"); got != nil {
		t.Errorf("RefsOf on an absent ADR = %v, want nil", got)
	}
	if _, err := c.Raw("9999"); err == nil {
		t.Error("Raw on an absent ADR returned no error")
	}

	// The present ADR answers for real, so the guards above are not passing
	// vacuously over an empty corpus.
	if got := c.DecisionItems("0001"); len(got) != 1 {
		t.Errorf("DecisionItems(0001) = %v, want one item", got)
	}
	if got := c.DeclaredSlugs("0001"); len(got) != 1 || got[0] != "only-slug" {
		t.Errorf("DeclaredSlugs(0001) = %v, want [only-slug]", got)
	}
	if _, err := c.Raw("0001"); err != nil {
		t.Errorf("Raw(0001): %v", err)
	}
}
