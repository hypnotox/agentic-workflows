package upgrade

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const sealedConfig = `prefix: example
domains:
  - alpha
`

// sealedRepo builds a prepared-tree fixture: a git repo whose committed HEAD
// carries a current-state config, one domain, one topic, one ADR, and the
// migration approval file. It returns the repo root, the committed HEAD hash,
// and the recomputed tree digest so a test can assemble a matching or mismatched
// attestation.
func sealedRepo(t *testing.T) (dir, head, digest string) {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	dir = repo.Root()
	files := map[string]string{
		".awf/config.yaml":                              sealedConfig,
		".awf/domains/alpha.yaml":                       "paths:\n  - internal/**\n",
		".awf/topics/metadata/alpha/core.yaml":          "title: Core\nsummary: Core rules.\npaths:\n  - internal/**\n",
		".awf/topics/parts/alpha/core/current-state.md": "Intro.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n",
		"docs/decisions/0001-first.md": testsupport.ADR("Implemented",
			testsupport.WithDate("2026-06-25"), testsupport.WithTitle("0001: First"),
			testsupport.WithBody("## Context\nx\n## Consequences\nc\n")),
		".awf/current-state-migration.yaml": "version: 1\ninvariantApprovals: []\n",
	}
	for rel, body := range files {
		testsupport.WriteFile(t, filepath.Join(dir, rel), body)
	}
	gitfixture.Stage(t, repo, files)
	gitfixture.Merge(t, repo, "prepared")
	head, err := headHash(t, dir)
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	digest = treeDigestForTest(t, dir)
	return dir, head, digest
}

func treeDigestForTest(t *testing.T, root string) string {
	t.Helper()
	tree, err := filesystem.Open(root)
	if err != nil {
		t.Fatalf("open tree: %v", err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	digest, err := treeDigest(root, tree)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return digest
}

func sealedAtt(head, digest string) *manifest.BridgeAttestation {
	return &manifest.BridgeAttestation{Version: attestationVersion, PreparedHead: head, TreeDigest: digest, ADRFormatV1From: 137, LegacyADRGaps: []int{7}}
}

func TestTreeDigestIsStableAndSensitive(t *testing.T) {
	dir, _, digest := sealedRepo(t)
	again := treeDigestForTest(t, dir)
	if again != digest {
		t.Fatalf("digest not stable: %q vs %q", digest, again)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("digest prefix: %q", digest)
	}
	// A change to any universe member moves the digest.
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/topics/parts/alpha/core/current-state.md"),
		"Intro changed.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n")
	moved := treeDigestForTest(t, dir)
	if moved == digest {
		t.Fatal("digest did not move on content change")
	}
}

func TestTreeDigestBranches(t *testing.T) {
	// No config at all: config.Load fails.
	missing := t.TempDir()
	tree, err := filesystem.Open(missing)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tree.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := treeDigest(missing, tree); err == nil {
		t.Fatal("treeDigest accepted a tree with no config")
	}
	// A minimal tree (config only, no domains/topics/decisions subtrees, no
	// approval file) exercises the missing-subtree and absent-universe-member
	// branches without faulting.
	min := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(min, ".awf/config.yaml"), "prefix: example\ndomains:\n  - alpha\n")
	_ = treeDigestForTest(t, min)
	// A config with a marker source glob plus a matching file and a nested adopter
	// project exercises the marker-source match and the nested-project skip.
	full := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(full, ".awf/config.yaml"),
		"prefix: example\ndomains:\n  - alpha\ncurrentState:\n  sources:\n    - globs:\n        - \"internal/**\"\n      marker: //\n")
	testsupport.WriteFile(t, filepath.Join(full, "internal/x.go"), "package x\n")
	testsupport.WriteFile(t, filepath.Join(full, "sub/.awf/config.yaml"), "prefix: nested\n")
	testsupport.WriteFile(t, filepath.Join(full, "sub/internal/y.go"), "package y\n")
	_ = treeDigestForTest(t, full)
}

func TestCollectMarkerSourcesPrunesNestedGitRoots(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gitPath string
		body    string
	}{
		{"git directory", "internal/nested-dir/.git/config", "repo\n"},
		{"gitdir pointer", "internal/nested-file/.git", "gitdir: /tmp/elsewhere\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, tc.gitPath), tc.body)
			base := filepath.Dir(filepath.Join(root, tc.gitPath))
			if filepath.Base(tc.gitPath) != ".git" {
				base = filepath.Dir(base)
			}
			testsupport.WriteFile(t, filepath.Join(base, "x.go"), "package nested\n")
			universe := map[string]bool{}
			sources := []config.CurrentStateSource{{Globs: []string{"internal/**"}}}
			tree, err := filesystem.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := tree.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := collectMarkerSources(tree, sources, universe); err != nil {
				t.Fatal(err)
			}
			for path := range universe {
				if strings.HasSuffix(path, "/x.go") {
					t.Fatalf("nested repository source included: %s", path)
				}
			}
		})
	}
}

