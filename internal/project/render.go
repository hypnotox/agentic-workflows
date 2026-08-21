package project

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
)

// runnerSections is the pure awf wrapper's single declared section: the body
// that resolves the bootstrap-pinned binary first, then PATH awf, and execs it
// with all arguments forwarded verbatim.
var runnerSections = []string{"runner-body"}

// hookNames are the git-hook payload scripts the hooks singleton renders as a
// unit under .awf/hooks/ (ADR-0048); each name resolves its template id
// through hookTID.
var hookNames = []string{"pre-commit", "commit-msg", "pre-push", "pre-merge-commit", "reference-transaction"}

type RenderedFile struct {
	Path         string
	Content      string
	TemplateID   string
	TemplateHash string
	ConfigHash   string
	// RegenChecked excludes this file from the frozen-OutputHash compare; its
	// drift is checked by regeneration instead (ADR-0100). Set on the generated
	// indexes and on any file carrying an in-place-editable section.
	RegenChecked bool
	// Policy declares all lifecycle checks for this path. It replaces
	// template-name and filename inference at plan consumers.
	Policy OutputPolicy
	// Declarer identifies the producer requesting this output.
	Declarer           string
	DeclarerProjection string
	Encoder            AgentDialect
	Provenance         render.CommentStyle
	// assembled is the executed template source (post section-overlay, pre
	// execution); unsetVarNotes scans it for referenced-but-unset vars (ADR-0045).
	assembled string
	// stubDefaults / stubParts feed the ADR-0070 unauthored-content advisory:
	// stub-attributed sections rendered at default, and convention parts
	// carrying the awf:stub marker. Consumed path-keyed by stubNotes.
	stubDefaults []string
	stubParts    []string
	// markerParts feeds the ADR-0083 part-marker advisory: the part paths
	// (EditPath) whose raw bodies carry a whole-line section-marker residue.
	// Consumed part-keyed and deduplicated by markerNotes.
	markerParts []string
	// kind/artifact identify the rendered artifact for the per-artifact
	// unused-data check; partVarRefs carries the part-placeholder var
	// consumption the assembled source cannot show (both ADR-0086).
	kind, artifact string
	partVarRefs    []string
	// ConsumedInputs is observed at the render seam, independently of
	// BuildOutputDeclarations: the context artifact report reads the observed
	// set, so a declaration omission or role mistake shows there rather than
	// being papered over by the declaration it was derived from.
	ConsumedInputs     []OutputInput
	ObservedTemplateID string
}

// data assembles the template data namespace for a target: the prefix, the
// project vars, the sidecar's structured data, and the awf-given docs layout.
func projectData(p renderInputs, sc config.Sidecar, eff map[string]bool) map[string]any {
	return map[string]any{
		"prefix":       p.cfg.Prefix,
		"profile":      string(p.cfg.Profile),
		"fullProfile":  fullProfile(p),
		"vars":         nonNil(p.cfg.Vars),
		"data":         nonNil(sc.Data),
		"layout":       layout(p).templateMap(),
		"version":      Version,
		"adrFormat":    adr.CurrentFormatMarker(),
		"skills":       eff,
		"commitScopes": commitScopesDisplay(p),
		// commitPolicy is a typed projection, not reparsed YAML. A nil value is
		// safe for publication templates: `with` and `if` treat it as absent.
		"commitPolicy":  p.cfg.CommitPolicy,
		"gatedCommands": gatedCommandsDisplay(),
		// Project-level session-handoff signal for the neutral (guide/singleton
		// doc) render; per-target renders overwrite it from targetTemplateData
		// (ADR-0157 Decision 6).
		"targetSessionHandoff": anyTargetHasCapability(p.targets(), CapabilitySessionHandoff),
	}
}

// commitScopesDisplay returns the display-formatted allowed commit-scope list
// (e.g. "`adr`, `awf`, `plans`") resolved from audit.allowedScopes - the same
// audit.Resolve path awf check staged commit reads, so prose and gate agree by
// construction - or "" when scopes are accept-any (ADR-0051).
func commitScopesDisplay(p renderInputs) string {
	scopes := audit.Resolve(config.AuditScopes(p.cfg.Audit)).AllowedScopes
	if len(scopes) == 0 {
		return ""
	}
	quoted := make([]string, len(scopes))
	for i, s := range scopes {
		quoted[i] = "`" + s.Name + "`"
	}
	return strings.Join(quoted, ", ")
}

// effectiveSkills returns the unconditional full catalog skill set.
func effectiveSkills(p renderInputs) (map[string]bool, error) {
	eff := map[string]bool{}
	for name := range projectCatalog(p).Skills {
		if _, err := p.cfg.Sidecar("skills", name); err != nil { // coverage-ignore: declaration-first planning just parsed this catalog skill sidecar
			return nil, err
		}
		eff[name] = true
	}
	return eff, nil
}

// partRel is the project-relative convention part path the awf:edit pointer names,
// derived from the absolute PartPath so the parts-path structure has one source.
func partRel(p renderInputs, kind, artifact, section string) string {
	rel, err := filepath.Rel(p.root(), p.cfg.PartPath(kind, artifact, section))
	if err != nil { // coverage-ignore: PartPath is always rooted under p.root(), so Rel cannot fail
		return ""
	}
	return filepath.ToSlash(rel)
}

