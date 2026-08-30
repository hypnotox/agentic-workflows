package migrate

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func writeDecision(t *testing.T, root, name string) {
	t.Helper()
	testsupport.WriteFile(t, filepath.Join(root, decisionDir, name), "# Historical decision\n")
}

func TestRetirePitfallRelationsMigrationPreservesBytesModesAndDecisionArchive(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 49)
	writeDecision(t, root, "0089-first.md")
	writeDecision(t, root, "0099-second.md")
	decision := filepath.Join(root, decisionDir, "0089-first.md")
	beforeDecision, err := os.ReadFile(decision)
	if err != nil {
		t.Fatal(err)
	}
	lf := filepath.Join(root, pitfall.SourceDir, "lf.md")
	crlf := filepath.Join(root, pitfall.SourceDir, "crlf.md")
	lfSource := "---\ntitle: LF\nrelated: [89, 99]\n---\nBody.\n"
	crlfSource := "---\r\ntitle: CRLF\r\nrelated:\r\n  - 89\r\n---\r\nBody."
	testsupport.WriteFile(t, lf, lfSource)
	testsupport.WriteFile(t, crlf, crlfSource)
	if err := os.Chmod(lf, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(crlf, 0o600); err != nil {
		t.Fatal(err)
	}

	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{retirePitfallRelationsName}) || len(changes) != 2 || len(mutations) != 2 {
		t.Fatalf("applied=%v changes=%v mutations=%#v", applied, changes, mutations)
	}
	want := map[string]struct {
		body string
		mode os.FileMode
	}{
		".awf/docs/pitfalls/lf.md":   {"---\ntitle: LF\n---\nBody.\n\nRelated decisions: [ADR-0089](../decisions/0089-first.md), [ADR-0099](../decisions/0099-second.md)\n", 0o640},
		".awf/docs/pitfalls/crlf.md": {"---\r\ntitle: CRLF\r\n---\r\nBody.\r\n\r\nRelated decisions: [ADR-0089](../decisions/0089-first.md)", 0o600},
	}
	for _, mutation := range mutations {
		wantMutation, found := want[mutation.Path]
		if !found || string(mutation.Content) != wantMutation.body || mutation.Mode != wantMutation.mode {
			t.Fatalf("mutation=%#v", mutation)
		}
	}
	if got, err := os.ReadFile(decision); err != nil || !bytes.Equal(got, beforeDecision) {
		t.Fatalf("Build changed decision archive: %q, %v", got, err)
	}
	if got, err := os.ReadFile(lf); err != nil || string(got) != lfSource {
		t.Fatalf("Build wrote pitfall: %q, %v", got, err)
	}
}

func TestRetirePitfallRelationsRefusesUnsafeRelationsAndFrontmatter(t *testing.T) {
	cases := []struct {
		name, source string
		setup        func(*testing.T, string)
		want         string
	}{
		{"missing target", "---\ntitle: Test\nrelated: [89]\n---\nBody\n", nil, "missing decision target 0089"},
		{"reserved and nested leaves do not qualify", "---\ntitle: Test\nrelated: [89]\n---\nBody\n", func(t *testing.T, root string) {
			writeDecision(t, root, "README.md")
			testsupport.WriteFile(t, filepath.Join(root, decisionDir, "nested", "0089-hidden.md"), "# Nested\n")
		}, "missing decision target 0089"},
		{"ambiguous target", "---\ntitle: Test\nrelated: [89]\n---\nBody\n", func(t *testing.T, root string) {
			writeDecision(t, root, "0089-one.md")
			writeDecision(t, root, "0089-two.md")
		}, "ambiguous decision identity 0089"},
		{"nonnumeric", "---\ntitle: Test\nrelated: [nope]\n---\nBody\n", nil, "nonnumeric"},
		{"out of range", "---\ntitle: Test\nrelated: [10000]\n---\nBody\n", nil, "out-of-range"},
		{"duplicate relation", "---\ntitle: Test\nrelated: [89, 89]\n---\nBody\n", nil, "duplicate related identity"},
		{"malformed frontmatter", "---\ntitle: [\nrelated: [89]\n---\nBody\n", nil, "did not find expected"},
		{"wrong relation shape", "---\ntitle: Test\nrelated: 89\n---\nBody\n", nil, "must be a non-empty numeric list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLock(t, root, 49)
			if tc.setup != nil {
				tc.setup(t, root)
			}
			if tc.name != "ambiguous target" && tc.name != "missing target" && tc.name != "reserved and nested leaves do not qualify" {
				writeDecision(t, root, "0089-good.md")
			}
			pitfallPath := filepath.Join(root, pitfall.SourceDir, "test.md")
			testsupport.WriteFile(t, pitfallPath, tc.source)
			_, _, mutations, err := Build(context.Background(), root)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(mutations) != 0 {
				t.Fatalf("mutations=%#v err=%v, want refusal containing %q", mutations, err, tc.want)
			}
			got, readErr := os.ReadFile(pitfallPath)
			if readErr != nil || string(got) != tc.source {
				t.Fatalf("refusal wrote source: %q, %v", got, readErr)
			}
		})
	}
}