func TestVerifyAcceptsSeal(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	if err := Verify(testContext(t), dir, sealedAtt(head, digest)); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyRejections(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	for _, tc := range []struct {
		name string
		att  *manifest.BridgeAttestation
		want string
	}{
		{"version", &manifest.BridgeAttestation{Version: 2, PreparedHead: head, TreeDigest: digest, ADRFormatV1From: 2, LegacyADRGaps: []int{}}, "version"},
		{"head", sealedAtt("0000000000000000000000000000000000000000", digest), "prepared head"},
		{"digest", sealedAtt(head, "sha256:0000"), "digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Verify(testContext(t), dir, tc.att); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestVerifyOutsideRepo(t *testing.T) {
	if err := Verify(testContext(t), filepath.Join(t.TempDir(), "missing"), sealedAtt("x", "y")); err == nil {
		t.Fatal("verify opened a missing root")
	}
	if err := Verify(testContext(t), t.TempDir(), sealedAtt("x", "y")); err == nil {
		t.Fatal("verify accepted a non-repo")
	}
}

func TestVerifyRejectsUnbornHead(t *testing.T) {
	root := gitfixture.InitRepo(t).Root()
	if err := Verify(testContext(t), root, sealedAtt("x", "y")); err == nil {
		t.Fatal("verify accepted a repository with an unborn HEAD")
	}
}

// finalLock writes an attested lock to the fixture and returns the loaded lock
// the final upgrade consumes.
func finalLock(t *testing.T, dir string, att *manifest.BridgeAttestation) *manifest.Lock {
	t.Helper()
	lock := &manifest.Lock{
		AWFVersion:        "0.18.0",
		SchemaVersion:     14,
		Files:             map[string]manifest.Entry{"docs/x.md": {OutputHash: "sha256:1"}},
		BridgeAttestation: att,
	}
	if err := lock.Save(config.LockPath(dir)); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := manifest.LoadOptional(config.LockPath(dir))
	if err != nil || !found {
		t.Fatalf("reload lock: %v found=%t", err, found)
	}
	return loaded
}

// invariant: tooling/upgrade-runtime:bridge-attestation-cutoff-payload-discarded (TestFinalUpgradeDiscardsBridgeADRRoutingPayload)
func TestFinalUpgradeDiscardsBridgeADRRoutingPayload(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	adrPath := filepath.Join(dir, "docs", "decisions", "0001-first.md")
	adrBefore, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatal(err)
	}
	lock := finalLock(t, dir, sealedAtt(head, digest))
	if _, err := FinalUpgrade(testContext(t), dir, lock); err != nil {
		t.Fatalf("final upgrade: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, approvalPath)); !os.IsNotExist(err) {
		t.Fatal("approval file not deleted")
	}
	if journalPresence(t, dir) {
		t.Fatal("journal residue after success")
	}
	after, found, err := manifest.LoadOptional(config.LockPath(dir))
	if err != nil || !found {
		t.Fatalf("reload: %v", err)
	}
	if after.BridgeAttestation != nil {
		t.Fatal("attestation not cleared")
	}
	if after.InitializedWithVersion != "" {
		t.Fatalf("bridge cutover fabricated initialization provenance %q", after.InitializedWithVersion)
	}
	lockBytes, err := after.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lockBytes), "adrFormatV1From") || strings.Contains(string(lockBytes), "legacyAdrGaps") {
		t.Fatalf("retired routing payload promoted into final lock: %s", lockBytes)
	}
	if after.Files["docs/x.md"].OutputHash != "sha256:1" {
		t.Fatal("existing lock files not preserved")
	}
	adrAfter, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(adrAfter, adrBefore) {
		t.Fatal("final upgrade rewrote ADR bytes")
	}

	invalidDir, invalidHead, invalidDigest := sealedRepo(t)
	invalidLock := finalLock(t, invalidDir, sealedAtt(invalidHead, invalidDigest+"-mismatch"))
	invalidADRPath := filepath.Join(invalidDir, "docs", "decisions", "0001-first.md")
	approvalBefore, err := os.ReadFile(filepath.Join(invalidDir, approvalPath))
	if err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(config.LockPath(invalidDir))
	if err != nil {
		t.Fatal(err)
	}
	invalidADRBefore, err := os.ReadFile(invalidADRPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalUpgrade(testContext(t), invalidDir, invalidLock); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("invalid seal error = %v, want digest refusal", err)
	}
	for path, before := range map[string][]byte{
		filepath.Join(invalidDir, approvalPath): approvalBefore,
		config.LockPath(invalidDir):             lockBefore,
		invalidADRPath:                          invalidADRBefore,
	} {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("invalid seal mutated %s", path)
		}
	}
	if journalPresence(t, invalidDir) {
		t.Fatal("invalid seal created a cutover journal")
	}
}

func TestFinalUpgradeRequiresAttestation(t *testing.T) {
	dir, _, _ := sealedRepo(t)
	lock := &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 15}
	if _, err := FinalUpgrade(testContext(t), dir, lock); err == nil || !strings.Contains(err.Error(), "no current-state attestation") {
		t.Fatalf("want no-attestation error, got %v", err)
	}
	invalid := &manifest.Lock{AWFVersion: "bad", InitializedWithVersion: "1.0.0"}
	if _, err := FinalUpgrade(testContext(t), dir, invalid); err == nil || !strings.Contains(err.Error(), "invalid authority") {
		t.Fatalf("want invalid-authority error, got %v", err)
	}
}

