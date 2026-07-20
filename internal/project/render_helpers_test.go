package project

import "testing"

// renderedByPath returns the content of the RenderAll output at path, failing if absent.
func renderedByPath(t *testing.T, files []RenderedFile, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("no rendered file at %s", path)
	return ""
}

// syncClean opens+syncs root and fails on any residual drift.
func syncClean(t *testing.T, root string) {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
}
