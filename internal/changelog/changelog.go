// Package changelog parses the embedded CHANGELOG.md (see the top-level changelog
// package) into structured, filterable entries (ADR-0041).
package changelog

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// Entry is one version's changelog section.
type Entry struct {
	Version string // e.g. "0.5.1", no leading "v"
	Date    string // YYYY-MM-DD
	Raw     string // the section text, header line through end of body
}

var headerRe = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})$`)

// Load reads "CHANGELOG.md" from fsys and parses it.
// touches-state: tooling/changelog-and-release:changelog-embed-decodes - embedded CHANGELOG.md decode; proof in changelog_test.go
func Load(fsys fs.FS) ([]Entry, error) {
	b, err := fs.ReadFile(fsys, "CHANGELOG.md")
	if err != nil {
		return nil, fmt.Errorf("read CHANGELOG.md: %w", err)
	}
	return Parse(b)
}

// Parse splits raw CHANGELOG.md content into ordered entries (newest first, matching
// file order). Content before the first header (the title and intro prose) is
// discarded - callers wanting the whole file read it directly instead. Parse errors
// if no version header is found.
func Parse(raw []byte) ([]Entry, error) {
	lines := strings.Split(string(raw), "\n")
	var entries []Entry
	var cur *Entry
	var body []string
	flush := func() {
		if cur != nil {
			cur.Raw = strings.TrimRight(strings.Join(body, "\n"), "\n") + "\n"
			entries = append(entries, *cur)
		}
	}
	for _, line := range lines {
		if m := headerRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Entry{Version: m[1], Date: m[2]}
			body = []string{line}
			continue
		}
		if cur != nil {
			body = append(body, line)
		}
	}
	flush()
	if len(entries) == 0 {
		return nil, errors.New("changelog: no version entries found")
	}
	return entries, nil
}

// normalize returns v in the single-leading-v form x/mod/semver requires, so callers
// may pass either "0.5.1" or "v0.5.1". Mirrors cmd/awf's own normalizeSemver - kept
// as a local four-line duplicate rather than a cross-package move (ADR-0041 Decision 2).
func normalize(v string) (string, bool) {
	sv := "v" + strings.TrimPrefix(v, "v")
	if !semver.IsValid(sv) {
		return "", false
	}
	return sv, true
}

func indexOf(entries []Entry, v string) (int, error) {
	target, ok := normalize(v)
	if !ok {
		return -1, fmt.Errorf("changelog: %q is not a valid version", v)
	}
	for i, e := range entries {
		if ev, _ := normalize(e.Version); ev == target {
			return i, nil
		}
	}
	return -1, fmt.Errorf("changelog: no entry for version %q", v)
}

// Version returns the single entry matching v.
func Version(entries []Entry, v string) (Entry, error) {
	i, err := indexOf(entries, v)
	if err != nil {
		return Entry{}, err
	}
	return entries[i], nil
}

// Since returns every entry strictly newer than v (exclusive), newest first. It
// errors if v does not match any entry; it returns an empty (non-nil-error) slice
// if v is already the newest entry.
func Since(entries []Entry, v string) ([]Entry, error) {
	i, err := indexOf(entries, v)
	if err != nil {
		return nil, err
	}
	return entries[:i], nil
}

// Range returns every entry in [from, to] inclusive, newest first. from must be
// chronologically older than or equal to the to end (git range convention:
// from..to); a reversed pair errors rather than silently reordering.
// touches-state: tooling/changelog-and-release:changelog-range-chronological - chronological range selection; proof in changelog_test.go
func Range(entries []Entry, from, to string) ([]Entry, error) {
	fromIdx, err := indexOf(entries, from)
	if err != nil {
		return nil, err
	}
	toIdx, err := indexOf(entries, to)
	if err != nil {
		return nil, err
	}
	if fromIdx < toIdx {
		return nil, fmt.Errorf("changelog: range start %q must be older than range end %q", from, to)
	}
	return entries[toIdx : fromIdx+1], nil
}
