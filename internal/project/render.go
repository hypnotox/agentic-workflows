package project

import (
	"encoding/json"
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
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
	"github.com/hypnotox/agentic-workflows/internal/topic"
	"github.com/hypnotox/agentic-workflows/templates"
)

const (
	bridgeTID    = "claude/CLAUDE.md.tmpl"
	bootstrapTID = "bootstrap/awf-bootstrap.sh.tmpl"
	upgradeTID   = "bootstrap/awf-upgrade.sh.tmpl"
	memoryTID    = "memory/gitignore.tmpl"
	metricsTID   = "metrics/gitignore.tmpl"
	runnerTID    = "runner/awf.tmpl"
)

// runnerSections is the pure awf wrapper's single declared section: the body
// that resolves one awf invocation (vars.awfInvokeCmd, else bootstrap-pinned,
// else PATH awf) and execs it with all arguments forwarded verbatim (ADR-0156).
var runnerSections = []string{"runner-body"}

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
	// ConsumedInputs is observed at the render seam. It is intentionally
	// independent of BuildOutputDeclarations so declaration omissions and role
	// mistakes fail output-plan parity.
	ConsumedInputs     []OutputInput
	ObservedTemplateID string
}

// data assembles the template data namespace for a target: the prefix, the
// project vars, the sidecar's structured data, and the awf-given docs layout.
func (p *Project) data(sc config.Sidecar) map[string]any {
	return map[string]any{
		"prefix":                  p.Cfg.Prefix,
		"vars":                    nonNil(p.Cfg.Vars),
		"data":                    nonNil(sc.Data),
		"layout":                  p.layout().templateMap(),
		"version":                 Version,
		"skills":                  p.effSkills,
		"taskSkillRows":           p.taskSkillRows(),
		"commitScopes":            p.commitScopesDisplay(),
		"gatedCommands":           gatedCommandsDisplay(),
		"telemetryWidgetEnabled":  p.Cfg.WorkflowTelemetry.Widget.Enabled,
		"telemetryWidgetShowCost": p.Cfg.WorkflowTelemetry.Widget.ShowCost,
		// Project-level session-handoff signal for the neutral (guide/singleton
		// doc) render; per-target renders overwrite it from targetTemplateData
		// (ADR-0157 Decision 6).
		"targetSessionHandoff": anyTargetHasCapability(p.Targets, CapabilitySessionHandoff),
	}
}

type workflowRouterEntry struct {
	Name, Kind, Effect string
}

type workflowLoaderEntry struct {
	Kind               string   `json:"kind"`
	PhaseEffect        string   `json:"phaseEffect"`
	Phase              string   `json:"phase,omitempty"`
	Activity           string   `json:"activity,omitempty"`
	ImplementationMode string   `json:"implementationMode,omitempty"`
	RouteEffect        string   `json:"routeEffect,omitempty"`
	TerminalEffect     string   `json:"terminalEffect,omitempty"`
	RequiresPhases     []string `json:"requiresPhases"`
}

func (p *Project) workflowRouterData(names []string) ([]workflowRouterEntry, error) {
	mappings, err := catalog.WorkflowMappingsForSkills(p.Cat, names)
	if err != nil {
		return nil, err
	}
	entries := make([]workflowRouterEntry, 0, len(mappings))
	for _, name := range slices.Sorted(maps.Keys(mappings)) {
		mapping := mappings[name]
		effect := string(mapping.PhaseEffect)
		if mapping.Phase != "" {
			effect += " phase " + mapping.Phase
		}
		if mapping.Activity != "" {
			effect += " activity " + mapping.Activity
		}
		if mapping.RouteEffect != catalog.RouteNone {
			effect += " route " + string(mapping.RouteEffect)
		}
		if mapping.TerminalEffect != catalog.TerminalNone {
			effect += " terminal " + string(mapping.TerminalEffect)
		}
		entries = append(entries, workflowRouterEntry{Name: name, Kind: string(mapping.Kind), Effect: effect})
	}
	return entries, nil
}

