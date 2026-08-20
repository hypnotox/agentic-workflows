package project

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/refs"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// CheckAdvisories separates ranked warnings from unranked information without
// adding another finding rank.
type CheckAdvisories struct {
	Warnings    []string
	Information []string
}

// AdvisoryNotes returns the compatibility projection of the non-failing notes
// produced by one operation-scoped plan parse.
func advisoryNotes(p renderInputs, ctx context.Context) ([]string, error) {
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return nil, err
	}
	plans := []plan.Plan{}
	if fullProfile(p) {
		plans, err = plan.ParseDir(filepath.Join(p.root(), config.DocsDir, "plans"))
		if err != nil {
			return nil, err
		}
	}
	op, err := outputPlanWithPitfalls(p, ctx, corpus, pitfalls, topics, eff)
	if err != nil {
		return nil, err
	}
	advisories, err := advisoryNotesWithState(p, pitfalls, plans, op)
	if err != nil { // coverage-ignore: operation state and sidecars were already parsed and validated before advisory projection
		return nil, err
	}
	return append(slices.Clone(advisories.Warnings), advisories.Information...), nil
}

// advisoryNotesWithState classifies the non-failing render advisories from
// operation-owned state, its already parsed plans, and its one prepared output
// plan.
func advisoryNotesWithState(p renderInputs, pitfalls pitfall.Corpus, plans []plan.Plan, op *OutputPlan) (CheckAdvisories, error) {
	files := op.writeFiles()
	all := advisoryCompatibilityFiles(op)
	information := append(unsetVarNotes(p, files), stubNotes(all)...)
	information = append(information, markerNotes(all)...)
	warnings, err := tagHealthNotes(p, pitfalls)
	if err != nil { // coverage-ignore: advisory read errors are covered by direct helper tests
		return CheckAdvisories{}, err
	}
	information = append(information, planCommitScopeNotes(p, plans)...)
	glossaryWarnings, err := glossaryTersenessNotes(p)
	if err != nil {
		return CheckAdvisories{}, err
	}
	warnings = append(warnings, glossaryWarnings...)
	return CheckAdvisories{Warnings: warnings, Information: information}, nil
}

// advisoryCompatibilityFiles preserves the established stub-note multiplicity
// without reconstructing output producers. Before CheckReport shared one plan,
// advisory preparation appended a second generated domain and config-reference
// set to the plan's write files. Successful command output keeps that cardinality
// by reusing those immutable plan nodes while every artifact is still produced
// exactly once.
func advisoryCompatibilityFiles(op *OutputPlan) []RenderedFile {
	files := op.writeFiles()
	all := slices.Clone(files)
	for _, node := range op.Nodes {
		if node.file == nil {
			continue
		}
		if slices.Contains(node.Declarers, "generated-domain") || slices.Contains(node.Declarers, "generated-config-reference") {
			all = append(all, *node.file)
		}
	}
	return all
}

// glossaryTersenessNotes returns advisory (non-failing) notes for each glossary
// term whose meaning exceeds glossaryMeaningMax. It evaluates the MERGED set,
// so the threshold bounds the vocabulary awf ships as well as the project's own
// terms (ADR-0207 decision 10). Inert when the glossary doc is disabled.
func glossaryTersenessNotes(p renderInputs) ([]string, error) {
	sc, err := p.cfg.Sidecar("docs", "glossary")
	if err != nil { // coverage-ignore: the glossary sidecar's YAML was already parsed and validated at Open, so this re-read cannot fail
		return nil, err
	}
	// The on-disk sidecar never carries standardTerms, so overlay the catalog
	// default exactly as render.go does upstream of the transform; without this
	// the shipped layer would escape the threshold entirely.
	records, err := mergedGlossaryRecords(withDefaultData(sc, projectCatalog(p).Docs["glossary"].Data, specializedListDataKeys("docs", "glossary")...))
	if err != nil {
		return nil, err
	}
	slices.SortFunc(records, func(a, b glossaryRecord) int {
		return strings.Compare(strings.ToLower(a.Term), strings.ToLower(b.Term))
	})
	var notes []string
	for _, r := range records {
		// Runes, not bytes: the guideline is a reading-length notion, and accented
		// letters are ordinary text.
		if n := utf8.RuneCountInString(r.Meaning); n > glossaryMeaningMax {
			notes = append(notes, fmt.Sprintf("%s: term %q meaning is %d characters, over the %d-character guideline; tighten it", glossarySidecarPath, r.Term, n, glossaryMeaningMax))
		}
	}
	return notes, nil
}

// tagFrequencyThreshold is the share of tag-bearing artifacts above which a tag
// is flagged as coarsening toward domain scale (ADR-0109 item 4). Advisory only;
// a documented constant, deliberately not a config key in this slice.
const tagFrequencyThreshold = 0.25

