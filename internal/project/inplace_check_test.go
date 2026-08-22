package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

func TestCheckLockedFilesInPlaceRegenDrift(t *testing.T) {
	// invariant: rendering/inplace-and-placeholders:in-place-tamper-drift (TestCheckLockedFilesInPlaceRegenDrift)
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	canonical := "#!/bin/sh\n# awf:edit-in-place s: your edits\nadopter line\n"
	xPath := filepath.Join(root, "x")
	lock := &manifest.Lock{Files: map[string]manifest.Entry{
		"x": {RegenChecked: true, OutputHash: manifest.Hash([]byte(canonical))},
	}}
	rendered := map[string]RenderedFile{"x": {Path: "x", Content: canonical, RegenChecked: true, TemplateID: "in-place/mock.tmpl", Policy: OutputPolicy{Regenerate: true}}}

	// On-disk equals the regenerated content (in-place body already read back) → clean.
	if err := os.WriteFile(xPath, []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	if d := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, rendered, nil); len(d) != 0 {
		t.Errorf("a matching in-place file must not drift, got %v", d)
	}

	// An awf-owned region edited on disk → regenerated content differs → hand-edited.
	tampered := "#!/bin/sh\n# awf owns this and it was TAMPERED\n# awf:edit-in-place s: your edits\nadopter line\n"
	if err := os.WriteFile(xPath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	d := checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, rendered, nil)
	if len(d) != 1 || d[0].Kind != "hand-edited" {
		t.Fatalf("a tampered awf region must drift hand-edited, got %v", d)
	}

	// Absent file → missing.
	if err := os.Remove(xPath); err != nil {
		t.Fatal(err)
	}
	d = checkLockedDrift(renderInputsForTest(p).residentRoots(), lock, rendered, nil)
	if len(d) != 1 || d[0].Kind != "missing" {
		t.Fatalf("an absent in-place file → missing, got %v", d)
	}
}
