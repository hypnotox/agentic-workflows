package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestApplyAnchoredGlobsRewritesNoSlashPatterns(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), `prefix: x
invariants:
  disabled: false
  sources:
    - globs:
        - '*.go'
        - cmd/**
      marker: //
audit:
  dependencyManifests:
    - go.mod
    - '**/package.json'
`)
	if err := applyAnchoredGlobs(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"**/*.go", "cmd/**", "**/go.mod", "**/package.json"} {
		if !strings.Contains(s, want) {
			t.Errorf("rewritten config missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "**/cmd/**") || strings.Contains(s, "**/**/package.json") {
		t.Errorf("slashed pattern was rewritten:\n%s", s)
	}
}

func TestApplyAnchoredGlobsNoConfigNoop(t *testing.T) {
	if err := applyAnchoredGlobs(t.TempDir()); err != nil {
		t.Fatalf("absent config must be a no-op, got %v", err)
	}
}