// tagHealthNotes returns advisory (non-failing) notes about the current pitfall
// tag vocabulary: a frequency note for any tag carried by more than
// tagFrequencyThreshold of the tag-bearing pitfalls, and a coverage note for
// any pitfall carrying zero tags. Legacy ADR tags remain parsed history but do
// not participate in current vocabulary health. Inert under an empty/absent
// vocabulary, so an un-curated adopter - and the example - stays note-free.
func tagHealthNotes(p renderInputs, supplied ...pitfall.Corpus) ([]string, error) {
	if len(p.cfg.Tags) == 0 {
		return nil, nil
	}
	pitfalls, err := compatPitfallCorpus(p, supplied)
	if err != nil { // coverage-ignore: aggregate operations always supply the validated corpus; direct malformed-load propagation is covered separately
		return nil, err
	}

	var notes []string
	tagged := 0
	freq := map[string]int{}
	for _, entry := range pitfalls.All() {
		if len(entry.Tags) == 0 {
			notes = append(notes, entry.SourcePath+" carries no tags: add a narrow topic tag")
			continue
		}
		// Count only vocabulary members - both the numerator and the denominator.
		// The invariant speaks of "vocabulary tags" and "pitfalls carrying at
		// least one vocabulary tag", and a non-member tag is already a hard
		// checkTagVocabulary failure, so it must not skew the coarsening signal.
		var vocab []string
		for _, tag := range entry.Tags {
			if _, ok := p.cfg.Tags[tag]; ok {
				vocab = append(vocab, tag)
			}
		}
		if len(vocab) == 0 {
			continue
		}
		tagged++
		for _, tag := range vocab {
			freq[tag]++
		}
	}
	// Empty-denominator guard: no tag-bearing artifacts, no frequency to compute.
	if tagged > 0 {
		for _, t := range slices.Sorted(maps.Keys(freq)) {
			if float64(freq[t]) > tagFrequencyThreshold*float64(tagged) {
				notes = append(notes, fmt.Sprintf("tag %q is on %d/%d tagged pitfalls (>%.0f%%): coarsening toward domain scale", t, freq[t], tagged, tagFrequencyThreshold*100))
			}
		}
	}
	return notes, nil
}

// unsetVarNotes reports, per rendered artifact, the vars its assembled template
// references whose key is present in config with an empty or null value - the
// non-failing render-completeness advisory (ADR-0045 item 4, narrowed by
// ADR-0087: an absent key is the deliberate, git-auditable decline and produces
// no note; deleting the key is the acknowledgement). One line per artifact with
// at least one hit, sorted. Adapter duplicates collapse by the note itself.
func unsetVarNotes(p renderInputs, files []RenderedFile) []string {
	seen := map[string]bool{}
	var notes []string
	for _, f := range files {
		var unset []string
		for _, r := range render.ReferencedVars(f.assembled) {
			if v, ok := p.cfg.Vars[r]; ok && (v == nil || v == "") {
				unset = append(unset, r)
			}
		}
		if len(unset) == 0 {
			continue
		}
		label := artifactLabel(f.TemplateID)
		note := fmt.Sprintf("%s references unset vars: %s; set a value, or delete the key to accept the generic prose",
			label, strings.Join(unset, ", "))
		if seen[note] {
			continue
		}
		seen[note] = true
		notes = append(notes, note)
	}
	sort.Strings(notes)
	return notes
}

// stubNotes reports, per rendered artifact, its unauthored stub content -
// stub-attributed sections still at default and awf:stub-marked parts. One line
// per output path: artifacts sharing a template id, including domain docs,
// each report independently, and a multi-target project prints one line
// per target path by design (ADR-0070).
// touches-state: rendering/doc-outputs:stub-notes-path-keyed - per-output-path stub note; proof in notes_test.go
func stubNotes(files []RenderedFile) []string {
	var notes []string
	for _, f := range files {
		if len(f.stubDefaults) == 0 && len(f.stubParts) == 0 {
			continue
		}
		var clauses []string
		if len(f.stubDefaults) > 0 {
			clauses = append(clauses, "sections at stub default: "+strings.Join(f.stubDefaults, ", "))
		}
		if len(f.stubParts) > 0 {
			clauses = append(clauses, "stub-marked parts: "+strings.Join(f.stubParts, ", "))
		}
		notes = append(notes, fmt.Sprintf("%s has unauthored stub content: %s",
			f.Path, strings.Join(clauses, "; ")))
	}
	sort.Strings(notes)
	return notes
}

// markerNotes reports each convention part whose raw body carries a whole-line
// section-marker residue - the ADR-0083 advisory. Keyed by the part path, a
// deliberate deviation from stubNotes' output-path keying (the actionable file
// is the part itself), and deduplicated: multi-target rendering consumes the
// same part once per target and must not repeat its note.
func markerNotes(files []RenderedFile) []string {
	seen := map[string]bool{}
	var notes []string
	for _, f := range files {
		for _, part := range f.markerParts {
			if seen[part] {
				continue
			}
			seen[part] = true
			notes = append(notes, fmt.Sprintf("part %s contains a marker-shaped line: section markers have no effect inside convention parts; fence the example to silence this note", part))
		}
	}
	sort.Strings(notes)
	return notes
}

