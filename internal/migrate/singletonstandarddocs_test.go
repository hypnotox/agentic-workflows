package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestSingletonStandardDocsRelocatesSidecarAndParts(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	testsupport.WriteFile(t, filepath.Join(awf, "config.yaml"), "prefix: ex\ndocs:\n  - architecture\n  - workflow\n  - doc-standard\n")
	testsupport.WriteFile(t, filepath.Join(awf, "docs", "workflow.yaml"), "data:\n  k: v\n")
	testsupport.WriteFile(t, filepath.Join(awf, "docs", "parts", "workflow", "local-hooks.md"), "LOCAL HOOKS BODY\n")

	var out bytes.Buffer
	if err := applySingletonStandardDocs(root, &out); err != nil {
		t.Fatalf("applySingletonStandardDocs: %v", err)
	}
	for _, want := range []string{
		"singleton-standard-docs: moved .awf/docs/workflow.yaml → .awf/workflow.yaml\n",
		"singleton-standard-docs: moved .awf/docs/parts/workflow → .awf/parts/workflow\n",
		`singleton-standard-docs: removed doc "workflow" from docs: (now always-on)` + "\n",
		`singleton-standard-docs: removed doc "doc-standard" from docs: (now always-on)` + "\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing provenance line %q in output:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), `"architecture"`) {
		t.Errorf("untouched doc must not be reported:\n%s", out.String())
	}

	if b, err := os.ReadFile(filepath.Join(awf, "workflow.yaml")); err != nil || !strings.Contains(string(b), "k: v") {
		t.Errorf("workflow sidecar not relocated: %v, %s", err, b)
	}
	if _, err := os.Stat(filepath.Join(awf, "docs", "workflow.yaml")); !os.IsNotExist(err) {
		t.Errorf("old workflow sidecar location should be gone, stat err = %v", err)
	}
	// invariant: singleton-doc-migration-relocates-parts
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
	testsupport.WriteFile(t, filepath.Join(awf, "config.yaml"), "prefix: ex\n")
	if err := applySingletonStandardDocs(root, io.Discard); err != nil {
		t.Fatalf("first run: %v", err)
	}
	var out bytes.Buffer
	if err := applySingletonStandardDocs(root, &out); err != nil {
		t.Fatalf("second run (no sidecars/parts/docs entries present) should be a no-op: %v", err)
	}
	if out.String() != "" {
		t.Errorf("a no-op run must print nothing, got:\n%s", out.String())
	}
}

func TestSingletonStandardDocsAbsentConfig(t *testing.T) {
	if err := applySingletonStandardDocs(t.TempDir(), io.Discard); err != nil {
		t.Errorf("applySingletonStandardDocs with no .awf/config.yaml should be a no-op, got %v", err)
	}
}

func TestSingletonStandardDocsMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "docs: [a, b\n")
	if err := applySingletonStandardDocs(root, io.Discard); err == nil {
		t.Error("expected error surfaced from the malformed docs: probe decode")
	}
}

func TestRelocateRefusesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	dst := filepath.Join(dir, "dst.yaml")
	testsupport.WriteFile(t, src, "SRC\n")
	testsupport.WriteFile(t, dst, "DST\n")

	if _, err := relocate(src, dst); err == nil {
		t.Fatal("relocate should refuse to overwrite an existing destination")
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "DST\n" {
		t.Errorf("destination was clobbered: %v, %s", err, b)
	}
}

// A partial prior migration — old and new sidecar location both present — is a
// realistic adopter tree; the migration must surface relocate's refusal rather
// than silently overwrite (the by-design branch its call site propagates).
func TestSingletonStandardDocsRefusesPartialPriorMigration(t *testing.T) {
	root := t.TempDir()
	awf := filepath.Join(root, ".awf")
	testsupport.WriteFile(t, filepath.Join(awf, "config.yaml"), "prefix: ex\n")
	testsupport.WriteFile(t, filepath.Join(awf, "docs", "workflow.yaml"), "data: {}\n")
	testsupport.WriteFile(t, filepath.Join(awf, "workflow.yaml"), "data: {}\n")

	err := applySingletonStandardDocs(root, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected the existing-destination refusal, got %v", err)
	}
}

// A path component of src that is a regular file makes the initial Stat fail
// with a non-NotExist error (ENOTDIR), which relocate must surface.
func TestRelocateSurfacesStatError(t *testing.T) {
	dir := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(dir, "docs"), "a file, not a dir\n")
	if _, err := relocate(filepath.Join(dir, "docs", "workflow.yaml"), filepath.Join(dir, "workflow.yaml")); err == nil {
		t.Fatal("expected the ENOTDIR stat error to surface")
	}
}

// A dst parent that exists as a regular file makes MkdirAll fail without any
// permission fault involved; relocate must surface it.
func TestRelocateSurfacesMkdirError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	testsupport.WriteFile(t, src, "SRC\n")
	testsupport.WriteFile(t, filepath.Join(dir, "parts"), "a file, not a dir\n")
	if _, err := relocate(src, filepath.Join(dir, "parts", "workflow", "x.md")); err == nil {
		t.Fatal("expected the not-a-directory MkdirAll error to surface")
	}
}

// A config path that exists as a directory makes ReadFile fail with a
// non-NotExist error (EISDIR), which removeFromDocsArray must surface.
func TestRemoveFromDocsArraySurfacesReadError(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := removeFromDocsArray(cfgDir, "workflow"); err == nil {
		t.Fatal("expected the directory read error to surface")
	}
}
