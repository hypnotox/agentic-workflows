// Package prosegate enforces punctuation restraint across a project's tracked
// text files. It rejects en dashes and paragraphs containing three or more em
// dashes while permitting ellipses and curly quotes.
package prosegate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	emDash = '\u2014'
	enDash = '\u2013'
)

// codepointNames includes the two guarded codepoints and the five retired
// codepoints accepted in existing exemption configuration. Retired exemptions
// are inert: compatibility parsing must not turn a policy relaxation into a
// configuration failure.
var codepointNames = map[rune]string{
	emDash:   "em-dash (U+2014)",
	enDash:   "en-dash (U+2013)",
	'\u2026': "ellipsis (U+2026)",
	'\u2018': "left single quote (U+2018)",
	'\u2019': "right single quote (U+2019)",
	'\u201c': "left double quote (U+201C)",
	'\u201d': "right double quote (U+201D)",
}

// Exemption permits a guarded codepoint in a path, optionally pinning its
// whole-file occurrence count.
type Exemption struct {
	Path      string
	Codepoint rune
	Count     *int
}

// Finding is one punctuation-restraint violation. Paragraph is nonzero for an
// em-dash threshold violation and identifies the blank-line-delimited text
// block within the file. Pinned is non-nil for an exemption count mismatch.
type Finding struct {
	Path      string
	Rune      rune
	Count     int
	Paragraph int
	Pinned    *int
}

// ViolationCounts is the comparable policy measure used by the historical
// advisory: all en dashes plus em dashes beyond the allowance of two in each
// paragraph.
type ViolationCounts struct {
	EnDashes     int
	EmDashExcess int
}

// ParseCodepoint turns a "U+2014" spelling into its rune. It accepts the five
// formerly guarded codepoints so existing adopter exemptions remain compatible.
func ParseCodepoint(s string) (rune, error) {
	if !strings.HasPrefix(s, "U+") {
		return 0, fmt.Errorf("codepoint %q: want the form U+2014", s)
	}
	n, err := strconv.ParseUint(s[2:], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("codepoint %q: %w", s, err)
	}
	r := rune(n)
	if _, ok := codepointNames[r]; !ok {
		return 0, fmt.Errorf("codepoint %q is not a guarded or compatibility punctuation codepoint", s)
	}
	return r, nil
}

// File is one staged regular file supplied by the command's git snapshot.
type File struct {
	Path  string
	Bytes []byte
}

type paragraphCount struct {
	number   int
	emDashes int
}

// CountViolations measures the guarded punctuation in text. A whitespace-only
// line is blank, CRLF and LF delimit paragraphs identically, and empty blocks
// between consecutive blank lines are ignored.
func CountViolations(text string) ViolationCounts {
	counts := ViolationCounts{EnDashes: strings.Count(text, string(enDash))}
	for _, paragraph := range emDashParagraphs(text) {
		if paragraph.emDashes > 2 {
			counts.EmDashExcess += paragraph.emDashes - 2
		}
	}
	return counts
}

func emDashParagraphs(text string) []paragraphCount {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []paragraphCount
	paragraph := 0
	inParagraph := false
	emDashes := 0
	flush := func() {
		if !inParagraph {
			return
		}
		out = append(out, paragraphCount{number: paragraph, emDashes: emDashes})
		inParagraph = false
		emDashes = 0
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if !inParagraph {
			paragraph++
			inParagraph = true
		}
		emDashes += strings.Count(line, string(emDash))
	}
	flush()
	return out
}

// Scan reports punctuation-restraint violations in the supplied staged text
// files. Files whose contents are not valid UTF-8 are silently skipped.
func Scan(files []File, exemptions []Exemption) ([]Finding, []string, error) {
	type key struct {
		path string
		rune rune
	}
	exempt := map[key]*int{}
	for _, e := range exemptions {
		// Exemptions for formerly guarded punctuation remain accepted but inert.
		if e.Codepoint != emDash && e.Codepoint != enDash {
			continue
		}
		exempt[key{e.Path, e.Codepoint}] = e.Count
	}

	actual := map[key]int{}
	processed := map[key]bool{}
	var out []Finding
	var skipped []string
	for _, file := range files {
		if !utf8.Valid(file.Bytes) {
			skipped = append(skipped, file.Path)
			continue
		}
		text := string(file.Bytes)
		for _, guarded := range []rune{emDash, enDash} {
			k := key{file.Path, guarded}
			count := strings.Count(text, string(guarded))
			actual[k] = count
			processed[k] = true
			pin, ok := exempt[k]
			if ok {
				if pin != nil && *pin != count {
					out = append(out, Finding{Path: file.Path, Rune: guarded, Count: count, Pinned: pin})
				}
				continue
			}
			switch guarded {
			case enDash:
				if count > 0 {
					out = append(out, Finding{Path: file.Path, Rune: guarded, Count: count})
				}
			case emDash:
				for _, paragraph := range emDashParagraphs(text) {
					if paragraph.emDashes >= 3 {
						out = append(out, Finding{Path: file.Path, Rune: guarded, Count: paragraph.emDashes, Paragraph: paragraph.number})
					}
				}
			}
		}
	}
	for k, pin := range exempt {
		if processed[k] || pin == nil || *pin == 0 {
			continue
		}
		out = append(out, Finding{Path: k.path, Rune: k.rune, Count: actual[k], Pinned: pin})
	}

	sort.Strings(skipped)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Rune != out[j].Rune {
			return out[i].Rune < out[j].Rune
		}
		return out[i].Paragraph < out[j].Paragraph
	})
	return out, skipped, nil
}

// Format renders one finding as a diagnostic line.
func Format(f Finding) string {
	if f.Pinned != nil {
		return fmt.Sprintf("%s: %s appears %d time(s); the exemption pins %d",
			f.Path, codepointNames[f.Rune], f.Count, *f.Pinned)
	}
	if f.Rune == emDash && f.Paragraph > 0 {
		return fmt.Sprintf("%s: paragraph %d contains %s %d time(s); use at most two per paragraph",
			f.Path, f.Paragraph, codepointNames[f.Rune], f.Count)
	}
	return fmt.Sprintf("%s: %s appears %d time(s); en dashes are not permitted",
		f.Path, codepointNames[f.Rune], f.Count)
}