// unusedVarDrift reports each non-empty vars: key referenced by no rendered
// artifact - neither a .vars.X reference in any assembled source (the render pass
// output and the generated domain docs, passed concatenated) nor a
// gateCmd/checkCmd part placeholder (ADR-0086 Decision 3). Empty values are
// exempt: they are the ADR-0022 seeded open-to-do state, which the unset-var
// note owns nudging (ADR-0087 - presence, not emptiness, is that note's
// trigger; this exemption keeps the seed-all-vars scaffold legal). A bare
// .vars reference conservatively consumes every key.
func unusedVarDrift(p renderInputs, files []RenderedFile) []manifest.Drift {
	used := map[string]bool{}
	for _, f := range files {
		if render.ReferencesBareVars(f.assembled) {
			return nil
		}
		for _, r := range render.ReferencedVars(f.assembled) {
			used[r] = true
		}
		for _, r := range f.partVarRefs {
			used[r] = true
		}
	}
	var drift []manifest.Drift
	for _, k := range slices.Sorted(maps.Keys(p.cfg.Vars)) {
		if v := p.cfg.Vars[k]; v == nil || v == "" || used[k] {
			continue
		}
		drift = append(drift, manifest.Drift{
			Path: config.DirName + "/config.yaml", Kind: "unused-var",
			Detail: fmt.Sprintf("var %q is set but referenced by no rendered artifact; delete it from vars: or enable an artifact that consumes it", k),
		})
	}
	return drift
}

// unusedDataDrift reports, per enabled artifact, the sidecar data: keys its
// assembled sources reference nowhere, unioned across enabled targets
// (ADR-0086 Decision 4). Domains are excluded - their sidecars are rejected
// as paths-only at open. A key referenced only inside a dropped section counts as
// unused: the drop makes it configuration that does nothing.
func unusedDataDrift(p renderInputs, files []RenderedFile) ([]manifest.Drift, error) {
	type refset struct {
		keys map[string]bool
		bare bool
	}
	refs := map[string]*refset{}
	for _, f := range files {
		key := f.kind + "\x00" + f.artifact
		rs := refs[key]
		if rs == nil {
			rs = &refset{keys: map[string]bool{}}
			refs[key] = rs
		}
		for _, k := range render.ReferencedDataKeys(f.assembled) {
			rs.keys[k] = true
		}
		rs.bare = rs.bare || render.ReferencesBareData(f.assembled)
	}
	var drift []manifest.Drift
	check := func(kind, name, sidecarRel string) error {
		sc, err := p.cfg.Sidecar(kind, name)
		if err != nil { // coverage-ignore: this sidecar was already read by the render pass in outputPlan (or by validation) in the same Check
			return err
		}
		if len(sc.Data) == 0 {
			return nil
		}
		rs := refs[kind+"\x00"+name]
		if rs != nil && rs.bare {
			return nil
		}
		var unused []string
		for _, k := range slices.Sorted(maps.Keys(sc.Data)) {
			if rs == nil || !rs.keys[k] {
				unused = append(unused, k)
			}
		}
		if len(unused) == 0 {
			return nil
		}
		detail := "data keys referenced by no rendered section: " + strings.Join(unused, ", ") + "; a key referenced only inside a dropped section counts as unused; remove the key or the drop"
		drift = append(drift, manifest.Drift{Path: sidecarRel, Kind: "unused-data", Detail: detail})
		return nil
	}
	for _, d := range kindDescriptors {
		if d.Plural == "domains" {
			continue
		}
		for _, name := range d.poolNames(projectCatalog(p)) {
			if err := check(d.Plural, name, config.DirName+"/"+d.Plural+"/"+name+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
				return nil, err
			}
		}
	}
	for _, kind := range catalog.SingletonKindsFor(projectCatalog(p)) {
		if err := check(kind, "", config.DirName+"/"+kind+".yaml"); err != nil { // coverage-ignore: see check's coverage-ignore
			return nil, err
		}
	}
	return drift, nil
}

// artifactLabel derives a human label from a template id: catalog kinds get
// "<kind> <name>" ("skill tdd", "agent code-reviewer", "doc testing"), hook
// payloads their script ("hooks pre-commit" - ADR-0048); the singletons read
// as their kind ("agents-doc").
func artifactLabel(tid string) string {
	segs := strings.Split(tid, "/")
	switch segs[0] {
	case "skills", "agents", "docs":
		name := segs[1]
		if segs[0] != "skills" {
			name = strings.TrimSuffix(name, ".md.tmpl")
		}
		return strings.TrimSuffix(segs[0], "s") + " " + name
	case "hooks":
		return "hooks " + strings.TrimSuffix(segs[1], ".sh.tmpl")
	default:
		return segs[0]
	}
}

// declaredSections returns the catalog-declared section names for a target.
func declaredSections(p renderInputs, kind, name string) []string {
	if d, ok := descriptorByPlural(kind); ok && d.sections != nil {
		s, _ := d.sections(projectCatalog(p), name)
		return s
	}
	return nil
}

