package project

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

const (
	bridgeTID    = "claude/CLAUDE.md.tmpl"
	bootstrapTID = "bootstrap/awf-bootstrap.sh.tmpl"
	upgradeTID   = "bootstrap/awf-upgrade.sh.tmpl"
	memoryTID    = "memory/gitignore.tmpl"
)

// hookNames are the git-hook payload scripts the hooks singleton renders as a
// unit under .awf/hooks/ (ADR-0048); template ids are hooks/<name>.sh.tmpl.
var hookNames = []string{"pre-commit", "commit-msg", "pre-push"}

// HookNames returns the git-hook payload names the hooks singleton renders
// (ADR-0048), for CLI surfaces that enumerate them (the KnownTargets pattern).
func HookNames() []string { return slices.Clone(hookNames) }

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
}

// data assembles the template data namespace for a target: the prefix, the
// project vars, the sidecar's structured data, and the awf-given docs layout.
func (p *Project) data(sc config.Sidecar) map[string]any {
	return map[string]any{
		"prefix":           p.Cfg.Prefix,
		"vars":             nonNil(p.Cfg.Vars),
		"data":             nonNil(sc.Data),
		"layout":           p.layout().templateMap(),
		"version":          Version,
		"skills":           p.effSkills,
		"taskSkills":       p.taskSkillsDisplay(),
		"commitScopes":     p.commitScopesDisplay(),
		"invariantMarkers": p.invariantMarkersDisplay(),
		"gatedCommands":    gatedCommandsDisplay(),
	}
}

// taskSkillsDisplay returns the enabled catalog task skills — standard,
// non-Chain entries of the effective set — as "`<prefix>-<name>`, …", or ""
// when none are enabled. Derived from the catalog so a new task skill cannot
// be dropped from the guide's sentence by a forgotten template edit.
func (p *Project) taskSkillsDisplay() string {
	var quoted []string
	for _, name := range slices.Sorted(maps.Keys(p.effSkills)) {
		if sp, ok := catalog.Standard.Skills[name]; !ok || sp.Chain {
			continue
		}
		quoted = append(quoted, "`"+p.Cfg.Prefix+"-"+name+"`")
	}
	return strings.Join(quoted, ", ")
}

// commitScopesDisplay returns the display-formatted allowed commit-scope list
// (e.g. "`adr`, `awf`, `plans`") resolved from audit.allowedScopes — the same
// audit.Resolve path awf commit-gate reads, so prose and gate agree by
// construction — or "" when scopes are accept-any (ADR-0051).
func (p *Project) commitScopesDisplay() string {
	scopes := audit.Resolve(p.Cfg.Audit).AllowedScopes
	if len(scopes) == 0 {
		return ""
	}
	quoted := make([]string, len(scopes))
	for i, s := range scopes {
		quoted[i] = "`" + s.Name + "`"
	}
	return strings.Join(quoted, ", ")
}

// invariantMarkersDisplay returns the inline glob→marker mapping derived from
// invariants.sources (e.g. "`*.go` → `//`, `*.py` → `#`"), the invariant-tagging
// analog of commitScopesDisplay — or "" when no sources are configured (ADR-0064).
func (p *Project) invariantMarkersDisplay() string {
	if p.Cfg.Invariants == nil || len(p.Cfg.Invariants.Sources) == 0 {
		return ""
	}
	entries := make([]string, len(p.Cfg.Invariants.Sources))
	for i, s := range p.Cfg.Invariants.Sources {
		globs := make([]string, len(s.Globs))
		for j, g := range s.Globs {
			globs[j] = "`" + g + "`"
		}
		entries[i] = strings.Join(globs, ", ") + " → `" + s.Marker + "`"
	}
	return strings.Join(entries, ", ")
}

