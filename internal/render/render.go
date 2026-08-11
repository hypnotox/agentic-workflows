// Package render parses awf section markers and renders templates with per-project overlays via text/template.
package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
)

// SectionPlan is the project layer's per-section resolution handed to Assemble.
// Exactly one of Drop / HasPart / (neither) holds: Drop omits the section,
// HasPart substitutes PartBody, neither renders the template default. EditPath is
// the project-relative convention part path named by the awf:edit pointer.
type SectionPlan struct {
	Drop     bool
	HasPart  bool
	PartBody string
	// PartStub marks a part body carrying the whole-line awf:stub marker -
	// declared-unauthored starter content (ADR-0070). Set by the project layer,
	// which reads part bodies; consumed by StubSections.
	PartStub bool
	// PartMarker marks a part whose raw, fence-excluded body carries a
	// whole-line section-marker residue (ADR-0083). Set by the project layer
	// over the on-disk bytes - pre placeholder substitution, whose multi-line
	// values must never create or mask a match; consumed part-keyed by the
	// marker advisory.
	PartMarker bool
	// PartVarRefs lists the config vars the raw part body consumes via
	// {{=awf:key}} placeholders (ADR-0086). Set by the project layer over
	// the on-disk bytes; consumed by the unused-var union, which cannot see
	// part bodies in the assembled source (they are sentinel-substituted raw).
	PartVarRefs []string
	EditPath    string
	// InPlace marks a section whose body the adopter edits directly in the
	// rendered output, preserved across syncs (ADR-0100). Mutually exclusive
	// with HasPart. InPlaceFound reports whether the section's region was located
	// in the existing output (its pointer was present); InPlaceBody is that
	// region's read-back content (possibly empty - an adopter may empty the
	// region). When InPlaceFound is false (first render / deleted pointer) the
	// template default renders instead; a found-but-empty region stays empty, so
	// emptying a region is not silently reverted to the default (ADR-0100 Decision 2).
	InPlace      bool
	InPlaceFound bool
	InPlaceBody  string
}

// CommentStyle is the comment syntax a rendered target uses for the surviving
// awf:edit-family provenance pointers. Because the pointers survive into output as
// comments, they must be valid comments in the target's language (ADR-0100
// Decision 7): a `#`-line comment for a `#!`-shebang target such as a shell script,
// an HTML comment otherwise. The zero value is HTMLComment, the historical default.
type CommentStyle int

const (
	HTMLComment  CommentStyle = iota // <!-- <text> -->
	HashComment                      // # <text>
	SlashComment                     // // <text>
)

// CommentStyleForSource picks the pointer comment style for a target from its
// (expanded) template source, by the same `#!`-shebang sniff injectBanner uses so
// the pointer emitter and the read-back matcher derive the style identically and
// cannot diverge (ADR-0100 Decision 7).
func CommentStyleForSource(src string) CommentStyle {
	if strings.HasPrefix(src, "#!") {
		return HashComment
	}
	return HTMLComment
}

// wrap renders inner as a one-line comment in this style, trailing newline included.
func (style CommentStyle) wrap(inner string) string {
	if style == HashComment {
		return "# " + inner + "\n"
	}
	if style == SlashComment {
		return "// " + inner + "\n"
	}
	return "<!-- " + inner + " -->\n"
}

// open is the comment opener this style prefixes a pointer line with.
func (style CommentStyle) open() string {
	if style == HashComment {
		return "# "
	}
	if style == SlashComment {
		return "// "
	}
	return "<!-- "
}

// PointerLinePrefixes returns the awf:edit-family pointer line prefixes (the
// awf:edit and awf:edit-in-place variants) for a section named `name` in the given
// comment style, up to and including the `: ` separator. Every editPointer variant
// emits `<open>awf:edit[-in-place] <name>: ...`, so a trimmed output line is that
// section's pointer iff it begins with one of these prefixes. Read-back matches a
// region boundary by these exact per-section strings, never a generic
// pointer shape, so adopter text resembling a pointer for a non-registered name
// cannot bound a region (ADR-0100 Decision 2 / in-place-readback).
func PointerLinePrefixes(name string, style CommentStyle) []string {
	o := style.open()
	return []string{
		o + "awf:edit " + name + ": ",
		o + "awf:edit-in-place " + name + ": ",
	}
}