// CheckReport is the ordinary check operation's drift, ranked warnings, and
// unranked information, derived from one operation-owned plan parse.
type CheckReport struct {
	Drift               []manifest.Drift
	Warnings            []string
	Information         []string
	TrackingInformation []string
	PlanWarnings        []string
	// Notes, TrackingNotes, and PlanNotes retain the pre-classification test and
	// API projection. New operation reports set classified and use the fields above.
	Notes         []string
	TrackingNotes []string
	PlanNotes     []string
	classified    bool
}

// OrdinaryWarnings returns ranked non-plan aggregate warning notes.
func (r CheckReport) OrdinaryWarnings() []string {
	if !r.classified {
		return slices.Clone(r.Notes)
	}
	return slices.Clone(r.Warnings)
}

// PlanWarningNotes returns ranked plan warning notes for sink deduplication.
func (r CheckReport) PlanWarningNotes() []string {
	if !r.classified {
		return slices.Clone(r.PlanNotes)
	}
	return slices.Clone(r.PlanWarnings)
}

// AggregateInformation returns unranked aggregate information notes.
func (r CheckReport) AggregateInformation() []string {
	if !r.classified {
		return nil
	}
	return slices.Clone(r.Information)
}

// DirectTrackingInformation returns unranked tracking information shown by
// both direct drift and aggregate checks.
func (r CheckReport) DirectTrackingInformation() []string {
	if !r.classified {
		return slices.Clone(r.TrackingNotes)
	}
	return slices.Clone(r.TrackingInformation)
}

const agentGuideAdvisoryBytes = 12 * 1024

// agentGuideSizeAdvisory reports the fixed aggregate-check guide-size advisory
// from the deterministic managed output, never a resident file.
func agentGuideSizeAdvisory(op *OutputPlan) []string {
	for _, file := range op.writeFiles() {
		if file.Path != "AGENTS.md" || len(file.Content) <= agentGuideAdvisoryBytes {
			continue
		}
		return []string{fmt.Sprintf("AGENTS.md is %d bytes, allowed %d bytes; see docs/agents-md-standard.md", len(file.Content), agentGuideAdvisoryBytes)}
	}
	return nil
}

// CheckReport performs one ordinary project check. Plans are parsed once and
// the typed set is threaded to both blocking and advisory consumers.
func checkReport(p renderInputs, repo *awfgit.Repo, ctx context.Context) (CheckReport, error) {
	if err := validateCommandWiring(p.cfg); err != nil {
		return CheckReport{}, err
	}
	corpus, pitfalls, topics, eff, err := deriveOperationStateWithPitfalls(p)
	if err != nil {
		return CheckReport{}, err
	}
	plans := []plan.Plan{}
	var parseErr error
	if fullProfile(p) {
		plans, parseErr = plan.ParseDir(filepath.Join(p.root(), config.DocsDir, "plans"))
	}
	var planDrift []manifest.Drift
	if parseErr != nil {
		var diagnostics *plan.DiagnosticsError
		if !errors.As(parseErr, &diagnostics) {
			return CheckReport{}, parseErr
		}
		rel := filepath.ToSlash(filepath.Join(config.DocsDir, "plans"))
		for _, diagnostic := range diagnostics.Diagnostics {
			planDrift = append(planDrift, manifest.Drift{
				Path: rel + "/" + diagnostic.Path, Kind: "plan-" + diagnostic.Category, Detail: diagnostic.Detail,
			})
		}
	}
	op, err := outputPlanWithPitfalls(p, ctx, corpus, pitfalls, topics, eff)
	if err != nil {
		return CheckReport{}, err
	}
	drift, trackingNotes, err := checkWithTrackingState(p, repo, ctx, corpus, pitfalls, topics, eff, plans, op)
	if err != nil {
		return CheckReport{}, err
	}
	planWarnings := []string(nil)
	if fullProfile(p) {
		contextDrift, notes := planArtifactReport(plans, corpus)
		planWarnings = notes
		planDrift = append(planDrift, contextDrift...)
	}
	advisories, err := advisoryNotesWithState(p, pitfalls, plans, op)
	report, err := finishCheckReport(drift, planDrift, planWarnings, advisories, op, err)
	report.TrackingInformation = trackingNotes
	report.TrackingNotes = slices.Clone(trackingNotes)
	return report, err
}

func finishCheckReport(drift, planDrift []manifest.Drift, planWarnings []string, advisories CheckAdvisories, op *OutputPlan, err error) (CheckReport, error) {
	if err != nil {
		return CheckReport{}, err
	}
	advisories.Warnings = append(advisories.Warnings, agentGuideSizeAdvisory(op)...)
	allNotes := append(slices.Clone(advisories.Information), advisories.Warnings...)
	return CheckReport{
		Drift: append(drift, planDrift...), Warnings: advisories.Warnings,
		Information: advisories.Information, PlanWarnings: planWarnings, Notes: allNotes,
		PlanNotes: slices.Clone(planWarnings), classified: true,
	}, nil
}

