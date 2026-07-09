package migrate

import (
	"bytes"
	"io"
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
	var out bytes.Buffer
	if err := applyAnchoredGlobs(root, &out); err != nil {
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
	for _, want := range []string{
		`anchored-globs: rewrote glob "*.go" → "**/*.go" (invariants.sources.globs)` + "\n",
		`anchored-globs: rewrote glob "go.mod" → "**/go.mod" (audit.dependencyManifests)` + "\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing provenance line %q in output:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "cmd/**") || strings.Contains(out.String(), "package.json") {
		t.Errorf("already-slashed patterns must not be reported:\n%s", out.String())
	}

	// Idempotent re-run prints nothing.
	out.Reset()
	if err := applyAnchoredGlobs(root, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Errorf("a no-op run must print nothing, got:\n%s", out.String())
	}
}

func TestApplyAnchoredGlobsNoConfigNoop(t *testing.T) {
	if err := applyAnchoredGlobs(t.TempDir(), io.Discard); err != nil {
		t.Fatalf("absent config must be a no-op, got %v", err)
	}
}

func TestApplyAnchoredGlobsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "not: [valid\n")
	if err := applyAnchoredGlobs(root, io.Discard); err == nil {
		t.Error("expected the parse error surfaced from AnchorNoSlashGlobs")
	}
}
