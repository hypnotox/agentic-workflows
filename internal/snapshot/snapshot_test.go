package snapshot_test

import (
	"errors"
	"os"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestMain(m *testing.M) { os.Exit(testsupport.RunIsolated(m, "awf-snapshot-test-home")) }

// TestNewTreeSortsAndCopiesBytes checks that NewTree returns files in path
// order and that neither the caller's input slice nor a List result can mutate
// the captured bytes.
func TestNewTreeSortsAndCopiesBytes(t *testing.T) {
	input := []snapshot.File{
		{Path: "b.txt", Mode: snapshot.Regular, Bytes: []byte("bee")},
		{Path: "a/c.sh", Mode: snapshot.Executable, Bytes: []byte("see")},
		{Path: "a.txt", Mode: snapshot.Regular, Bytes: nil},
	}
	tree, err := snapshot.NewTree(input)
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}

	// Mutating the caller's original bytes must not reach the Tree.
	input[0].Bytes[0] = 'X'

	got := tree.List()
	want := []struct {
		path string
		mode snapshot.Mode
		body string
	}{
		{"a.txt", snapshot.Regular, ""},
		{"a/c.sh", snapshot.Executable, "see"},
		{"b.txt", snapshot.Regular, "bee"},
	}
	if len(got) != len(want) {
		t.Fatalf("List returned %d files, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Path != w.path || got[i].Mode != w.mode || string(got[i].Bytes) != w.body {
			t.Errorf("file %d = {%q, %d, %q}, want {%q, %d, %q}", i, got[i].Path, got[i].Mode, got[i].Bytes, w.path, w.mode, w.body)
		}
	}
	// A nil-byte file clones to an empty, non-nil slice.
	if got[0].Bytes == nil {
		t.Errorf("nil input bytes should clone to a non-nil empty slice")
	}
	// Mutating a List result must not reach the Tree.
	got[2].Bytes[0] = 'Z'
	if again := tree.List(); string(again[2].Bytes) != "bee" {
		t.Errorf("List result aliases the Tree: second read = %q", again[2].Bytes)
	}
}

// TestLookup covers both the hit and miss branches, and that a hit returns an
// independent byte copy.
func TestLookup(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{
		{Path: "one.txt", Mode: snapshot.Regular, Bytes: []byte("1")},
		{Path: "two.txt", Mode: snapshot.Regular, Bytes: []byte("2")},
	})
	if err != nil {
		t.Fatalf("NewTree: %v", err)
	}
	f, ok := tree.Lookup("two.txt")
	if !ok || string(f.Bytes) != "2" {
		t.Fatalf("Lookup(two.txt) = %q, %v", f.Bytes, ok)
	}
	f.Bytes[0] = 'X'
	if again, _ := tree.Lookup("two.txt"); string(again.Bytes) != "2" {
		t.Errorf("Lookup result aliases the Tree: %q", again.Bytes)
	}
	if _, ok := tree.Lookup("missing.txt"); ok {
		t.Errorf("Lookup(missing.txt) reported present")
	}
}

// TestNewTreeRejections exercises every construction fault.
func TestNewTreeRejections(t *testing.T) {
	cases := []struct {
		name  string
		files []snapshot.File
		want  error
	}{
		{"unsupported mode", []snapshot.File{{Path: "a.txt", Mode: snapshot.Mode(9)}}, snapshot.ErrUnsupportedMode},
		{"empty path", []snapshot.File{{Path: "", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"absolute path", []snapshot.File{{Path: "/a.txt", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"backslash", []snapshot.File{{Path: "a\\b.txt", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"trailing slash", []snapshot.File{{Path: "a/", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"dot segment", []snapshot.File{{Path: "a/./b.txt", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"dot", []snapshot.File{{Path: ".", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"dotdot", []snapshot.File{{Path: "..", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"traversal", []snapshot.File{{Path: "../escape.txt", Mode: snapshot.Regular}}, snapshot.ErrUnsafePath},
		{"duplicate", []snapshot.File{
			{Path: "a.txt", Mode: snapshot.Regular},
			{Path: "a.txt", Mode: snapshot.Regular},
		}, snapshot.ErrDuplicatePath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := snapshot.NewTree(tc.files); !errors.Is(err, tc.want) {
				t.Fatalf("NewTree(%s) = %v, want %v", tc.name, err, tc.want)
			}
		})
	}
}