func TestFinalUpgradeRejectsInvalidSeal(t *testing.T) {
	dir, head, _ := sealedRepo(t)
	lock := finalLock(t, dir, sealedAtt(head, "sha256:bad"))
	if _, err := FinalUpgrade(testContext(t), dir, lock); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("want digest rejection, got %v", err)
	}
	// The tree is untouched: the approval file survives a refused upgrade.
	if _, err := os.Stat(filepath.Join(dir, approvalPath)); err != nil {
		t.Fatalf("approval file removed despite refusal: %v", err)
	}
}

func TestCutoverOperationsRequiresApprovalPresent(t *testing.T) {
	dir, _, _ := sealedRepo(t)
	if err := os.Remove(filepath.Join(dir, approvalPath)); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14, Files: map[string]manifest.Entry{}}
	if _, err := cutoverOperations(dir, lock); err == nil || !strings.Contains(err.Error(), "approval file") {
		t.Fatalf("want absent-approval error, got %v", err)
	}
}

// TestResetLegacyResidentsRefusals pins the two ways the resident reset stops
// before moving a byte: no lock to hang the commit point on, and a resident
// plan the journal contract itself rejects.
// invariant: config/migrations-and-locks:unified-effort-resident-migration (TestResetLegacyResidentsRefusals)
func TestResetLegacyResidentsRefusals(t *testing.T) {
	t.Run("residents-without-a-lock", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf", "efforts"))
		mustWrite(t, filepath.Join(root, ".awf", "efforts", "legacy.json"), []byte("{}"))
		_, err := ResetLegacyResidents(root, []string{".awf/efforts/legacy.json"}, 22)
		if err == nil || !strings.Contains(err.Error(), "cannot reset 1 legacy resident") {
			t.Fatalf("want a missing-lock refusal, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "legacy.json")); err != nil {
			t.Fatalf("refusal changed the resident: %v", err)
		}
	})
	t.Run("no-lock-and-nothing-to-reset-converges", func(t *testing.T) {
		// The legacy layout port leaves a lockless tree whose terminal sync
		// stamps the first lock; there is nothing to advance and nothing a
		// modern binary could have left behind.
		root := t.TempDir()
		if _, err := ResetLegacyResidents(root, nil, 22); err != nil {
			t.Fatalf("lockless tree with no residents: %v", err)
		}
		if journalPresence(t, root) {
			t.Fatal("journal created for a no-op reset")
		}
	})
	t.Run("unusable-resident-plan", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf"))
		mustWrite(t, filepath.Join(root, LockRel()), []byte(`{"awfVersion":"0.25.0","schemaVersion":21,"files":{}}`))
		_, err := ResetLegacyResidents(root, []string{"../escape"}, 22)
		if err == nil || !strings.Contains(err.Error(), "invalid resident reset plan") {
			t.Fatalf("want a plan refusal, got %v", err)
		}
		if journalPresence(t, root) {
			t.Fatal("journal created for a refused plan")
		}
	})
}

// TestResetLegacyResidentsCommitsSchemaAndDiscards pins the whole reset: an
// unordered resident plan is sorted into a valid journal, every resident is
// discarded, and the lock lands last carrying the new generation.
// invariant: config/migrations-and-locks:unified-effort-resident-migration (TestResetLegacyResidentsCommitsSchemaAndDiscards)
func TestResetLegacyResidentsCommitsSchemaAndDiscards(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".awf", "efforts"))
	mustMkdir(t, filepath.Join(root, ".awf", "memory"))
	mustWrite(t, filepath.Join(root, ".awf", "efforts", "legacy.json"), []byte(`{"schemaVersion":1}`))
	mustWrite(t, filepath.Join(root, ".awf", "memory", "notes.md"), []byte("standalone"))
	mustWrite(t, filepath.Join(root, LockRel()), []byte(`{"awfVersion":"0.25.0","schemaVersion":21,"files":{}}`))

	// Deliberately unsorted: the journal contract requires sorted operations and
	// this entry point owns putting them in order.
	if _, err := ResetLegacyResidents(root, []string{".awf/memory", ".awf/efforts/legacy.json"}, 22); err != nil {
		t.Fatalf("reset: %v", err)
	}
	for _, gone := range []string{".awf/efforts/legacy.json", ".awf/memory", QuarantineRel()} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Fatalf("%s survived the committed reset: %v", gone, err)
		}
	}
	lock, err := manifest.Load(filepath.Join(root, LockRel()))
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 22 {
		t.Fatalf("schema = %d, want 22", lock.SchemaVersion)
	}
	// The release version is left to the terminal sync, exactly as every other
	// migration leaves it.
	if lock.AWFVersion != "0.25.0" {
		t.Fatalf("awfVersion = %q, want the pre-reset value", lock.AWFVersion)
	}
	if journalPresence(t, root) {
		t.Fatal("journal residue after a committed reset")
	}
}
