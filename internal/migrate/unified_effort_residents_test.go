package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const (
	legacyIDA = "b9b5a3e7-28f7-4b51-a05f-bbecea09eb76"
	legacyIDB = "5b47f3b1-dd96-4b12-94d4-222545a7afa3"
)

func residentTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{legacyEffortsRel, legacyMemoryRel} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeLeaf(t *testing.T, root, rel string, content []byte) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLegacyRecord(t *testing.T, root, id string) {
	t.Helper()
	raw, err := json.Marshal(legacyRecord{SchemaVersion: 1, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	writeLeaf(t, root, legacyEffortsRel+"/"+id+".json", raw)
}

func writeLegacyPartial(t *testing.T, root, id, action, path string) string {
	t.Helper()
	raw, err := json.Marshal(legacyPartial{SchemaVersion: 1, EffortID: id, Action: action, Branch: legacyBranchPrefix + id, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	rel := fmt.Sprintf("%s/.%s.%s.partial", legacyEffortsRel, id, action)
	writeLeaf(t, root, rel, raw)
	return rel
}

// requireRefusal asserts the classifier refused with the shared refusal grammar
// and that it named the expected condition and next action.
func requireRefusal(t *testing.T, err error, condition, nextAction string) {
	t.Helper()
	if err == nil {
		t.Fatal("classification accepted an unusable resident set")
	}
	for _, want := range []string{condition, "changed bytes: no", "next action: " + nextAction} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q is missing %q", err, want)
		}
	}
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestClassifyLegacyResidentsKnownLeaves(t *testing.T) {
	root := residentTree(t)
	writeLeaf(t, root, legacyEffortsRel+"/.gitignore", []byte("*\n"))
	writeLeaf(t, root, legacyEffortsRel+"/"+legacyLockName, []byte("lock"))
	writeLegacyRecord(t, root, legacyIDA)
	writeLegacyRecord(t, root, legacyIDB)
	for _, action := range []string{"worktree", "integration", "removal"} {
		writeLegacyPartial(t, root, legacyIDA, action, "")
	}
	// A protocol-2 resident and a finishing reservation are the current
	// binary's own directories and must survive untouched.
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(legacyEffortsRel), "live-effort"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(legacyEffortsRel), ".finishing-"+legacyIDB+"-live-effort"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeLeaf(t, root, legacyMemoryRel+"/.gitignore", []byte("*\n"))
	writeLeaf(t, root, legacyMemoryRel+"/notes.md", []byte("standalone"))
	writeLeaf(t, root, legacyMemoryRel+"/nested/deeper.md", []byte("nested"))

	result, err := ClassifyLegacyResidents(testContext(t), root)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if result.PrimaryRoot != filepath.Clean(root) {
		t.Fatalf("primary root = %q, want %q", result.PrimaryRoot, root)
	}
	want := []string{
		".awf/efforts/." + legacyIDA + ".integration.partial",
		".awf/efforts/." + legacyIDA + ".removal.partial",
		".awf/efforts/." + legacyIDA + ".worktree.partial",
		".awf/efforts/.lock",
		".awf/efforts/" + legacyIDB + ".json",
		".awf/efforts/" + legacyIDA + ".json",
		".awf/memory",
	}
	// The result is sorted bytewise, so sort the expectation the same way
	// rather than relying on the order the fixture wrote the leaves.
	if len(result.Quarantine) != len(want) {
		t.Fatalf("quarantine = %v, want %d entries", result.Quarantine, len(want))
	}
	got := strings.Join(result.Quarantine, "\n")
	for _, rel := range want {
		if !strings.Contains(got, rel) {
			t.Fatalf("quarantine %v is missing %q", result.Quarantine, rel)
		}
	}
	for i := 1; i < len(result.Quarantine); i++ {
		if result.Quarantine[i-1] >= result.Quarantine[i] {
			t.Fatalf("quarantine is not sorted and unique: %v", result.Quarantine)
		}
	}
	for _, kept := range []string{".gitignore", "live-effort", ".finishing-" + legacyIDB + "-live-effort"} {
		if strings.Contains(got, legacyEffortsRel+"/"+kept) {
			t.Fatalf("quarantine claims the retained entry %q: %v", kept, result.Quarantine)
		}
	}
	// Classification is read-only: every byte it inspected is still there.
	for _, rel := range []string{
		legacyEffortsRel + "/.gitignore", legacyEffortsRel + "/" + legacyLockName,
		legacyEffortsRel + "/" + legacyIDA + ".json", legacyMemoryRel + "/nested/deeper.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("classification changed %s: %v", rel, err)
		}
	}
}

func TestClassifyLegacyResidentsAbsentAndEmptyRoots(t *testing.T) {
	t.Run("no-resident-roots", func(t *testing.T) {
		result, err := ClassifyLegacyResidents(testContext(t), t.TempDir())
		if err != nil {
			t.Fatalf("classify: %v", err)
		}
		if len(result.Quarantine) != 0 {
			t.Fatalf("quarantine = %v, want none", result.Quarantine)
		}
	})
	t.Run("empty-memory-root-is-still-owned", func(t *testing.T) {
		// The root itself is the thing protocol 2 stops owning, so an empty one
		// is still quarantined rather than left behind as an orphan directory.
		result, err := ClassifyLegacyResidents(testContext(t), residentTree(t))
		if err != nil {
			t.Fatalf("classify: %v", err)
		}
		if len(result.Quarantine) != 1 || result.Quarantine[0] != legacyMemoryRel {
			t.Fatalf("quarantine = %v, want just %s", result.Quarantine, legacyMemoryRel)
		}
	})
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestClassifyLegacyResidentsUnknownAndMalformedLeaves(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rel       string
		content   string
		condition string
	}{
		{"unknown-extension", legacyEffortsRel + "/stray.txt", "x", "unknown resident leaf"},
		{"non-uuid-record", legacyEffortsRel + "/not-a-uuid.json", `{"schemaVersion":1}`, "unknown resident leaf"},
		{"partial-without-leading-dot", legacyEffortsRel + "/" + legacyIDA + ".worktree.partial", "{}", "unknown resident leaf"},
		{"partial-unknown-action", legacyEffortsRel + "/." + legacyIDA + ".rebase.partial", "{}", "unknown resident leaf"},
		{"partial-non-uuid", legacyEffortsRel + "/.not-a-uuid.worktree.partial", "{}", "unknown resident leaf"},
		{"partial-no-action", legacyEffortsRel + "/." + legacyIDA + ".partial", "{}", "unknown resident leaf"},
		{"unparseable-record", legacyEffortsRel + "/" + legacyIDA + ".json", "{not json", "malformed resident"},
		{"wrong-schema-record", legacyEffortsRel + "/" + legacyIDA + ".json", `{"schemaVersion":2,"id":"` + legacyIDA + `"}`, "malformed legacy effort record"},
		{"mismatched-record-id", legacyEffortsRel + "/" + legacyIDA + ".json", `{"schemaVersion":1,"id":"` + legacyIDB + `"}`, "malformed legacy effort record"},
		{"unparseable-partial", legacyEffortsRel + "/." + legacyIDA + ".worktree.partial", "{not json", "malformed resident"},
		{
			"mismatched-partial-branch",
			legacyEffortsRel + "/." + legacyIDA + ".worktree.partial",
			`{"schemaVersion":1,"effortId":"` + legacyIDA + `","action":"worktree","branch":"awf/other"}`,
			"malformed legacy partial-mutation evidence",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := residentTree(t)
			path := writeLeaf(t, root, tc.rel, []byte(tc.content))
			_, err := ClassifyLegacyResidents(testContext(t), root)
			requireRefusal(t, err, tc.condition, preserveManually)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("refusal changed %s: %v", tc.rel, err)
			}
		})
	}
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestClassifyLegacyResidentsUnsafeResidents(t *testing.T) {
	t.Run("symlinked-leaf", func(t *testing.T) {
		root := residentTree(t)
		target := writeLeaf(t, root, "outside.json", []byte(`{"schemaVersion":1}`))
		link := filepath.Join(root, filepath.FromSlash(legacyEffortsRel), legacyIDA+".json")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "unsafe resident", preserveManually)
		if _, err := os.Lstat(link); err != nil {
			t.Fatalf("refusal removed the symlink: %v", err)
		}
	})
	t.Run("hard-linked-leaf", func(t *testing.T) {
		root := residentTree(t)
		target := writeLeaf(t, root, "outside.json", []byte(`{"schemaVersion":1}`))
		link := filepath.Join(root, filepath.FromSlash(legacyEffortsRel), legacyIDA+".json")
		if err := os.Link(target, link); err != nil {
			t.Skipf("hard link unavailable: %v", err)
		}
		// The same bytes are reachable from outside the resident root, so awf
		// cannot prove discarding this copy is its own decision to make.
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "unsafe resident", preserveManually)
	})
	t.Run("symlinked-memory-root", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), filepath.Join(root, filepath.FromSlash(legacyMemoryRel))); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "symlinked resident", preserveManually)
	})
	t.Run("memory-root-is-a-file", func(t *testing.T) {
		root := t.TempDir()
		writeLeaf(t, root, legacyMemoryRel, []byte("not a directory"))
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "is not a directory", preserveManually)
	})
	t.Run("efforts-root-is-a-file", func(t *testing.T) {
		root := t.TempDir()
		writeLeaf(t, root, legacyEffortsRel, []byte("not a directory"))
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "is not a directory", preserveManually)
	})
	t.Run("symlinked-memory-descendant", func(t *testing.T) {
		root := residentTree(t)
		target := writeLeaf(t, root, "outside.md", []byte("elsewhere"))
		link := filepath.Join(root, filepath.FromSlash(legacyMemoryRel), "linked.md")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "unsafe standalone memory resident", preserveManually)
		if _, err := os.Lstat(link); err != nil {
			t.Fatalf("refusal removed the symlink: %v", err)
		}
	})
	t.Run("symlinked-memory-subdirectory", func(t *testing.T) {
		root := residentTree(t)
		link := filepath.Join(root, filepath.FromSlash(legacyMemoryRel), "linked")
		if err := os.Symlink(t.TempDir(), link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, "unsafe standalone memory resident", preserveManually)
	})
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestClassifyLegacyResidentsRefusesLiveWorktreeFacts(t *testing.T) {
	// newRepo builds a committed repository whose primary checkout carries the
	// legacy record for legacyIDA.
	newRepo := func(t *testing.T) gitfixture.Fixture {
		t.Helper()
		primary := filepath.Join(t.TempDir(), "primary")
		repo := gitfixture.InitNativeAt(t, primary)
		if err := os.WriteFile(filepath.Join(primary, "tracked"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, repo, "tracked")
		gitfixture.NativeCommit(t, repo, "base")
		if err := os.MkdirAll(filepath.Join(primary, filepath.FromSlash(legacyEffortsRel)), 0o700); err != nil {
			t.Fatal(err)
		}
		writeLegacyRecord(t, primary, legacyIDA)
		return repo
	}
	managedRel := legacyWorktreesRel + "/" + legacyIDA

	t.Run("registered-managed-worktree", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		gitfixture.NativeWorktreeAdd(t, repo, filepath.Join(primary, filepath.FromSlash(managedRel)), legacyBranchPrefix+legacyIDA)
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "legacy managed worktree path", legacyWorktreeNextAction(legacyIDA))
		if _, err := os.Stat(filepath.Join(primary, filepath.FromSlash(legacyEffortsRel), legacyIDA+".json")); err != nil {
			t.Fatalf("refusal changed the record: %v", err)
		}
	})
	t.Run("registration-without-its-path", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		managed := filepath.Join(primary, filepath.FromSlash(managedRel))
		gitfixture.NativeWorktreeAdd(t, repo, managed, legacyBranchPrefix+legacyIDA)
		if err := os.RemoveAll(managed); err != nil {
			t.Fatal(err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "is still registered with Git", legacyWorktreeNextAction(legacyIDA))
	})
	t.Run("branch-only", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		gitfixture.NativeBranch(t, repo, legacyBranchPrefix+legacyIDA)
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "legacy managed branch", legacyWorktreeNextAction(legacyIDA))
	})
	t.Run("branch-with-no-surviving-record", func(t *testing.T) {
		// Git topology alone is enough: the identifier is recovered from the
		// branch name even though nothing under .awf names it any more.
		repo := newRepo(t)
		primary := repo.Root()
		if err := os.Remove(filepath.Join(primary, filepath.FromSlash(legacyEffortsRel), legacyIDA+".json")); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeBranch(t, repo, legacyBranchPrefix+legacyIDB)
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "legacy managed branch "+legacyBranchPrefix+legacyIDB, legacyWorktreeNextAction(legacyIDB))
	})
	t.Run("managed-directory-without-git-facts", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		if err := os.MkdirAll(filepath.Join(primary, filepath.FromSlash(managedRel)), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "legacy managed worktree path", legacyWorktreeNextAction(legacyIDA))
	})
	t.Run("deterministic-refusal-order", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		writeLegacyRecord(t, primary, legacyIDB)
		for _, id := range []string{legacyIDA, legacyIDB} {
			gitfixture.NativeBranch(t, repo, legacyBranchPrefix+id)
		}
		first, err := ClassifyLegacyResidents(testContext(t), primary)
		second, secondErr := ClassifyLegacyResidents(testContext(t), primary)
		if len(first.Quarantine) != 0 || len(second.Quarantine) != 0 {
			t.Fatal("a refused classification returned quarantine targets")
		}
		if err == nil || secondErr == nil || err.Error() != secondErr.Error() {
			t.Fatalf("refusal is not deterministic: %v vs %v", err, secondErr)
		}
		// legacyIDB sorts before legacyIDA, so it is always reported first.
		requireRefusal(t, err, legacyIDB, legacyWorktreeNextAction(legacyIDB))
	})
	t.Run("managed-branch-checked-out-elsewhere", func(t *testing.T) {
		// The branch is what makes the effort's work reachable, so it refuses
		// wherever it is checked out, not only under the managed root.
		repo := newRepo(t)
		primary := repo.Root()
		elsewhere := filepath.Join(filepath.Dir(primary), "elsewhere")
		gitfixture.NativeWorktreeAdd(t, repo, elsewhere, legacyBranchPrefix+legacyIDA)
		_, err := ClassifyLegacyResidents(testContext(t), primary)
		requireRefusal(t, err, "is checked out at "+elsewhere, legacyWorktreeNextAction(legacyIDA))
	})
	t.Run("unreadable-managed-root-propagates", func(t *testing.T) {
		root := residentTree(t)
		writeLeaf(t, root, legacyWorktreesRel, []byte("not a directory"))
		if _, err := ClassifyLegacyResidents(testContext(t), root); err == nil || !strings.Contains(err.Error(), legacyWorktreesRel) {
			t.Fatalf("want an inspection failure naming the managed root, got %v", err)
		}
	})
	t.Run("clean-repository-classifies", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		result, err := ClassifyLegacyResidents(testContext(t), primary)
		if err != nil {
			t.Fatalf("classify: %v", err)
		}
		if len(result.Quarantine) != 1 || result.Quarantine[0] != legacyEffortsRel+"/"+legacyIDA+".json" {
			t.Fatalf("quarantine = %v", result.Quarantine)
		}
		if result.PrimaryRoot != primary {
			t.Fatalf("primary root = %q, want %q", result.PrimaryRoot, primary)
		}
	})
	t.Run("linked-worktree-classifies-the-primary-root", func(t *testing.T) {
		repo := newRepo(t)
		primary := repo.Root()
		linked := filepath.Join(filepath.Dir(primary), "linked")
		gitfixture.NativeWorktreeAddDetached(t, repo, linked, "HEAD")
		result, err := ClassifyLegacyResidents(testContext(t), linked)
		if err != nil {
			t.Fatalf("classify from a linked checkout: %v", err)
		}
		if result.PrimaryRoot != primary {
			t.Fatalf("primary root = %q, want %q", result.PrimaryRoot, primary)
		}
		if len(result.Quarantine) != 1 {
			t.Fatalf("quarantine = %v", result.Quarantine)
		}
	})
}