func TestRetirePitfallRelationsRefusesUnsafeDecisionTarget(t *testing.T) {
	root := t.TempDir()
	writeLock(t, root, 49)
	outside := filepath.Join(t.TempDir(), "0089-unsafe.md")
	testsupport.WriteFile(t, outside, "# Outside\n")
	if err := os.MkdirAll(filepath.Join(root, decisionDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, decisionDir, "0089-unsafe.md")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	testsupport.WriteFile(t, filepath.Join(root, pitfall.SourceDir, "test.md"), "---\ntitle: Test\nrelated: [89]\n---\nBody\n")
	_, _, mutations, err := Build(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "unsafe decision target") || len(mutations) != 0 {
		t.Fatalf("mutations=%#v err=%v, want unsafe target refusal", mutations, err)
	}
}

func TestRetirePitfallRelationsIgnoresLookalikesAndIsIdempotent(t *testing.T) {
	decisions := map[int]string{89: "docs/decisions/0089-good.md"}
	source := []byte("---\ntitle: Test\nrelated: [89]\n---\n```yaml\nrelated: [9999]\n```\n\nrelated: [89] is prose.\n")
	got, changed, err := replacePitfallRelations(source, decisions)
	if err != nil || !changed {
		t.Fatalf("changed=%t err=%v", changed, err)
	}
	want := "---\ntitle: Test\n---\n```yaml\nrelated: [9999]\n```\n\nrelated: [89] is prose.\n\nRelated decisions: [ADR-0089](../decisions/0089-good.md)\n"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	again, changed, err := replacePitfallRelations(got, decisions)
	if err != nil || changed || !bytes.Equal(again, got) {
		t.Fatalf("idempotence changed=%t err=%v got=%q", changed, err, again)
	}
}

func TestPitfallBytesForGenerationInTreeUsesSchema50Semantics(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "0089-good.md")
	sourcePath := pitfall.SourceDir + "/test.md"
	testsupport.WriteFile(t, filepath.Join(root, sourcePath), "---\ntitle: Test\nrelated: [89]\n---\nBody\n")
	handle, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	tree := &ProposedTree{files: handle, mutations: map[string]FileMutation{}}
	got, err := PitfallBytesForGenerationInTree(49, sourcePath, tree)
	if err != nil || !strings.Contains(string(got), "[ADR-0089](../decisions/0089-good.md)") {
		t.Fatalf("tree projection=%q err=%v", got, err)
	}
	if _, err := PitfallBytesForGeneration(49, []byte("unresolvable")); err == nil {
		t.Fatal("byte-only schema-49 projection accepted an invented relation context")
	}
	if _, err := PitfallBytesForGenerationInTree(49, "docs/pitfalls/test.md", tree); err == nil {
		t.Fatal("tree projection accepted path outside authored pitfall sources")
	}
	if _, _, _, err := Build(context.Background(), root); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("independent migration build failed: %v", err)
	}
}
