package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// temporaryAuthoredAdopter builds a representative authored adopter without
// retaining a second generated tree in the repository.
func temporaryAuthoredAdopter(t *testing.T) string {
	t.Helper()
	root := scaffoldFiles(t, `prefix: fixture
integrationBranch: main
vars:
  testCmd: go test ./...
  gateCmd: ./x gate
domains: [alpha]
currentState:
bootstrap:
  enabled: true
`, map[string]string{
		"domains/alpha.yaml":                        "paths: [\"internal/**\"]\n",
		"domains/parts/alpha/current-state.md":      "Fixture domain guidance.\n",
		"topics/metadata/alpha/model.yaml":          "title: Model\nsummary: Fixture model rules.\npaths: [\"internal/**\"]\n",
		"topics/parts/alpha/model/current-state.md": "Fixture model guidance.\n\n## Claims\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if _, _, _, err := initializeReportProject(p, publisher.InitAuthority{InitializedWithVersion: Version}); err != nil {
		t.Fatalf("initialize fixture: %v", err)
	}
	return root
}

func TestTemporaryAdopterRenderDriftLifecycle(t *testing.T) {
	root := temporaryAuthoredAdopter(t)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("reopen fixture: %v", err)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Fatalf("initial check: drift=%v err=%v", drift, err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("tamper AGENTS.md: %v", err)
	}
	drift, err := checkProject(p, testContext(t))
	if err != nil {
		t.Fatalf("check tampered fixture: %v", err)
	}
	if !hasFixtureDrift(drift, "AGENTS.md", "hand-edited") {
		t.Fatalf("drift=%v, want AGENTS.md hand-edited", drift)
	}
	if _, _, _, err := syncReportProject(p); err != nil {
		t.Fatalf("repair fixture: %v", err)
	}
	if drift, err := checkProject(p, testContext(t)); err != nil || len(drift) != 0 {
		t.Fatalf("final check: drift=%v err=%v", drift, err)
	}
}

func hasFixtureDrift(drift []manifest.Drift, path, kind string) bool {
	for _, item := range drift {
		if item.Path == path && item.Kind == kind {
			return true
		}
	}
	return false
}
