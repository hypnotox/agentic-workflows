package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

func TestOpenRejectsMalformedRepository(t *testing.T) {
	root := scaffold(t, sampleYAML)
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTestSession(testContext(t), root); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
		t.Fatalf("malformed repository error = %v", err)
	}
}

func TestOpenValidConfigSucceeds(t *testing.T) {
	root := scaffold(t, sampleYAML)
	_, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("expected valid config to open cleanly, got: %v", err)
	}
}

func TestOpenRejectsUnknownSectionOverride(t *testing.T) {
	// debugging in the catalog has sections [surfaces, notes]; "bogus" is not declared.
	cfg := "prefix: example\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/debugging.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected error for unknown section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
	// The label carries the artifact name for a named artifact (name != ""), so
	// the message identifies which skill; assert it so that branch is pinned.
	if !strings.Contains(err.Error(), `"debugging"`) {
		t.Errorf("error should name the offending skill \"debugging\", got: %v", err)
	}
}

func TestOpenAllowsValidSectionOverride(t *testing.T) {
	// "oracle-and-handoff" is a declared section for debugging.
	cfg := "prefix: example\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/debugging.yaml": "sections:\n  oracle-and-handoff:\n    drop: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("valid section override 'oracle-and-handoff' should succeed, got: %v", err)
	}
}

func TestOpenRejectsUnknownAgentSectionOverride(t *testing.T) {
	// reviewer in the catalog has sections universal-lenses/project-focus/doc-currency.
	cfg := "prefix: example\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"agents/reviewer.yaml": "sections:\n  bogus:\n    drop: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err == nil {
		t.Fatal("expected error for unknown agent section override 'bogus'")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention 'bogus', got: %v", err)
	}
}
