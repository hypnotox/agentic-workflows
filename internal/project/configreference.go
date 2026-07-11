package project

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// crefRel is the generated config reference's project-relative output path,
// derived from its catalog entry like every doc path.
func (p *Project) crefRel() string {
	return strings.TrimRight(p.Cfg.DocsDir, "/") + "/" + p.Cat.Docs["config-reference"].Path
}

// PotentialVarConsumers inverts the full catalog's raw template sources into
// var → sorted consumer labels: the dormant-hint side of the consumption
// graph (ADR-0088). Raw-source scanning is sound because no partial
// references .vars — guarded by a test beside the reference's goldens.
func PotentialVarConsumers() (map[string][]string, error) {
	byVar := map[string]map[string]bool{}
	add := func(tid string) error {
		varSet := map[string]bool{}
		if err := collectVars(templates.FS, tid, varSet); err != nil { // coverage-ignore: every catalog name has a backing embedded template
			return err
		}
		for v := range varSet {
			if byVar[v] == nil {
				byVar[v] = map[string]bool{}
			}
			byVar[v][artifactLabel(tid)] = true
		}
		return nil
	}
	cat := catalog.Standard
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, name := range d.poolNames(cat) {
			if err := add(d.tid(name)); err != nil { // coverage-ignore: see add
				return nil, err
			}
		}
	}
	if err := add("agents-doc/AGENTS.md.tmpl"); err != nil { // coverage-ignore: the agents-doc template is always embedded
		return nil, err
	}
	for _, sg := range plainSingletons {
		if err := add(sg.tid); err != nil { // coverage-ignore: every plainSingletons entry has a backing embedded template
			return nil, err
		}
	}
	for _, name := range hookNames {
		if err := add("hooks/" + name + ".sh.tmpl"); err != nil { // coverage-ignore: every hookNames entry has a backing embedded template
			return nil, err
		}
	}
	out := make(map[string][]string, len(byVar))
	for v, labels := range byVar {
		out[v] = slices.Sorted(maps.Keys(labels))
	}
	return out, nil
}

// enabledVarConsumers unions each var's enabled consumers from the rendered
// files' assembled sources and part-placeholder refs — the same consumption
// definition the unused-var check applies (ADR-0086).
func enabledVarConsumers(files []RenderedFile) map[string][]string {
	byVar := map[string]map[string]bool{}
	for _, f := range files {
		label := artifactLabel(f.TemplateID)
		if f.TemplateID == baseSkillTID || f.TemplateID == baseAgentTID {
			label = localLabel(f.TemplateID, f.Path)
		}
		if f.TemplateID == "" { // generated domain docs carry no template id
			label = "domain doc " + f.Path
		}
		for _, v := range slices.Concat(render.ReferencedVars(f.assembled), f.partVarRefs) {
			if byVar[v] == nil {
				byVar[v] = map[string]bool{}
			}
			byVar[v][label] = true
		}
	}
	out := make(map[string][]string, len(byVar))
	for v, labels := range byVar {
		out[v] = slices.Sorted(maps.Keys(labels))
	}
	return out
}

