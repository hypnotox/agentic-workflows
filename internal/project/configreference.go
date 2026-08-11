package project

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configspec"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// crefRel is the generated config reference's project-relative output path,
// derived from its catalog entry like every doc path.
func (p *Project) crefRel() string {
	return config.DocsDir + "/" + p.Cat.Docs["config-reference"].Path
}

// PotentialVarConsumers inverts the full catalog's raw template sources into
// var → sorted consumer labels: the dormant-hint side of the consumption
// graph (ADR-0088). Raw-source scanning is sound because no partial
// references .vars - guarded by a test beside the reference's goldens.
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
	if err := add(cat.Docs["agents-doc"].TID); err != nil { // coverage-ignore: the agents-doc template is always embedded
		return nil, err
	}
	for _, sg := range plainSingletons {
		if err := add(sg.tid); err != nil { // coverage-ignore: every plainSingletons entry has a backing embedded template
			return nil, err
		}
	}
	for _, name := range hookNames {
		if err := add(hookTID(name)); err != nil { // coverage-ignore: every hookNames entry has a backing embedded template
			return nil, err
		}
	}
	out := make(map[string][]string, len(byVar))
	for v, labels := range byVar {
		out[v] = slices.Sorted(maps.Keys(labels))
	}
	return out, nil
}

// renderedVarConsumers unions each var's current consumers from the rendered
// files' assembled sources and part-placeholder refs - the same consumption
// definition the unused-var check applies (ADR-0086).
func renderedVarConsumers(files []RenderedFile) map[string][]string {
	byVar := map[string]map[string]bool{}
	for _, f := range files {
		label := artifactLabel(f.TemplateID)
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

// currentValueResolvers couples each live classification to the function that
// computes its project value. Static keys have no resolver.
func (p *Project) currentValueResolvers() map[string]func() string {
	var scopes []config.ScopeSpec
	if p.Cfg.Audit != nil {
		scopes = p.Cfg.Audit.AllowedScopes
	}
	res := audit.Resolve(scopes)
	return map[string]func() string{
		"prefix":            func() string { return "`" + p.Cfg.Prefix + "`" },
		"integrationBranch": func() string { return "`" + p.Cfg.IntegrationBranch + "`" },
		"render.templateSourceRoot": func() string {
			if p.Cfg.Render == nil {
				return "(none)"
			}
			return "`" + p.Cfg.Render.TemplateSourceRoot + "`"
		},
		"vars": func() string {
			set := 0
			for _, v := range p.Cfg.Vars {
				if v != nil && v != "" {
					set++
				}
			}
			return fmt.Sprintf("%d keys, %d set", len(p.Cfg.Vars), set)
		},
		"domains": func() string { return strconv.Itoa(len(p.Cfg.Domains)) + " configured" },
		"tags": func() string {
			if len(p.Cfg.Tags) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.Tags)) + " tags"
		},
		"contextIgnore": func() string {
			if len(p.Cfg.ContextIgnore) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.ContextIgnore)) + " patterns"
		},
		"commitPolicy.grandfatheredThrough": func() string {
			if p.Cfg.CommitPolicy == nil || p.Cfg.CommitPolicy.GrandfatheredThrough == "" {
				return "(none)"
			}
			return "`" + p.Cfg.CommitPolicy.GrandfatheredThrough + "`"
		},
		"commitPolicy.allowedIdentities": func() string {
			if p.Cfg.CommitPolicy == nil || len(p.Cfg.CommitPolicy.AllowedIdentities) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.CommitPolicy.AllowedIdentities)) + " identities"
		},
		"commitPolicy.requireSignedCommits": func() string {
			if p.Cfg.CommitPolicy == nil {
				return "false (default)"
			}
			return strconv.FormatBool(p.Cfg.CommitPolicy.RequireSignedCommits)
		},
		"commitPolicy.allowedSigners": func() string {
			if p.Cfg.CommitPolicy == nil || len(p.Cfg.CommitPolicy.AllowedSigners) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.CommitPolicy.AllowedSigners)) + " signers"
		},
		"currentState.sources": func() string {
			if p.Cfg.CurrentState == nil || len(p.Cfg.CurrentState.Sources) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.CurrentState.Sources)) + " sources"
		},
		"currentState.testGlobs": func() string {
			if p.Cfg.CurrentState == nil || len(p.Cfg.CurrentState.TestGlobs) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.Cfg.CurrentState.TestGlobs)) + " globs"
		},
		"audit.allowedScopes": func() string {
			if len(res.AllowedScopes) == 0 {
				return "accept any (default)"
			}
			return strconv.Itoa(len(res.AllowedScopes)) + " scopes"
		},
		"bootstrap.enabled": func() string {
			return strconv.FormatBool(p.Cfg.Bootstrap != nil && p.Cfg.Bootstrap.Enabled)
		},
		"proseGate.exemptions": func() string {
			if p.Cfg.ProseGate == nil || len(p.Cfg.ProseGate.Exemptions) == 0 {
				return "(none)"
			}
			return fmt.Sprintf("%d entries", len(p.Cfg.ProseGate.Exemptions))
		},
		"memoryCite.exemptions": func() string {
			if p.Cfg.MemoryCite == nil || len(p.Cfg.MemoryCite.Exemptions) == 0 {
				return "(none)"
			}
			return fmt.Sprintf("%d entries", len(p.Cfg.MemoryCite.Exemptions))
		},
	}
}