func checkWithTrackingState(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool, plans []plan.Plan, op *OutputPlan) ([]manifest.Drift, []string, error) {
	trackingDrift, trackingNotes, err := checkGeneratedTracking(p.isNested(), repo, ctx, op)
	if err != nil {
		return nil, nil, err
	}
	lock, found, err := manifest.LoadOptional(lockPath(p.root()))
	if err != nil {
		return nil, nil, err
	}
	if !found {
		if len(trackingDrift) > 0 {
			return trackingDrift, trackingNotes, nil
		}
		return nil, nil, errors.New("no lock (run awf render)")
	}
	files := op.writeFiles()
	rendered := map[string]RenderedFile{}
	for _, f := range files {
		rendered[f.Path] = f
	}
	var drift []manifest.Drift
	drift = append(drift, trackingDrift...)
	drift = append(drift, checkLockedFiles(p.residentRoots(), lock, rendered, trackingDrift)...)
	// Closed-tree sweep: orphans, strays, backups (ADR-0086 Decision 1).
	od, err := sweepConfigTree(p, files, topics)
	if err != nil { // coverage-ignore: the sweep errors only on faults the outputPlan render above would have surfaced first (see its coverage-ignores)
		return nil, nil, err
	}
	drift = append(drift, od...)

	// Generated and ordinary outputs are already exactly the plan write nodes;
	// do not regenerate a second, duplicate node set in Check.
	drift = append(drift, unusedVarDrift(p, files)...)
	ud, err := unusedDataDrift(p, files)
	if err != nil { // coverage-ignore: unusedDataDrift re-reads sidecars the render pass in outputPlan already read
		return nil, nil, err
	}
	drift = append(drift, ud...)

	drift = append(drift, checkDeadRefs(p, files)...)
	drift = append(drift, checkDeadSkillRefs(p, files, eff)...)

	if fullProfile(p) {
		drift = append(drift, checkPlans(p, corpus, plans)...)
	}
	pitfallDrift, err := checkPitfalls(p, corpus, pitfalls)
	if err != nil { // coverage-ignore: the operation supplied its already validated pitfall corpus
		return nil, nil, err
	}
	drift = append(drift, pitfallDrift...)
	glossaryDrift, err := checkGlossary(p)
	if err != nil {
		return nil, nil, err
	}
	drift = append(drift, glossaryDrift...)
	tagDrift, err := checkTagVocabulary(p, pitfalls)
	if err != nil { // coverage-ignore: checkTagVocabulary now fails only through pitfallTagEntries, which reads the same data.pitfalls that checkPitfalls above already read and failed on
		return nil, nil, err
	}
	drift = append(drift, tagDrift...)
	if fullProfile(p) {
		drift = append(drift, checkADRRelatedLinks(corpus)...)
		drift = append(drift, checkPendingADRs(p, repo, ctx, corpus)...)
	}
	return drift, trackingNotes, nil
}

// checkGeneratedTracking compares the operation-owned render writes plus the
// separately written lock against metadata-only index membership. Resident
// outputs are outside a nested adopter's index authority and remain excluded.
func checkGeneratedTracking(nested bool, repo *awfgit.Repo, ctx context.Context, op *OutputPlan) ([]manifest.Drift, []string, error) {
	if repo == nil {
		return nil, []string{"generated-artifact tracking is unavailable outside a Git repository"}, nil
	}
	indexed, err := repo.IndexPaths(ctx)
	if err != nil {
		return nil, nil, err
	}
	present := make(map[string]bool, len(indexed))
	for _, path := range indexed {
		present[path] = true
	}
	required := map[string]bool{config.DirName + "/awf.lock": true}
	for _, file := range op.writeFiles() {
		if nested && resident.IsResidentPath(file.Path) {
			continue
		}
		required[file.Path] = true
	}
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(required)) {
		if !present[path] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "untracked", Detail: "generated artifact is absent from the Git index; run awf render, then git add -f " + path})
		}
	}
	return drift, nil, nil
}

func untrackedPaths(drift []manifest.Drift) map[string]bool {
	paths := map[string]bool{}
	for _, finding := range drift {
		if finding.Kind == "untracked" {
			paths[finding.Path] = true
		}
	}
	return paths
}

// checkPendingADRs refuses a slug-identified pending record on the integration
// branch. Numbering happens at integration, so a pending record that reached
// the integration branch was never numbered, and every `ADR-<slug>` provenance
// reference it left behind resolves to nothing.
//
// The block fires only on a positive branch identification (ADR-0202 item 7):
// a detached HEAD, another branch, or an unreadable repository emits nothing,
// because an indeterminate answer is not evidence that the record is in the
// wrong place. That deliberately leaves automated detached-HEAD runs to the
// branch-independent duplicate-identity check, which is the real corruption
// backstop; this check exists to make the missing numbering step visible where
// it is actually owed.
func checkPendingADRs(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus) []manifest.Drift {
	if !onIntegrationBranch(p.root(), p.cfg, repo, ctx) {
		return nil
	}
	rel := filepath.ToSlash(filepath.Join(config.DocsDir, "decisions"))
	var drift []manifest.Drift
	for _, a := range corpus.All() {
		if a.Number != "" {
			continue
		}
		drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "pending-adr-on-integration-branch", Detail: a.Slug})
	}
	return drift
}

