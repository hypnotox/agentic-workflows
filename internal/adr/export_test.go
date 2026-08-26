package adr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// SetNowForTest overrides the now seam for a test and returns the previous
// value, so the caller can restore it. It lives in an in-package _test.go file
// (package adr) so the external adr_test package can reach it without the seam
// shipping in the production binary (ADR-0063).
func SetNowForTest(fn func() time.Time) (prev func() time.Time) {
	prev = now
	now = fn
	return prev
}

func TestValidateV2HistoryRejectsImplementingWithoutAppliedOperations(t *testing.T) {
	digest := ContentDigest(nil)
	record := ADR{
		Status: "Implementing",
		History: []HistoryEvent{
			{Kind: HistoryStatus, Date: "2026-08-04", Status: "Proposed"},
			{Kind: HistoryStatus, Date: "2026-08-04", Status: "Implementing", Digest: digest},
			{Kind: HistoryApplied, Date: "2026-08-04"},
		},
	}
	if err := validateV2History(record); err == nil || !strings.Contains(err.Error(), "requires at least one applied operation") {
		t.Fatalf("Implementing without applied operations error = %v", err)
	}
}

type treeReaderForTest struct {
	paths   []string
	files   map[string][]byte
	pathErr error
	readErr error
}

func (r treeReaderForTest) Paths(string) ([]string, error) { return r.paths, r.pathErr }
func (r treeReaderForTest) ReadFile(name string) ([]byte, bool, error) {
	if r.readErr != nil {
		return nil, false, r.readErr
	}
	data, ok := r.files[name]
	return data, ok, nil
}

func TestLoadCorpusFromTreeRejectsReaderFaultsAndMalformedAuthority(t *testing.T) {
	valid := []byte("---\nstatus: Implemented\n---\n# ADR: Valid\n")
	cases := []struct {
		name string
		read treeReaderForTest
		want string
	}{
		{"paths", treeReaderForTest{pathErr: errors.New("paths failed")}, "paths failed"},
		{"read", treeReaderForTest{paths: []string{"docs/decisions/0001-valid.md"}, readErr: errors.New("read failed")}, "read 0001-valid.md: read failed"},
		{"malformed", treeReaderForTest{paths: []string{"docs/decisions/0001-valid.md"}, files: map[string][]byte{"docs/decisions/0001-valid.md": []byte("---\nstatus: [\n---\n")}}, "parse 0001-valid.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadCorpusFromTree(tc.read, "docs/decisions"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadCorpusFromTree error = %v, want %q", err, tc.want)
			}
		})
	}
	corpus, err := LoadCorpusFromTree(treeReaderForTest{paths: []string{
		"docs/decisions/nested/0002-hidden.md", "docs/decisions/README.md", "docs/decisions/note.txt", "docs/decisions/0001-valid.md", "docs/decisions/0003-absent.md",
	}, files: map[string][]byte{"docs/decisions/0001-valid.md": valid}}, "docs/decisions")
	if err != nil || len(corpus.All()) != 1 || corpus.All()[0].Path != "docs/decisions/0001-valid.md" {
		t.Fatalf("filtered tree corpus = %#v, %v", corpus.All(), err)
	}
}

// NewFileForTest exposes the confined scaffold only to external ADR tests.
func NewFileForTest(dir, title string) (string, error) {
	files, err := filesystem.Open(dir)
	if err != nil {
		return "", err
	}
	defer files.Close()
	path, err := scaffoldRecordConfined(files, ".", title, CurrentFormat(), false)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(path)), nil
}

// NewPendingFileForTest exposes the confined pending scaffold only to external ADR tests.
func NewPendingFileForTest(dir, title string) (string, error) {
	files, err := filesystem.Open(dir)
	if err != nil {
		return "", err
	}
	defer files.Close()
	path, err := scaffoldRecordConfined(files, ".", title, CurrentFormat(), true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(path)), nil
}