// planSections resolves each catalog-declared section into a render.SectionPlan:
// a sidecar drop wins; an in-place-editable section (declared by the template's
// `inplace` marker) is sourced by reading its body back from the existing output;
// otherwise an existing convention part substitutes its body; otherwise the
// template default renders. Precedence: drop > in-place read-back > convention
// part > default. In-place and part sourcing are mutually exclusive per section
// (ADR-0100 section-source-exclusive).
func planSections(p renderInputs, kind, artifact string, declared []string, sec map[string]config.SectionOverride, segs []render.Segment, outPath string, style render.CommentStyle, expectedHeadings ...map[string]string) (map[string]render.SectionPlan, error) {
	headings := map[string]string{}
	if len(expectedHeadings) > 0 && expectedHeadings[0] != nil {
		headings = expectedHeadings[0]
	}
	plan := map[string]render.SectionPlan{}
	reg, err := placeholderRegistry(p)
	if err != nil {
		return nil, err
	}
	inPlace := map[string]bool{}
	for _, s := range segs {
		if s.IsSection && s.InPlace {
			inPlace[s.Name] = true
		}
	}
	// The existing output is read at most once, lazily, and only when the template
	// actually declares an in-place section - every other artifact avoids the read.
	var output string
	outputRead := false
	readOutput := func() (string, error) {
		if !outputRead {
			b, ok, err := projectTreeReader(p).ReadFile(outPath)
			if err != nil {
				return "", err
			}
			if !ok {
				b = nil
			}
			output, outputRead = string(b), true // "" when absent (first render)
		}
		return output, nil
	}
	for _, s := range declared {
		sp := render.SectionPlan{EditPath: partRel(p, kind, artifact, s)}
		if ov, ok := sec[s]; ok && ov.Drop {
			sp.Drop = true
			plan[s] = sp
			continue
		}
		if inPlace[s] {
			// section-source-exclusive: an in-place section must not also carry a
			// convention part - the two override channels are mutually exclusive.
			if _, exists, partErr := p.cfg.ReadPart(kind, artifact, s); partErr != nil {
				return nil, partErr
			} else if exists {
				return nil, fmt.Errorf("section %q is in-place-editable and must not also have a convention part at %s (ADR-0100)", s, partRel(p, kind, artifact, s))
			}
			out, readErr := readOutput()
			if readErr != nil {
				return nil, fmt.Errorf("read output %s: %w", outPath, readErr)
			}
			sp.InPlace = true
			// A located region (its pointer present) is used verbatim even when
			// empty; only an unlocated region falls back to the template default
			// in Assemble (ADR-0100 in-place-readback).
			sp.InPlaceBody, sp.InPlaceFound = readBackInPlaceBodyWithExpectations(out, s, declared, style, headings[s], templateSourceSectionMarkers(segs, templateSourceRoot(p)))
			plan[s] = sp
			continue
		}
		b, exists, err := p.cfg.ReadPart(kind, artifact, s)
		if err != nil { // coverage-ignore: declaration-first output planning already parses this consumed part in the same operation
			return nil, err
		}
		if exists {
			// Stripped before substitution (ADR-0121 Decision 2): a substituted
			// value can never create or mask a whole-line directive, and an
			// unknown placeholder demonstrated inside a comment must not error.
			raw, serr := render.StripAuthoringComments(string(b))
			if serr != nil {
				return nil, fmt.Errorf("part %s: %w", partRel(p, kind, artifact, s), serr)
			}
			body, serr := substitutePlaceholders(partRel(p, kind, artifact, s), raw, reg)
			if serr != nil {
				return nil, serr
			}
			sp.HasPart = true
			sp.PartBody = body
			sp.PartStub = render.HasStubMarker(body)
			// Scanned on the stripped pre-substitution bytes (ADR-0083 Decision 4's
			// raw-bytes contract preserved in effect - the strip cannot add or
			// remove a marker-shaped line; ADR-0121), fenced examples excluded.
			sp.PartMarker = render.HasMarkerLine(refs.WithoutFences(raw))
			sp.PartVarRefs = render.PlaceholderVarRefs(raw)
		}
		plan[s] = sp
	}
	return plan, nil
}

// readBackInPlaceBodyWithExpectations extracts the current body of the in-place section `name`
// from the existing rendered `output`. The region runs from just after `name`'s
// awf:edit-in-place pointer line to the first later line that is the pointer of
// any *other* registered (declared) section - matched by that section's expected
// pointer prefix in the target's comment style, never a generic pointer shape, so
// a pointer-shaped line for a non-registered name in adopter text cannot truncate
// it - or end-of-file when none follows. Leading/trailing blank lines (awf-owned
// framing) are trimmed; the interior, including internal blank lines, is returned
// verbatim. Returns ("", false) when `name`'s own pointer is absent (first render
// or a deleted anchor), so the caller falls back to the template default.
// touches-state: rendering/inplace-and-placeholders:in-place-readback - read-back between the section pointer and awf's next registered pointer; proof in inplace_test.go
// touches-state: rendering/inplace-and-placeholders:in-place-spacing-owned - verbatim interior, trimmed framing; proof in inplace_test.go
func readBackInPlaceBodyWithExpectations(output, name string, declared []string, style render.CommentStyle, expectedHeading string, expectedSymbols map[string]string) (string, bool) {
	lines := strings.Split(output, "\n")
	ownPrefixes := render.PointerLinePrefixes(name, style)
	start := -1
	for i, ln := range lines {
		if hasAnyPrefix(strings.TrimSpace(ln), ownPrefixes) {
			start = i
			break
		}
	}
	if start < 0 {
		return "", false
	}
	var boundaryPrefixes []string
	for _, d := range declared {
		if d != name {
			boundaryPrefixes = append(boundaryPrefixes, render.PointerLinePrefixes(d, style)...)
		}
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if hasAnyPrefix(strings.TrimSpace(lines[i]), boundaryPrefixes) {
			end = i
			// Only the exact renderer-owned symbol for this registered next
			// section is framing. Lookalikes remain adopter body content.
			if i > start+1 {
				for _, d := range declared {
					if d != name && hasAnyPrefix(strings.TrimSpace(lines[i]), render.PointerLinePrefixes(d, style)) && strings.TrimSpace(lines[i-1]) == expectedSymbols[d] {
						end = i - 1
						break
					}
				}
			}
			break
		}
	}
	body := lines[start+1 : end]
	if expectedHeading != "" && len(body) > 0 {
		// A structural slot is awf-owned. Any ATX heading occupying it is tamper,
		// regardless of level; a body heading is preserved only when that slot is
		// genuinely absent.
		if body[0] == expectedHeading || atxHeadingLine(strings.TrimSpace(body[0])) {
			body = body[1:]
		}
	}
	return trimBlankFraming(body), true
}

