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
)

// dir is the working-memory directory prefix every citation starts with.
const dir = ".awf/memory/"

// terminators end a path segment: a separator, any ASCII whitespace character,
// or a quoting character that commonly closes an inline mention. The whitespace
// set is the full ASCII one so the shell approximation used to sweep a corpus
// (which spells it as the POSIX space class) stays exactly equivalent. Newline
// is absent because ScanText has already split on it.
const terminators = "/ \t\v\f\r`\"'"

// ignoreFile is the one concrete name that is not a citation: the directory's
// self-ignoring gitignore file, which prose legitimately names.
const ignoreFile = ".gitignore"

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
	for i, line := range strings.Split(string(b), "\n") {
		for pos := 0; ; {
			j := strings.Index(line[pos:], dir)
			if j < 0 {
				break
			}
			after := pos + j + len(dir)
			if seg, ok := concreteSegment(line[after:]); ok {
				out = append(out, Reference{Path: path, Line: i + 1, Segment: seg})
			}
			pos = after
		}
	}
	return out
}

// concreteSegment reads the path segment following the prefix and reports it
// only when it names an actual file: it starts with a path-segment character
// (so the bare directory and an angle-bracket placeholder both pass), contains
// no angle bracket (so a placeholder mid-segment passes too), and is not the
// ignore file.
func concreteSegment(rest string) (string, bool) {
	end := strings.IndexAny(rest, terminators)
	if end < 0 {
		end = len(rest)
	}
	seg := rest[:end]
	if seg == "" || !isSegmentByte(seg[0]) {
		return "", false
	}
	if strings.ContainsAny(seg, "<>") {
		return "", false
	}
	if seg == ignoreFile {
		return "", false
	}
	return seg, true
}

func isSegmentByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '.' || c == '_' || c == '-'
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

// Format renders one finding as a diagnostic line.
func Format(f Finding) string {
	if f.Pinned != nil {
		return fmt.Sprintf("%s: %d working-memory citation(s); the exemption pins %d",
			f.Path, len(f.Lines), *f.Pinned)
	}
	nums := make([]string, len(f.Lines))
	for i, n := range f.Lines {
		nums[i] = strconv.Itoa(n)
	}
	return fmt.Sprintf("%s: %d working-memory citation(s) on line(s) %s; name the file separately from the prefix, write the segment as an angle-bracket placeholder, or name the bare directory",
		f.Path, len(f.Lines), strings.Join(nums, ", "))
}
