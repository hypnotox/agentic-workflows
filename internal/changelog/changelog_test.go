package changelog

import (
	"strings"
	"testing"
	"testing/fstest"

	changelogfs "github.com/hypnotox/agentic-workflows/changelog"
)

func TestLoadFromEmbed(t *testing.T) {
	entries, err := Load(changelogfs.FS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// invariant: changelog-embed-decodes
	if len(entries) == 0 {
		t.Fatal("no entries parsed from the embedded CHANGELOG.md")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(fstest.MapFS{}); err == nil {
		t.Fatal("Load of an fs.FS missing CHANGELOG.md should error")
	}
}

func TestParse(t *testing.T) {
	raw := "# Changelog\n\nintro line\n\n" +
		"## [0.2.0] - 2026-01-02\n### Features\n- second\n\n" +
		"## [0.1.0] - 2026-01-01\n### Features\n- first\n"
	entries, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Version != "0.2.0" || entries[0].Date != "2026-01-02" {
		t.Errorf("entries[0] = %+v", entries[0])
	}
	if entries[1].Version != "0.1.0" || entries[1].Date != "2026-01-01" {
		t.Errorf("entries[1] = %+v", entries[1])
	}
	if !strings.Contains(entries[0].Raw, "- second") {
		t.Errorf("entries[0].Raw missing its own body: %q", entries[0].Raw)
	}
	if strings.Contains(entries[0].Raw, "- first") {
		t.Errorf("entries[0].Raw bled into entries[1]'s body: %q", entries[0].Raw)
	}
}

func TestParseNoHeaders(t *testing.T) {
	if _, err := Parse([]byte("# Changelog\n\nno entries here\n")); err == nil {
		t.Fatal("Parse of a headerless file should error")
	}
}

var testEntries = []Entry{
	{Version: "0.3.0", Date: "2026-01-03", Raw: "## [0.3.0] - 2026-01-03\nthree\n"},
	{Version: "0.2.0", Date: "2026-01-02", Raw: "## [0.2.0] - 2026-01-02\ntwo\n"},
	{Version: "0.1.0", Date: "2026-01-01", Raw: "## [0.1.0] - 2026-01-01\none\n"},
}

func TestVersion(t *testing.T) {
	e, err := Version(testEntries, "v0.2.0")
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if e.Version != "0.2.0" {
		t.Errorf("Version = %q", e.Version)
	}
	if _, err := Version(testEntries, "0.2.0"); err != nil {
		t.Errorf("Version without a leading v: %v", err)
	}
	if _, err := Version(testEntries, "9.9.9"); err == nil {
		t.Error("Version of an unmatched release should error")
	}
	if _, err := Version(testEntries, "not-a-version"); err == nil {
		t.Error("Version of an invalid version string should error")
	}
}

func TestSince(t *testing.T) {
	got, err := Since(testEntries, "0.1.0")
	if err != nil {
		t.Fatalf("Since: %v", err)
	}
	if len(got) != 2 || got[0].Version != "0.3.0" || got[1].Version != "0.2.0" {
		t.Errorf("Since(0.1.0) = %+v", got)
	}
	got, err = Since(testEntries, "0.3.0")
	if err != nil {
		t.Fatalf("Since latest: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Since(latest) = %+v, want empty", got)
	}
	if _, err := Since(testEntries, "9.9.9"); err == nil {
		t.Error("Since of an unmatched release should error")
	}
}

func TestRange(t *testing.T) {
	got, err := Range(testEntries, "0.1.0", "0.3.0")
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Range(0.1.0..0.3.0) = %+v", got)
	}
	got, err = Range(testEntries, "0.2.0", "0.2.0")
	if err != nil {
		t.Fatalf("Range single version: %v", err)
	}
	if len(got) != 1 || got[0].Version != "0.2.0" {
		t.Errorf("Range(0.2.0..0.2.0) = %+v", got)
	}
	// invariant: changelog-range-chronological
	if _, err := Range(testEntries, "0.3.0", "0.1.0"); err == nil {
		t.Error("Range with a reversed from/to should error")
	}
	if _, err := Range(testEntries, "9.9.9", "0.2.0"); err == nil {
		t.Error("Range with an unmatched from should error")
	}
	if _, err := Range(testEntries, "0.1.0", "9.9.9"); err == nil {
		t.Error("Range with an unmatched to should error")
	}
}
