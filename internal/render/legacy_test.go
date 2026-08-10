package render

// ParseSections preserves concise legacy test setup while production callers use
// ParseSourceSections directly.
func ParseSections(src string, markdown ...bool) []Segment {
	return ParseSourceSections(SourceText{Spans: []SourceSpan{{Text: src}}}, markdown...)
}

// Assemble preserves concise legacy test assertions over the authored projection.
func Assemble(segs []Segment, plan map[string]SectionPlan, style CommentStyle) (string, map[string]string) {
	assembled, parts := AssembleSourceWithTemplateSource(segs, plan, style, TemplateSource{})
	return assembled.AuthoredText(), parts
}