func (p *Project) workflowLoaderData(names []string) (string, error) {
	mappings, err := catalog.WorkflowMappingsForSkills(p.Cat, names)
	if err != nil {
		return "", err
	}
	entries := make(map[string]workflowLoaderEntry, len(mappings))
	for name, mapping := range mappings {
		entries[name] = workflowLoaderEntry{
			Kind:               string(mapping.Kind),
			PhaseEffect:        string(mapping.PhaseEffect),
			Phase:              mapping.Phase,
			Activity:           mapping.Activity,
			ImplementationMode: mapping.ImplementationMode,
			RouteEffect:        string(mapping.RouteEffect),
			TerminalEffect:     string(mapping.TerminalEffect),
			RequiresPhases:     append([]string{}, mapping.RequiresPhases...),
		}
	}
	encoded, err := json.Marshal(entries)
	if err != nil { // coverage-ignore: the closed string/slice-only metadata shape cannot fail JSON encoding
		return "", fmt.Errorf("encode workflow loader metadata: %w", err)
	}
	return string(encoded), nil
}

func (p *Project) routedWorkflowNames() ([]string, error) {
	var names []string
	for _, name := range p.Cfg.Skills {
		spec, ok := p.Cat.Skills[name]
		if !ok || spec.Workflow == nil {
			continue
		}
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil {
			return nil, err
		}
		if !sc.Local {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// taskSkillRows returns the guide's trigger-table rows for the enabled catalog
// task skills - standard, non-Chain entries of the effective set - one
// "- `<prefix>-<name>`: <trigger>." line per skill, sorted by name and joined
// by newlines, or "" when none are enabled. Derived from the catalog so a new
// task skill cannot be dropped from the guide's table by a forgotten template
// edit (ADR-0157).
func (p *Project) taskSkillRows() string {
	var rows []string
	for _, name := range slices.Sorted(maps.Keys(p.effSkills)) {
		sp, ok := catalog.Standard.Skills[name]
		if !ok || sp.Chain {
			continue
		}
		rows = append(rows, "- `"+p.Cfg.Prefix+"-"+name+"`: "+sp.Trigger+".")
	}
	return strings.Join(rows, "\n")
}

// commitScopesDisplay returns the display-formatted allowed commit-scope list
// (e.g. "`adr`, `awf`, `plans`") resolved from audit.allowedScopes - the same
// audit.Resolve path awf commit-gate reads, so prose and gate agree by
// construction - or "" when scopes are accept-any (ADR-0051).
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

// effectiveSkills returns the skill names whose files exist on disk under
// awf's model: exactly the enabled set - closure validation (ADR-0081) makes
// enabled mean rendered, and local-declared names are hand-maintained but
// present. The sidecar read stays as the validation choke point Open relies
// on (amended semantics; formerly enabled minus ADR-0013 doc-gate-suppressed).
func (p *Project) effectiveSkills() (map[string]bool, error) {
	eff := map[string]bool{}
	for _, name := range p.Cfg.Skills {
		if _, err := p.Cfg.Sidecar("skills", name); err != nil { // coverage-ignore: declaration-first planning just parsed this enabled skill sidecar
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
	// actually declares an in-place section - every other artifact avoids the read.
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
			// convention part - the two override channels are mutually exclusive.
			if _, exists, partErr := p.Cfg.ReadPart(kind, artifact, s); partErr != nil {
				return nil, partErr
			} else if exists {
				return nil, fmt.Errorf("section %q is in-place-editable and must not also have a convention part at %s (ADR-0100)", s, p.partRel(kind, artifact, s))
			}
			out, rerr := readOutput()
			if rerr != nil { // coverage-ignore: os.ReadFile errors only on a permission/IO fault that root bypasses (NotExist is folded into an empty read above)
				return nil, fmt.Errorf("read output %s: %w", outPath, rerr)
			}
			sp.InPlace = true
			// A located region (its pointer present) is used verbatim even when
			// empty; only an unlocated region falls back to the template default
			// in Assemble (ADR-0100 in-place-readback).
			sp.InPlaceBody, sp.InPlaceFound = readBackInPlaceBody(out, s, declared, style)
			plan[s] = sp
			continue
		}
		b, exists, err := p.Cfg.ReadPart(kind, artifact, s)
		if err != nil {
			return nil, err
		}
		if exists {
			// Stripped before substitution (ADR-0121 Decision 2): a substituted
			// value can never create or mask a whole-line directive, and an
			// unknown placeholder demonstrated inside a comment must not error.
			raw, serr := render.StripAuthoringComments(string(b))
			if serr != nil {
				return nil, fmt.Errorf("part %s: %w", p.partRel(kind, artifact, s), serr)
			}
			body, serr := p.substitutePlaceholders(p.partRel(kind, artifact, s), raw, reg)
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

// readBackInPlaceBody extracts the current body of the in-place section `name`
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
// kinds that share the sort → sidecar → skip-local → render → append shape. tid
// and sections derive from the artifact name; outPath also takes the adapter
// target (ignored by neutral kinds like docs); target is the adapter this pass
// renders for (zero for neutral kinds).
type renderOutputOptions struct {
	encode      func(string) (string, error)
	bannerStyle render.CommentStyle
	target      *Target
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
	// workflowRouter selects the Pi-only governed successor instructions for
	// hidden workflow bodies without changing other target projections.
	workflowRouter bool
	// encode projects the rendered instruction body into an output dialect before
	// provenance injection (nil leaves ordinary skill/doc rendering unchanged).
	encode func(name, body string, data map[string]any) (string, error)
}

// skillTID resolves a skill's template id: the shared base template for a
// synthesized local entry, else the name-derived catalog path (ADR-0068).
// touches-state: rendering/local-artifacts:local-renders-from-base - skillTID resolves a local skill to the base template; proof in local_test.go
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
// touches-state: rendering/local-artifacts:local-doc-renders-from-base - docTID resolves a local doc to the base template; proof in local_test.go
func (p *Project) docTID(n string) string {
	return p.Cat.Docs[n].TID
}

func (p *Project) renderKind(spec renderKindSpec) ([]RenderedFile, error) {
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(spec.names)) {
		sc, err := p.Cfg.Sidecar(spec.kind, name)
		if err != nil { // coverage-ignore: declaration-first planning just parsed this enabled artifact sidecar
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
		data := p.data(sc)
		if spec.workflowRouter {
			data["targetWorkflowRouter"] = true
		}
		var options *renderOutputOptions
		if spec.target.Name != "" {
			for key, value := range spec.target.targetTemplateData() {
				data[key] = value
			}
			target := spec.target
			options = &renderOutputOptions{bannerStyle: render.HTMLComment, target: &target}
		}
		if spec.encode != nil {
			options.bannerStyle = spec.target.agentCommentStyle()
			options.encode = func(body string) (string, error) { return spec.encode(name, body, data) }
		}
		rf, err := p.renderTarget(spec.kind, name, spec.tid(name), spec.sections(name), sc, data, spec.outPath(spec.target, name), options)
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
func (p *Project) renderAllBase(targetOutputs map[string]targetOutputDeclaration) ([]RenderedFile, error) {
	var out []RenderedFile
	eff, err := p.effectiveSkills()
	if err != nil { // coverage-ignore: OutputPlan has already resolved the same enabled sidecars
		return nil, err
	}
	p.effSkills = eff
	// Neutral: docs render once - the output path is docsDir-relative, not adapter-placed.
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
	// touches-state: rendering/project-output-plan:multi-target-render - skills/agents render once per enabled target; proof in target_test.go
	for _, t := range p.Targets {
		skillNames := p.Cfg.Skills
		skillPath := func(t Target, n string) string { return t.SkillPath(p.Cfg.Prefix, n) }
		if t.routesWorkflows() {
			routed, routeErr := p.routedWorkflowNames()
			if routeErr != nil { // coverage-ignore: effectiveSkills parsed the same enabled sidecars above
				return nil, routeErr
			}
			routedSet := map[string]bool{}
			for _, name := range routed {
				routedSet[name] = true
			}
			skillNames = slices.DeleteFunc(slices.Clone(p.Cfg.Skills), func(name string) bool { return routedSet[name] })
			hidden, routeErr := p.renderKind(renderKindSpec{
				kind: "skills", names: routed, target: t, workflowRouter: true,
				tid:      p.skillTID,
				sections: func(n string) []string { return p.Cat.Skills[n].Sections },
				outPath:  func(t Target, n string) string { return t.HiddenWorkflowPath(n) },
				defaults: func(n string) map[string]any { return p.Cat.Skills[n].Data },
			})
			if routeErr != nil {
				return nil, routeErr
			}
			out = append(out, hidden...)
			entries, routeErr := p.workflowRouterData(routed)
			if routeErr != nil { // coverage-ignore: project open validates every catalog mapping before routed names can reach this helper
				return nil, routeErr
			}
			data := p.data(config.Sidecar{})
			data["workflowSkills"] = entries
			target := t
			router, routeErr := p.renderTarget("skills", "awf-workflow", "pi/awf-workflow/SKILL.md.tmpl", nil, config.Sidecar{}, data, t.WorkflowRouterPath(), &renderOutputOptions{bannerStyle: render.HTMLComment, target: &target})
			if routeErr != nil { // coverage-ignore: the embedded fixed router template and closed string-only data cannot fail after project validation
				return nil, routeErr
			}
			router.Declarer, router.DeclarerProjection = t.Name, targetDescriptorProjection(t)
			router.Encoder, router.Provenance = MarkdownAgentDialect, render.HTMLComment
			out = append(out, router)
		}
		for _, spec := range []renderKindSpec{
			{
				kind: "skills", names: skillNames, target: t,
				tid:      p.skillTID,
				sections: func(n string) []string { return p.Cat.Skills[n].Sections },
				outPath:  skillPath,
				defaults: func(n string) map[string]any { return p.Cat.Skills[n].Data },
			},
			{
				kind: "agents", names: p.Cfg.Agents, target: t,
				tid:      p.agentTID,
				sections: func(n string) []string { return p.Cat.Agents[n].Sections },
				outPath:  func(t Target, n string) string { return t.AgentPath(n) },
				defaults: func(n string) map[string]any { return p.Cat.Agents[n].Data },
				encode: func(n, body string, data map[string]any) (string, error) {
					return p.encodeAgent(t, n, body, data)
				},
			},
		} {
			rfs, err := p.renderKind(spec)
			if err != nil {
				return nil, err
			}
			out = append(out, rfs...)
		}
		for _, targetOutput := range t.Outputs {
			if targetOutputs[targetOutput.Path].canonical != t.Name {
				continue
			}
			target := t
			data := p.data(config.Sidecar{})
			if targetOutput.TemplateID == "pi/awf-dashboard/index.ts.tmpl" {
				routed, routeErr := p.routedWorkflowNames()
				if routeErr != nil { // coverage-ignore: effectiveSkills and the earlier routed render parsed the same enabled sidecars in this render pass
					return nil, routeErr
				}
				metadata, routeErr := p.workflowLoaderData(routed)
				if routeErr != nil { // coverage-ignore: project open validates every catalog mapping before routed names can reach this helper
					return nil, routeErr
				}
				data["workflowMappingsJSON"] = metadata
			}
			for key, value := range t.targetTemplateData() {
				data[key] = value
			}
			switch targetOutput.Producer {
			case TargetOutputTemplate:
			case TargetOutputTelemetryProtocol:
				data["telemetryProtocolBody"] = telemetry.ProjectTypeScript()
			default: // coverage-ignore: target validation rejects unknown producers
				return nil, fmt.Errorf("unknown target output producer %q", targetOutput.Producer)
			}
			rf, err := p.renderTarget("target-output", "", targetOutput.TemplateID, nil,
				config.Sidecar{}, data, targetOutput.Path, &renderOutputOptions{
					bannerStyle: targetOutput.Provenance,
					target:      &target,
				})
			if err != nil { // coverage-ignore: targetOutputDeclarations read this same embedded template before render.
				return nil, err
			}
			rf.Policy = targetOutput.Policy
			rf.Declarer = t.Name
			rf.DeclarerProjection = targetDescriptorProjection(t)
			rf.Encoder = targetOutput.Encoder
			rf.Provenance = targetOutput.Provenance
			for _, input := range targetOutput.Inputs {
				rf.ConsumedInputs = append(rf.ConsumedInputs, OutputInput(input))
			}
			rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
			out = append(out, rf)
		}
	}
	// agents-doc / AGENTS.md (always-on singleton unless its sidecar is local), neutral - once.
	ad, err := p.Cfg.Sidecar("agents-doc", "")
	if err != nil { // coverage-ignore: declaration-first planning just parsed the agents-doc sidecar
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
		for _, name := range p.Cfg.Docs {
			if ok, sidecarErr := p.Cfg.HasSidecar("docs", name); sidecarErr != nil { // coverage-ignore: declaration-first planning already read every enabled doc sidecar from the same filesystem invocation
				return nil, sidecarErr
			} else if ok {
				rf.ConsumedInputs = append(rf.ConsumedInputs, OutputInput{Path: config.DirName + "/docs/" + name + ".yaml", Role: ArtifactAuthoredData})
			}
		}
		rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
		out = append(out, rf)
		// Bridge: each adapter that wants one imports AGENTS.md verbatim (ADR-0037).
		// Gated on the agents-doc render above - a local (hand-maintained) AGENTS.md
		// must not get a bridge pointing at an un-rendered file. cursor has an empty
		// BridgeFile and emits nothing (inv: cursor-no-bridge).
		// touches-state: rendering/project-output-plan:cursor-no-bridge - bridge suppression for an empty BridgeFile; proof in target_test.go
		for _, t := range p.Targets {
			if t.BridgeFile == "" {
				continue
			}
			brf, err := p.renderTarget("claude", "", t.BridgeTemplate,
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
	// as a unit only when enabled - ADR-0040 for the pinned installer, relocated by
	// ADR-0047; ADR-0085 for the upgrade porcelain). No catalog spec / no
	// overridable sections, like the CLAUDE.md bridge.
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
	// as a unit only when enabled - ADR-0048). No catalog spec / no overridable
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
	// The pure awf wrapper `awf` at the repo root (config-tree singleton rendered
	// only when enabled - ADR-0156; fully awf-owned, no in-place sections, not a
	// catalog DocEntry, so it stays out of SingletonKinds()).
	if p.Cfg.Runner != nil && p.Cfg.Runner.Enabled {
		rrf, err := p.renderTarget("runner", "", runnerTID,
			runnerSections, config.Sidecar{}, p.data(config.Sidecar{}), "awf")
		if err != nil {
			return nil, err
		}
		out = append(out, rrf)
	}
	// .awf/memory/.gitignore (neutral config-tree singleton; ALWAYS rendered -
	// ADR-0069, no config gate unlike bootstrap/hooks). Self-ignoring, so the
	// working-memory convention's ephemerality is mechanical, not remembered.
	// Deliberately non-configurable: no catalog spec, no sections, no CLI kind.
	mrf, err := p.renderTarget("memory", "", memoryTID,
		nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/memory/.gitignore")
	if err != nil { // coverage-ignore: the memory gitignore template is static, part-free, and references no vars, so renderTarget cannot produce <no value> or a read error
		return nil, err
	}
	out = append(out, mrf)
	// .awf/metrics/.gitignore is the sole governed node for the dynamic resident
	// telemetry tree. Runtime descendants are intentionally outside the manifest.
	metricsRF, err := p.renderTarget("metrics", "", metricsTID,
		nil, config.Sidecar{}, p.data(config.Sidecar{}), config.DirName+"/metrics/.gitignore")
	if err != nil { // coverage-ignore: the metrics gitignore template is static, part-free, and references no vars
		return nil, err
	}
	out = append(out, metricsRF)
	// Duplicate declarations are deliberately retained for OutputPlan to
	// coalesce or reject from normalized recipes.
	return out, nil
}

// renderTarget assembles an artifact (sidecar sections + convention parts), executes
// the template, rejects publication-unsafe <no value> output, and projects the
// per-artifact ConfigHash over the artifact's effective inputs.
func (p *Project) renderTarget(kind, artifact, tid string, declared []string, sc config.Sidecar, data map[string]any, outPath string, outputOptions ...*renderOutputOptions) (RenderedFile, error) {
	var options *renderOutputOptions
	if len(outputOptions) != 0 {
		options = outputOptions[0]
	}
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("read template %s: %w", tid, err)
	}
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a missing/nested/section-bearing include, so ExpandIncludes cannot fail through RenderAll; its error branches are unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	stripped, err := render.StripAuthoringComments(expanded)
	if err != nil { // coverage-ignore: awf-owned embedded templates never author a malformed awf:comment opener, so the strip cannot fail through RenderAll; its error branch is unit-tested in internal/render
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	segs := render.ParseSections(stripped)
	style := render.CommentStyleForSource(stripped)
	plan, err := p.planSections(kind, artifact, declared, sc.Sections, segs, outPath, style)
	if err != nil {
		return RenderedFile{}, fmt.Errorf("render %s: %w", tid, err)
	}
	consumedInputs, err := p.observeRenderInputs(kind, artifact, tid, outPath, plan)
	if err != nil { // coverage-ignore: the producer already parsed the same sidecar and planSections read the same parts in this invocation
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
	} else {
		content = injectBanner(content, tid)
	}
	var targetInput []Target
	if options != nil && options.target != nil {
		targetInput = []Target{*options.target}
	}
	cfgHash, err := p.artifactConfigHash(assembled, sc, p.consumedParts(kind, artifact, plan), targetInput...)
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
		assembled:    assembled, stubDefaults: stubDefaults, stubParts: stubParts,
		markerParts: markerParts, kind: kind, artifact: artifact, partVarRefs: partVarRefs,
		ConsumedInputs: consumedInputs, ObservedTemplateID: tid,
	}, nil
}

func (p *Project) observeRenderInputs(kind, artifact, tid, outPath string, plan map[string]render.SectionPlan) ([]OutputInput, error) {
	inputs := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}}
	if tid != "" {
		inputs = append(inputs, OutputInput{Path: "templates/" + tid, Role: ArtifactTemplate})
	}
	if kind != "target-output" && kind != "claude" && kind != "bootstrap" && kind != "hooks" && kind != "memory" && kind != "runner" {
		has, err := p.Cfg.HasSidecar(kind, artifact)
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
			inputs = append(inputs, OutputInput{Path: p.partRel(kind, artifact, section), Role: ArtifactConventionPart})
		}
		inPlaceRead = inPlaceRead || sp.InPlace
	}
	if inPlaceRead {
		if _, ok := (filesystemProjectReader{root: p.Root}).ReadFile(outPath); ok {
			inputs = append(inputs, OutputInput{Path: outPath, Role: ArtifactManagedOutput})
		}
	}
	return normalizeOutputInputs(inputs), nil
}

// encodeAgent renders catalog metadata with normal template data and combines it
// with the independently section-rendered instruction body in the target dialect.
func (p *Project) encodeAgent(t Target, name, body string, data map[string]any) (string, error) {
	description, err := render.Execute(p.Cat.Agents[name].Description, data, nil, "agent description")
	if err != nil {
		return "", err
	}
	a := agent{Name: p.Cat.Agents[name].Name, Description: description, Body: body}
	switch t.AgentDialect {
	case MarkdownAgentDialect:
		return encodeMarkdownAgent(a)
	case TOMLAgentDialect:
		return encodeTOMLAgent(a)
	default:
		return "", fmt.Errorf("unknown agent dialect %q", t.AgentDialect)
	}
}

// generateIndexMD renders the ADR INDEX for the project's decisions directory
// (ADR-0135 item 8). It always produces a file: In flight and History sections,
// each with a placeholder line when empty, so the document-map link always
// resolves (ADR-0020 Decision 6 - partial-item supersedence of ADR-0005/ADR-0006).
func (p *Project) generateIndexMD() (RenderedFile, error) {
	corpus, err := p.Corpus()
	if err != nil { // coverage-ignore: OutputPlan loads the same corpus through topic generation before this producer
		return RenderedFile{}, err
	}
	content := adr.RenderIndexMD(corpus)
	content = injectBanner(content, "")
	inputs := []OutputInput{{Path: config.DirName + "/config.yaml", Role: ArtifactConfig}}
	for _, record := range corpus.All() {
		inputs = append(inputs, OutputInput{Path: p.layout().ADRDir + "/" + record.Filename, Role: ArtifactDecisionRecord})
	}
	return RenderedFile{Path: p.layout().IndexMd, Content: content, RegenChecked: true, Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}, ConsumedInputs: normalizeOutputInputs(inputs)}, nil
}

// generateDomainDocs renders one content-only doc per declared domain
// (<docsDir>/domains/<name>.md): the domain template + its convention parts and
// the domain's current-state topic navigation. Under current-state authority the
// per-domain ADR index is gone (ADR-0135 item 8): a domain doc points at topics,
// not decisions. Like INDEX.md the result carries no TemplateID/Hash - drift is
// checked by regeneration, since the topic navigation depends on external state.
func (p *Project) generateDomainDocs() ([]RenderedFile, error) {
	topics, err := p.Topics()
	if err != nil {
		return nil, err
	}
	lay := p.layout()
	var out []RenderedFile
	for _, name := range slices.Sorted(slices.Values(p.Cfg.Domains)) {
		data := p.data(config.Sidecar{})
		data["data"] = map[string]any{"domain": name, "topics": topic.BuildNavigationModel(name, topics.ForDomain(name))}
		rf, err := p.renderTarget("domains", name, mustDescriptor("domains").tid(name),
			p.Cat.DomainDoc.Sections, config.Sidecar{}, data,
			lay.DomainsDir+"/"+name+".md")
		if err != nil { // coverage-ignore: .data.domain/.data.topics are always set and the template is embedded, so renderTarget cannot produce <no value> or a read error here
			return nil, err
		}
		for _, currentTopic := range topics.ForDomain(name) {
			rf.ConsumedInputs = append(rf.ConsumedInputs,
				OutputInput{Path: relSlash(p.Root, currentTopic.MetadataPath), Role: ArtifactTopicMetadata},
				OutputInput{Path: relSlash(p.Root, currentTopic.PartPath), Role: ArtifactClaimPart})
		}
		rf.ConsumedInputs = normalizeOutputInputs(rf.ConsumedInputs)
		out = append(out, RenderedFile{Path: rf.Path, Content: rf.Content,
			stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
			markerParts: rf.markerParts, assembled: rf.assembled,
			partVarRefs: rf.partVarRefs, RegenChecked: true, Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}, ConsumedInputs: rf.ConsumedInputs, ObservedTemplateID: rf.ObservedTemplateID})
	}
	return out, nil
}
