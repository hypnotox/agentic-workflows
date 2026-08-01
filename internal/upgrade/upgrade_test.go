package upgrade

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const sealedConfig = `prefix: example
domains:
  - alpha
currentState:
  maxTopicsPerPath: 8
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
	digest, err = treeDigest(dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return dir, head, digest
}

func sealedAtt(head, digest string) *manifest.BridgeAttestation {
	return &manifest.BridgeAttestation{Version: attestationVersion, PreparedHead: head, TreeDigest: digest, ADRFormatV1From: 137, LegacyADRGaps: []int{7}}
}

func TestTreeDigestIsStableAndSensitive(t *testing.T) {
	dir, _, digest := sealedRepo(t)
	again, err := treeDigest(dir)
	if err != nil || again != digest {
		t.Fatalf("digest not stable: %q vs %q (%v)", digest, again, err)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("digest prefix: %q", digest)
	}
	// A change to any universe member moves the digest.
	testsupport.WriteFile(t, filepath.Join(dir, ".awf/topics/parts/alpha/core/current-state.md"),
		"Intro changed.\n\n## Claims\n\n### `rule: r`\nRule prose.\nOrigin: ADR-0001\n")
	moved, err := treeDigest(dir)
	if err != nil || moved == digest {
		t.Fatalf("digest did not move on content change: %v", err)
	}
}

func TestTreeDigestBranches(t *testing.T) {
	// No config at all: config.Load fails.
	if _, err := treeDigest(t.TempDir()); err == nil {
		t.Fatal("treeDigest accepted a tree with no config")
	}
	// A minimal tree (config only, no domains/topics/decisions subtrees, no
	// approval file) exercises the missing-subtree and absent-universe-member
	// branches without faulting.
	min := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(min, ".awf/config.yaml"), "prefix: example\ndomains:\n  - alpha\n")
	if _, err := treeDigest(min); err != nil {
		t.Fatalf("minimal digest: %v", err)
	}
	// A config with a marker source glob plus a matching file and a nested adopter
	// project exercises the marker-source match and the nested-project skip.
	full := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(full, ".awf/config.yaml"),
		"prefix: example\ndomains:\n  - alpha\ncurrentState:\n  sources:\n    - globs:\n        - \"internal/**\"\n      marker: //\n")
	testsupport.WriteFile(t, filepath.Join(full, "internal/x.go"), "package x\n")
	testsupport.WriteFile(t, filepath.Join(full, "sub/.awf/config.yaml"), "prefix: nested\n")
	testsupport.WriteFile(t, filepath.Join(full, "sub/internal/y.go"), "package y\n")
	if _, err := treeDigest(full); err != nil {
		t.Fatalf("full digest: %v", err)
	}
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
			if err := collectMarkerSources(root, sources, universe); err != nil {
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

func TestCollectMarkerSourcesPropagatesGitBoundaryStatError(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, "internal/x.go"), "package x\n")
	testsupport.SwapVar(t, &lstat, func(string) (fs.FileInfo, error) { return nil, errors.New("stat fault") })
	if err := collectMarkerSources(root, nil, map[string]bool{}); err == nil || !strings.Contains(err.Error(), "stat fault") {
		t.Fatalf("boundary stat error = %v", err)
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
		{"version", &manifest.BridgeAttestation{Version: 2, PreparedHead: head, TreeDigest: digest}, "version"},
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

func TestFinalUpgradeConsumesSeal(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	lock := finalLock(t, dir, sealedAtt(head, digest))
	var log bytes.Buffer
	if err := FinalUpgrade(testContext(t), dir, lock, &log); err != nil {
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
	if after.ADRFormatV1From != 137 || len(after.LegacyADRGaps) != 1 || after.LegacyADRGaps[0] != 7 {
		t.Fatalf("cutoff/gaps not promoted: %d %v", after.ADRFormatV1From, after.LegacyADRGaps)
	}
	if after.Files["docs/x.md"].OutputHash != "sha256:1" {
		t.Fatal("existing lock files not preserved")
	}
	for _, want := range []string{"operation: applied .awf/current-state-migration.yaml", "operation: applied .awf/awf.lock", "operation: upgrade committed"} {
		if !strings.Contains(log.String(), want) {
			t.Fatalf("log missing %q: %s", want, log.String())
		}
	}
}

func TestFinalUpgradeRequiresAttestation(t *testing.T) {
	dir, _, _ := sealedRepo(t)
	lock := &manifest.Lock{AWFVersion: "0.19.0", SchemaVersion: 15}
	if err := FinalUpgrade(testContext(t), dir, lock, bytes.NewBuffer(nil)); err == nil || !strings.Contains(err.Error(), "no current-state attestation") {
		t.Fatalf("want no-attestation error, got %v", err)
	}
	invalid := &manifest.Lock{AWFVersion: "0.19.0", InitializedWithVersion: "0.19.0"}
	if err := FinalUpgrade(testContext(t), dir, invalid, bytes.NewBuffer(nil)); err == nil || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("want invalid-authority error, got %v", err)
	}
}

func TestFinalUpgradeRejectsInvalidSeal(t *testing.T) {
	dir, head, _ := sealedRepo(t)
	lock := finalLock(t, dir, sealedAtt(head, "sha256:bad"))
	if err := FinalUpgrade(testContext(t), dir, lock, bytes.NewBuffer(nil)); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("want digest rejection, got %v", err)
	}
	// The tree is untouched: the approval file survives a refused upgrade.
	if _, err := os.Stat(filepath.Join(dir, approvalPath)); err != nil {
		t.Fatalf("approval file removed despite refusal: %v", err)
	}
}

func TestCutoverOperationsRequiresApprovalPresent(t *testing.T) {
	dir, head, digest := sealedRepo(t)
	if err := os.Remove(filepath.Join(dir, approvalPath)); err != nil {
		t.Fatal(err)
	}
	lock := &manifest.Lock{AWFVersion: "0.18.0", SchemaVersion: 14, Files: map[string]manifest.Entry{}}
	if _, err := cutoverOperations(dir, lock, sealedAtt(head, digest)); err == nil || !strings.Contains(err.Error(), "approval file") {
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
		err := ResetLegacyResidents(root, []string{".awf/efforts/legacy.json"}, 22, io.Discard)
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
		if err := ResetLegacyResidents(root, nil, 22, io.Discard); err != nil {
			t.Fatalf("lockless tree with no residents: %v", err)
		}
		if journalPresence(t, root) {
			t.Fatal("journal created for a no-op reset")
		}
	})
	t.Run("unusable-resident-plan", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, ".awf"))
		mustWrite(t, filepath.Join(root, LockRel()), []byte(`{"awfVersion":"0.25.0","schemaVersion":21,"files":{},"adrFormatV1From":1,"adrFormatV2From":1,"legacyADRGaps":[]}`))
		err := ResetLegacyResidents(root, []string{"../escape"}, 22, io.Discard)
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
	mustWrite(t, filepath.Join(root, LockRel()), []byte(`{"awfVersion":"0.25.0","schemaVersion":21,"files":{},"adrFormatV1From":1,"adrFormatV2From":1,"legacyADRGaps":[]}`))

	var log bytes.Buffer
	// Deliberately unsorted: the journal contract requires sorted operations and
	// this entry point owns putting them in order.
	if err := ResetLegacyResidents(root, []string{".awf/memory", ".awf/efforts/legacy.json"}, 22, &log); err != nil {
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
	if !strings.Contains(log.String(), "operation: upgrade committed") {
		t.Fatalf("log = %q", log.String())
	}
}