// checkLockedFiles compares each lock entry (except the separately-checked
// regeneration-checked artifacts - the generated INDEX.md / domain docs / config
// reference) against the freshly-rendered output and the on-disk file: orphaned,
// stale, missing, hand-edited, or invalid-frontmatter. The reverse direction is
// checked too: a rendered path with no lock entry - an artifact enabled since the
// last sync - is flagged unsynced rather than silently skipped.
func checkLockedFiles(roots resident.Roots, lock *manifest.Lock, rendered map[string]RenderedFile, trackingDrift []manifest.Drift) []manifest.Drift {
	untracked := untrackedPaths(trackingDrift)
	var drift []manifest.Drift
	for _, path := range slices.Sorted(maps.Keys(rendered)) {
		if _, ok := lock.Files[path]; !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "unsynced", Detail: "enabled but not in lock; run awf render"})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(lock.Files)) {
		e := lock.Files[path]
		rf, ok := rendered[path]
		if rf.Policy.Regenerate {
			// Every regeneration-checked path is a planned rendered node. Compare
			// it once here rather than reconstructing generated outputs elsewhere.
			if !ok { // coverage-ignore: full Check first builds the complete planned node set; only a direct malformed lock/map call can omit a regeneration node.
				drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
				continue
			}
			onDisk, err := os.ReadFile(roots.ResolveOutput(path))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) && untracked[path] {
					continue
				}
				drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf render"})
				continue
			}
			if manifest.Hash(onDisk) != manifest.Hash([]byte(rf.Content)) {
				if rf.TemplateID == "" {
					drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "generated output out of date; run awf render"})
				} else {
					// touches-state: rendering/inplace-and-placeholders:in-place-tamper-drift - awf-region/structure edit drifts, in-place edit does not; proof in check_test.go
					drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from the regenerated file; run awf render to restore awf-owned regions"})
				}
			}
			continue
		}
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if rf.TemplateHash != e.TemplateHash || rf.ConfigHash != e.ConfigHash {
			// stale takes precedence: a re-sync overwrites any hand-edit, so it
			// is the actionable signal - one drift entry per path.
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "template or config changed; run awf render"})
			continue
		}
		if finding, found := classifyFrozenOutputFreshness(rf, e); found {
			drift = append(drift, finding)
			continue
		}
		onDisk, err := os.ReadFile(roots.ResolveOutput(path))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && untracked[path] {
				continue
			}
			drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf render"})
			continue
		}
		if finding, found := classifyFrozenObservedDrift(rf, e, onDisk, "on-disk output differs from lock; run awf render to discard the edit, or move it into a .awf convention part to keep it"); found {
			drift = append(drift, finding)
			continue
		}
		// In-sync skill/agent files must still carry valid frontmatter (subordinate
		// to the hash kinds above - a re-sync is the fix for those).
		if rf.Policy.ValidateFrontmatter {
			if err := validateArtifact(onDisk, rf.Encoder); err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "invalid-frontmatter", Detail: err.Error()})
			}
		}
	}
	return drift
}

// checkDeadSkillRefs scans managed rendered markdown for <prefix>-<name> tokens
// whose <name> is a catalog-known skill outside the effective rendered set.
// Names matching no known skill are ignored
// (inv: skill-ref-unknown-ignored); fenced code blocks are skipped like the
// dead-link scan. Matching is whole-token (ADR-0046 item 3): the token must not
// start mid-word (no word-ish rune before the prefix) and the regex captures
// the maximal word run after it.
func checkDeadSkillRefs(p renderInputs, files []RenderedFile, effective map[string]bool) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files))
	for _, f := range files {
		if f.Policy.ScanSkillReferences {
			scan = append(scan, f)
		}
	}
	re := regexp.MustCompile(`(?:^|[^a-zA-Z0-9_-])` + regexp.QuoteMeta(p.cfg.Prefix) + `-([a-z0-9]+(?:-[a-z0-9]+)*)`)
	var drift []manifest.Drift
	for _, f := range scan {
		seen := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(refs.WithoutFences(f.Content), -1) {
			name := m[1]
			if _, known := projectCatalog(p).Skills[name]; !known || effective[name] || seen[name] {
				continue
			}
			seen[name] = true
			drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-skill-reference", Detail: p.cfg.Prefix + "-" + name})
		}
	}
	return drift
}