// effectiveSkills returns the skill names whose files exist on disk under
// awf's model: exactly the enabled set — closure validation (ADR-0081) makes
// enabled mean rendered, and local-declared names are hand-maintained but
// present. The sidecar read stays as the validation choke point Open relies
// on (amended semantics; formerly enabled minus ADR-0013 doc-gate-suppressed).
// invariant: skills-context-effective-set
func (p *Project) effectiveSkills() (map[string]bool, error) {
	eff := map[string]bool{}
	for _, name := range p.Cfg.Skills {
		if _, err := p.Cfg.Sidecar("skills", name); err != nil {
			return nil, err
		}
		eff[name] = true
	}
	return eff, nil
}

// partRel is the project-relative convention part path the awf:edit pointer names,
// derived from the absolute PartPath so the parts-path structure has one source.
func (p *Project) partRel(kind, artifact, section string) string {
	rel, err := filepath.Rel(p.Root, p.Cfg.PartPath(kind, artifact, section))
	if err != nil { // coverage-ignore: PartPath is always rooted under p.Root, so Rel cannot fail
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
func (p *Project) planSections(kind, artifact string, declared []string, sec map[string]config.SectionOverride, segs []render.Segment, outPath string, style render.CommentStyle) (map[string]render.SectionPlan, error) {
	plan := map[string]render.SectionPlan{}
	reg, err := p.placeholderRegistry()
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
	// actually declares an in-place section — every other artifact avoids the read.
	var output string
	outputRead := false
	readOutput := func() (string, error) {
		if !outputRead {
			b, rerr := os.ReadFile(filepath.Join(p.Root, outPath))
			if rerr != nil && !errors.Is(rerr, os.ErrNotExist) { // coverage-ignore: os.ReadFile errors only on a permission/IO fault that root bypasses; absence is folded into an empty read
				return "", rerr
			}
			output, outputRead = string(b), true // "" when absent (first render)
		}
		return output, nil
	}
	for _, s := range declared {
		sp := render.SectionPlan{EditPath: p.partRel(kind, artifact, s)}
		if ov, ok := sec[s]; ok && ov.Drop {
			sp.Drop = true
			plan[s] = sp
			continue
		}
		if inPlace[s] {
			// section-source-exclusive: an in-place section must not also carry a
			// convention part — the two override channels are mutually exclusive.
			if _, statErr := os.Stat(p.Cfg.PartPath(kind, artifact, s)); statErr == nil {
				return nil, fmt.Errorf("section %q is in-place-editable and must not also have a convention part at %s (ADR-0100)", s, p.partRel(kind, artifact, s))
			} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: os.Stat errors only on a permission/IO fault that root bypasses
				return nil, fmt.Errorf("stat part %s/%s/%s: %w", kind, artifact, s, statErr)
			}
			out, rerr := readOutput()
			if rerr != nil { // coverage-ignore: os.ReadFile errors only on a permission/IO fault that root bypasses (NotExist is folded into an empty read above)
				return nil, fmt.Errorf("read output %s: %w", outPath, rerr)
			}
			sp.InPlace = true
			if body, found := readBackInPlaceBody(out, s, declared, style); found {
				sp.InPlaceBody = body
			}
			plan[s] = sp
			continue
		}
		b, err := os.ReadFile(p.Cfg.PartPath(kind, artifact, s))
		if err == nil {
			body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), string(b), reg)
			if serr != nil {
				return nil, serr
			}
			sp.HasPart = true
			sp.PartBody = body
			sp.PartStub = render.HasStubMarker(body)
			// Scanned on the raw on-disk bytes, never the substituted body
			// (ADR-0083 Decision 4), with fenced examples excluded.
			sp.PartMarker = render.HasMarkerLine(refs.WithoutFences(string(b)))
			sp.PartVarRefs = render.PlaceholderVarRefs(string(b))
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read part %s/%s/%s: %w", kind, artifact, s, err)
		}
		plan[s] = sp
	}
	return plan, nil
}

