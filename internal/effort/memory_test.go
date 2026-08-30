package effort

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// invariant: tooling/effort-management:memory-skeleton-purpose-partition (TestNewMemoryIsPlainMarkdownAndExistingBytesAreOpaque)
func TestNewMemoryIsPlainMarkdownAndExistingBytesAreOpaque(t *testing.T) {
	raw := memorySkeleton()
	for _, heading := range []string{"## Brief", "## Checkpoint", "## Decision log", "## Observations", "## Handoff log"} {
		if !bytes.Contains(raw, []byte(heading)) {
			t.Fatalf("missing %s", heading)
		}
	}
	if bytes.HasPrefix(raw, []byte("---")) {
		t.Fatal("new memory has frontmatter")
	}
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	if _, err := service.New(testContext(t), NewInput{Slug: "opaque", Title: "Opaque memory"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "efforts", "opaque", "memory.md")
	legacy := []byte("---\neffort: unrelated\nphase: nonsense\nnext: also nonsense\nupdated: never\n---\nlegacy bytes\n")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show("opaque"); err != nil {
		t.Fatalf("legacy memory was parsed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, legacy) {
		t.Fatalf("memory changed: %q %v", got, err)
	}
}
