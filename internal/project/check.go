package project

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configcheck"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/referencecheck"
	"github.com/hypnotox/agentic-workflows/internal/render"
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
func advisoryNotes(p renderInputs, pitfalls pitfall.Corpus, plans []plan.Plan, plansErr error, op *OutputPlan) ([]string, error) {
	if plansErr != nil {
		return nil, plansErr
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
	files := planWriteFiles(op)
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
	files := planWriteFiles(op)
	all := slices.Clone(files)
	for _, node := range op.Nodes() {
		output, ok := node.Output()
		if !ok {
			continue
		}
		declarers := node.Declarers()
		if slices.Contains(declarers, "generated-domain") || slices.Contains(declarers, "generated-config-reference") {
			all = append(all, checkFile(output))
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

// CheckReport is the ordinary check operation's drift, ranked warnings, and
// unranked information, derived from one operation-owned plan parse.
type CheckReport struct {
	Drift               []manifest.Drift
	Warnings            []string
	Information         []string
	TrackingInformation []string
	PlanWarnings        []string
	Result              checkresult.Result
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
	return resultFindingDetails(r.Result, severity.Warn, "advisory")
}

// PlanWarningNotes returns ranked plan warning notes for sink deduplication.
func (r CheckReport) PlanWarningNotes() []string {
	if !r.classified {
		return slices.Clone(r.PlanNotes)
	}
	return resultFindingDetails(r.Result, severity.Warn, "plan-advisory")
}

// AggregateInformation returns unranked aggregate information notes.
func (r CheckReport) AggregateInformation() []string {
	if !r.classified {
		return nil
	}
	return resultInformationDetails(r.Result, "advisory")
}

// DirectTrackingInformation returns unranked tracking information shown by
// both direct drift and aggregate checks.
func (r CheckReport) DirectTrackingInformation() []string {
	if !r.classified {
		return slices.Clone(r.TrackingNotes)
	}
	return resultInformationDetails(r.Result, "tracking")
}

func resultFindingDetails(result checkresult.Result, rank severity.Rank, kind string) []string {
	var details []string
	for _, finding := range result.Findings() {
		if finding.Rank == rank && finding.Evidence.Kind == kind {
			details = append(details, finding.Evidence.Detail)
		}
	}
	return details
}

func resultInformationDetails(result checkresult.Result, kind string) []string {
	var details []string
	for _, information := range result.Information() {
		if information.Evidence.Kind == kind {
			details = append(details, information.Evidence.Detail)
		}
	}
	return details
}

// CheckReport performs one ordinary project check. Plans are parsed once and
// the typed set is threaded to both blocking and advisory consumers.
func checkReport(p renderInputs, repo *awfgit.Repo, ctx context.Context, semantics OperationSemantics, op *OutputPlan) (CheckReport, error) {
	if err := configcheck.ValidateCommandWiring(p.cfg); err != nil {
		return CheckReport{}, err
	}
	corpus, pitfalls, topics, eff := semantics.ADRs, semantics.Pitfalls, semantics.Topics, semantics.EffectiveSkills
	plans, parseErr := semantics.Plans, semantics.PlansError
	batch := checkBatch{}
	planResults := checkBatch{}
	if parseErr != nil {
		var diagnostics *plan.DiagnosticsError
		if !errors.As(parseErr, &diagnostics) {
			return CheckReport{}, parseErr
		}
		rel := filepath.ToSlash(filepath.Join(config.DocsDir, "plans"))
		for _, diagnostic := range diagnostics.Diagnostics {
			if !knownDynamicPlanDiagnosticCategory(diagnostic.Category) {
				return CheckReport{}, fmt.Errorf("unknown plan diagnostic category %q", diagnostic.Category)
			}
			planResults.error(propertyAuthority, "plan-"+diagnostic.Category, rel+"/"+diagnostic.Path, diagnostic.Detail)
		}
	}
	producerResults, trackingNotes, err := checkWithTrackingState(p, repo, ctx, corpus, pitfalls, topics, eff, plans, op)
	if err != nil {
		return CheckReport{}, err
	}
	batch.append(producerResults)
	batch.append(planResults)
	planArtifacts := checkBatch{}
	if fullProfile(p) {
		planArtifacts = planArtifactResults(plans, corpus)
		batch.append(planArtifacts.withoutWarnings())
	}
	advisories, err := advisoryResultsWithState(p, pitfalls, plans, op)
	if err != nil {
		return CheckReport{}, err
	}
	batch.append(advisories)
	for _, note := range trackingNotes {
		batch.informationItem("tracking", "", note)
	}
	batch.append(planArtifacts.warningsOnly())
	return reportFromBatch(batch)
}

// Dynamic plan diagnostics originate in a closed parser category set. Refusing
// an unknown category keeps parser evolution from silently acquiring checker
// policy or a fabricated finding identity.
func knownDynamicPlanDiagnosticCategory(category string) bool {
	switch category {
	case "field", "frontmatter", "numbering", "path", "paths", "phase-close", "projection", "relationship", "structure":
		return true
	default:
		return false
	}
}

const (
	propertyAuthority       checkresult.Property = "authority"
	propertyCorrectness     checkresult.Property = "correctness"
	propertyReproducibility checkresult.Property = "reproducibility"
	propertyHeuristic       checkresult.Property = "heuristic-quality"
	propertyPlanDetail      checkresult.Property = "plan-detail-quality"
)

func checkWithTrackingState(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus, pitfalls pitfall.Corpus, topics topic.Corpus, eff map[string]bool, plans []plan.Plan, op *OutputPlan) (checkBatch, []string, error) {
	var indexPaths generatedcheck.IndexPaths
	if repo != nil {
		indexPaths = repo.IndexPaths
	}
	tracking, err := generatedcheck.Tracking(ctx, p.isNested(), indexPaths, *op)
	if err != nil { // coverage-ignore: the prepared output plan already read every output
		return checkBatch{}, nil, err
	}
	var trackingNotes []string
	for _, item := range tracking.Information() {
		trackingNotes = append(trackingNotes, item.Evidence.Detail)
	}
	lock, found, err := manifest.LoadOptional(lockPath(p.root()))
	if err != nil { // coverage-ignore: prepared configuration facts already validated generated inputs
		return checkBatch{}, nil, err
	}
	results := checkBatch{}
	results.appendResult(tracking)
	if !found {
		if len(tracking.Findings()) > 0 {
			return results, trackingNotes, nil
		}
		return checkBatch{}, nil, errors.New("no lock (run awf render)")
	}
	locked, err := generatedcheck.Locked(p.isNested(), lock, *op, func(path string) ([]byte, error) { return os.ReadFile(p.residentRoots().ResolveOutput(path)) }, tracking)
	if err != nil { // coverage-ignore: ReferenceChecker has no operational failure path for prepared inputs
		return checkBatch{}, nil, err
	}
	results.appendResult(locked)
	generated, err := generatedcheck.Additional(generatedcheck.AdditionalInput{
		Root: p.root(), ResidentRoot: p.residentRoots().Resident, Config: p.cfg,
		Catalog: projectCatalog(p), Topics: topics.All(), Paths: p.read.Paths,
	}, *op)
	if err != nil { // coverage-ignore: the operation supplied its validated pitfall corpus
		return checkBatch{}, nil, err
	}
	results.appendResult(generated)
	references, err := referenceResult(p, *op, eff)
	if err != nil { // coverage-ignore: pitfall preparation read the tag inputs
		return checkBatch{}, nil, err
	}
	results.appendResult(references)
	if fullProfile(p) {
		results.append(planResult(p, corpus, plans))
	}
	pitfallsResult, err := pitfallResult(p, corpus, pitfalls)
	if err != nil { // coverage-ignore: the operation supplied its validated pitfall corpus
		return checkBatch{}, nil, err
	}
	results.append(pitfallsResult)
	glossary, err := glossaryResult(p)
	if err != nil {
		return checkBatch{}, nil, err
	}
	results.append(glossary)
	tags, err := tagVocabularyResult(p, pitfalls)
	if err != nil { // coverage-ignore: pitfall preparation already read the tag inputs
		return checkBatch{}, nil, err
	}
	results.append(tags)
	if fullProfile(p) {
		related, err := adrRelatedResult(corpus)
		if err != nil { // coverage-ignore: the immutable ADR corpus is already validated
			return checkBatch{}, nil, err
		}
		results.appendResult(related)
		results.append(pendingADRResult(p, repo, ctx, corpus))
	}
	return results, trackingNotes, nil
}

func referenceResult(p renderInputs, op outputplan.Plan, effective map[string]bool) (checkresult.Result, error) {
	known := map[string]bool{}
	for name := range projectCatalog(p).Skills {
		known[name] = true
	}
	return referencecheck.Check(op, p.cfg.Prefix, effective, known, func(path string) bool { _, err := os.Stat(filepath.Join(p.root(), path)); return err == nil })
}
func adrRelatedResult(corpus adr.Corpus) (checkresult.Result, error) {
	adrs := corpus.All()
	values := make([]referencecheck.ADR, len(adrs))
	for i, a := range adrs {
		values[i] = referencecheck.ADR{Number: a.Number, Filename: a.Filename, Related: a.Related}
	}
	return referencecheck.ADRRelated(values)
}

// The result adapters are the Phase 1 producer boundary. Their legacy helpers
// remain available to direct callers, but ordinary CheckReport composition only
// receives owner-classified batches.
func planResult(p renderInputs, corpus adr.Corpus, plans []plan.Plan) checkBatch {
	batch := checkBatch{}
	batch.errorDrift(propertyAuthority, checkPlans(p, corpus, plans))
	return batch
}
func pitfallResult(p renderInputs, corpus adr.Corpus, pitfalls pitfall.Corpus) (checkBatch, error) {
	drift, err := checkPitfalls(p, corpus, pitfalls)
	if err != nil { // coverage-ignore: this adapter receives the operation's already validated pitfall corpus
		return checkBatch{}, err
	}
	batch := checkBatch{}
	batch.errorDrift(propertyCorrectness, drift)
	return batch, nil
}
func glossaryResult(p renderInputs) (checkBatch, error) {
	drift, err := checkGlossary(p)
	if err != nil { // coverage-ignore: GuideSizeAdvisory has no fallible prepared-plan path
		return checkBatch{}, err
	}
	batch := checkBatch{}
	batch.errorDrift(propertyCorrectness, drift)
	return batch, nil
}
func tagVocabularyResult(p renderInputs, pitfalls pitfall.Corpus) (checkBatch, error) {
	drift, err := checkTagVocabulary(p, pitfalls)
	if err != nil { // coverage-ignore: pitfallResult already read and failed on the only fallible tag-vocabulary input
		return checkBatch{}, err
	}
	batch := checkBatch{}
	batch.errorDrift(propertyCorrectness, drift)
	return batch, nil
}
func pendingADRResult(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus) checkBatch {
	batch := checkBatch{}
	batch.errorDrift(propertyAuthority, checkPendingADRs(p, repo, ctx, corpus))
	return batch
}
func planArtifactResults(plans []plan.Plan, corpus adr.Corpus) checkBatch {
	drift, notes := planArtifactReport(plans, corpus)
	batch := checkBatch{}
	batch.errorDrift(propertyAuthority, drift)
	for _, note := range notes {
		batch.warning(propertyPlanDetail, "plan-advisory", note)
	}
	return batch
}
func advisoryResultsWithState(p renderInputs, pitfalls pitfall.Corpus, plans []plan.Plan, op *OutputPlan) (checkBatch, error) {
	advisories, err := advisoryNotesWithState(p, pitfalls, plans, op)
	if err != nil {
		return checkBatch{}, err
	}
	batch := checkBatch{}
	for _, note := range advisories.Warnings {
		batch.warning(propertyHeuristic, "advisory", note)
	}
	guide, err := generatedcheck.GuideSizeAdvisory(*op)
	if err != nil { // coverage-ignore: GuideSizeAdvisory has no fallible prepared-plan path
		return checkBatch{}, err
	}
	batch.appendResult(guide)
	for _, note := range advisories.Information {
		batch.informationItem("advisory", "", note)
	}
	return batch, nil
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
