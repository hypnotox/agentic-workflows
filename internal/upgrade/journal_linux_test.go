//go:build linux

package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"golang.org/x/sys/unix"
)

func TestPossibleResidueInspectionDoesNotBlockOnSpecialParent(t *testing.T) {
	root := t.TempDir()
	journal := lockJournal(phaseApplying)
	journal.Operations[0].Path = "parent/a.txt"
	journal.Operations[0].PossibleResidue = true
	writeRawJournal(t, root, journal)
	mustWrite(t, filepath.Join(root, LockRel()), []byte("old-lock"))
	if err := unix.Mkfifo(filepath.Join(root, "parent"), 0o600); err != nil {
		t.Skipf("FIFO unsupported: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := Recover(root)
		result <- err
	}()
	select {
	case err := <-result:
		if !errors.Is(err, filesystem.ErrIdentityChanged) {
			t.Fatalf("Recover error = %v, want parent identity refusal", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Recover blocked on possible-residue parent FIFO")
	}
}

func TestJournalReadsDoNotBlockOnSpecialLeaf(t *testing.T) {
	for _, call := range []struct {
		name string
		run  func(string) error
	}{
		{
			name: "presence",
			run: func(root string) error {
				_, err := JournalPresent(root)
				return err
			},
		},
		{
			name: "load",
			run: func(root string) error {
				_, err := LoadJournal(root)
				return err
			},
		},
		{
			name: "recover",
			run: func(root string) error {
				_, err := Recover(root)
				return err
			},
		},
	} {
		t.Run(call.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(JournalPath(root), 0o600); err != nil {
				t.Skipf("FIFO unsupported: %v", err)
			}
			result := make(chan error, 1)
			go func() { result <- call.run(root) }()
			select {
			case err := <-result:
				if !errors.Is(err, filesystem.ErrIdentityChanged) {
					t.Fatalf("%s error = %v, want non-regular identity refusal", call.name, err)
				}
			case <-time.After(time.Second):
				t.Fatalf("%s blocked on journal FIFO", call.name)
			}
		})
	}
}