// checkDeadRefs runs the dead-reference scan (inv: dead-reference-gated): every
// awf-managed rendered markdown file's inline links must resolve file-relative on
// disk. Generated nodes use the same declared policy; bridges remain out of
// scope through theirs.
func checkDeadRefs(p renderInputs, files []RenderedFile) []manifest.Drift {
	scan := make([]RenderedFile, 0, len(files))
	for _, f := range files {
		if f.Policy.ScanReferences {
			scan = append(scan, f)
		}
	}
	var drift []manifest.Drift
	for _, f := range scan {
		base := filepath.Dir(f.Path)
		for _, target := range refs.Links(f.Content) {
			// A leading-/ target is repo-root-relative; everything else resolves
			// file-relative. A target escaping the root is dead by definition -
			// a host path outside the repo must never validate it.
			resolved := filepath.Join(p.root(), base, target)
			if strings.HasPrefix(target, "/") {
				resolved = filepath.Join(p.root(), target)
			}
			if rel, err := filepath.Rel(p.root(), resolved); err != nil || (rel != "." && !filepath.IsLocal(rel)) {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
				continue
			}
			if _, err := os.Stat(resolved); err != nil {
				drift = append(drift, manifest.Drift{Path: f.Path, Kind: "dead-reference", Detail: target})
			}
		}
	}
	return drift
}

// checkPlans validates plan frontmatter, plan→ADR links, and planned commit
// subjects over docs/plans/, scanning the YYYY-MM-DD-*.md set only (excluding
// template.md and README.md). Frontmatter-less plans (the grandfathered corpus,
// ADR-0098) are skipped. A ```commit subject's length/type/shape violation is
// drift; an unknown scope is advisory (planCommitScopeNotes), not drift (ADR-0111).
// An adrs: entry resolves by identity, so a number and a pending record's slug
// resolve through one lookup and a link survives numbering (ADR-0202 item 14).
func checkPlans(p renderInputs, corpus adr.Corpus, plans []plan.Plan) []manifest.Drift {
	aset := audit.Resolve(config.AuditScopes(p.cfg.Audit))
	rel := filepath.ToSlash(filepath.Join(config.DocsDir, "plans"))
	var drift []manifest.Drift
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		path := rel + "/" + pl.Filename
		if !plan.ValidStatuses[pl.Status] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "plan-frontmatter", Detail: fmt.Sprintf("status %q not in {Proposed, Implemented}", pl.Status)})
		}
		for _, link := range pl.ADRs {
			id := link.Identity()
			if _, ok := corpus.ByIdentity(id); !ok {
				drift = append(drift, manifest.Drift{Path: path, Kind: "plan-adr-link", Detail: "ADR-" + id})
			}
		}
		for _, sub := range pl.CommitSubjects {
			for _, f := range audit.CheckPlannedSubject(sub, aset) {
				if f.Severity == severity.Error {
					drift = append(drift, manifest.Drift{Path: path, Kind: "plan-commit-subject", Detail: f.Detail})
				}
			}
		}
	}
	return drift
}

// planCommitScopeNotes returns advisory (non-failing) notes for a plan's ```commit
// subject naming a scope outside the configured allow-list. Unlike an over-length or
// mistyped subject (hard drift in checkPlans), an unknown scope is advisory: a plan
// may be the change that adds the scope (ADR-0111). Mirrors checkPlans' scan; a
// frontmatter-less plan is skipped.
func planCommitScopeNotes(p renderInputs, plans []plan.Plan) []string {
	aset := audit.Resolve(config.AuditScopes(p.cfg.Audit))
	rel := filepath.ToSlash(filepath.Join(config.DocsDir, "plans"))
	var notes []string
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		for _, sub := range pl.CommitSubjects {
			for _, f := range audit.CheckPlannedSubject(sub, aset) {
				if f.Severity == severity.Warn {
					notes = append(notes, fmt.Sprintf("%s/%s: planned commit %s", rel, pl.Filename, f.Detail))
				}
			}
		}
	}
	return notes
}

// checkPitfalls resolves corpus metadata against project-owned domains and ADRs.
func checkPitfalls(p renderInputs, corpus adr.Corpus, supplied ...pitfall.Corpus) ([]manifest.Drift, error) {
	pitfalls, err := compatPitfallCorpus(p, supplied)
	if err != nil { // coverage-ignore: aggregate operations always supply their validated operation-owned corpus
		return nil, err
	}
	domains := map[string]bool{}
	for _, d := range p.cfg.Domains {
		domains[d] = true
	}
	var drift []manifest.Drift
	for _, e := range pitfalls.All() {
		for _, d := range e.Domains {
			if !domains[d] {
				drift = append(drift, manifest.Drift{Path: e.SourcePath, Kind: "pitfall-domain", Detail: fmt.Sprintf("%s (%q): unknown domain %q", e.Slug, e.Title, d)})
			}
		}
		for _, n := range e.Related {
			if !corpus.Has(fmt.Sprintf("%04d", n)) {
				drift = append(drift, manifest.Drift{Path: e.SourcePath, Kind: "pitfall-adr-link", Detail: fmt.Sprintf("%s (%q): ADR-%04d", e.Slug, e.Title, n)})
			}
		}
	}
	return drift, nil
}