func validateLiveStateAuthority(classes map[string]configspec.LiveStateClass, resolvers map[string]func() string) error {
	for path, class := range classes {
		_, hasResolver := resolvers[path]
		switch class {
		case configspec.LiveStateProjection:
			if !hasResolver {
				return fmt.Errorf("live-state key %q has no resolver", path)
			}
		case configspec.StaticNotApplicable:
			if hasResolver {
				return fmt.Errorf("static live-state key %q has a resolver", path)
			}
		default:
			return fmt.Errorf("live-state key %q has unknown class %d", path, class)
		}
	}
	for path := range resolvers {
		if _, ok := classes[path]; !ok {
			return fmt.Errorf("live-state resolver %q has no classification", path)
		}
	}
	return nil
}

// varState renders the three-way var state: set, present-but-empty (an open
// to-do), or absent (the deliberate decline).
func (p *Project) varState(key string) string {
	v, ok := p.Cfg.Vars[key]
	switch {
	case !ok:
		return "absent, declined; the generic prose renders"
	case v == nil || v == "":
		return "empty, an open to-do"
	default:
		return fmt.Sprintf("set (`%v`)", v)
	}
}

// ConfigKeyRow renders one config.yaml key or sidecar field in the config
// reference: a `configKeys` row always carries a resolved Current value, a
// `sidecarFields` row never does (there is no project-relative live value
// for a sidecar's own field), and the static catalog-only model leaves
// Current empty on every row.
type ConfigKeyRow struct {
	Path, Type, Default, Description, Availability, Current string
}

// VarRow renders one catalog var. State is the three-way live var state
// (set, empty, absent); it is empty in the static catalog-only model.
type VarRow struct {
	Key, Description, Availability, State, Consumers string
}

// DataKeyRow renders one per-artifact data key. State reports its observable
// catalog/project layering state.
type DataKeyRow struct {
	Artifact, Key, Description, State string
}

// ConfigReference is the typed presentation model for `awf config`: the four
// dedicated collections PrintConfigReference renders, produced either with
// live project state (ConfigReferenceModel) or catalog-only
// (StaticConfigReference). It is the typed counterpart to the map[string]any
// shape configReferenceData still builds for the doc generator - a renamed
// field here is a compile error, never a silently empty render.
type ConfigReference struct {
	ConfigKeys    []ConfigKeyRow
	VarEntries    []VarRow
	SidecarFields []ConfigKeyRow
	DataKeys      []DataKeyRow
}

