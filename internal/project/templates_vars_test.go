package project

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/templates"
)

// TestNoDocPathVarsInTemplates asserts the standard's templates reference none of
// the doc-path or project-specific vars that ADR-0013 migrated onto .layout or
// deleted outright. Doc paths are awf-given (.layout); these var names must not
// reappear in any template.
// invariant: no-doc-path-vars
func TestNoDocPathVarsInTemplates(t *testing.T) {
	banned := []string{
		"workflowDoc", "debuggingDoc", "pitfallsDoc", "roadmapDoc", "stateDocsPath",
		"oracleStateDoc", "autonomousAdrRef", "hostGitAdrRef", "keyInvariantAdrRef",
		"noDivingAdrRef", "perTaskReviewAdrRef",
	}
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		b, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		for _, v := range banned {
			if strings.Contains(string(b), v) {
				t.Errorf("%s references removed var %q — doc refs are awf-given via .layout", path, v)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
}

// TestCommitScopeSingleStorage asserts commit scopes have one storage
// (ADR-0051): no template references .vars.commitScope and the catalog vars
// block carries no commitScope descriptor — every rendered scope mention
// derives from audit.allowedScopes via the commitScopes render-context key.
// invariant: commit-scope-single-storage
func TestCommitScopeSingleStorage(t *testing.T) {
	err := fs.WalkDir(templates.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(templates.FS, path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), ".vars.commitScope") {
			t.Errorf("%s references .vars.commitScope — commit scopes live in audit.allowedScopes (ADR-0051)", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cat := catalog.Standard
	for _, d := range cat.Vars {
		if d.Key == "commitScope" {
			t.Error("catalog still carries a commitScope var descriptor (ADR-0051)")
		}
	}
}
