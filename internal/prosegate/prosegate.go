// Package prosegate scans a project's tracked text files for the seven banned
// typographic punctuation substitutes (ADR-0119). It is the presence-level
// counterpart to the net-increase audit rule in internal/audit: this package
// answers "is the tree clean", not "did this commit make it worse".
package prosegate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Banned maps each banned rune to its display name. Each key is written as an
// escape, never as the character: this file is itself a tracked file the scan
// reads, so a typed glyph here would make the scanner fail its own gate.
// internal/project/residue_scan_test.go's bannedRunes map is the precedent and
// must stay in agreement with this one. Notation (arrows, mathematical symbols,
// accented letters) is deliberately absent: this is a closed blocklist of
// substitutes for ASCII punctuation, never an ASCII-only allowlist.
var Banned = map[rune]string{
	'\u2014': "em-dash (U+2014)",
	'\u2013': "en-dash (U+2013)",
	'\u2026': "ellipsis (U+2026)",
	'\u2018': "left single quote (U+2018)",
	'\u2019': "right single quote (U+2019)",
	'\u201c': "left double quote (U+201C)",
	'\u201d': "right double quote (U+201D)",
}

// Exemption permits a codepoint in a path, optionally pinning its count.
type Exemption struct {
	Path      string
	Codepoint rune
	Count     *int
}

// Finding is one banned codepoint in one file, with the number found. Pinned is
// set when an exemption pinned a count that did not match.
type Finding struct {
	Path   string
	Rune   rune
	Count  int
	Pinned int
}

// ParseCodepoint turns a "U+2014" spelling into its rune. It rejects anything
// outside the banned set, so a typo cannot silently widen an exemption.
func ParseCodepoint(s string) (rune, error) {
	if !strings.HasPrefix(s, "U+") {
		return 0, fmt.Errorf("codepoint %q: want the form U+2014", s)
	}
	n, err := strconv.ParseUint(s[2:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("codepoint %q: %w", s, err)
	}
	r := rune(n)
	if _, ok := Banned[r]; !ok {
		return 0, fmt.Errorf("codepoint %q is not one of the seven banned substitutes", s)
	}
	return r, nil
}

// Scan reads each path relative to root and reports every banned rune outside
// the exemptions. Paths whose contents are not valid UTF-8 are skipped: a
// default-deny gate must not guess at binary input. An unreadable path is an
// error, never a silent pass.
func Scan(root string, paths []string, exemptions []Exemption) ([]Finding, error) {
	exempt := map[string]*int{}
	for _, e := range exemptions {
		exempt[e.Path+"\x00"+string(e.Codepoint)] = e.Count
	}
	var out []Finding
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if !utf8.Valid(b) {
			continue
		}
		counts := map[rune]int{}
		for _, r := range string(b) {
			if _, bad := Banned[r]; bad {
				counts[r]++
			}
		}
		for r, n := range counts {
			pin, ok := exempt[p+"\x00"+string(r)]
			switch {
			case !ok:
				out = append(out, Finding{Path: p, Rune: r, Count: n})
			case pin != nil && *pin != n:
				out = append(out, Finding{Path: p, Rune: r, Count: n, Pinned: *pin})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Rune < out[j].Rune
	})
	return out, nil
}

// Format renders one finding as a diagnostic line.
func Format(f Finding) string {
	if f.Pinned > 0 {
		return fmt.Sprintf("%s: %s appears %d time(s); the exemption pins %d",
			f.Path, Banned[f.Rune], f.Count, f.Pinned)
	}
	return fmt.Sprintf("%s: %s appears %d time(s); use plain punctuation",
		f.Path, Banned[f.Rune], f.Count)
}
