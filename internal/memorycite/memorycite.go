// Package memorycite answers one question about a piece of text: does it cite a
// specific working-memory file (ADR-0158)? The working-memory convention allows
// a decision record to name the directory or a placeholder, and bans naming an
// actual file, so the discrimination is entirely about what follows the prefix.
//
// The prefix lives in a constant rather than inline in each literal because this
// file is itself scanned material: a source line writing a concrete name right
// after the prefix would be the very shape the detector flags, and the
// convention is to keep the shipped surfaces free of it.
package memorycite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// dir is the unified effort-resident prefix every owned-memory citation starts
// with. A bare directory or an angle-bracket placeholder below it is durable
// prose; only a concrete slug followed by the exact memory basename is banned.
const dir = ".awf/efforts/"
const memoryBase = "memory.md"

// terminators bound a path mention in prose, links, and code spans. Slash is
// included so a reference to the owned file remains a citation even when prose
// appends another path component. Colon is included so a `path:line` citation,
// the clickable form this project's prose uses, stays a citation: no filename
// legitimately continues past a colon.
const terminators = "/ \t\v\f\r`\"'()[]{}<>;,!?:"

// extensionTerminator ends a mention only when nothing attaches after it. A
// period both ends a sentence and opens a further extension, so `memory.md.`
// closing a sentence is a citation while `memory.md.bak` names a different file
// and is not.
const extensionTerminator = '.'

// Reference is one citation, with a 1-based line number.
type Reference struct {
	Path    string
	Line    int
	Segment string
}

// File is one staged file supplied by the command's git snapshot.
type File struct {
	Path  string
	Bytes []byte
}

// Exemption permits citations in a path, optionally pinning their count.
type Exemption struct {
	Path  string
	Count *int
}

// Finding is one path's citations; the count is len(Lines). Pinned is non-nil
// when an exemption pinned a count that did not match, carrying the pin (which
// may legitimately be zero); nil when the path was not exempt at all.
type Finding struct {
	Path   string
	Lines  []int
	Pinned *int
}

// ScanText reports every citation in b, in line order and left-to-right within
// a line. Path is used verbatim, so a caller with no file may pass a synthetic
// label such as the name of the message it is scanning.
func ScanText(path string, b []byte) []Reference {
	var out []Reference
	for i, rawLine := range strings.Split(string(b), "\n") {
		line := strings.ReplaceAll(rawLine, `\`, "/")
		for pos := 0; ; {
			j := strings.Index(line[pos:], dir)
			if j < 0 {
				break
			}
			after := pos + j + len(dir)
			if segment, ok := concreteOwnedMemory(line[after:]); ok {
				out = append(out, Reference{Path: path, Line: i + 1, Segment: segment})
			}
			pos = after
		}
	}
	return out
}

func concreteOwnedMemory(rest string) (string, bool) {
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return "", false
	}
	slug := rest[:slash]
	if strings.ContainsAny(slug, "<>") || !validSlug(slug) {
		return "", false
	}
	afterSlug := rest[slash+1:]
	if !strings.HasPrefix(afterSlug, memoryBase) {
		return "", false
	}
	if !terminatesMention(afterSlug[len(memoryBase):]) {
		return "", false
	}
	return slug + "/" + memoryBase, true
}

// terminatesMention reports whether what follows the memory basename leaves the
// mention naming that exact file.
func terminatesMention(remaining string) bool {
	if remaining == "" {
		return true
	}
	if strings.ContainsRune(terminators, rune(remaining[0])) {
		return true
	}
	if rune(remaining[0]) != extensionTerminator {
		return false
	}
	next := remaining[1:]
	return next == "" || strings.ContainsRune(terminators, rune(next[0])) || rune(next[0]) == extensionTerminator
}

func validSlug(slug string) bool {
	if len(slug) < 1 || len(slug) > 63 {
		return false
	}
	lastHyphen := true
	for _, c := range []byte(slug) {
		letterOrDigit := c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		if !letterOrDigit && c != '-' {
			return false
		}
		if c == '-' && lastHyphen {
			return false
		}
		lastHyphen = c == '-'
	}
	return !lastHyphen
}

// Scan reports every citation in the supplied files outside the exemptions,
// grouped by path and sorted by path.
func Scan(files []File, exemptions []Exemption) []Finding {
	exempt := map[string]*int{}
	for _, e := range exemptions {
		exempt[e.Path] = e.Count
	}
	lines := map[string][]int{}
	for _, file := range files {
		for _, ref := range ScanText(file.Path, file.Bytes) {
			lines[file.Path] = append(lines[file.Path], ref.Line)
		}
	}

	var out []Finding
	for path, ls := range lines {
		pin, ok := exempt[path]
		switch {
		case !ok:
			out = append(out, Finding{Path: path, Lines: ls})
		case pin != nil && *pin != len(ls):
			out = append(out, Finding{Path: path, Lines: ls, Pinned: pin})
		}
	}
	for path, pin := range exempt {
		if pin != nil && len(lines[path]) == 0 && *pin != 0 {
			out = append(out, Finding{Path: path, Pinned: pin})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Result adapts completed memory-citation findings to immutable owner-classified results.
func Result(findings []Finding) (checkresult.Result, error) {
	out := make([]checkresult.Finding, 0, len(findings))
	for _, finding := range findings {
		out = append(out, checkresult.Finding{Rank: severity.Error, Property: "effort-memory-citation", Evidence: checkresult.Evidence{Kind: "memory", Path: finding.Path, Detail: Format(finding)}})
	}
	return checkresult.New(out, nil)
}

// Format renders one finding as a diagnostic line.
func Format(f Finding) string {
	if f.Pinned != nil {
		return fmt.Sprintf("%s: %d effort-owned memory citation(s); the exemption pins %d",
			f.Path, len(f.Lines), *f.Pinned)
	}
	nums := make([]string, len(f.Lines))
	for i, n := range f.Lines {
		nums[i] = strconv.Itoa(n)
	}
	return fmt.Sprintf("%s: %d effort-owned memory citation(s) on line(s) %s; name the .awf/efforts/ directory, use an angle-bracket slug placeholder, or remove the ephemeral file citation",
		f.Path, len(f.Lines), strings.Join(nums, ", "))
}
