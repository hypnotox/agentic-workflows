package project

import (
	"errors"
	"strings"
	"testing"
)

type failingPitfallReader struct {
	paths            []string
	pathErr, readErr error
}

func (r failingPitfallReader) Paths(string) ([]string, error)        { return r.paths, r.pathErr }
func (r failingPitfallReader) ReadFile(string) ([]byte, bool, error) { return nil, false, r.readErr }

func TestPitfallCorpusReaderErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := loadPitfallCorpusFrom(failingPitfallReader{pathErr: boom}); !errors.Is(err, boom) {
		t.Fatalf("paths error = %v", err)
	}
	if _, err := loadPitfallCorpusFrom(failingPitfallReader{paths: []string{".awf/docs/pitfalls/a.md"}, readErr: boom}); err == nil || !strings.Contains(err.Error(), "read pitfall") {
		t.Fatalf("read error = %v", err)
	}
}
