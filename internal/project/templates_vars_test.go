package project

import (
	"io/fs"
	"strings"
	"testing"

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