func templateSourceSectionMarkers(segs []render.Segment, root string) map[string]string {
	markers := map[string]string{}
	if root == "" {
		return markers
	}
	for _, seg := range segs {
		if seg.IsSection {
			markers[seg.Name] = "<!-- awf:template-source " + path.Join(root, seg.SectionSource) + "#" + seg.Name + " -->"
		}
	}
	return markers
}

func templateSourceRoot(p renderInputs) string {
	if p.cfg.Render == nil {
		return ""
	}
	return p.cfg.Render.TemplateSourceRoot
}

// touches-state: rendering/render-engine:template-source-symbol - configured source mapping for Markdown producers outside renderTarget; proof in template_source_marker_test.go
// templateSourceRootMarker is the project-owned bridge for Markdown producers
// that execute their template outside renderTarget. It keeps their configured
// source mapping, validation, and observed inputs identical to ordinary renders.
func templateSourceRootMarker(p renderInputs, tid string) (string, []OutputInput, error) {
	root := templateSourceRoot(p)
	if root == "" || tid == "" {
		return "", nil, nil
	}
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return "", nil, fmt.Errorf("read template %s: %w", tid, err)
	}
	expanded, err := render.ExpandIncludesSource(string(src), tid, templates.FS)
	if err != nil { // coverage-ignore: live embedded templates are include-validated; render package tests own malformed expansion
		return "", nil, fmt.Errorf("render %s: %w", tid, err)
	}
	if err := validateTemplateSources(p, expanded, root); err != nil {
		return "", nil, fmt.Errorf("render %s: %w", tid, err)
	}
	inputs := []OutputInput{}
	seen := map[string]bool{}
	for _, span := range expanded.Spans {
		if span.Source != "" && !seen[span.Source] {
			seen[span.Source] = true
			inputs = append(inputs, OutputInput{Path: path.Join(root, span.Source), Role: ArtifactTemplate})
		}
	}
	return "<!-- awf:template-source " + path.Join(root, tid) + " -->\n", normalizeOutputInputs(inputs), nil
}

func templateSourceConfigHash(hash, root string) string {
	if root == "" {
		return hash
	}
	return manifest.Hash([]byte(hash + "\x00templateSourceRoot=" + root))
}

func atxHeadingLine(s string) bool {
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	return i > 0 && i <= 6 && i < len(s) && (s[i] == ' ' || s[i] == '\t')
}

// trimBlankFraming drops leading and trailing blank (whitespace-only) lines - the
// awf-owned framing - and returns the interior lines joined verbatim.
func trimBlankFraming(lines []string) string {
	lo, hi := 0, len(lines)
	for lo < hi && strings.TrimSpace(lines[lo]) == "" {
		lo++
	}
	for hi > lo && strings.TrimSpace(lines[hi-1]) == "" {
		hi--
	}
	return strings.Join(lines[lo:hi], "\n")
}

// anyInPlace reports whether a section plan contains an in-place-editable section -
// the property that makes a rendered file regeneration-checked (ADR-0100).
func anyInPlace(plan map[string]render.SectionPlan) bool {
	for _, sp := range plan {
		if sp.InPlace {
			return true
		}
	}
	return false
}

// hasAnyPrefix reports whether s begins with any of the given prefixes.
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// renderKindSpec drives one catalog-backed render loop (skills/agents/docs): the
// kinds that share the sort → sidecar → render → append shape. tid
// and sections derive from the artifact name; outPath also takes the adapter
// target (ignored by neutral kinds like docs); target is the adapter this pass
// renders for (zero for neutral kinds).
type renderOutputOptions struct {
	encode      func(string) (string, error)
	bannerStyle render.CommentStyle
	// sources are producer-selected reader guidance, separate from machine inputs.
	sources []string
	target  *Target
	// encoder is the output node's declared representation policy. Structural
	// heading parsing follows it rather than target identity or filename shape.
	encoder AgentDialect
}

type renderKindSpec struct {
	kind     string
	names    []string
	target   Target
	tid      func(name string) string
	sections func(name string) []string
	outPath  func(t Target, name string) string
	// defaults returns the artifact's catalog default data (nil = none).
	defaults func(name string) map[string]any
	// transform computes sidecar data into rendered content after the defaults
	// merge, upstream of BOTH renderTarget and artifactConfigHash so the
	// computation participates in the drift signal (ADR-0089; nil = none).
	transform func(name string, sc config.Sidecar) (config.Sidecar, error)
	// encode projects the rendered instruction body into an output dialect before
	// provenance injection (nil leaves ordinary skill/doc rendering unchanged).
	encode func(name, body string, data map[string]any) (string, error)
	// sources supplies compact reader-facing provenance for this producer only.
	sources func(name string) []string
}

// skillTID resolves a catalog skill's name-derived template id.
func skillTID(p renderInputs, n string) string {
	return mustDescriptor("skills").templateID(projectCatalog(p), n)
}

// agentTID resolves a catalog agent's name-derived template id.
func agentTID(p renderInputs, n string) string {
	return mustDescriptor("agents").templateID(projectCatalog(p), n)
}

