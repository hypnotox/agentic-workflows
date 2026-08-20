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
func crefRel(p renderInputs) string {
	return config.DocsDir + "/" + projectCatalog(p).Docs["config-reference"].Path
}

// PotentialVarConsumers inverts the complete catalog's raw template sources
// into var -> sorted consumer labels for static catalog presentation.
func PotentialVarConsumers() (map[string][]string, error) {
	return potentialVarConsumers(catalog.CompleteView().Catalog())
}

// potentialVarConsumers is the shared inversion over the caller-owned catalog
// view. Raw-source scanning is sound because no partial references .vars.
func potentialVarConsumers(cat *catalog.Catalog) (map[string][]string, error) {
	byVar := map[string]map[string]bool{}
	add := func(tid string) error {
		varSet := map[string]bool{}
		if err := collectVars(templates.FS, tid, varSet); err != nil {
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
	for _, kind := range []string{"skills", "agents", "docs"} {
		d, _ := descriptorByPlural(kind)
		for _, name := range d.poolNames(cat) {
			if err := add(d.templateID(cat, name)); err != nil {
				return nil, err
			}
		}
	}
	if err := add(cat.Docs["agents-doc"].TID); err != nil { // coverage-ignore: the agents-doc template is always embedded
		return nil, err
	}
	for _, sg := range plainSingletons(cat) {
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
func currentValueResolvers(p renderInputs) map[string]func() string {
	var scopes []config.ScopeSpec
	if p.cfg.Audit != nil {
		scopes = p.cfg.Audit.AllowedScopes
	}
	res := audit.Resolve(scopes)
	return map[string]func() string{
		"profile":           func() string { return "`" + string(p.cfg.Profile) + "`" },
		"prefix":            func() string { return "`" + p.cfg.Prefix + "`" },
		"integrationBranch": func() string { return "`" + p.cfg.IntegrationBranch + "`" },
		"render.templateSourceRoot": func() string {
			if p.cfg.Render == nil {
				return "(none)"
			}
			return "`" + p.cfg.Render.TemplateSourceRoot + "`"
		},
		"vars": func() string {
			set := 0
			for _, v := range p.cfg.Vars {
				if v != nil && v != "" {
					set++
				}
			}
			return fmt.Sprintf("%d keys, %d set", len(p.cfg.Vars), set)
		},
		"localDocs": func() string { return strconv.Itoa(len(p.cfg.LocalDocs)) + " configured" },
		"domains":   func() string { return strconv.Itoa(len(p.cfg.Domains)) + " configured" },
		"tags": func() string {
			if len(p.cfg.Tags) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.Tags)) + " tags"
		},
		"contextIgnore": func() string {
			if len(p.cfg.ContextIgnore) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.ContextIgnore)) + " patterns"
		},
		"commitPolicy.grandfatheredThrough": func() string {
			if p.cfg.CommitPolicy == nil || p.cfg.CommitPolicy.GrandfatheredThrough == "" {
				return "(none)"
			}
			return "`" + p.cfg.CommitPolicy.GrandfatheredThrough + "`"
		},
		"commitPolicy.allowedIdentities": func() string {
			if p.cfg.CommitPolicy == nil || len(p.cfg.CommitPolicy.AllowedIdentities) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.CommitPolicy.AllowedIdentities)) + " identities"
		},
		"commitPolicy.requireSignedCommits": func() string {
			if p.cfg.CommitPolicy == nil {
				return "false (default)"
			}
			return strconv.FormatBool(p.cfg.CommitPolicy.RequireSignedCommits)
		},
		"commitPolicy.allowedSigners": func() string {
			if p.cfg.CommitPolicy == nil || len(p.cfg.CommitPolicy.AllowedSigners) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.CommitPolicy.AllowedSigners)) + " signers"
		},
		"currentState.sources": func() string {
			if p.cfg.CurrentState == nil || len(p.cfg.CurrentState.Sources) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.CurrentState.Sources)) + " sources"
		},
		"currentState.testGlobs": func() string {
			if p.cfg.CurrentState == nil || len(p.cfg.CurrentState.TestGlobs) == 0 {
				return "(none)"
			}
			return strconv.Itoa(len(p.cfg.CurrentState.TestGlobs)) + " globs"
		},
		"audit.allowedScopes": func() string {
			if len(res.AllowedScopes) == 0 {
				return "accept any (default)"
			}
			return strconv.Itoa(len(res.AllowedScopes)) + " scopes"
		},
		"bootstrap.enabled": func() string {
			return strconv.FormatBool(p.cfg.Bootstrap != nil && p.cfg.Bootstrap.Enabled)
		},
		"proseGate.exemptions": func() string {
			if p.cfg.ProseGate == nil || len(p.cfg.ProseGate.Exemptions) == 0 {
				return "(none)"
			}
			return fmt.Sprintf("%d entries", len(p.cfg.ProseGate.Exemptions))
		},
		"memoryCite.exemptions": func() string {
			if p.cfg.MemoryCite == nil || len(p.cfg.MemoryCite.Exemptions) == 0 {
				return "(none)"
			}
			return fmt.Sprintf("%d entries", len(p.cfg.MemoryCite.Exemptions))
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
func varState(p renderInputs, key string) string {
	v, ok := p.cfg.Vars[key]
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
func configReferenceRows(p renderInputs, files []RenderedFile) (ConfigReference, error) {
	var ref ConfigReference
	classes := configspec.LiveStateClassifications()
	resolvers := currentValueResolvers(p)
	if err := validateLiveStateAuthority(classes, resolvers); err != nil { // coverage-ignore: production constructs both fixed authorities together; mutation tests exercise every mismatch in validateLiveStateAuthority directly
		return ConfigReference{}, err
	}
	for _, e := range configspec.Keys() {
		if !fullProfile(p) && fullOnlyConfigKey(e.Path) {
			continue
		}
		row := ConfigKeyRow{
			Path: e.Path, Type: e.Type, Default: e.Default,
			Description: e.Description, Availability: e.Availability,
		}
		if !fullProfile(p) {
			switch e.Path {
			case "profile":
				row.Description = "Selects the Core governance footprint, which includes the operational workflow at the shared correctness, autonomy, maintainability, and review-quality bar."
			case "integrationBranch":
				row.Description = "The branch effort work integrates into. It must be non-empty and free of whitespace and must not start with `-`; slashes are legal, so `release/1.0` is accepted."
			case "localDocs":
				row.Description = "Additive repository-local documents. Each name is a lowercase kebab-case path below docs without .md; awf-managed names are reserved. Title and description are nonblank one-line metadata."
			case "audit.allowedScopes":
				row.Description = "The project's Conventional Commits scope taxonomy: the single home for commit scopes. Absent means accept any scope; entries are enforced by `awf check staged commit` and quoted by rendered guidance."
				row.Availability = "Read by `awf check staged commit` and every rendered artifact quoting the scope list."
			case "sidecar.data":
				row.Availability = "Keys must be referenced by the artifact's template. An unreferenced key is unranked Information and exits zero; rejected on the config-reference sidecar because its tables are generated."
			case "sidecar.sections":
				row.Availability = "Section names must be catalog-declared for the artifact; unknown names refuse at open."
			}
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
	potential, err := potentialVarConsumers(projectCatalog(p))
	if err != nil {
		return ConfigReference{}, err
	}
	for _, v := range configspec.VarEntries() {
		if !fullProfile(p) && (v.Key == "activeMdRegenCmd" || v.Key == "invariantTestPath") {
			continue
		}
		consumers := "No catalog artifact references it."
		if c := rendered[v.Key]; len(c) > 0 {
			consumers = "Consumed by: " + strings.Join(c, ", ") + "."
		} else if c := potential[v.Key]; len(c) > 0 {
			consumers = "Potential catalog consumers: " + strings.Join(c, ", ") + "; no rendered output currently references it."
		}
		ref.VarEntries = append(ref.VarEntries, VarRow{
			Key: v.Key, Description: v.Description, Availability: v.Availability,
			State: varState(p, v.Key), Consumers: consumers,
		})
	}

	dataKeys, err := dataKeyRowsTyped(p)
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
func configReferenceData(p renderInputs, files []RenderedFile) (map[string]any, error) {
	ref, err := configReferenceRows(p, files)
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
func fullOnlyConfigKey(path string) bool {
	return path == "domains" || path == "tags" || path == "contextIgnore" || path == "sidecar.paths" ||
		strings.HasPrefix(path, "currentState.") || strings.HasPrefix(path, "memoryCite.")
}

func dataKeyRowsTyped(p renderInputs) ([]DataKeyRow, error) {
	var rows []DataKeyRow
	for _, d := range configspec.DataKeys() {
		if d.Artifact != "agents-doc" {
			_, skill := projectCatalog(p).Skills[d.Artifact]
			_, agent := projectCatalog(p).Agents[d.Artifact]
			_, doc := projectCatalog(p).Docs[d.Artifact]
			if !skill && !agent && !doc {
				continue
			}
		}
		label := strings.TrimSuffix(d.Kind, "s") + " " + d.Artifact
		if d.Kind == "docs" && d.Artifact == "agents-doc" {
			label = "agents-doc"
		}
		state := ""
		var declared map[string]any
		switch d.Kind {
		case "skills":
			declared = projectCatalog(p).Skills[d.Artifact].Data
		case "agents":
			declared = projectCatalog(p).Agents[d.Artifact].Data
		case "docs":
			declared = projectCatalog(p).Docs[d.Artifact].Data
		}
		sidecarKind, sidecarName := d.Kind, d.Artifact
		if d.Artifact == "agents-doc" {
			sidecarKind, sidecarName = "agents-doc", ""
		}
		sc, err := p.cfg.Sidecar(sidecarKind, sidecarName)
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
		description := d.Description
		if !fullProfile(p) && d.Kind == "docs" && d.Artifact == "glossary" && d.Key == "terms" {
			description = "The glossary's terms as an ordered list of `{term, meaning}` records; the table renders always sorted (case-insensitive, pipes escaped), and an empty term or meaning, an interior newline, an unknown record key, or a case-insensitive duplicate term fails the render naming the offending term. A term here overrides the standard vocabulary awf ships of the same case-insensitive name. Unset, the doc renders the standard vocabulary alone."
		}
		if d.Kind == "agents" && d.Key == "focusItems" {
			var names []string
			if items, ok := defaultValue.([]any); ok {
				for _, item := range items {
					if record, ok := item.(map[string]any); ok {
						if name, ok := record["name"].(string); ok {
							names = append(names, "`"+name+"`")
						}
					}
				}
			}
			if len(names) > 0 {
				description = "The reviewer's project-focus lens items (list of {name, description}); the selected catalog default contains " + strings.Join(names, ", ") + "."
			}
		}
		rows = append(rows, DataKeyRow{Artifact: label, Key: d.Key, Description: description, State: state})
	}
	return rows, nil
}

// generateConfigReference renders the always-on generated config reference
// (ADR-class: generated index, no template/config hashes - drift is checked
// by regeneration). files is the consumption input (the plan write files plus
// generated domain docs).
func generateConfigReference(p renderInputs, files []RenderedFile, eff map[string]bool) (*RenderedFile, bool, error) {
	sc, err := p.cfg.Sidecar("config-reference", "")
	if err != nil { // coverage-ignore: validation already read this sidecar at open
		return nil, false, err
	}
	data := projectData(p, sc, eff)
	collections, err := configReferenceData(p, files)
	if err != nil { // coverage-ignore: configReferenceData errors only on faults earlier passes already surfaced
		return nil, false, err
	}
	data["data"] = collections
	rf, err := renderTarget(p, "config-reference", "", projectCatalog(p).Docs["config-reference"].TID,
		projectCatalog(p).Docs["config-reference"].Sections, sc, data, crefRel(p), eff,
		&renderOutputOptions{sources: []string{"derived:configspec", "derived:project-configuration"}})
	if err != nil {
		return nil, false, err
	}
	wrapped := RenderedFile{Path: rf.Path, Content: rf.Content,
		stubDefaults: rf.stubDefaults, stubParts: rf.stubParts,
		markerParts: rf.markerParts, assembled: rf.assembled,
		partVarRefs: rf.partVarRefs, kind: rf.kind, artifact: rf.artifact,
		RegenChecked: true, ConsumedInputs: rf.ConsumedInputs, ObservedTemplateID: rf.ObservedTemplateID, Encoder: rf.Encoder,
		Policy: OutputPolicy{Regenerate: true, ScanReferences: true, ScanSkillReferences: true}}
	if templateSourceRoot(p) != "" {
		wrapped.TemplateID, wrapped.TemplateHash, wrapped.ConfigHash = rf.TemplateID, rf.TemplateHash, rf.ConfigHash
	}
	return &wrapped, true, nil
}

// ConfigReferenceModel computes the reference's four typed collections
// (ConfigKeys, VarEntries, SidecarFields, DataKeys) with live project state -
// the `awf config` command's data source.
func configReferenceModel(p renderInputs, ctx context.Context) (ConfigReference, error) {
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return ConfigReference{}, err
	}
	op, err := outputPlanWithPitfalls(p, ctx, corpus, pitfalls, topics, eff)
	if err != nil {
		return ConfigReference{}, err
	}
	dds, err := generateDomainDocs(p, topics, eff)
	if err != nil { // coverage-ignore: the same producer ran inside outputPlan above over these identical inputs, so a second call cannot newly fail
		return ConfigReference{}, err
	}
	return configReferenceRows(p, slices.Concat(op.writeFiles(), dds))
}
