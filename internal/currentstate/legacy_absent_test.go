package currentstate_test

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
)

// legacyAuthorityIdents are identifiers of the deleted ADR-derived authority
// engines. Canonical topic claims are the sole active authority, so none may
// reappear in shipped Go or an embedded runtime template.
var legacyAuthorityIdents = []string{
	"SupersessionRef", "AnnotatedAnchors", "Chains", "Retirers",
	"StateCovered", "PartiallySuperseded", "DeclaringADRs",
	"RenderActiveMD", "RenderDomainIndex",
}

const migrationApprovalPath = "current-state-migration.yaml"
const bridgeImportPath = `"github.com/hypnotox/agentic-workflows/internal/bridge"`

func bannedWholeWords(body string, banned []string) []string {
	var hit []string
	for _, word := range banned {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`).MatchString(body) {
			hit = append(hit, word)
		}
	}
	return hit
}

func productionGoSources(t *testing.T, fn func(path, body string)) int {
	t.Helper()
	seen := 0
	repoRoot := filepath.Join("..", "..")
	testsupport.WalkRepoSources(t, repoRoot, func(path string, body []byte) {
		if !strings.HasPrefix(path, "internal/") && !strings.HasPrefix(path, "cmd/") {
			return
		}
		seen++
		fn(path, string(body))
	})
	return seen
}

// TestLegacyAuthorityAbsent keeps retired ADR authority, bridge imports, and
// migration approval authority out of shipped code and runtime templates.
func TestLegacyAuthorityAbsent(t *testing.T) {
	goSeen := productionGoSources(t, func(path, body string) {
		for _, word := range bannedWholeWords(body, legacyAuthorityIdents) {
			t.Errorf("%s reintroduces retired authority identifier %q", path, word)
		}
		if strings.Contains(body, bridgeImportPath) {
			t.Errorf("%s imports the deleted internal/bridge package", path)
		}
		if strings.Contains(body, migrationApprovalPath) {
			t.Errorf("%s names the retired migration approval file", path)
		}
	})
	if goSeen < 60 {
		t.Fatalf("inspected only %d production Go file(s); scan is not reaching the tree", goSeen)
	}

	templateSeen := 0
	err := fs.WalkDir(templates.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		templateSeen++
		for _, word := range bannedWholeWords(string(body), legacyAuthorityIdents) {
			t.Errorf("template %s reintroduces retired authority identifier %q", path, word)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if templateSeen < 40 {
		t.Fatalf("inspected only %d embedded template file(s); scan is not reaching the FS", templateSeen)
	}
}

func TestLegacyAuthorityScannerFires(t *testing.T) {
	if got := bannedWholeWords("x := adr.SupersessionRef{}\ncorpus.Chains()", legacyAuthorityIdents); len(got) != 2 {
		t.Errorf("planted tokens = %v, want SupersessionRef and Chains", got)
	}
	for _, clean := range []string{"the retirers list", "unRelated code", "chainsaw", "// background material"} {
		if got := bannedWholeWords(clean, legacyAuthorityIdents); len(got) != 0 {
			t.Errorf("%q wrongly flagged %v", clean, got)
		}
	}
	for _, root := range []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")} {
		if strings.Contains(root, "decisions") || strings.Contains(root, "plans") {
			t.Errorf("scan root %q would sweep historical ADRs or plans", root)
		}
	}
}