// editPointer is the awf:edit provenance comment emitted before a section body,
// in the target's CommentStyle (ADR-0100 Decision 7). A stub-attributed section
// rendering its template default gets a distinct pointer so the rendered file
// itself distinguishes a must-replace default from a valid one (ADR-0070). An
// in-place section gets a distinct `awf:edit-in-place` pointer whose token is
// deliberately not awf:section/awf:end-shaped, so it survives into the shipped
// output (unlike structural markers) and bounds the read-back region without
// tripping the residual-marker guards (ADR-0100). Only the comment delimiters
// vary by style; the token and phrasing are constant.
// touches-state: rendering/render-engine:section-edit-pointer - awf:edit provenance pointer emission; proof in render_test.go
func editPointer(name string, stub bool, headed bool, p SectionPlan, style CommentStyle) string {
	switch {
	case p.InPlace && headed:
		return style.wrap(fmt.Sprintf("awf:edit-in-place %s: the heading immediately below is awf-owned; only the body after it is preserved across syncs", name))
	case p.InPlace:
		return style.wrap(fmt.Sprintf("awf:edit-in-place %s: your edits below are preserved across syncs; awf owns the rest", name))
	case p.HasPart:
		return style.wrap(fmt.Sprintf("awf:edit %s: from %s", name, p.EditPath))
	case stub:
		return style.wrap(fmt.Sprintf("awf:edit %s: stub; replace by creating %s", name, p.EditPath))
	default:
		return style.wrap(fmt.Sprintf("awf:edit %s: default; create %s to override", name, p.EditPath))
	}
}

// partSentinel is the brace-free, NUL-delimited placeholder emitted in a part's
// slot. NUL bytes cannot occur in template or markdown text, so the token can
// never collide with rendered content, and being brace-free it is inert to the
// template parser.
func partSentinel(name string) string {
	return "\x00awf:part:" + name + "\x00"
}

// SectionDefaultSentinel is the brace-free, NUL-delimited token the project layer
// substitutes for the {{=awf:sectionDefault}} placeholder (ADR-0072). Assemble splits
// a part body at each occurrence and splices the section's raw default source between
// the verbatim fragments, so Execute renders the default in place. Brace-free (inert to
// the template parser) and NUL-delimited (cannot collide with template or markdown text).
const SectionDefaultSentinel = "\x00awf:section-default\x00"

// TemplateSource enables renderer-owned regional template provenance. Root is a
// normalized repository-relative directory; an empty Root preserves historical
// output exactly.
type TemplateSource struct{ Root string }

