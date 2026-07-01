package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingletonStandardDocsRelocatesSidecarAndParts(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	mustWrite(t, filepath.Join(awf, "config.yaml"), "prefix: ex\ndocs:\n  - architecture\n  - workflow\n  - doc-standard\n")
	mustWrite(t, filepath.Join(awf, "docs", "workflow.yaml"), "data:\n  k: v\n")
	mustWrite(t, filepath.Join(awf, "docs", "parts", "workflow", "local-hooks.md"), "LOCAL HOOKS BODY\n")

	if err := applySingletonStandardDocs(root); err != nil {
		t.Fatalf("applySingletonStandardDocs: %v", err)
	}

	if b, err := os.ReadFile(filepath.Join(awf, "workflow.yaml")); err != nil || !strings.Contains(string(b), "k: v") {
		t.Errorf("workflow sidecar not relocated: %v, %s", err, b)
	}
	if _, err := os.Stat(filepath.Join(awf, "docs", "workflow.yaml")); !os.IsNotExist(err) {
		t.Errorf("old workflow sidecar location should be gone, stat err = %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(awf, "parts", "workflow", "local-hooks.md")); err != nil || string(b) != "LOCAL HOOKS BODY\n" {
		t.Errorf("workflow part not relocated: %v, %s", err, b)
	}
	if _, err := os.Stat(filepath.Join(awf, "docs", "parts", "workflow")); !os.IsNotExist(err) {
		t.Errorf("old workflow parts dir should be gone, stat err = %v", err)
	}
	cfg, err := os.ReadFile(filepath.Join(awf, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cfg), "workflow") || strings.Contains(string(cfg), "doc-standard") {
		t.Errorf("docs: array should no longer list workflow/doc-standard:\n%s", cfg)
	}
	if !strings.Contains(string(cfg), "- architecture") {
		t.Errorf("untouched docs: entry lost:\n%s", cfg)
	}
}

func TestSingletonStandardDocsIdempotent(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	mustWrite(t, filepath.Join(awf, "config.yaml"), "prefix: ex\n")
	if err := applySingletonStandardDocs(root); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := applySingletonStandardDocs(root); err != nil {
		t.Fatalf("second run (no sidecars/parts/docs entries present) should be a no-op: %v", err)
	}
}

func TestSingletonStandardDocsAbsentConfig(t *testing.T) {
	if err := applySingletonStandardDocs(t.TempDir()); err != nil {
		t.Errorf("applySingletonStandardDocs with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

func TestSingletonStandardDocsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".awf", "config.yaml"), "docs: [a, b\n")
	if err := applySingletonStandardDocs(root); err == nil {
		t.Error("expected error surfaced from the malformed docs: probe decode")
	}
}