// currentValue renders a config key's live value for the reference table.
func (p *Project) currentValue(path string) string {
	res := audit.Resolve(p.Cfg.Audit)
	withDefault := func(s string, isDefault bool) string {
		if isDefault {
			return s + " (default)"
		}
		return s
	}
	a := p.Cfg.Audit
	switch path {
	case "prefix":
		return "`" + p.Cfg.Prefix + "`"
	case "docsDir":
		return "`" + p.Cfg.DocsDir + "`"
	case "vars":
		set := 0
		for _, v := range p.Cfg.Vars {
			if v != nil && v != "" {
				set++
			}
		}
		return fmt.Sprintf("%d keys, %d set", len(p.Cfg.Vars), set)
	case "skills":
		return strconv.Itoa(len(p.Cfg.Skills)) + " enabled"
	case "agents":
		return strconv.Itoa(len(p.Cfg.Agents)) + " enabled"
	case "docs":
		return strconv.Itoa(len(p.Cfg.Docs)) + " enabled"
	case "domains":
		return strconv.Itoa(len(p.Cfg.Domains)) + " configured"
	case "targets":
		return "`" + strings.Join(p.Cfg.Targets, "`, `") + "`"
	case "invariants.disabled":
		if p.Cfg.Invariants == nil {
			return "(unset)"
		}
		return strconv.FormatBool(p.Cfg.Invariants.Disabled)
	case "invariants.sources":
		if p.Cfg.Invariants == nil || len(p.Cfg.Invariants.Sources) == 0 {
			return "(none)"
		}
		return strconv.Itoa(len(p.Cfg.Invariants.Sources)) + " sources"
	case "audit.baseBranch":
		return withDefault("`"+res.BaseBranch+"`", a == nil || a.BaseBranch == "")
	case "audit.allowedTypes":
		if len(res.AllowedTypes) == 0 {
			return "accept any"
		}
		return withDefault(strconv.Itoa(len(res.AllowedTypes))+" types", a == nil || a.AllowedTypes == nil)
	case "audit.allowedScopes":
		if len(res.AllowedScopes) == 0 {
			return "accept any (default)"
		}
		return strconv.Itoa(len(res.AllowedScopes)) + " scopes"
	case "audit.subjectMaxLength":
		return withDefault(strconv.Itoa(res.SubjectMaxLength), a == nil || a.SubjectMaxLength == nil)
	case "audit.dependencyManifests":
		if len(res.DependencyManifests) == 0 {
			return "rule off"
		}
		return withDefault(strconv.Itoa(len(res.DependencyManifests))+" globs", a == nil || a.DependencyManifests == nil)
	case "audit.diffThreshold":
		return withDefault(strconv.Itoa(res.DiffThreshold), a == nil || a.DiffThreshold == nil)
	case "audit.domainDocStaleness":
		return withDefault(strconv.FormatBool(res.DomainDocStaleness), a == nil || a.DomainDocStaleness == nil)
	case "audit.domainCodeStaleness":
		return withDefault(strconv.FormatBool(res.DomainCodeStaleness), a == nil || a.DomainCodeStaleness == nil)
	case "audit.undocumentedDomain":
		return withDefault(strconv.FormatBool(res.UndocumentedDomain), a == nil || a.UndocumentedDomain == nil)
	case "audit.uncommittedChanges":
		return withDefault(strconv.FormatBool(res.UncommittedChanges), a == nil || a.UncommittedChanges == nil)
	case "bootstrap.enabled":
		return strconv.FormatBool(p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled)
	case "hooks.enabled":
		return strconv.FormatBool(p.Cfg.Hooks != nil && p.Cfg.Hooks.Enabled)
	default: // per-entry leaves (invariants.sources[].…, audit.allowedScopes[].…)
		return "—"
	}
}

// varState renders the three-way var state: set, present-but-empty (an open
// to-do), or absent (the deliberate decline).
func (p *Project) varState(key string) string {
	v, ok := p.Cfg.Vars[key]
	switch {
	case !ok:
		return "absent — declined; the generic prose renders"
	case v == nil || v == "":
		return "empty — open to-do"
	default:
		return fmt.Sprintf("set (`%v`)", v)
	}
}

// configReferenceData builds the four dedicated template collections. files
// is the consumption input: RenderAll output plus the generated domain docs.
func (p *Project) configReferenceData(files []RenderedFile) (map[string]any, error) {
	var configKeys, sidecarFields []map[string]any
	for _, e := range configspec.Keys() {
		row := map[string]any{
			"path": e.Path, "type": e.Type, "default": e.Default,
			"description": e.Description, "availability": e.Availability,
		}
		if strings.HasPrefix(e.Path, "sidecar.") {
			sidecarFields = append(sidecarFields, row)
			continue
		}
		row["current"] = p.currentValue(e.Path)
		configKeys = append(configKeys, row)
	}

	enabled := enabledVarConsumers(files)
	potential, err := PotentialVarConsumers()
	if err != nil { // coverage-ignore: PotentialVarConsumers reads only embedded templates
		return nil, err
	}
	var varEntries []map[string]any
	for _, v := range configspec.VarEntries() {
		consumers := "No catalog artifact references it."
		if c := enabled[v.Key]; len(c) > 0 {
			consumers = "Consumed by: " + strings.Join(c, ", ") + "."
		} else if c := potential[v.Key]; len(c) > 0 {
			consumers = "Dormant: no enabled artifact references it; enabling " + strings.Join(c, ", ") + " would."
		}
		varEntries = append(varEntries, map[string]any{
			"key": v.Key, "description": v.Description, "availability": v.Availability,
			"state": p.varState(v.Key), "consumers": consumers,
		})
	}

	dataKeys, err := p.dataKeyRows()
	if err != nil { // coverage-ignore: dataKeyRows re-reads sidecars RenderAll already read
		return nil, err
	}
	return map[string]any{
		"configKeys": configKeys, "varEntries": varEntries,
		"sidecarFields": sidecarFields, "dataKeys": dataKeys,
	}, nil
}