// invariant: adr-system/adr-lifecycle:adr-new-no-overwrite (TestScaffoldRecordConfinedRefusesParentSwap)
func TestScaffoldRecordConfinedRefusesParentSwap(t *testing.T) {
	root := t.TempDir()
	decisions := filepath.Join(root, "docs", "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisions, "template.md"), []byte("---\nformat: current-state-v1\nstatus: Proposed\ndate: YYYY-MM-DD\n---\n# ADR-NNNN: Title\n\n## State changes\n\nNone.\n\n## Status history\n\n- YYYY-MM-DD: Proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := filesystem.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	previous := now
	now = func() time.Time {
		moved := filepath.Join(root, "docs", "decisions-before-swap")
		if err := os.Rename(decisions, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, decisions); err != nil {
			t.Fatal(err)
		}
		return time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	}
	defer func() { now = previous }()
	_, err = scaffoldRecordConfined(files, "docs/decisions", "Swap Refusal", CurrentFormat(), false)
	if err == nil {
		t.Fatal("parent swap unexpectedly published")
	}
	if !strings.Contains(err.Error(), "path escapes from parent") {
		t.Fatalf("parent-swap refusal = %v, want root-confinement identity", err)
	}
	if got, readErr := os.ReadFile(sentinel); readErr != nil || string(got) != "outside" {
		t.Fatalf("outside sentinel = %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "0001-swap-refusal.md")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("outside ADR = %v, want not exist; refusal=%v", statErr, err)
	}
}

func TestADRCreationRefusesMismatchedHandleBeforeReadingOrPublishing(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	decisions := filepath.Join(rootB, "docs", "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisions, "template.md"), []byte("---\nformat: current-state-v1\nstatus: Proposed\ndate: YYYY-MM-DD\n---\n# ADR-NNNN: Title\n\n## State changes\n\nNone.\n\n## Status history\n\n- YYYY-MM-DD: Proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lease, err := filesystem.AcquireTrackedLease(context.Background(), rootA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Errorf("release tracked lease: %v", err)
		}
	}()
	files, err := filesystem.Open(rootB)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	for _, create := range []func(string, *filesystem.Lease, *filesystem.Handle, string, string) (string, error){NewFileLeased, NewPendingFileLeased} {
		if _, err := create(rootA, lease, files, "docs/decisions", "Refused"); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("mismatched scaffold error = %v", err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(decisions, "*refused.md"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("mismatched handle published in root B: %v, %v", matches, err)
	}
}

func TestADRCreationRequiresMatchingLiveCapability(t *testing.T) {
	if _, err := NewFileLeased(t.TempDir(), nil, nil, "docs/decisions", "No Lease"); err == nil || !strings.Contains(err.Error(), "covering tracked lease") {
		t.Fatalf("unleased ADR writer error = %v", err)
	}
	root := t.TempDir()
	lease, err := filesystem.AcquireTrackedLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Errorf("release tracked lease: %v", err)
		}
	}()
	if _, err := NewFileLeased(root, lease, nil, "docs/decisions", "No Handle"); err == nil || !strings.Contains(err.Error(), "selected-root handle") {
		t.Fatalf("handle-free ADR writer error = %v", err)
	}
}

// TestADRWriterSingleHome rejects a return to host-path writers or the nested
// ADR allocation lock. Production creation accepts both the transaction lease
// and selected-root handle at its only entry points.
func TestADRWriterSingleHome(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(testsupport.RepoRoot(t), "internal", "adr", "adr.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, forbidden := range []string{"func NewFile(", "func NewPendingFile(", "func acquireScaffoldLock", "filesystem.Acquire("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("obsolete or unleased ADR writer survived: %s", forbidden)
		}
	}
	for _, required := range []string{"func NewFileLeased(root string, lease *filesystem.Lease, files *filesystem.Handle", "func NewPendingFileLeased(root string, lease *filesystem.Lease, files *filesystem.Handle", "func scaffoldRecordConfined(files *filesystem.Handle"} {
		if !strings.Contains(source, required) {
			t.Fatalf("confined writer wiring absent: %s", required)
		}
	}
}