// docTID resolves a catalog document's declared template id.
func docTID(p renderInputs, n string) string { return projectCatalog(p).Docs[n].TID }

func renderKind(p renderInputs, spec renderKindSpec, eff map[string]bool) ([]RenderedFile, error) {
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(spec.names)) {
		sc, err := p.cfg.Sidecar(spec.kind, name)
		if err != nil { // coverage-ignore: declaration-first planning just parsed this enabled artifact sidecar
			return nil, err
		}
		if spec.defaults != nil {
			sc = withDefaultData(sc, spec.defaults(name), specializedListDataKeys(spec.kind, name)...)
		}
		if spec.transform != nil {
			if sc, err = spec.transform(name, sc); err != nil {
				return nil, err
			}
		}
		data := projectData(p, sc, eff)
		var options *renderOutputOptions
		if spec.target.Name != "" || spec.sources != nil {
			options = &renderOutputOptions{}
		}
		if spec.sources != nil {
			options.sources = spec.sources(name)
		}
		if spec.target.Name != "" {
			for key, value := range spec.target.targetTemplateData() {
				data[key] = value
			}
			target := spec.target
			sources := options.sources
			options = &renderOutputOptions{bannerStyle: render.HTMLComment, target: &target, encoder: MarkdownAgentDialect}
			options.sources = sources
		}
		if spec.encode != nil {
			options.bannerStyle = spec.target.agentCommentStyle()
			options.encoder = spec.target.AgentDialect
			options.encode = func(body string) (string, error) { return spec.encode(name, body, data) }
		}
		rf, err := renderTarget(p, spec.kind, name, spec.tid(name), spec.sections(name), sc, data, spec.outPath(spec.target, name), eff, options)
		if err != nil {
			return nil, err
		}
		if spec.target.Name != "" {
			rf.Declarer = spec.target.Name
			rf.DeclarerProjection = targetDescriptorProjection(spec.target)
			rf.Provenance = options.bannerStyle
			if spec.encode != nil {
				rf.Encoder = spec.target.AgentDialect
			} else {
				rf.Encoder = MarkdownAgentDialect
			}
		} else {
			rf.Declarer, rf.DeclarerProjection, rf.Encoder, rf.Provenance = rf.TemplateID, rf.TemplateID, MarkdownAgentDialect, render.HTMLComment
		}
		out = append(out, rf)
	}
	return out, nil
}

