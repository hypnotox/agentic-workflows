package snapshot_test

import (
	"errors"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

// invariant: tooling/audit-and-snapshots:sparse-snapshot-explicit-selection (TestSelectionOwnsExplicitFileSet)
// TestSelectionOwnsExplicitFileSet verifies that an explicit selection owns its
// exact, path-sorted set of regular, executable, and symlink files without
// exposing its captured bytes.
func TestSelectionOwnsExplicitFileSet(t *testing.T) {
	input := []snapshot.File{
		{Path: "link", Mode: snapshot.Symlink, Bytes: []byte("target")},
		{Path: "run", Mode: snapshot.Executable, Bytes: []byte("run")},
		{Path: "a.txt", Mode: snapshot.Regular, Bytes: []byte("a")},
	}
	selection, err := snapshot.NewSelection(input)
	if err != nil {
		t.Fatalf("NewSelection: %v", err)
	}

	// Selection and Tree deliberately have distinct APIs and cannot substitute
	// for one another at their consumers' boundaries.
	selectionOnly(selection)
	tree, err := snapshot.NewTree(nil)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	treeOnly(tree)

	input[0].Bytes[0] = 'X'
	got := selection.List()
	want := []struct {
		path string
		mode snapshot.Mode
		body string
	}{
		{"a.txt", snapshot.Regular, "a"},
		{"link", snapshot.Symlink, "target"},
		{"run", snapshot.Executable, "run"},
	}
	if len(got) != len(want) {
		t.Fatalf("List returned %d files, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Mode != w.mode || string(got[i].Bytes) != w.body {
			t.Errorf("file %d = {%q, %d, %q}, want {%q, %d, %q}", i, got[i].Path, got[i].Mode, got[i].Bytes, w.path, w.mode, w.body)
		}
	}

	got[1].Bytes[0] = 'X'
	if again := selection.List(); string(again[1].Bytes) != "target" {
		t.Errorf("List result aliases the Selection: %q", again[1].Bytes)
	}
	file, ok := selection.Lookup("run")
	if !ok || string(file.Bytes) != "run" {
		t.Fatalf("Lookup(run) = %q, %v", file.Bytes, ok)
	}
	file.Bytes[0] = 'X'
	if again, _ := selection.Lookup("run"); string(again.Bytes) != "run" {
		t.Errorf("Lookup result aliases the Selection: %q", again.Bytes)
	}
	if _, ok := selection.Lookup("missing.txt"); ok {
		t.Error("Lookup(missing.txt) reported present")
	}

	for _, tc := range []struct {
		name  string
		files []snapshot.File
		want  error
	}{
		{"duplicate", []snapshot.File{
			{Path: "a.txt", Mode: snapshot.Regular},
			{Path: "a.txt", Mode: snapshot.Regular},
		}, snapshot.ErrDuplicatePath},
		{"unsafe", []snapshot.File{{Path: "../escape.txt", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := snapshot.NewSelection(tc.files); !errors.Is(err, tc.want) {
				t.Fatalf("NewSelection(%s) = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
}

// TestNewSelectionFromBlobs translates every Git blob mode at the snapshot
// boundary, then applies Selection's ownership, ordering, and validation rules.
func TestNewSelectionFromBlobs(t *testing.T) {
	input := []awfgit.IndexBlob{
		{Path: "z-link", Mode: awfgit.BlobSymlink, Bytes: []byte("target")},
		{Path: "m-run", Mode: awfgit.BlobExecutable, Bytes: []byte("run")},
		{Path: "a-file", Mode: awfgit.BlobRegular, Bytes: []byte("file")},
	}
	selection, err := snapshot.NewSelectionFromBlobs(input)
	if err != nil {
		t.Fatalf("NewSelectionFromBlobs: %v", err)
	}
	input[0].Bytes[0] = 'X'

	got := selection.List()
	want := []struct {
		path string
		mode snapshot.Mode
		body string
	}{
		{"a-file", snapshot.Regular, "file"},
		{"m-run", snapshot.Executable, "run"},
		{"z-link", snapshot.Symlink, "target"},
	}
	for i, want := range want {
		if got[i].Path != want.path || got[i].Mode != want.mode || string(got[i].Bytes) != want.body {
			t.Errorf("file %d = %#v, want {%q %d %q}", i, got[i], want.path, want.mode, want.body)
		}
	}
	got[2].Bytes[0] = 'X'
	if again := selection.List(); string(again[2].Bytes) != "target" {
		t.Errorf("translated selection aliases returned bytes: %q", again[2].Bytes)
	}

	for _, blobs := range [][]awfgit.IndexBlob{
		{{Path: "bad", Mode: awfgit.BlobMode(99)}},
		{{Path: "../escape", Mode: awfgit.BlobRegular}},
		{{Path: "same", Mode: awfgit.BlobRegular}, {Path: "same", Mode: awfgit.BlobRegular}},
	} {
		if _, err := snapshot.NewSelectionFromBlobs(blobs); err == nil {
			t.Fatalf("NewSelectionFromBlobs(%#v) accepted invalid translated selection", blobs)
		}
	}
}

func selectionOnly(*snapshot.Selection) {}

func treeOnly(*snapshot.Tree) {}