// invariant: config/migrations-and-locks:unified-effort-resident-migration
func TestClassifyLegacyResidentsPartialEvidenceGitFacts(t *testing.T) {
	t.Run("live-declared-checkout-refuses", func(t *testing.T) {
		root := residentTree(t)
		// Partial worktree evidence names an arbitrary absolute checkout path
		// that need not sit under the managed root, so proving the evidence
		// obsolete requires inspecting that exact path.
		declared := filepath.Join(t.TempDir(), "half-created")
		if err := os.MkdirAll(declared, 0o700); err != nil {
			t.Fatal(err)
		}
		rel := writeLegacyPartial(t, root, legacyIDA, "worktree", declared)
		_, err := ClassifyLegacyResidents(testContext(t), root)
		requireRefusal(t, err, declared, legacyWorktreeNextAction(legacyIDA))
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("refusal changed the evidence: %v", err)
		}
		if _, err := os.Stat(declared); err != nil {
			t.Fatalf("refusal changed the declared checkout: %v", err)
		}
	})
	t.Run("absent-declared-checkout-is-obsolete", func(t *testing.T) {
		root := residentTree(t)
		declared := filepath.Join(t.TempDir(), "never-created")
		rel := writeLegacyPartial(t, root, legacyIDA, "worktree", declared)
		result, err := ClassifyLegacyResidents(testContext(t), root)
		if err != nil {
			t.Fatalf("classify: %v", err)
		}
		if len(result.Quarantine) != 2 || result.Quarantine[0] != rel {
			t.Fatalf("quarantine = %v, want %s and the memory root", result.Quarantine, rel)
		}
	})
}