// renderAllBase renders declarative catalog and singleton producers. OutputPlan
// owns the public render/sync/check lifecycle and adds generated producers.
func renderAllBase(p renderInputs, targetOutputs map[string]targetOutputDeclaration, eff map[string]bool, pitfalls pitfall.Corpus) ([]RenderedFile, error) {
	var out []RenderedFile
	// Neutral: docs render once - the output path is docsDir-relative, not adapter-placed.
	docsRfs, err := renderKind(p, renderKindSpec{
		kind: "docs", names: catalog.NameDerivedDocNames(projectCatalog(p)),
		tid:      func(n string) string { return docTID(p, n) },
		sections: func(n string) []string { return projectCatalog(p).Docs[n].Sections },
		outPath:  func(_ Target, n string) string { return docOutPath(p, n) },
		defaults: func(n string) map[string]any { return projectCatalog(p).Docs[n].Data },
		transform: func(n string, sc config.Sidecar) (config.Sidecar, error) {
			if n == "pitfalls" {
				return pitfallIndexSidecar(sc, pitfalls), nil
			}
			return docDataTransform(n, sc)
		},
		sources: func(n string) []string {
			switch n {
			case "glossary":
				return []string{".awf/docs/glossary.yaml", "derived:awf-standard-vocabulary"}
			case "pitfalls":
				return []string{".awf/docs/pitfalls/*.md"}
			}
			return nil
		},
	}, eff)
	if err != nil {
		return nil, err
	}
	for i := range docsRfs {
		if docsRfs[i].TemplateID == docTID(p, "pitfalls") {
			for _, source := range pitfallSourcePaths(pitfalls) {
				docsRfs[i].ConsumedInputs = append(docsRfs[i].ConsumedInputs, OutputInput{Path: source, Role: ArtifactAuthoredData})
			}
			docsRfs[i].ConsumedInputs = normalizeOutputInputs(docsRfs[i].ConsumedInputs)
		}
	}
	out = append(out, docsRfs...)
	// Adapter: skills + agents render once per fixed target (inv: multi-target-render).
	// touches-state: rendering/project-output-plan:multi-target-render - skills/agents render once per fixed target; proof in target_test.go
	for _, t := range p.targets() {
		skillNames := slices.Sorted(maps.Keys(projectCatalog(p).Skills))
		skillPath := func(t Target, n string) string { return t.SkillPath(p.cfg.Prefix, n) }
		for _, spec := range []renderKindSpec{
			{
				kind: "skills", names: skillNames, target: t,
				tid:      func(n string) string { return skillTID(p, n) },
				sections: func(n string) []string { return projectCatalog(p).Skills[n].Sections },
				outPath:  skillPath,
				defaults: func(n string) map[string]any { return projectCatalog(p).Skills[n].Data },
			},
			{
				kind: "agents", names: slices.Sorted(maps.Keys(projectCatalog(p).Agents)), target: t,
				tid:      func(n string) string { return agentTID(p, n) },
				sections: func(n string) []string { return projectCatalog(p).Agents[n].Sections },
				outPath:  func(t Target, n string) string { return t.AgentPath(n) },
				defaults: func(n string) map[string]any { return projectCatalog(p).Agents[n].Data },
				encode: func(n, body string, data map[string]any) (string, error) {
					return encodeAgent(p, t, n, body, data)
				},
			},
		} {
			rfs, err := renderKind(p, spec, eff)
			if err != nil {
				return nil, err
			}
			out = append(out, rfs...)
		}
		for _, targetOutput := range resolvedTargetOutputs(t, p.cfg.Prefix, skillNames) {
			if targetOutputs[targetOutput.Path].canonical != t.Name {
				continue
			}
			target := t
			data := projectData(p, config.Sidecar{}, eff)

			for key, value := range t.targetTemplateData() {
				data[key] = value
			}
			rf, err := renderTarget(p, "target-output", "", targetOutput.TemplateID, nil,
				config.Sidecar{}, data, targetOutput.Path, eff, &renderOutputOptions{
					bannerStyle: targetOutput.Provenance,
					target:      &target,
					encoder:     targetOutput.Encoder,
				})
			if err != nil { // coverage-ignore: targetOutputDeclarations read this same embedded template before render.
				return nil, err
			}
			rf.Policy = targetOutput.Policy
			rf.Declarer = t.Name
			rf.DeclarerProjection = targetDescriptorProjection(t)
			rf.Encoder = targetOutput.Encoder
			rf.Provenance = targetOutput.Provenance
			for _, input := range targetOutput.Inputs { // coverage-ignore: validated target outputs cannot carry producer inputs
				rf.ConsumedInputs = append(rf.ConsumedInputs, OutputInput(input))
			}
			rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
			out = append(out, rf)
		}
	}
	// agents-doc / AGENTS.md, neutral - once.
	ad, err := p.cfg.Sidecar("agents-doc", "")
	if err != nil { // coverage-ignore: declaration-first planning just parsed the agents-doc sidecar
		return nil, err
	}
	ad = withDefaultData(ad, projectCatalog(p).Docs["agents-doc"].Data)
	data := projectData(p, ad, eff)
	data["docs"] = resolvedDocs(p)
	data["mandatoryDocs"] = documentMapDocs(p)
	data["localDocs"] = localDocumentMapDocs(p)
	data["documentMapFallbackHeading"] = len(p.cfg.LocalDocs) != 0 && ad.Sections["document-map"].Drop
	rf, err := renderTarget(p, "agents-doc", "", projectCatalog(p).Docs["agents-doc"].TID,
		projectCatalog(p).Docs["agents-doc"].Sections, ad, data, "AGENTS.md", eff)
	if err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(projectCatalog(p).Docs)) {
		if ok, sidecarErr := p.cfg.HasSidecar("docs", name); sidecarErr != nil { // coverage-ignore: declaration-first planning already read every catalog doc sidecar from the same filesystem invocation
			return nil, sidecarErr
		} else if ok {
			rf.ConsumedInputs = append(rf.ConsumedInputs, OutputInput{Path: config.DirName + "/docs/" + name + ".yaml", Role: ArtifactAuthoredData})
		}
	}
	rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
	if len(p.cfg.LocalDocs) != 0 {
		rf.ConfigHash = manifest.Hash([]byte(rf.ConfigHash + "\x00localDocs=" + localDocsProjection(p.cfg.NormalizedLocalDocs())))
	}
	out = append(out, rf)
	// Each descriptor-owned bridge is gated on the agents-doc render above. A
	// target with an empty BridgeFile emits no bridge, so neutral instructions
	// never point at an unrendered target-owned file.
	for _, t := range p.targets() {
		if t.BridgeFile == "" {
			continue
		}
		brf, err := renderTarget(p, targetBridgeKind, "", t.BridgeTemplate,
			nil, config.Sidecar{}, projectData(p, config.Sidecar{}, eff), t.BridgeFile, eff,
			&renderOutputOptions{sources: []string{"AGENTS.md"}})
		if err != nil {
			return nil, err
		}
		out = append(out, brf)
	}
	// Plain singletons: every Mandatory non-agents-doc entry in the catalog doc
	// collection, derived into plainSingletons (ADR-0021, ADR-0043, ADR-0059,
	// ADR-0061).
	lay := layout(p)
	for _, sg := range plainSingletons(projectCatalog(p)) {
		rfs, err := renderKind(p, renderKindSpec{
			kind: sg.kind, names: []string{""},
			tid:      func(string) string { return sg.tid },
			sections: func(string) []string { return sg.sections(projectCatalog(p)) },
			outPath:  func(Target, string) string { return sg.outPath(lay) },
			defaults: func(string) map[string]any { return projectCatalog(p).Docs[sg.kind].Data },
		}, eff)
		if err != nil {
			return nil, err
		}
		out = append(out, rfs...)
	}
	// .awf/bootstrap.sh, hook payloads, and the runner share only their declarative selection facts. Rendering
	// retains its own data construction and lifecycle behavior at this seam.
	for _, unit := range conditionalUnits() {
		if !unit.enabled(p.cfg) {
			continue
		}
		rf, err := renderTarget(p, unit.kind, "", unit.tid, unit.sections,
			config.Sidecar{}, projectData(p, config.Sidecar{}, eff), unit.path, eff,
			&renderOutputOptions{encoder: PlainAgentDialect})
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	// Every resident root has exactly one tracked self-ignoring node. Dynamic
	// descendants are local authority and never enter the manifest.
	for _, name := range resident.RootNames() {
		rf, err := renderResidentMarker(p, name, eff)
		if err != nil { // coverage-ignore: resident templates are embedded and registered at startup
			return nil, err
		}
		out = append(out, rf)
	}
	// Duplicate declarations are deliberately retained for OutputPlan to
	// coalesce or reject from normalized recipes.
	return out, nil
}