// checkGlossary validates the glossary sidecar when the doc is enabled: each
// record's domains: must resolve to a configured domain, mirroring checkPitfalls.
// Structural validation (term/meaning) is the transform's job; this resolves the
// domains the transform cannot see. A disabled glossary doc, or a sidecar with no
// data.terms, yields no drift.
func checkGlossary(p renderInputs) ([]manifest.Drift, error) {
	sc, err := p.cfg.Sidecar("docs", "glossary")
	if err != nil { // coverage-ignore: the glossary sidecar's YAML was already parsed and validated at Open, so this re-read cannot fail
		return nil, err
	}
	records, err := glossaryRecords(sc.Data["terms"])
	if err != nil {
		return nil, err
	}
	domains := map[string]bool{}
	for _, d := range p.cfg.Domains {
		domains[d] = true
	}
	var drift []manifest.Drift
	for _, r := range records {
		for _, d := range r.Domains {
			if !domains[d] {
				drift = append(drift, manifest.Drift{Path: glossarySidecarPath, Kind: "glossary-domain", Detail: fmt.Sprintf("%q: unknown domain %q", r.Term, d)})
			}
		}
	}
	return drift, nil
}

// checkTagVocabulary validates current tag governance when the config tags:
// vocabulary is non-empty: every tag used by a pitfall must be a declared
// vocabulary member, and every member must declare a non-empty meaning. Legacy
// ADR tags remain parsed history and do not require current membership. An empty
// or absent vocabulary is inert (tags are then free-form). A declared member no
// pitfall uses is intentionally permitted for generic adopters, mirroring an
// unused configured domain under pitfall-domains-resolved.
func checkTagVocabulary(p renderInputs, supplied ...pitfall.Corpus) ([]manifest.Drift, error) {
	if len(p.cfg.Tags) == 0 {
		return nil, nil
	}
	pitfalls, err := compatPitfallCorpus(p, supplied)
	if err != nil {
		return nil, err
	}
	cfgPath := config.DirName + "/config.yaml"
	domainName := map[string]bool{}
	for _, d := range p.cfg.Domains {
		domainName[d] = true
	}
	var drift []manifest.Drift
	for _, tag := range slices.Sorted(maps.Keys(p.cfg.Tags)) {
		if strings.TrimSpace(p.cfg.Tags[tag]) == "" {
			drift = append(drift, manifest.Drift{Path: cfgPath, Kind: "tag-vocabulary", Detail: fmt.Sprintf("tag %q has an empty meaning", tag)})
		}
		// A tag must be finer than a domain (ADR-0109): a vocabulary member that
		// names a configured domain is the coarse-tag regression, gated exactly.
		if domainName[tag] {
			drift = append(drift, manifest.Drift{Path: cfgPath, Kind: "tag-domain-collision", Detail: fmt.Sprintf("tag %q equals a configured domain name: tags must be finer than domains", tag)})
		}
	}
	for _, e := range pitfalls.All() {
		for _, tag := range e.Tags {
			if _, ok := p.cfg.Tags[tag]; !ok {
				drift = append(drift, manifest.Drift{Path: e.SourcePath, Kind: "pitfall-tag", Detail: fmt.Sprintf("%s (%q): unknown tag %q", e.Slug, e.Title, tag)})
			}
		}
	}
	return drift, nil
}

func compatPitfallCorpus(p renderInputs, supplied []pitfall.Corpus) (pitfall.Corpus, error) {
	if len(supplied) > 0 {
		return supplied[0], nil
	}
	return loadPitfallCorpus(p)
}

// checkADRRelatedLinks fails an ADR whose related: names an ADR number with no
// matching file under the decisions dir - structurally identical to the
// pitfall/plan link checks. Unconditional (independent of the tag vocabulary).
func checkADRRelatedLinks(corpus adr.Corpus) []manifest.Drift {
	adrs := corpus.All()
	rel := filepath.ToSlash(filepath.Join(config.DocsDir, "decisions"))
	var drift []manifest.Drift
	for _, a := range adrs {
		for _, n := range a.Related {
			if !corpus.Has(fmt.Sprintf("%04d", n)) {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-related-link", Detail: fmt.Sprintf("ADR-%s: ADR-%04d", a.Number, n)})
			}
		}
		// Ordering is scanned separately from resolution so that stopping at
		// the first descent cannot also stop the dangling-link scan
		// (adr-related-ascending). `related:` ascends, so a back-pointer edge
		// has exactly one correct position and appending a low-numbered
		// carrier is visibly wrong. One finding per array: the whole array is
		// one authoring act to fix.
		for i := 1; i < len(a.Related); i++ {
			if a.Related[i] < a.Related[i-1] {
				drift = append(drift, manifest.Drift{Path: rel + "/" + a.Filename, Kind: "adr-related-order", Detail: fmt.Sprintf("ADR-%s: related: descends at %d after %d; the array is ascending", a.Number, a.Related[i], a.Related[i-1])})
				break
			}
		}
	}
	return drift
}