// AssembleSourceWithTemplateSource applies section assembly and emits source
// transitions when template provenance is enabled.
// touches-state: rendering/render-engine:no-section-marker-leak - structural directives remain excluded while template-source markers are renderer-owned; proof in render_test.go
// touches-state: rendering/render-engine:template-source-symbol - root, include, return, and structural-section marker emission; proof in render_test.go and project/template_source_marker_test.go
func AssembleSourceWithTemplateSource(segs []Segment, plan map[string]SectionPlan, style CommentStyle, provenance TemplateSource) (SourceText, map[string]string) {
	var out SourceText
	if len(segs) > 0 {
		out.Root = segs[0].Source.Root
	}
	parts := map[string]string{}
	appendSource := func(source SourceText) {
		for _, span := range source.Spans {
			out.appendProvenanceText(span.Source, span.Source, span.Text)
		}
	}
	for _, s := range segs {
		if !s.IsSection {
			appendSource(s.Source)
			continue
		}
		p := plan[s.Name]
		if p.Drop {
			continue
		}
		out.Root = s.Source.Root
		out.appendProvenanceText(s.SectionSource, s.SectionSource+"#"+s.Name, editPointer(s.Name, s.Stub, s.Heading != "", p, style))
		if s.Heading != "" {
			appendSource(s.HeadingSource)
			if !strings.HasSuffix(s.HeadingSource.AuthoredText(), "\n") {
				out.appendText(s.SectionSource, "\n")
			}
		}
		switch {
		case p.InPlace:
			// A located region is adopter-owned raw text. On first render the
			// recorded default supplies its bytes, but the structural section marker
			// is its only provenance: nested include transitions would become part
			// of the later adopter-owned read-back region.
			if p.InPlaceFound {
				out.appendText("", p.InPlaceBody)
			} else {
				out.appendText(s.SectionSource, s.Source.AuthoredText())
			}
		case p.HasPart:
			writePartBodySource(&out, parts, s, p, appendSource)
		default:
			appendSource(s.Source)
		}
	}
	return out, parts
}

// writePartBodySource emits a section's part into the skeleton. When the part
// re-injects its default via the sectionDefault split marker (ADR-0072), it is
// split into raw fragments interleaved with the recorded default source spans.
// A part without the marker emits one provenance-free sentinel for its raw body.
func writePartBodySource(out *SourceText, parts map[string]string, s Segment, p SectionPlan, appendSource func(SourceText)) {
	if !strings.Contains(p.PartBody, SectionDefaultSentinel) {
		sent := partSentinel(s.Name)
		parts[sent] = p.PartBody
		out.appendText("", sent)
		return
	}
	for i, frag := range strings.Split(p.PartBody, SectionDefaultSentinel) {
		if i > 0 {
			// Re-injection reuses the recorded default spans rather than a
			// flattened reconstruction, retaining partial transitions exactly.
			appendSource(s.Source)
		}
		// The index separator is a NUL, which can never occur in a section name
		// (template source is text): it guarantees a fragment sentinel can never
		// equal a plain part sentinel of some other section, whatever its name.
		sent := partSentinel(s.Name + "\x00" + strconv.Itoa(i))
		parts[sent] = frag
		out.appendText("", sent)
	}
}

// StubSections reports a parsed template's unauthored stub content under a plan
// (ADR-0070): defaults = stub-attributed sections rendering their template
// default; parts = sections whose convention part carries the awf:stub marker.
// Dropped sections report nothing.
func StubSections(segs []Segment, plan map[string]SectionPlan) (defaults, parts []string) {
	for _, s := range segs {
		if !s.IsSection {
			continue
		}
		p := plan[s.Name]
		switch {
		case p.Drop:
		case p.HasPart && p.PartStub:
			parts = append(parts, s.Name)
		case !p.HasPart && s.Stub:
			defaults = append(defaults, s.Name)
		}
	}
	return defaults, parts
}

// CheckSectionDefaultStubs hard-errors when a part re-injects the default of a
// stub-attributed section (ADR-0072 Decision 4): a stub default is an authoring prompt,
// not shippable prose, so there is nothing valid to re-inject and the section must stay
// in must-author state. Runs pre-Assemble on the same segs+plan StubSections consumes;
// it scans the substituted part body for the render-layer sentinel, since planSections
// has already replaced the {{=awf:sectionDefault}} token.
func CheckSectionDefaultStubs(segs []Segment, plan map[string]SectionPlan) error {
	for _, s := range segs {
		if !s.IsSection {
			continue
		}
		p := plan[s.Name]
		if s.Stub && p.HasPart && strings.Contains(p.PartBody, SectionDefaultSentinel) {
			return fmt.Errorf("section %q re-injects a stub default via {{=awf:sectionDefault}}: a stub default is an authoring prompt, not shippable prose; author the part instead", s.Name)
		}
	}
	return nil
}