// RenderResidentMarker returns the exact resident marker from the ordinary
// output plan, including template execution and provenance banner injection.
func renderResidentMarkerOperation(p renderInputs, name string) (RenderedFile, error) {
	plan, err := outputPlan(p)
	if err != nil {
		return RenderedFile{}, err
	}
	want := config.DirName + "/" + name + "/.gitignore"
	for _, node := range plan.Nodes {
		if node.Path == want && node.file != nil {
			return *node.file, nil
		}
	}
	return RenderedFile{}, fmt.Errorf("resident marker %s is absent from the output plan", want) // coverage-ignore: the closed resident registry and output planner always declare every registered marker
}

// renderResidentMarker is the single resident-marker renderer. It owns template
// execution and provenance-banner injection, so every caller sees the exact bytes
// ordinary rendering plans and publishes.
func renderResidentMarker(p renderInputs, name string, eff map[string]bool) (RenderedFile, error) {
	return renderTarget(p, name, "", residentGitignoreTID(name), nil, config.Sidecar{}, projectData(p, config.Sidecar{}, eff), config.DirName+"/"+name+"/.gitignore", eff, &renderOutputOptions{encoder: PlainAgentDialect})
}

// renderTarget assembles an artifact (sidecar sections + convention parts), executes
// the template, rejects publication-unsafe <no value> output, and projects the
// per-artifact ConfigHash over the artifact's effective inputs.
func renderTarget(p renderInputs, kind, artifact, tid string, declared []string, sc config.Sidecar, data map[string]any, outPath string, eff map[string]bool, outputOptions ...*renderOutputOptions) (RenderedFile, error) {
	var options *renderOutputOptions
	if len(outputOptions) != 0 {
		options = outputOptions[0]
	}
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	expandedSource, err := render.ExpandIncludesSource(string(src), tid, templates.FS)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a missing/nested/section-bearing include, so ExpandIncludes cannot fail through the render pass; its error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	strippedSource, err := render.StripAuthoringCommentsSource(expandedSource)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a malformed awf:comment opener, so the strip cannot fail through the render pass; its error branch is unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	expanded := expandedSource.AuthoredText()
	stripped := strippedSource.AuthoredText()
	// Ordinary catalog and neutral Markdown producers retain the declared
	// Markdown default. Explicit producers supply their representation directly.
	encoder := MarkdownAgentDialect
	if options != nil && options.encoder != "" {
		encoder = options.encoder
	}
	segs := render.ParseSourceSections(strippedSource, encoder == MarkdownAgentDialect)
	provenance := render.TemplateSource{}
	if encoder == MarkdownAgentDialect && p.cfg.Render != nil {
		provenance.Root = p.cfg.Render.TemplateSourceRoot
		if provenance.Root != "" {
			if err := validateTemplateSources(p, expandedSource, provenance.Root); err != nil {
				return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
			}
		}
	}
	style := render.CommentStyleForSource(stripped)
	headings, err := captureStructuralHeadings(segs, data, tid)
	if err != nil {
		return RenderedFile{}, err
	}
	plan, err := planSections(p, kind, artifact, declared, sc.Sections, segs, outPath, style, headings)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	consumedInputs, err := observeRenderInputs(p, kind, artifact, tid, outPath, plan)
	if provenance.Root != "" {
		for _, span := range expandedSource.Spans {
			if span.Source != "" {
				consumedInputs = append(consumedInputs, OutputInput{Path: path.Join(provenance.Root, span.Source), Role: ArtifactTemplate})
			}
		}
		consumedInputs = normalizeOutputInputs(consumedInputs)
	}
	if err != nil { // coverage-ignore: the producer already parsed the same sidecar and planSections read the same parts in this invocation
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if err := render.CheckSectionDefaultStubs(segs, plan); err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	assembledSource, parts := render.AssembleSourceWithTemplateSource(segs, plan, style, provenance)
	assembled := assembledSource.AuthoredText()
	if err := render.CheckResidualMarkersSource(assembledSource); err != nil { // coverage-ignore: awf-owned embedded templates are marker-well-formed, so the guard cannot fire through the render pass; its error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	stubDefaults, stubParts := render.StubSections(segs, plan)
	var markerParts, partVarRefs []string
	for _, name := range slices.Sorted(maps.Keys(plan)) {
		if plan[name].PartMarker {
			markerParts = append(markerParts, plan[name].EditPath)
		}
		partVarRefs = append(partVarRefs, plan[name].PartVarRefs...)
	}
	var content string
	if provenance.Root != "" {
		content, err = render.ExecuteSourceWithTemplateSource(assembledSource, data, parts, tid, provenance)
	} else {
		content, err = render.Execute(assembled, data, parts, tid)
	}
	if err != nil { // coverage-ignore: with raw convention parts (ADR-0034) and always-valid embedded template defaults, render.Execute cannot fail through the render pass; its own parse/execute error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if options != nil && options.encode != nil {
		content, err = options.encode(content)
		if err != nil {
			return RenderedFile{}, fmt.Errorf("render %s: encode artifact: %w", tid, err)
		}
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\"; a referenced var or data key is unset", outPath)
	}
	if options != nil {
		content = injectBanner(content, tid, options.bannerStyle)
		content = injectSourceMarker(content, options.sources)
	} else {
		content = injectBanner(content, tid)
	}
	var targetInput []Target
	if options != nil && options.target != nil {
		targetInput = []Target{*options.target}
	}
	cfgHash, err := artifactConfigHash(p, assembled, sc, consumedParts(p, kind, artifact, plan), eff, targetInput...)
	if provenance.Root != "" {
		cfgHash = manifest.Hash([]byte(cfgHash + "\x00templateSourceRoot=" + provenance.Root))
	}
	if err != nil { // coverage-ignore: artifactConfigHash only fails on an unreadable consumed part, but planSections above already read every HasPart part, so consumedParts holds only readable paths
		return RenderedFile{}, err
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		// TemplateHash covers the post-expansion source so an edit to an included
		// partial flags every including artifact stale (ADR-0052).
		// touches-state: rendering/inplace-and-placeholders:authoring-comment-stripped - TemplateHash covers the pre-strip source, so a comment-only template edit reflags stale and self-settles
		TemplateHash: manifest.Hash([]byte(expanded)), ConfigHash: cfgHash,
		// A file carrying an in-place-editable section is drift-checked by
		// regeneration-with-read-back, never the frozen OutputHash (ADR-0100).
		RegenChecked: anyInPlace(plan),
		Policy:       declaredPolicy(kind, anyInPlace(plan)),
		Encoder:      encoder,
		assembled:    assembled, stubDefaults: stubDefaults, stubParts: stubParts,
		markerParts: markerParts, kind: kind, artifact: artifact, partVarRefs: partVarRefs,
		ConsumedInputs: consumedInputs, ObservedTemplateID: tid,
	}, nil
}

// captureStructuralHeadings executes a marker-free copy of the complete
// skeleton, rather than each heading as an independent template. This retains
// surrounding dot, variables, and control context while producing the expected
// line needed for in-place read-back before final assembly.
// touches-state: config/configuration:template-source-root - selected working/staged repository mapping validation; proof in render_test.go and staged_drift_test.go
// validateTemplateSources requires every provenance identity used by an
// instrumented output to exist in the operation's selected repository universe.
// The selected ProjectTreeReader keeps staged drift from consulting worktree files.
func validateTemplateSources(p renderInputs, source render.SourceText, root string) error {
	seen := map[string]bool{}
	for _, span := range source.Spans {
		if span.Source == "" || seen[span.Source] {
			continue
		}
		seen[span.Source] = true
		candidate := path.Join(root, span.Source)
		if reader, working := projectTreeReader(p).(filesystemProjectReader); working {
			components := strings.Split(candidate, "/")
			current := reader.root
			for i, component := range components {
				current = filepath.Join(current, filepath.FromSlash(component))
				info, statErr := os.Lstat(current)
				if errors.Is(statErr, fs.ErrNotExist) {
					break
				}
				if statErr != nil { // coverage-ignore: non-absence Lstat failures are OS-dependent and the error-preserving branch is direct
					return fmt.Errorf("read configured template source %s for %s: %w", candidate, span.Source, statErr)
				}
				last := i == len(components)-1
				if (!last && !info.IsDir()) || (last && !info.Mode().IsRegular()) {
					return fmt.Errorf("configured render.templateSourceRoot %q cannot resolve template source %q (%s): repository-confined regular file required", root, span.Source, candidate)
				}
			}
		}
		_, ok, err := projectTreeReader(p).ReadFile(candidate)
		if err != nil { // coverage-ignore: composed working and immutable snapshot readers return absence separately; their I/O failures are tested at reader ownership
			return fmt.Errorf("read configured template source %s for %s: %w", candidate, span.Source, err)
		}
		if !ok {
			return fmt.Errorf("configured render.templateSourceRoot %q cannot resolve template source %q (%s): regular file required", root, span.Source, candidate)
		}
	}
	return nil
}

func captureStructuralHeadings(segs []render.Segment, data map[string]any, tid string) (map[string]string, error) {
	headingSkeleton, headingTokens := render.StructuralHeadingCapture(segs)
	headingOutput, err := render.Execute(headingSkeleton, data, nil, tid+" headings")
	if err != nil {
		return nil, fmt.Errorf("render %s headings: %w", tid, err)
	}
	headings, err := render.ExtractStructuralHeadings(headingOutput, headingTokens)
	if err != nil {
		return nil, fmt.Errorf("render %s headings: %w", tid, err)
	}
	return headings, nil
}

func observeRenderInputs(p renderInputs, kind, artifact, tid, outPath string, plan map[string]render.SectionPlan) ([]OutputInput, error) {
	inputs := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}}
	if tid != "" {
		inputs = append(inputs, OutputInput{Path: "templates/" + tid, Role: ArtifactTemplate})
	}
	if kind != "target-output" && kind != targetBridgeKind && kind != "bootstrap" && kind != "hooks" && kind != "runner" && kind != "pitfall-entry" && !resident.IsResidentKind(kind) {
		has, err := p.cfg.HasSidecar(kind, artifact)
		if err != nil { // coverage-ignore: render producers parse this sidecar before input observation, and filesystem stat cannot newly fail without a concurrent race
			return nil, err
		}
		if has {
			rel := kind + "/" + artifact + ".yaml"
			if config.IsSingletonKind(kind) {
				rel = kind + ".yaml"
			}
			inputs = append(inputs, OutputInput{Path: config.DirName + "/" + rel, Role: ArtifactAuthoredData})
		}
	}
	inPlaceRead := false
	for _, section := range slices.Sorted(maps.Keys(plan)) {
		sp := plan[section]
		if sp.HasPart {
			inputs = append(inputs, OutputInput{Path: partRel(p, kind, artifact, section), Role: ArtifactConventionPart})
		}
		inPlaceRead = inPlaceRead || sp.InPlace
	}
	if inPlaceRead {
		if _, ok, err := projectTreeReader(p).ReadFile(outPath); err != nil {
			return nil, err
		} else if ok {
			inputs = append(inputs, OutputInput{Path: outPath, Role: ArtifactManagedOutput})
		}
	}
	return normalizeOutputInputs(inputs), nil
}