// configReferenceRows builds the four reference collections as struct rows -
// the single implementation behind both the `awf config` live model and, via
// configReferenceData's map adaptation, the doc generator's template input.
func (p *Project) configReferenceRows(files []RenderedFile) (ConfigReference, error) {
	var ref ConfigReference
	classes := configspec.LiveStateClassifications()
	resolvers := p.currentValueResolvers()
	if err := validateLiveStateAuthority(classes, resolvers); err != nil { // coverage-ignore: production constructs both fixed authorities together; mutation tests exercise every mismatch in validateLiveStateAuthority directly
		return ConfigReference{}, err
	}
	for _, e := range configspec.Keys() {
		row := ConfigKeyRow{
			Path: e.Path, Type: e.Type, Default: e.Default,
			Description: e.Description, Availability: e.Availability,
		}
		if strings.HasPrefix(e.Path, "sidecar.") {
			ref.SidecarFields = append(ref.SidecarFields, row)
			continue
		}
		if classes[e.Path] == configspec.LiveStateProjection {
			row.Current = resolvers[e.Path]()
		} else {
			row.Current = "n/a"
		}
		ref.ConfigKeys = append(ref.ConfigKeys, row)
	}

	rendered := renderedVarConsumers(files)
	potential, err := PotentialVarConsumers()
	if err != nil { // coverage-ignore: PotentialVarConsumers reads only embedded templates
		return ConfigReference{}, err
	}
	for _, v := range configspec.VarEntries() {
		consumers := "No catalog artifact references it."
		if c := rendered[v.Key]; len(c) > 0 {
			consumers = "Consumed by: " + strings.Join(c, ", ") + "."
		} else if c := potential[v.Key]; len(c) > 0 {
			consumers = "Potential catalog consumers: " + strings.Join(c, ", ") + "; no rendered output currently references it."
		}
		ref.VarEntries = append(ref.VarEntries, VarRow{
			Key: v.Key, Description: v.Description, Availability: v.Availability,
			State: p.varState(v.Key), Consumers: consumers,
		})
	}

	dataKeys, err := p.dataKeyRowsTyped()
	if err != nil { // coverage-ignore: dataKeyRowsTyped re-reads sidecars the render pass in outputPlan already read
		return ConfigReference{}, err
	}
	ref.DataKeys = dataKeys
	return ref, nil
}

// configReferenceData adapts configReferenceRows to the map[string]any shape
// the doc generator's Go-template rendering consumes (a template keys a map by
// field name). One builder feeds both surfaces, so the CLI model and
// docs/config-reference.md cannot silently diverge; the render drift oracle
// pins the adaptation. files is the consumption input: the output plan's
// write files plus the generated domain docs.
func (p *Project) configReferenceData(files []RenderedFile) (map[string]any, error) {
	ref, err := p.configReferenceRows(files)
	if err != nil { // coverage-ignore: configReferenceRows fails only on the embedded-template and sidecar re-reads its own body already coverage-ignores
		return nil, err
	}
	keyRow := func(r ConfigKeyRow, withCurrent bool) map[string]any {
		row := map[string]any{
			"path": r.Path, "type": r.Type, "default": r.Default,
			"description": r.Description, "availability": r.Availability,
		}
		if withCurrent {
			row["current"] = r.Current
		}
		return row
	}
	var configKeys, sidecarFields []map[string]any
	for _, r := range ref.ConfigKeys {
		configKeys = append(configKeys, keyRow(r, true))
	}
	for _, r := range ref.SidecarFields {
		sidecarFields = append(sidecarFields, keyRow(r, false))
	}
	var varEntries []map[string]any
	for _, v := range ref.VarEntries {
		varEntries = append(varEntries, map[string]any{
			"key": v.Key, "description": v.Description, "availability": v.Availability,
			"state": v.State, "consumers": v.Consumers,
		})
	}
	dataKeys := make([]map[string]any, len(ref.DataKeys))
	for i, r := range ref.DataKeys {
		dataKeys[i] = map[string]any{
			"artifact": r.Artifact, "key": r.Key, "description": r.Description, "state": r.State,
		}
	}
	return map[string]any{
		"configKeys": configKeys, "varEntries": varEntries,
		"sidecarFields": sidecarFields, "dataKeys": dataKeys,
	}, nil
}