// StructuralHeadingCapture returns a marker-free copy of the complete template
// skeleton with each structural heading bracketed by inert tokens. Executing this
// source preserves the template parse tree's surrounding variables, dot, and
// control flow while exposing the rendered heading lines to the project layer.
func StructuralHeadingCapture(segs []Segment) (string, map[string][2]string) {
	var b strings.Builder
	tokens := make(map[string][2]string)
	for _, s := range segs {
		if !s.IsSection {
			b.WriteString(s.Text)
			continue
		}
		if s.Heading != "" {
			// A per-section digest makes framing practically unique to this exact
			// skeleton. Extraction still rejects every duplicate or malformed
			// occurrence rather than trusting that uniqueness.
			digest := sha256.Sum256([]byte(s.Name + "\x00" + s.Heading))
			prefix := "\x00awf:heading:" + hex.EncodeToString(digest[:])
			start := prefix + ":start\x00"
			end := prefix + ":end\x00"
			tokens[s.Name] = [2]string{start, end}
			b.WriteString(start)
			b.WriteString(s.Heading)
			b.WriteString(end)
		}
		b.WriteString(s.Text)
	}
	return b.String(), tokens
}

// ExtractStructuralHeadings recovers each heading captured during execution.
func ExtractStructuralHeadings(output string, tokens map[string][2]string) (map[string]string, error) {
	headings := make(map[string]string, len(tokens))
	for _, name := range slices.Sorted(maps.Keys(tokens)) {
		pair := tokens[name]
		starts := strings.Count(output, pair[0])
		ends := strings.Count(output, pair[1])
		if starts == 0 && ends == 0 {
			continue
		}
		if starts != 1 || ends != 1 {
			return nil, fmt.Errorf("structural heading %q capture has ambiguous framing: found %d start token(s) and %d end token(s)", name, starts, ends)
		}
		start := strings.Index(output, pair[0])
		end := strings.Index(output, pair[1])
		if end < start+len(pair[0]) {
			return nil, fmt.Errorf("structural heading %q capture framing is out of order", name)
		}
		headings[name] = output[start+len(pair[0]) : end]
	}
	return headings, nil
}

// Execute runs text/template over the awf-owned skeleton (part bodies stood in by
// sentinels) under missingkey=zero, then restores each raw part body verbatim - so
// a convention part is never parsed or executed as a template. name labels parse
// and execute errors with the target rather than a hardcoded literal.
// touches-state: rendering/render-engine:parts-raw-except-authoring-comments - part bodies restored verbatim post-strip, never templated; proof in render_test.go
func Execute(assembled string, data map[string]any, parts map[string]string, name string) (string, error) {
	return execute(assembled, data, parts, name, nil)
}

// ExecuteSourceWithTemplateSource executes an assembled source while deriving
// markers from the bytes that survive template control flow. Instrumenting the
// parsed tree preserves controls that cross source-span and section boundaries.
func ExecuteSourceWithTemplateSource(assembled SourceText, data map[string]any, parts map[string]string, name string, provenance TemplateSource) (string, error) {
	return execute(assembled.AuthoredText(), data, parts, name, func(t *template.Template) func(string) string {
		identities := map[string]string{}
		instrumentTemplateSources(t, assembled, identities)
		return func(rendered string) string {
			return renderTemplateSourceMarkers(rendered, identities, provenance.Root)
		}
	})
}

func execute(assembled string, data map[string]any, parts map[string]string, name string, instrument func(*template.Template) func(string) string) (string, error) {
	t, err := template.New(name).Option("missingkey=zero").Parse(assembled)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var postprocess func(string) string
	if instrument != nil {
		postprocess = instrument(t)
	}
	var out strings.Builder
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	rendered := out.String()
	for sent, body := range parts {
		rendered = strings.ReplaceAll(rendered, sent, body)
	}
	if postprocess != nil {
		rendered = postprocess(rendered)
	}
	return rendered, nil
}