// dataKeyRows filters the described data keys to this project: enabled
// artifacts, the local base entries when a synthesized project-local artifact
// of that kind exists, and the always-on agents-doc.
func (p *Project) dataKeyRows() ([]map[string]any, error) {
	hasLocal := map[string]bool{
		"skills": p.hasLocalArtifact("skills"),
		"agents": p.hasLocalArtifact("agents"),
		"docs":   p.hasLocalArtifact("docs"),
	}
	var rows []map[string]any
	for _, d := range configspec.DataKeys() {
		var label string
		switch {
		case d.Artifact == "_base":
			if !hasLocal[d.Kind] {
				continue
			}
			label = "local " + strings.TrimSuffix(d.Kind, "s") + "s"
		case d.Kind == "docs" && d.Artifact == "agents-doc":
			label = "agents-doc"
		default:
			if !slices.Contains(p.enableArray(d.Kind), d.Artifact) {
				continue
			}
			label = strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		}
		state := ""
		var declared map[string]any
		if d.Artifact != "_base" {
			switch d.Kind {
			case "skills":
				declared = p.Cat.Skills[d.Artifact].Data
			case "agents":
				declared = p.Cat.Agents[d.Artifact].Data
			case "docs":
				declared = p.Cat.Docs[d.Artifact].Data
			}
			sidecarKind, sidecarName := d.Kind, d.Artifact
			if d.Artifact == "agents-doc" {
				sidecarKind, sidecarName = "agents-doc", ""
			}
			sc, err := p.Cfg.Sidecar(sidecarKind, sidecarName)
			if err != nil { // coverage-ignore: these sidecars were already read by RenderAll in the same pass
				return nil, err
			}
			if _, ok := sc.Data[d.Key]; ok {
				state = " (overridden)"
			} else if _, ok := declared[d.Key]; ok {
				state = " (catalog default)"
			}
		}
		rows = append(rows, map[string]any{
			"artifact": label, "key": d.Key, "description": d.Description, "state": state,
		})
	}
	return rows, nil
}

// hasLocalArtifact reports whether the project enables a synthesized
// project-local artifact of the plural kind — one rendered from an awf-owned
// base template, so the config reference should document that kind's `_base`
// data keys (ADR-0068/0091). Skills and agents carry a `Base` flag; a local
// doc's synthesized `DocEntry.TID` is the base doc template. A `local: true`
// opt-out is hand-authored and never synthesized, so it correctly does not
// count — its body is not rendered from the base template.
func (p *Project) hasLocalArtifact(kind string) bool {
	switch kind {
	case "skills":
		for _, n := range p.Cfg.Skills {
			if p.Cat.Skills[n].Base {
				return true
			}
		}
	case "agents":
		for _, n := range p.Cfg.Agents {
			if p.Cat.Agents[n].Base {
				return true
			}
		}
	case "docs":
		for _, n := range p.Cfg.Docs {
			if p.Cat.Docs[n].TID == baseDocTID {
				return true
			}
		}
	}
	return false
}

// enableArray returns the enable array for a plural kind name.
func (p *Project) enableArray(kind string) []string {
	switch kind {
	case "skills":
		return p.Cfg.Skills
	case "agents":
		return p.Cfg.Agents
	default:
		return p.Cfg.Docs
	}
}

// generateConfigReference renders the always-on generated config reference
// (ADR-class: generated index, no template/config hashes — drift is checked
// by regeneration). files is the consumption input (RenderAll output plus
// generated domain docs). The bool reports whether a reference was produced —
// false when a local: sidecar opts out (the manifest.LoadOptional found-flag
// idiom).
func (p *Project) generateConfigReference(files []RenderedFile) (*RenderedFile, bool, error) {
	sc, err := p.Cfg.Sidecar("config-reference", "")
	if err != nil { // coverage-ignore: validation already read this sidecar at open
		return nil, false, err
	}
	if sc.Local {
		return nil, false, nil
	}
	data := p.data(sc)
	collections, err := p.configReferenceData(files)
	if err != nil { // coverage-ignore: configReferenceData errors only on faults earlier passes already surfaced
		return nil, false, err
	}
	data["data"] = collections
	rf, err := p.renderTarget("config-reference", "", p.Cat.Docs["config-reference"].TID,
		p.Cat.Docs["config-reference"].Sections, sc, data, p.crefRel())
	if err != nil { // reachable: an unreadable intro part fails the read here — this is its first render
		return nil, false, err
	}
	return &RenderedFile{Path: rf.Path, Content: rf.Content,
		stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
		markerParts: rf.markerParts, assembled: rf.assembled,
		partVarRefs: rf.partVarRefs}, true, nil
}

// ConfigReferenceModel computes the reference's four collections
// (configKeys, varEntries, sidecarFields, dataKeys) with live project state —
// the `awf config` command's data source, sharing the doc's builder.
func (p *Project) ConfigReferenceModel() (map[string]any, error) {
	files, err := p.RenderAll()
	if err != nil {
		return nil, err
	}
	dds, err := p.generateDomainDocs()
	if err != nil { // coverage-ignore: RenderAll above already surfaced any malformed-ADR fault via project state
		return nil, err
	}
	return p.configReferenceData(slices.Concat(files, dds))
}