// readBackInPlaceBody extracts the current body of the in-place section `name`
// from the existing rendered `output`. The region runs from just after `name`'s
// awf:edit-in-place pointer line to the first later line that is the pointer of
// any *other* registered (declared) section — matched by that section's expected
// pointer prefix in the target's comment style, never a generic pointer shape, so
// a pointer-shaped line for a non-registered name in adopter text cannot truncate
// it — or end-of-file when none follows. Leading/trailing blank lines (awf-owned
// framing) are trimmed; the interior, including internal blank lines, is returned
// verbatim. Returns ("", false) when `name`'s own pointer is absent (first render
// or a deleted anchor), so the caller falls back to the template default.
// touches-invariant: in-place-readback — read-back between the section pointer and awf's next registered pointer; proof in inplace_test.go
// touches-invariant: in-place-spacing-owned — verbatim interior, trimmed framing; proof in inplace_test.go
func readBackInPlaceBody(output, name string, declared []string, style render.CommentStyle) (string, bool) {
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
			break
		}
	}
	return trimBlankFraming(lines[start+1 : end]), true
}

// trimBlankFraming drops leading and trailing blank (whitespace-only) lines — the
// awf-owned framing — and returns the interior lines joined verbatim.
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

// anyInPlace reports whether a section plan contains an in-place-editable section —
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
// kinds that share the sort → sidecar → skip-local → render → append shape. tid
// and sections derive from the artifact name; outPath also takes the adapter
// target (ignored by neutral kinds like docs); target is the adapter this pass
// renders for (zero for neutral kinds).
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
}

// skillTID resolves a skill's template id: the shared base template for a
// synthesized local entry, else the name-derived catalog path (ADR-0068).
// touches-invariant: local-renders-from-base — skillTID resolves a local skill to the base template; proof in local_test.go
func (p *Project) skillTID(n string) string {
	if p.Cat.Skills[n].Base {
		return baseSkillTID
	}
	return mustDescriptor("skills").tid(n)
}

// agentTID mirrors skillTID for agents.
func (p *Project) agentTID(n string) string {
	if p.Cat.Agents[n].Base {
		return baseAgentTID
	}
	return mustDescriptor("agents").tid(n)
}

// docTID resolves a doc's template id through the effective catalog: the base
// doc template for a synthesized local doc (its DocEntry.TID), else the Standard
// doc's own template. Reading p.Cat (not the package global) is what lets a
// synthesized local doc render at all (ADR-0091).
// touches-invariant: local-doc-renders-from-base — docTID resolves a local doc to the base template; proof in local_test.go
func (p *Project) docTID(n string) string {
	return p.Cat.Docs[n].TID
}

