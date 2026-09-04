//go:build linux

package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"golang.org/x/sys/unix"
)

func TestConfinedLockLoadDoesNotFollowOrBlockOnSpecialLeaf(t *testing.T) {
	for _, tc := range []struct {
		name   string
		create func(string) error
	}{
		{
			name: "symlink",
			create: func(path string) error {
				return os.Symlink("outside", path)
			},
		},
		{
			name: "fifo",
			create: func(path string) error {
				return unix.Mkfifo(path, 0o600)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".awf", "outside"), []byte(`{"schemaVersion":50}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.create(filepath.Join(root, ".awf", "awf.lock")); err != nil {
				t.Skipf("special file unsupported: %v", err)
			}
			for _, load := range []struct {
				name string
				run  func() error
			}{
				{name: "live file", run: func() error {
					_, _, err := LoadLiveFileOptional(root, ".awf/awf.lock", 50, 50)
					return err
				}},
				{name: "schema", run: func() error {
					_, _, err := LoadSchemaConfinedOptional(root, ".awf/awf.lock")
					return err
				}},
			} {
				t.Run(load.name, func(t *testing.T) {
					result := make(chan error, 1)
					go func() { result <- load.run() }()
					select {
					case err := <-result:
						if !errors.Is(err, filesystem.ErrIdentityChanged) {
							t.Fatalf("confined load error = %v, want no-follow identity refusal", err)
						}
					case <-time.After(time.Second):
						t.Fatal("confined load blocked on special lock leaf")
					}
				})
			}
		})
	}
}
