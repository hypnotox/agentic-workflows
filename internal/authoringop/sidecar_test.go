package authoringop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// invariant: tooling/cli:semantic-artifact-authoring (TestSidecarAuthoringCreatesAndRemovesLeaf)
func TestSidecarAuthoringCreatesAndRemovesLeaf(t *testing.T) {
	root, loader := transactionFixture(t, false)
	req := Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: "data.example", Sidecar: true, SidecarMode: "value", Value: "text"}
	out, err := Run(context.Background(), root, req, loader, nil)
	if err != nil || out.Source != SourceCreated {
		t.Fatalf("out=%#v err=%v", out, err)
	}
	source := filepath.Join(root, ".awf/skills/awf-maintenance.yaml")
	if got := string(bytesAt(t, root, ".awf/skills/awf-maintenance.yaml")); got != "data:\n  example: text\n" {
		t.Fatalf("source=%q", got)
	}
	// Repeating the exact authored scalar does not rewrite it.
	before := bytesAt(t, root, ".awf/skills/awf-maintenance.yaml")
	out, err = Run(context.Background(), root, req, loader, nil)
	if err != nil || out.Source != SourceNone {
		t.Fatalf("idempotent out=%#v err=%v", out, err)
	}
	if got := bytesAt(t, root, ".awf/skills/awf-maintenance.yaml"); string(got) != string(before) {
		t.Fatal("idempotent edit rewrote source")
	}
	req.Mode = Reset
	req.SidecarMode = "reset"
	req.Value = nil
	out, err = Run(context.Background(), root, req, loader, nil)
	if err != nil || out.Source != SourceRemoved {
		t.Fatalf("reset out=%#v err=%v", out, err)
	}
	if _, err = os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sidecar remains: %v", err)
	}
}

func TestUnchangedSidecarAuthoringStillSynchronizesCommittedAuthority(t *testing.T) {
	root, loader := transactionFixture(t, false)
	req := Request{Mode: Edit, Kind: "skill", Name: "awf-maintenance", Part: "data.example", Sidecar: true, SidecarMode: "value", Value: "text"}
	if _, err := Run(context.Background(), root, req, loader, nil); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, ".claude/skills/awf-maintenance/SKILL.md")
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(before, []byte("\nmanual drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := Run(context.Background(), root, req, loader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Source != SourceNone || !outcome.Publisher.HasCommittedEffects() {
		t.Fatalf("unchanged synchronized outcome = %#v", outcome)
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("unchanged authoring left drift: %q", after)
	}
}
