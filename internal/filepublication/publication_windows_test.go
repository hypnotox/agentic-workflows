//go:build windows

package filepublication

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// invariant: tooling/file-publication:exclusive-file-publication-single-home (TestWindowsNoReplacePublicationContract)
func TestWindowsNoReplacePublicationContract(t *testing.T) {
	files := map[string]string{"temporary": "complete"}
	move := func(from, to string, flags uint32) error {
		if flags != windows.MOVEFILE_WRITE_THROUGH {
			t.Fatalf("MoveFileEx flags = %#x, want MOVEFILE_WRITE_THROUGH", flags)
		}
		contents, ok := files[from]
		if !ok {
			return os.ErrNotExist
		}
		if _, exists := files[to]; exists {
			return os.ErrExist
		}
		files[to] = contents
		delete(files, from)
		return nil
	}

	if err := publishNoReplaceWindows("temporary", "artifact", move); err != nil {
		t.Fatal(err)
	}
	if got := files["artifact"]; got != "complete" {
		t.Fatalf("published bytes = %q", got)
	}

	files["loser"], files["artifact"] = "loser", "winner"
	if err := publishNoReplaceWindows("loser", "artifact", move); !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision error = %v", err)
	}
	if got := files["artifact"]; got != "winner" {
		t.Fatalf("winner bytes = %q", got)
	}
	if got := files["loser"]; got != "loser" {
		t.Fatalf("loser temporary bytes = %q", got)
	}
}
