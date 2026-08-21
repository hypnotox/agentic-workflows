package adr

import (
	"errors"
	"strings"
	"testing"
	"time"
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
