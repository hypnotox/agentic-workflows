package render

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

// SourceSpan is one ordered authored region and its template identity. Empty
// Source identifies renderer-generated or convention-part text, which must not
// acquire template provenance.
type SourceSpan struct {
	Source string
	Text   string
	// Provenance is the effective template identity emitted for surviving
	// rendered bytes. It is populated during assembly and excluded from the
	// authored-text projection used by template hashes.
	Provenance string
}

// SourceText retains the root template identity and each ordered authored span.
// Its AuthoredText projection is the byte-for-byte input used for template hashes.
type SourceText struct {
	Root  string
	Spans []SourceSpan
}

// AuthoredText flattens the ordered source spans without changing their bytes.
func (s SourceText) AuthoredText() string {
	var b strings.Builder
	for _, span := range s.Spans {
		b.WriteString(span.Text)
	}
	return b.String()
}

// slice retains source identity for the byte interval in AuthoredText.
func (s SourceText) slice(start, end int) SourceText {
	out := SourceText{Root: s.Root}
	offset := 0
	for _, span := range s.Spans {
		spanEnd := offset + len(span.Text)
		if start < spanEnd && end > offset {
			from, to := max(start, offset)-offset, min(end, spanEnd)-offset
			out.Spans = append(out.Spans, SourceSpan{Source: span.Source, Text: span.Text[from:to], Provenance: span.Provenance})
		}
		offset = spanEnd
	}
	return out
}

func (s *SourceText) appendText(source, text string) {
	s.appendProvenanceText(source, "", text)
}

func (s *SourceText) appendProvenanceText(source, provenance, text string) {
	if text != "" {
		s.Spans = append(s.Spans, SourceSpan{Source: source, Text: text, Provenance: provenance})
	}
}

func (s *SourceText) appendSource(other SourceText) {
	for _, span := range other.Spans {
		s.appendText(span.Source, span.Text)
	}
}

// includeRE matches an awf:include directive occupying its own line, capturing the
// partial name. The trailing `\n` is consumed so the splice preserves line structure:
// the directive line is replaced wholesale by the partial body (which ends in `\n`).
var includeRE = regexp.MustCompile(`(?m)^[ \t]*<!-- awf:include (\S+) -->[ \t]*\n`)

// ExpandIncludesSource replaces directives while retaining the root template and
// each included partial as distinct ordered spans.
func ExpandIncludesSource(src, root string, partialFS fs.FS) (SourceText, error) {
	locs := includeRE.FindAllStringSubmatchIndex(src, -1)
	if locs == nil {
		return SourceText{Root: root, Spans: []SourceSpan{{Source: root, Text: src}}}, nil
	}
	out := SourceText{Root: root}
	last := 0
	for _, m := range locs {
		name := src[m[2]:m[3]]
		body, err := fs.ReadFile(partialFS, "partials/"+name+".md")
		if err != nil {
			return SourceText{}, fmt.Errorf("awf:include: unknown partial %q", name)
		}
		if strings.Contains(string(body), "awf:include") {
			return SourceText{}, fmt.Errorf("awf:include: partial %q contains a nested include", name)
		}
		if strings.Contains(string(body), "awf:section") || strings.Contains(string(body), "awf:end") {
			return SourceText{}, fmt.Errorf("awf:include: partial %q contains a section marker", name)
		}
		out.appendText(root, src[last:m[0]])
		out.appendText("partials/"+name+".md", string(body))
		last = m[1]
	}
	out.appendText(root, src[last:])
	return out, nil
}

// ExpandIncludes replaces each directive with its partial body for callers
// whose contract is authored text rather than regional provenance.
func ExpandIncludes(src string, partialFS fs.FS) (string, error) {
	expanded, err := ExpandIncludesSource(src, "", partialFS)
	if err != nil {
		return "", err
	}
	return expanded.AuthoredText(), nil
}
