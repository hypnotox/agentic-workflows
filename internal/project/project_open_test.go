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
	// awf-maintenance has a closed section set; "bogus" is not declared.
	cfg := "prefix: example\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/awf-maintenance.yaml": "sections:\n  bogus:\n    drop: true\n",
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
	if !strings.Contains(err.Error(), `"awf-maintenance"`) {
		t.Errorf("error should name the offending skill \"awf-maintenance\", got: %v", err)
	}
}

func TestOpenAllowsValidSectionOverride(t *testing.T) {
	// "upgrades" is a declared section for awf-maintenance.
	cfg := "prefix: example\nintegrationBranch: main\n"
	root := scaffoldFiles(t, cfg, map[string]string{
		"skills/awf-maintenance.yaml": "sections:\n  upgrades:\n    drop: true\n",
	})
	_, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatalf("valid section override 'upgrades' should succeed, got: %v", err)
	}
}