func (p *Project) renderKind(spec renderKindSpec) ([]RenderedFile, error) {
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(spec.names)) {
		sc, err := p.Cfg.Sidecar(spec.kind, name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		if spec.defaults != nil {
			sc = withDefaultData(sc, spec.defaults(name))
		}
		if spec.transform != nil {
			if sc, err = spec.transform(name, sc); err != nil {
				return nil, err
			}
		}
		rf, err := p.renderTarget(spec.kind, name, spec.tid(name), spec.sections(name), sc, p.data(sc), spec.outPath(spec.target, name))
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, nil
}

func (p *Project) RenderAll() ([]RenderedFile, error) {
	var out []RenderedFile
	eff, err := p.effectiveSkills()
	if err != nil {
		return nil, err
	}
	p.effSkills = eff
	// Neutral: docs render once — the output path is docsDir-relative, not adapter-placed.
	docsRfs, err := p.renderKind(renderKindSpec{
		kind: "docs", names: p.Cfg.Docs,
		tid:       p.docTID,
		sections:  func(n string) []string { return p.Cat.Docs[n].Sections },
		outPath:   func(_ Target, n string) string { return p.docOutPath(n) },
		defaults:  func(n string) map[string]any { return p.Cat.Docs[n].Data },
		transform: docDataTransform,
	})
	if err != nil {
		return nil, err
	}
	out = append(out, docsRfs...)
	// Adapter: skills + agents render once per enabled target (inv: multi-target-render).
	// touches-invariant: multi-target-render — skills/agents render once per enabled target; proof in target_test.go
	for _, t := range p.Targets {
		for _, spec := range []renderKindSpec{
			{
				kind: "skills", names: p.Cfg.Skills, target: t,
				tid:      p.skillTID,
				sections: func(n string) []string { return p.Cat.Skills[n].Sections },
				outPath:  func(t Target, n string) string { return t.SkillPath(p.Cfg.Prefix, n) },
				defaults: func(n string) map[string]any { return p.Cat.Skills[n].Data },
			},
			{
				kind: "agents", names: p.Cfg.Agents, target: t,
				tid:      p.agentTID,
				sections: func(n string) []string { return p.Cat.Agents[n].Sections },
				outPath:  func(t Target, n string) string { return t.AgentPath(n) },
				defaults: func(n string) map[string]any { return p.Cat.Agents[n].Data },
			},
		} {
			rfs, err := p.renderKind(spec)
			if err != nil {
				return nil, err
			}
			out = append(out, rfs...)
		}
	}
	// agents-doc / AGENTS.md (always-on singleton unless its sidecar is local), neutral — once.
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil {
		return nil, err
	}
	if !ad.Local {
		ad = withDefaultData(ad, p.Cat.Docs["agents-doc"].Data)
		data := p.data(ad)
		docs, err := p.resolvedDocs()
		if err != nil { // coverage-ignore: resolvedDocs only errors on a docs-sidecar read failure, which RenderAll's docs loop already surfaces earlier
			return nil, err
		}
		data["docs"] = docs
		data["mandatoryDocs"] = p.documentMapDocs()
		rf, err := p.renderTarget("agents-doc", "", "agents-doc/AGENTS.md.tmpl",
			p.Cat.Docs["agents-doc"].Sections, ad, data, "AGENTS.md")
		if err != nil {
			return nil, err
		}
		out = append(out, rf)
		// Bridge: each adapter that wants one imports AGENTS.md verbatim (ADR-0037).
		// Gated on the agents-doc render above — a local (hand-maintained) AGENTS.md
		// must not get a bridge pointing at an un-rendered file. cursor has an empty
		// BridgeFile and emits nothing (inv: cursor-no-bridge).
		// touches-invariant: cursor-no-bridge — bridge suppression for an empty BridgeFile; proof in target_test.go
		for _, t := range p.Targets {
			if t.BridgeFile == "" {
				continue
			}
			brf, err := p.renderTarget("claude", "", bridgeTID,
				nil, config.Sidecar{}, p.data(config.Sidecar{}), t.BridgeFile)
			if err != nil { // coverage-ignore: the bridge template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, brf)
		}
	}
	// Plain singletons: every Mandatory non-agents-doc entry in the catalog doc
	// collection, derived into plainSingletons (always-on unless local; ADR-0021,
	// ADR-0043, ADR-0059, ADR-0061).
	lay := p.layout()
	for _, sg := range plainSingletons {
		rfs, err := p.renderKind(renderKindSpec{
			kind: sg.kind, names: []string{""},
			tid:      func(string) string { return sg.tid },
			sections: func(string) []string { return sg.sections(p.Cat) },
			outPath:  func(Target, string) string { return sg.outPath(lay) },
			defaults: func(string) map[string]any { return p.Cat.Docs[sg.kind].Data },
		})
		if err != nil {
			return nil, err
		}
		out = append(out, rfs...)
	}
	// .awf/bootstrap.sh + .awf/upgrade.sh (neutral config-tree singleton; rendered
	// as a unit only when enabled — ADR-0040 for the pinned installer, relocated by
	// ADR-0047; ADR-0085 for the upgrade porcelain). No catalog spec / no
	// overridable sections, like the CLAUDE.md bridge.
	// invariant: bootstrap-two-files
	if p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled {
		for _, u := range []struct{ tid, path string }{
			{bootstrapTID, config.DirName + "/bootstrap.sh"},
			{upgradeTID, config.DirName + "/upgrade.sh"},
		} {
			brf, err := p.renderTarget("bootstrap", "", u.tid,
				nil, config.Sidecar{}, p.data(config.Sidecar{}), u.path)
			if err != nil { // coverage-ignore: the bootstrap-unit templates reference only .version (always set) and no parts, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, brf)
		}
	}
	// .awf/hooks/*.sh git-hook payloads (neutral config-tree singleton; rendered
	// as a unit only when enabled — ADR-0048). No catalog spec / no overridable
	// sections, like the bootstrap; awf never activates them.
	if p.Cfg.Hooks != nil && p.Cfg.Hooks.Enabled {
		for _, name := range hookNames {
			hrf, err := p.renderTarget("hooks", "", "hooks/"+name+".sh.tmpl",
				nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/hooks/"+name+".sh")
			if err != nil { // coverage-ignore: every var reference in the hook templates is with/else- or if-wrapped (ADR-0045), and they use no parts, so renderTarget cannot produce <no value> or a read error
				return nil, err
			}
			out = append(out, hrf)
		}
	}
	// .awf/memory/.gitignore (neutral config-tree singleton; ALWAYS rendered —
	// ADR-0069, no config gate unlike bootstrap/hooks). Self-ignoring, so the
	// working-memory convention's ephemerality is mechanical, not remembered.
	// Deliberately non-configurable: no catalog spec, no sections, no CLI kind.
	mrf, err := p.renderTarget("memory", "", memoryTID,
		nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/memory/.gitignore")
	if err != nil { // coverage-ignore: the memory gitignore template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
		return nil, err
	}
	out = append(out, mrf)
	if err := assertNoDuplicateOutputPaths(out); err != nil {
		return nil, err
	}
	return out, nil
}

// assertNoDuplicateOutputPaths fails loudly when two rendered artifacts resolve
// to the same output path — a silent last-write-wins overwrite otherwise. Path-
// aware local doc names (ADR-0091) make this reachable: a name like
// `domains/<x>` or `decisions/template` can collide with awf's reserved output
// territory, which the name validator deliberately does not pre-reserve.
func assertNoDuplicateOutputPaths(files []RenderedFile) error {
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if seen[f.Path] {
			return fmt.Errorf("two artifacts render to the same output path %q — rename one (a local doc name may collide with awf's reserved decisions/, plans/, or domains/ output)", f.Path)
		}
		seen[f.Path] = true
	}
	return nil
}

// PlannedOutputs returns the project-relative paths Sync would write: every
// RenderAll output plus the generated ACTIVE.md, domain docs, and config
// reference. Used by awf init to detect collisions before writing (ADR-0016).
func (p *Project) PlannedOutputs() ([]string, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	amd, err := p.generateActiveMD()
	if err != nil {
		return nil, err
	}
	paths = append(paths, amd.Path)
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: unreachable — generateActiveMD above parses the same decisions dir and fails first on a malformed ADR
		return nil, err
	}
	for _, dd := range dds {
		paths = append(paths, dd.Path)
	}
	cref, ok, err := p.generateConfigReference(slices.Concat(files, dds))
	if err != nil { // reachable: the intro part is read here for the first time (RenderAll never renders the reference)
		return nil, err
	}
	if ok {
		paths = append(paths, cref.Path)
	}
	return paths, nil
}

// renderTarget assembles an artifact (sidecar sections + convention parts), executes
// the template, rejects publication-unsafe <no value> output, and projects the
// per-artifact ConfigHash over the artifact's effective inputs.
func (p *Project) renderTarget(kind, artifact, tid string, declared []string, sc config.Sidecar, data map[string]any, outPath string) (RenderedFile, error) {
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a missing/nested/section-bearing include, so ExpandIncludes cannot fail through RenderAll; its error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	segs := render.ParseSections(expanded)
	style := render.CommentStyleForSource(expanded)
	plan, err := p.planSections(kind, artifact, declared, sc.Sections, segs, outPath, style)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if err := render.CheckSectionDefaultStubs(segs, plan); err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	assembled, parts := render.Assemble(segs, plan, style)
	if err := render.CheckResidualMarkers(assembled); err != nil { // coverage-ignore: awf-owned embedded templates are marker-well-formed, so the guard cannot fire through RenderAll; its error branch is unit-tested in internal/render
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
	content, err := render.Execute(assembled, data, parts, tid)
	if err != nil { // coverage-ignore: with raw convention parts (ADR-0034) and always-valid embedded template defaults, render.Execute cannot fail through RenderAll; its own parse/execute error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	if strings.Contains(content, "<no value>") {
		return RenderedFile{}, fmt.Errorf("render %s: output contains \"<no value>\" — a referenced var or data key is unset", outPath)
	}
	content = injectBanner(content, tid)
	cfgHash, err := p.artifactConfigHash(assembled, sc, p.consumedParts(kind, artifact, plan))
	if err != nil { // coverage-ignore: artifactConfigHash only fails on an unreadable consumed part, but planSections above already read every HasPart part, so consumedParts holds only readable paths
		return RenderedFile{}, err
	}
	return RenderedFile{
		Path: outPath, Content: content, TemplateID: tid,
		// TemplateHash covers the post-expansion source so an edit to an included
		// partial flags every including artifact stale (ADR-0052).
		// touches-invariant: include-in-templatehash — TemplateHash over expanded (post-include) source; proof in golden_test.go
		TemplateHash: manifest.Hash([]byte(expanded)), ConfigHash: cfgHash,
		// A file carrying an in-place-editable section is drift-checked by
		// regeneration-with-read-back, never the frozen OutputHash (ADR-0100).
		RegenChecked: anyInPlace(plan),
		assembled:    assembled, stubDefaults: stubDefaults, stubParts: stubParts,
		markerParts: markerParts, kind: kind, artifact: artifact, partVarRefs: partVarRefs,
	}, nil
}

// generateActiveMD renders the ADR index for the project's decisions directory.
// It always produces a file: a populated index when ADRs exist, else a placeholder
// (ADR-0020 Decision 6 — partial-item supersedence of ADR-0005/ADR-0006).
func (p *Project) generateActiveMD() (RenderedFile, error) {
	content, err := adr.RenderActiveMD(p.decisionsDir())
	if err != nil {
		return RenderedFile{}, err
	}
	content = injectBanner(content, "")
	return RenderedFile{Path: p.layout().ActiveMd, Content: content, RegenChecked: true}, nil
}

// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts, with
// the per-domain ADR index injected as .data.decisions. Like ACTIVE.md, the result
// carries no TemplateID/Hash — drift is checked by regeneration, since the index
// depends on external ADR frontmatter state.
func (p *Project) generateDomainDocs() ([]RenderedFile, error) {
	decisionsDir := p.decisionsDir()
	lay := p.layout()
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
		index, err := adr.RenderDomainIndex(decisionsDir, name)
		if err != nil {
			return nil, err
		}
		data := p.data(config.Sidecar{})
		data["data"] = map[string]any{"domain": name, "decisions": index}
		rf, err := p.renderTarget("domains", name, mustDescriptor("domains").tid(name),
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			lay.DomainsDir+"/"+name+".md")
		if err != nil { // coverage-ignore: .data.domain/.data.decisions are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content,
			stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
			markerParts: rf.markerParts, assembled: rf.assembled,
			partVarRefs: rf.partVarRefs, RegenChecked: true})
	}
	return out, nil
}