// encodeAgent renders catalog metadata with normal template data and combines it
// with the independently section-rendered instruction body in the target dialect.
func encodeAgent(p renderInputs, t Target, name, body string, data map[string]any) (string, error) {
	description, err := render.Execute(projectCatalog(p).Agents[name].Description, data, nil, "agent description")
	if err != nil {
		return "", err
	}
	a := agent{Name: projectCatalog(p).Agents[name].Name, Description: description, Body: body}
	if t.AgentDialect != MarkdownAgentDialect {
		return "", fmt.Errorf("unknown agent dialect %q", t.AgentDialect)
	}
	return encodeMarkdownAgent(a)
}

// generateLocalDocs owns the separate configured document family. It uses the
// ordinary in-place renderer but has no catalog entry, sidecar, or layout slot.
func generateLocalDocs(p renderInputs, eff map[string]bool) ([]RenderedFile, error) {
	out := make([]RenderedFile, 0, len(p.cfg.LocalDocs))
	for _, local := range p.cfg.NormalizedLocalDocs() {
		sc := config.Sidecar{Data: map[string]any{"title": local.Title}}
		rf, err := renderTarget(p, "local-doc", local.Name, localDocTID, []string{"body"}, sc,
			projectData(p, sc, eff), config.DocsDir+"/"+local.Name+".md", eff)
		if err != nil {
			return nil, err
		}
		rf.Declarer, rf.DeclarerProjection = "local-doc:"+local.Name, local.Name+"\x00"+local.Title+"\x00"+local.Description
		rf.ConfigHash = manifest.Hash([]byte(rf.ConfigHash + "\x00" + rf.DeclarerProjection))
		rf.Policy = OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}
		out = append(out, rf)
	}
	return out, nil
}