// dataKeyRowsTyped filters described data keys to catalog artifacts and the
// always-on agents-doc.
func (p *Project) dataKeyRowsTyped() ([]DataKeyRow, error) {
	var rows []DataKeyRow
	for _, d := range configspec.DataKeys() {
		label := strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		if d.Kind == "docs" && d.Artifact == "agents-doc" {
			label = "agents-doc"
		}
		state := ""
		var declared map[string]any
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
		if err != nil { // coverage-ignore: these sidecars were already read by the render pass in outputPlan
			return nil, err
		}
		_, hasAuthored := sc.Data[d.Key]
		defaultValue, hasDefault := declared[d.Key]
		_, catalogList := defaultValue.([]any)
		catalogList = catalogList && !slices.Contains(specializedListDataKeys(sidecarKind, sidecarName), d.Key)
		switch {
		case catalogList:
			keep, configured := sc.DataDefaults[d.Key]
			switch {
			case configured && !keep:
				state = " (explicitly suppressed default; project entries only)"
			case hasAuthored:
				state = " (catalog default + project entries)"
			case configured:
				state = " (catalog default; dataDefaults explicitly true)"
			default:
				state = " (catalog default)"
			}
		case hasAuthored:
			state = " (project-only/specialized)"
		case hasDefault:
			state = " (catalog default)"
		}
		rows = append(rows, DataKeyRow{Artifact: label, Key: d.Key, Description: d.Description, State: state})
	}
	return rows, nil
}

// generateConfigReference renders the always-on generated config reference
// (ADR-class: generated index, no template/config hashes - drift is checked
// by regeneration). files is the consumption input (the plan write files plus
// generated domain docs).
func (p *Project) generateConfigReference(files []RenderedFile, eff map[string]bool) (*RenderedFile, bool, error) {
	sc, err := p.Cfg.Sidecar("config-reference", "")
	if err != nil { // coverage-ignore: validation already read this sidecar at open
		return nil, false, err
	}
	data := p.data(sc, eff)
	collections, err := p.configReferenceData(files)
	if err != nil { // coverage-ignore: configReferenceData errors only on faults earlier passes already surfaced
		return nil, false, err
	}
	data["data"] = collections
	rf, err := p.renderTarget("config-reference", "", p.Cat.Docs["config-reference"].TID,
		p.Cat.Docs["config-reference"].Sections, sc, data, p.crefRel(), eff,
		&renderOutputOptions{sources: []string{"derived:configspec", "derived:project-configuration"}})
	if err != nil { // reachable: an unreadable intro part fails the read here - this is its first render
		return nil, false, err
	}
	wrapped := RenderedFile{Path: rf.Path, Content: rf.Content,
		stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
		markerParts: rf.markerParts, assembled: rf.assembled,
		partVarRefs: rf.partVarRefs, kind: rf.kind, artifact: rf.artifact,
		RegenChecked: true, ConsumedInputs: rf.ConsumedInputs, ObservedTemplateID: rf.ObservedTemplateID, Encoder: rf.Encoder,
		Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}}
	if p.templateSourceRoot() != "" {
		wrapped.TemplateID, wrapped.TemplateHash, wrapped.ConfigHash = rf.TemplateID, rf.TemplateHash, rf.ConfigHash
	}
	return &wrapped, true, nil
}

// ConfigReferenceModel computes the reference's four typed collections
// (ConfigKeys, VarEntries, SidecarFields, DataKeys) with live project state -
// the `awf config` command's data source.
func (p *Project) ConfigReferenceModel(ctx context.Context) (ConfigReference, error) {
	corpus, topics, eff, err := p.deriveOperationState()
	if err != nil {
		return ConfigReference{}, err
	}
	op, err := p.outputPlan(ctx, corpus, topics, eff)
	if err != nil {
		return ConfigReference{}, err
	}
	dds, err := p.generateDomainDocs(topics, eff)
	if err != nil { // coverage-ignore: the same producer ran inside outputPlan above over these identical inputs, so a second call cannot newly fail
		return ConfigReference{}, err
	}
	return p.configReferenceRows(slices.Concat(op.writeFiles(), dds))
}