func TestClassifyLegacyResidentsGitFailures(t *testing.T) {
	t.Run("broken-checkout-propagates", func(t *testing.T) {
		root := residentTree(t)
		writeLegacyRecord(t, root, legacyIDA)
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := awfgit.Open(root); err == nil || errors.Is(err, awfgit.ErrNotARepository) {
			t.Fatalf("fixture did not produce a present broken Git error: %v", err)
		}
		if _, err := ClassifyLegacyResidents(testContext(t), root); err == nil {
			t.Fatal("a broken checkout classified as a plain directory")
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(legacyEffortsRel), legacyIDA+".json")); err != nil {
			t.Fatalf("broken checkout changed the record: %v", err)
		}
	})
	t.Run("unsafe-topology-propagates", func(t *testing.T) {
		primary := filepath.Join(t.TempDir(), "primary")
		gitfixture.InitNativeAt(t, primary)
		alias := filepath.Join(filepath.Dir(primary), "alias")
		if err := os.Symlink(primary, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := ClassifyLegacyResidents(testContext(t), alias); err == nil {
			t.Fatal("an unsafe checkout classified")
		}
	})
}

// TestApplyUnifiedEffortResidentsRefusals pins the migration's two stop
// conditions: it propagates any preflight refusal untouched, and it refuses a
// checkout whose residents belong to a different root than the lock it would
// commit.
func TestApplyUnifiedEffortResidentsRefusals(t *testing.T) {
	t.Run("propagates-a-preflight-refusal", func(t *testing.T) {
		root := residentTree(t)
		path := writeLeaf(t, root, legacyEffortsRel+"/stray.txt", []byte("x"))
		var out bytes.Buffer
		requireRefusal(t, applyUnifiedEffortResidents(testContext(t), root, &out), "unknown resident leaf", preserveManually)
		if out.Len() != 0 {
			t.Fatalf("a refused migration announced the reset: %q", out.String())
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("refusal changed the resident: %v", err)
		}
	})
	t.Run("residents-outside-the-invoking-checkout", func(t *testing.T) {
		primary := filepath.Join(t.TempDir(), "primary")
		repo := gitfixture.InitNativeAt(t, primary)
		if err := os.WriteFile(filepath.Join(primary, "tracked"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, repo, "tracked")
		gitfixture.NativeCommit(t, repo, "base")
		if err := os.MkdirAll(filepath.Join(primary, filepath.FromSlash(legacyMemoryRel)), 0o700); err != nil {
			t.Fatal(err)
		}
		linked := filepath.Join(filepath.Dir(primary), "linked")
		gitfixture.NativeWorktreeAddDetached(t, repo, linked, "HEAD")
		var out bytes.Buffer
		// One journal spans one root, so the split is refused rather than
		// half-applied across two checkouts.
		requireRefusal(t, applyUnifiedEffortResidents(testContext(t), linked, &out), "belong to the primary checkout", "run `awf upgrade` from "+primary)
		if _, err := os.Stat(filepath.Join(primary, filepath.FromSlash(legacyMemoryRel))); err != nil {
			t.Fatalf("refusal changed the resident root: %v", err)
		}
	})
}
