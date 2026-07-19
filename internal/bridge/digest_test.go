package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDigestInputInvalidation(t *testing.T) {
	root := validLiveBridgeProject(t)
	base, err := Digest(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Every digest-universe input, when its content changes, changes the digest.
	for _, rel := range []string{
		".awf/config.yaml",
		".awf/domains/core.yaml",
		".awf/topics/metadata/core/contracts.yaml",
		".awf/topics/parts/core/contracts/current-state.md",
		"docs/decisions/0001-one.md",
		"src/x_test.go",
		".awf/current-state-migration.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		orig, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		mustWrite(t, path, append(append([]byte{}, orig...), []byte("\n# extra\n")...), 0o644)
		changed, err := Digest(root, nil)
		if err != nil {
			t.Fatalf("digest after %s change: %v", rel, err)
		}
		if changed == base {
			t.Fatalf("changing %s did not change the digest", rel)
		}
		mustWrite(t, path, orig, 0o644)
		if restored, _ := Digest(root, nil); restored != base {
			t.Fatalf("restoring %s did not restore the digest", rel)
		}
	}
	// A file outside the universe never enters the digest.
	mustWrite(t, filepath.Join(root, "docs", "decisions", "ACTIVE.md"), []byte("regenerated\n"), 0o644)
	mustWrite(t, filepath.Join(root, "README.md"), []byte("outside\n"), 0o644)
	if outside, _ := Digest(root, nil); outside != base {
		t.Fatal("a non-universe path entered the digest")
	}
}

func TestDigestApprovalModeAndPresence(t *testing.T) {
	root := validLiveBridgeProject(t)
	base, err := Digest(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	approval := filepath.Join(root, ".awf", "current-state-migration.yaml")
	// Mode is part of the record: a chmod alone changes the digest.
	if err := os.Chmod(approval, 0o600); err != nil {
		t.Fatal(err)
	}
	if moded, _ := Digest(root, nil); moded == base {
		t.Fatal("approval mode change did not change the digest")
	}
	if err := os.Chmod(approval, 0o640); err != nil {
		t.Fatal(err)
	}
	// Removing the approval file (present:false) changes the digest.
	if err := os.Remove(approval); err != nil {
		t.Fatal(err)
	}
	removed, err := Digest(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if removed == base {
		t.Fatal("removing the approval file did not change the digest")
	}
}

func TestDigestErrorsAndOptionalSubtrees(t *testing.T) {
	// No config layout: config.Load fails.
	if _, err := Digest(t.TempDir(), nil); err == nil {
		t.Fatal("missing config accepted")
	}
	// A config that cannot be converted fails at the conversion step.
	badConvert := t.TempDir()
	mustMkdir(t, filepath.Join(badConvert, ".awf"))
	mustWrite(t, filepath.Join(badConvert, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  disabled: true\n"), 0o644)
	if _, err := Digest(badConvert, nil); err == nil {
		t.Fatal("unconvertible config accepted")
	}
	// A minimal tree with no domains, topics, decisions, or marker sources: every
	// optional subtree is absent and the digest is still computed.
	minimal := t.TempDir()
	mustMkdir(t, filepath.Join(minimal, ".awf"))
	mustWrite(t, filepath.Join(minimal, ".awf", "config.yaml"), []byte("prefix: awf\ntargets: [claude]\ninvariants:\n  sources: []\n"), 0o644)
	if _, err := Digest(minimal, nil); err != nil {
		t.Fatalf("minimal tree: %v", err)
	}
}

func TestDigestSkipsNestedProjectsAndDependencyTrees(t *testing.T) {
	root := validLiveBridgeProject(t)
	base, err := Digest(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A dependency tree and a nested project under a scanned path are skipped, so
	// a marker-shaped file inside either never enters the digest.
	mustMkdir(t, filepath.Join(root, "vendor"))
	mustWrite(t, filepath.Join(root, "vendor", "v_test.go"), []byte("package v\n// invariant: stable\n"), 0o644)
	mustMkdir(t, filepath.Join(root, "src", "nested", ".awf"))
	mustWrite(t, filepath.Join(root, "src", "nested", "n_test.go"), []byte("package n\n// invariant: stable\n"), 0o644)
	// A non-regular entry under a scanned subtree contributes nothing.
	if err := os.Symlink("contracts", filepath.Join(root, ".awf", "topics", "parts", "core", "link")); err != nil {
		t.Fatal(err)
	}
	if got, _ := Digest(root, nil); got != base {
		t.Fatal("a skipped nested/dependency/non-regular file entered the digest")
	}
}

func TestCutoffFacts(t *testing.T) {
	root := validLiveBridgeProject(t)
	cutoff, gaps, err := CutoffFacts(root)
	if err != nil || cutoff != 2 || len(gaps) != 0 {
		t.Fatalf("cutoff=%d gaps=%v err=%v", cutoff, gaps, err)
	}
	// No config layout: config.Load fails.
	if _, _, err := CutoffFacts(t.TempDir()); err == nil {
		t.Fatal("missing config accepted")
	}
	// A malformed ADR makes corpus loading fail.
	mustWrite(t, filepath.Join(root, "docs", "decisions", "0002-bad.md"), []byte("---\nstatus: [\n---\n"), 0o644)
	if _, _, err := CutoffFacts(root); err == nil {
		t.Fatal("malformed corpus accepted")
	}
}

func TestDigestUsesMutationAfterImage(t *testing.T) {
	root := validLiveBridgeProject(t)
	base, err := Digest(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A normalization mutation's after-image, not the on-disk bytes, is digested.
	present := Mutation{Path: ".awf/config.yaml", AfterPresent: true, AfterMode: 0o644, After: []byte("prefix: awf\ntargets: [claude]\n")}
	withPresent, err := Digest(root, []Mutation{present})
	if err != nil {
		t.Fatal(err)
	}
	if withPresent == base {
		t.Fatal("mutation after-image was ignored")
	}
	// An after-absent mutation drops the path from the universe.
	absent := Mutation{Path: ".awf/config.yaml", AfterPresent: false}
	withAbsent, err := Digest(root, []Mutation{absent})
	if err != nil {
		t.Fatal(err)
	}
	if withAbsent == base || withAbsent == withPresent {
		t.Fatalf("after-absent mutation not distinct: base=%s present=%s absent=%s", base, withPresent, withAbsent)
	}
}