func generatePitfallLeaves(p renderInputs, corpus pitfall.Corpus, eff map[string]bool) ([]RenderedFile, error) {
	out := make([]RenderedFile, 0, corpus.Len())
	for _, entry := range corpus.All() {
		sc := config.Sidecar{Data: pitfallLeafData(entry)}
		rf, err := renderTarget(p, "pitfall-entry", entry.Slug, pitfallEntryTID, nil, sc,
			projectData(p, sc, eff), config.DocsDir+"/pitfalls/"+entry.Slug+".md", eff,
			&renderOutputOptions{sources: []string{entry.SourcePath}})
		if err != nil { // coverage-ignore: embedded leaf template and validated typed data cannot fail at this point
			return nil, err
		}
		rf.ConfigHash = manifest.Hash([]byte(rf.ConfigHash + "\x00" + string(entry.Source)))
		rf.ConsumedInputs = normalizeOutputInputs(append(rf.ConsumedInputs, OutputInput{Path: entry.SourcePath, Role: ArtifactAuthoredData}))
		rf.Declarer, rf.DeclarerProjection = "pitfall:"+entry.Slug, entry.SourcePath
		out = append(out, rf)
	}
	return out, nil
}

// generateIndexMD renders the ADR INDEX for the project's decisions directory
// (ADR-0135 item 8). It always produces a file: In flight and History sections,
// each with a placeholder line when empty, so the document-map link always
// resolves (ADR-0020 Decision 6 - partial-item supersedence of ADR-0005/ADR-0006).
func generateIndexMD(p renderInputs, corpus adr.Corpus) RenderedFile {
	content := adr.RenderIndexMD(corpus)
	content = injectSourceMarker(injectBanner(content, ""), []string{"derived:authored-adr-corpus"})
	inputs := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}}
	for _, record := range corpus.All() {
		inputs = append(inputs, OutputInput{Path: layout(p).ADRDir + "/" + record.Filename, Role: ArtifactDecisionRecord})
	}
	return RenderedFile{Path: layout(p).IndexMd, Content: content, RegenChecked: true, Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}, ConsumedInputs: normalizeOutputInputs(inputs)}
}

// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts and
// the domain's current-state topic navigation. Under current-state authority the
// per-domain ADR index is gone (ADR-0135 item 8): a domain doc points at topics,
// not decisions. Like INDEX.md the result carries no TemplateID/Hash - drift is
// checked by regeneration, since the topic navigation depends on external state.
func generateDomainDocs(p renderInputs, topics topic.Corpus, eff map[string]bool) ([]RenderedFile, error) {
	lay := layout(p)
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(p.cfg.Domains)) {
		data := projectData(p, config.Sidecar{}, eff)
		data["data"] = map[string]any{"domain": name, "topics": topic.BuildNavigationModel(name, topics.ForDomain(name))}
		rf, err := renderTarget(p, "domains", name, mustDescriptor("domains").templateID(projectCatalog(p), name),
			projectCatalog(p).DomainDoc.Sections, config.Sidecar{}, data,
			lay.DomainsDir+"/"+name+".md", eff, &renderOutputOptions{sources: []string{
				".awf/topics/metadata/" + name + "/*.yaml",
				".awf/topics/parts/" + name + "/*/current-state.md",
			}})
		if err != nil { // coverage-ignore: .data.domain/.data.topics are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		for _, currentTopic := range topics.ForDomain(name) {
			rf.ConsumedInputs = append(rf.ConsumedInputs,
				OutputInput{Path: relSlash(p.root(), currentTopic.MetadataPath), Role: ArtifactTopicMetadata},
				OutputInput{Path: relSlash(p.root(), currentTopic.PartPath), Role: ArtifactClaimPart})
		}
		rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
		wrapped := RenderedFile{Path: rf.Path, Content: rf.Content,
			stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
			markerParts: rf.markerParts, assembled: rf.assembled,
			partVarRefs: rf.partVarRefs, kind: rf.kind, artifact: rf.artifact,
			RegenChecked: true, Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}, Encoder: rf.Encoder,
			ConsumedInputs: rf.ConsumedInputs, ObservedTemplateID: rf.ObservedTemplateID}
		if templateSourceRoot(p) != "" {
			wrapped.TemplateID, wrapped.TemplateHash, wrapped.ConfigHash = rf.TemplateID, rf.TemplateHash, rf.ConfigHash
		}
		out = append(out, wrapped)
	}
	return out, nil
}