const templateSourceTokenPrefix = "\x00awf:template-source:"

type sourceInterval struct {
	start, end int
	identity   string
}

func instrumentTemplateSources(t *template.Template, assembled SourceText, identities map[string]string) {
	intervals := make([]sourceInterval, 0, len(assembled.Spans))
	offset := 0
	for _, span := range assembled.Spans {
		intervals = append(intervals, sourceInterval{start: offset, end: offset + len(span.Text), identity: span.Provenance})
		offset += len(span.Text)
	}
	nextToken := 0
	token := func(identity string, pos parse.Pos) *parse.TextNode {
		nextToken++
		value := templateSourceTokenPrefix + strconv.Itoa(nextToken) + "\x00"
		identities[value] = identity
		return &parse.TextNode{NodeType: parse.NodeText, Pos: pos, Text: []byte(value)}
	}
	identityAt := func(pos int) string {
		identity := ""
		for _, interval := range intervals {
			if pos >= interval.start && pos < interval.end {
				identity = interval.identity
				break
			}
		}
		return identity
	}
	var instrumentList func(*parse.ListNode)
	instrumentList = func(list *parse.ListNode) {
		if list == nil {
			return
		}
		nodes := make([]parse.Node, 0, len(list.Nodes)*2)
		for _, node := range list.Nodes {
			switch n := node.(type) {
			case *parse.TextNode:
				start := int(n.Position())
				end := start + len(n.Text)
				cursor := start
				for _, interval := range intervals {
					from, to := max(cursor, interval.start), min(end, interval.end)
					if from >= to {
						continue
					}
					nodes = append(nodes, token(interval.identity, parse.Pos(from)))
					nodes = append(nodes, &parse.TextNode{NodeType: parse.NodeText, Pos: parse.Pos(from), Text: append([]byte(nil), n.Text[from-start:to-start]...)})
					cursor = to
				}
				// Source intervals cover AuthoredText exactly; partitioning therefore
				// consumes every retained TextNode byte, including after trim actions.
				continue
			case *parse.ActionNode, *parse.TemplateNode:
				nodes = append(nodes, token(identityAt(int(node.Position())), node.Position()))
			case *parse.IfNode:
				instrumentList(n.List)
				instrumentList(n.ElseList)
			case *parse.RangeNode:
				instrumentList(n.List)
				instrumentList(n.ElseList)
			case *parse.WithNode:
				instrumentList(n.List)
				instrumentList(n.ElseList)
			}
			nodes = append(nodes, node)
		}
		list.Nodes = nodes
	}
	for _, tmpl := range t.Templates() {
		instrumentList(tmpl.Root)
	}
}

func renderTemplateSourceMarkers(rendered string, tokenIdentities map[string]string, root string) string {
	var out strings.Builder
	lastIdentity := ""
	pendingIdentity := ""
	cursor := 0
	emit := func(content string) {
		if strings.TrimSpace(content) != "" {
			if pendingIdentity == "" {
				lastIdentity = ""
			} else if pendingIdentity != lastIdentity {
				out.WriteString("<!-- awf:template-source ")
				out.WriteString(strings.TrimSuffix(root, "/") + "/" + pendingIdentity)
				out.WriteString(" -->\n")
				lastIdentity = pendingIdentity
			}
		}
		out.WriteString(content)
	}
	for {
		start := strings.Index(rendered[cursor:], templateSourceTokenPrefix)
		if start < 0 {
			emit(rendered[cursor:])
			return out.String()
		}
		start += cursor
		emit(rendered[cursor:start])
		// Tokens are renderer-created NUL-delimited values; authored template,
		// part, and in-place text cannot contain NUL bytes.
		endOffset := strings.IndexByte(rendered[start+len(templateSourceTokenPrefix):], 0)
		end := start + len(templateSourceTokenPrefix) + endOffset + 1
		pendingIdentity = tokenIdentities[rendered[start:end]]
		cursor = end
	}
}
