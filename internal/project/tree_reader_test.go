package project

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestFilesystemProjectReaderPathsErrors(t *testing.T) {
	validRoot := t.TempDir()
	for _, tc := range []struct {
		name, root, prefix, subject string
	}{
		{
			name:    "invalid prefix",
			root:    validRoot,
			prefix:  "invalid\x00prefix",
			subject: "invalid\x00prefix",
		},
		{
			name:    "invalid root",
			root:    "invalid\x00root",
			subject: "project tree",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (filesystemProjectReader{root: tc.root}).Paths(tc.prefix)
			if err == nil {
				t.Fatal("Paths error = nil")
			}
			var pathErr *fs.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("Paths error identity = %T, want *fs.PathError: %v", err, err)
			}
			if diagnostic := "enumerate " + tc.subject; !strings.Contains(err.Error(), diagnostic) {
				t.Fatalf("Paths error = %q, want diagnostic subject %q", err, diagnostic)
			}
		})
	}
}
