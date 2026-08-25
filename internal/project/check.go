package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/configcheck"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/pitfallcheck"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
	"github.com/hypnotox/agentic-workflows/internal/referencecheck"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/vocabularycheck"
)

// CheckAdvisories separates ranked warnings from unranked information without
// adding another finding rank.
type CheckAdvisories struct {
	Warnings    []string
	Information []string
}

// AdvisoryNotes returns the compatibility projection of the non-failing notes
// produced by one operation-scoped plan parse.
func advisoryNotes(p renderInputs, plans []plan.Plan, plansErr error, op *OutputPlan, vocabulary vocabularycheck.Input) ([]string, error) {
	if plansErr != nil {
		return nil, plansErr
	}
	vocabularyResults, err := vocabularycheck.Evaluate(vocabulary)
	if err != nil { // coverage-ignore: prepared vocabulary checks are infallible
		return nil, err
	}
	advisories := advisoryNotesWithState(p, plans, op, vocabularyResults)
	return append(slices.Clone(advisories.Warnings), advisories.Information...), nil
}

// advisoryNotesWithState classifies the non-failing render advisories from
// operation-owned state, its already parsed plans, and its one prepared output
// plan.
func advisoryNotesWithState(p renderInputs, plans []plan.Plan, op *OutputPlan, vocabularyResults vocabularycheck.Results) CheckAdvisories {
	files := planWriteFiles(op)
	all := advisoryCompatibilityFiles(op)
	information := append(unsetVarNotes(p, files), stubNotes(all)...)
	information = append(information, markerNotes(all)...)
	information = append(information, planCommitScopeNotes(p, plans)...)
	var warnings []string
	for _, result := range []checkresult.Result{vocabularyResults.Tags, vocabularyResults.Glossary} {
		for _, finding := range result.Findings() {
			if finding.Rank == severity.Warn {
				warnings = append(warnings, finding.Evidence.Detail)
			}
		}
	}
	return CheckAdvisories{Warnings: warnings, Information: information}
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

// CheckReport is the ordinary check operation's compatibility projection.
type CheckReport = repositorycheck.Report

// CheckReport performs one ordinary project check. Plans are parsed once and
// completed owner results are placed into RepositoryChecker's explicit slots.
func checkReport(p renderInputs, repo *awfgit.Repo, ctx context.Context, semantics OperationSemantics, op *OutputPlan) (CheckReport, error) {
	if err := configcheck.ValidateCommandWiring(p.cfg); err != nil {
		return CheckReport{}, err
	}
	corpus, pitfalls, eff := semantics.ADRs, semantics.Pitfalls, semantics.EffectiveSkills
	plans, parseErr := semantics.Plans, semantics.PlansError
	planDiagnostics, err := plancheck.Diagnostics(parseErr)
	if err != nil {
		return CheckReport{}, err
	}
	vocabularyResults, err := vocabularycheck.Evaluate(semantics.Vocabulary)
	if err != nil { // coverage-ignore: Publisher preparation supplied validated vocabulary semantics
		return CheckReport{}, err
	}
	trackingResult, producerResults, tracking, err := checkWithTrackingState(p, repo, ctx, corpus, pitfalls, eff, plans, semantics.GeneratedOutput, op, vocabularyResults)
	if err != nil {
		return CheckReport{}, err
	}
	var planArtifacts checkresult.Result
	var planArtifactErrors, deferredPlanWarnings checkresult.Result
	if fullProfile(p) {
		planArtifacts = planArtifactResults(plans, corpus)
		planArtifactErrors, deferredPlanWarnings, err = repositorycheck.SplitWarnings(planArtifacts)
		if err != nil { // coverage-ignore: splitting PlanChecker's validated immutable result cannot invalidate evidence
			return CheckReport{}, err
		}
	}
	advisories, err := advisoryResultsWithState(p, plans, op, vocabularyResults)
	if err != nil { // coverage-ignore: Publisher preparation validated advisory inputs and the output plan is immutable
		return CheckReport{}, err
	}
	return repositorycheck.Compose(repositorycheck.Inputs{
		Tracking:             trackingResult,
		ProducerResults:      producerResults,
		PlanDiagnostics:      repositorycheck.Slot{Result: planDiagnostics},
		PlanArtifactErrors:   repositorycheck.Slot{Result: planArtifactErrors},
		OrdinaryAdvisories:   repositorycheck.Slot{Result: advisories},
		TrackingInformation:  repositorycheck.Slot{Result: tracking},
		DeferredPlanWarnings: repositorycheck.Slot{Result: deferredPlanWarnings},
	})
}

// Dynamic plan diagnostics originate in a closed parser category set. Refusing
// an unknown category keeps parser evolution from silently acquiring checker
// policy or a fabricated finding identity.
const (
	propertyAuthority       checkresult.Property = "authority"
	propertyCorrectness     checkresult.Property = "correctness"
	propertyReproducibility checkresult.Property = "reproducibility"
	propertyHeuristic       checkresult.Property = "heuristic-quality"
	propertyPlanDetail      checkresult.Property = "plan-detail-quality"
)

func checkWithTrackingState(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus, pitfalls pitfall.Corpus, eff map[string]bool, plans []plan.Plan, generatedInput generatedcheck.AdditionalInput, op *OutputPlan, vocabularyResults vocabularycheck.Results) (repositorycheck.Slot, []repositorycheck.Slot, checkresult.Result, error) {
	var indexPaths generatedcheck.IndexPaths
	if repo != nil {
		indexPaths = repo.IndexPaths
	}
	tracking, err := generatedcheck.Tracking(ctx, p.isNested(), indexPaths, *op)
	if err != nil {
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	lock, found, err := manifest.LoadOptional(lockPath(p.root()))
	if err != nil {
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	results := []repositorycheck.Slot{}
	if !found {
		if len(tracking.Findings()) > 0 {
			return repositorycheck.Slot{Result: trackingFindings(tracking)}, results, trackingInformation(tracking), nil
		}
		return repositorycheck.Slot{}, nil, checkresult.Result{}, errors.New("no lock (run awf render)")
	}
	locked, err := generatedcheck.Locked(p.isNested(), lock, *op, func(path string) ([]byte, error) { return os.ReadFile(p.residentRoots().ResolveOutput(path)) }, tracking)
	if err != nil { // coverage-ignore: ReferenceChecker has no operational failure path for prepared inputs
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	results = append(results, repositorycheck.Slot{Result: locked})
	generated, err := generatedcheck.Additional(generatedInput, *op)
	if err != nil { // coverage-ignore: Additional constructs fixed nonempty evidence from immutable prepared semantic values
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	results = append(results, repositorycheck.Slot{Result: generated, IncludeInformationInDrift: true})
	references, err := referenceResult(p, *op, eff)
	if err != nil { // coverage-ignore: pitfall preparation read the tag inputs
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	results = append(results, repositorycheck.Slot{Result: references})
	if fullProfile(p) {
		results = append(results, repositorycheck.Slot{Result: planResult(p, corpus, plans)})
	}
	pitfallsResult, err := pitfallResult(p, corpus, pitfalls)
	if err != nil { // coverage-ignore: the operation supplied its validated pitfall corpus
		return repositorycheck.Slot{}, nil, checkresult.Result{}, err
	}
	results = append(results, repositorycheck.Slot{Result: pitfallsResult})
	for _, result := range []checkresult.Result{vocabularyResults.Glossary, vocabularyResults.Tags} {
		withoutWarnings, _, err := repositorycheck.SplitWarnings(result)
		if err != nil { // coverage-ignore: splitting VocabularyChecker's validated immutable result cannot invalidate evidence
			return repositorycheck.Slot{}, nil, checkresult.Result{}, err
		}
		results = append(results, repositorycheck.Slot{Result: withoutWarnings})
	}
	if fullProfile(p) {
		related, err := adrRelatedResult(corpus)
		if err != nil { // coverage-ignore: the immutable ADR corpus is already validated
			return repositorycheck.Slot{}, nil, checkresult.Result{}, err
		}
		results = append(results, repositorycheck.Slot{Result: related})
		results = append(results, repositorycheck.Slot{Result: pendingADRResult(p, repo, ctx, corpus)})
	}
	return repositorycheck.Slot{Result: trackingFindings(tracking)}, results, trackingInformation(tracking), nil
}

func trackingFindings(result checkresult.Result) checkresult.Result {
	tracking, err := checkresult.New(result.Findings(), nil)
	if err != nil { // coverage-ignore: tracking owner already validates its immutable evidence
		return checkresult.Result{}
	}
	return tracking
}

func trackingInformation(result checkresult.Result) checkresult.Result {
	tracking, err := checkresult.New(nil, result.Information())
	if err != nil { // coverage-ignore: tracking owner already validates its immutable evidence
		return checkresult.Result{}
	}
	return tracking
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

// Result adapters preserve owner-classified results for ordinary CheckReport
// composition. Legacy helpers remain available to direct callers without
// changing the normal composition boundary.
func planResult(p renderInputs, corpus adr.Corpus, plans []plan.Plan) checkresult.Result {
	result, err := plancheck.Validity(plans, corpus, config.AuditScopes(p.cfg.Audit), fullProfile(p))
	if err != nil { // coverage-ignore: validity over prepared semantic values is infallible
		return checkresult.Result{}
	}
	return result
}
func pitfallResult(p renderInputs, corpus adr.Corpus, pitfalls pitfall.Corpus) (checkresult.Result, error) {
	return pitfallcheck.Check(p.cfg.Domains, pitfalls, corpus)
}
func pendingADRResult(p renderInputs, repo *awfgit.Repo, ctx context.Context, corpus adr.Corpus) checkresult.Result {
	var findings []checkresult.Finding
	for _, drift := range checkPendingADRs(p, repo, ctx, corpus) {
		findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: propertyAuthority, Evidence: checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: drift.Detail}})
	}
	result, err := checkresult.New(findings, nil)
	if err != nil { // coverage-ignore: pending ADR evidence is constructed from nonempty corpus records
		return checkresult.Result{}
	}
	return result
}
func planArtifactResults(plans []plan.Plan, corpus adr.Corpus) checkresult.Result {
	result, err := plancheck.Artifact(plans, corpus)
	if err != nil { // coverage-ignore: artifact checks over prepared semantic values are infallible
		return checkresult.Result{}
	}
	return result
}
func advisoryResultsWithState(p renderInputs, plans []plan.Plan, op *OutputPlan, vocabularyResults vocabularycheck.Results) (checkresult.Result, error) {
	advisories := advisoryNotesWithState(p, plans, op, vocabularyResults)
	var findings []checkresult.Finding
	for _, note := range advisories.Warnings {
		findings = append(findings, checkresult.Finding{Rank: severity.Warn, Property: propertyHeuristic, Evidence: checkresult.Evidence{Kind: "advisory", Detail: note}})
	}
	guide, err := generatedcheck.GuideSizeAdvisory(*op)
	if err != nil { // coverage-ignore: GuideSizeAdvisory has no fallible prepared-plan path
		return checkresult.Result{}, err
	}
	findings = append(findings, guide.Findings()...)
	information := guide.Information()
	for _, note := range advisories.Information {
		information = append(information, checkresult.Information{Evidence: checkresult.Evidence{Kind: "advisory", Detail: note}})
	}
	return checkresult.New(findings, information)
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
// planCommitScopeNotes returns advisory (non-failing) notes for a plan's ```commit
// subject naming a scope outside the configured allow-list. Unlike an over-length or
// mistyped subject (hard drift in checkPlans), an unknown scope is advisory: a plan
// may be the change that adds the scope (ADR-0111). Mirrors checkPlans' scan; a
// frontmatter-less plan is skipped.
func planCommitScopeNotes(p renderInputs, plans []plan.Plan) []string {
	information, err := plancheck.ScopeInformation(plans, config.AuditScopes(p.cfg.Audit))
	if err != nil { // coverage-ignore: prepared subject evaluation is infallible
		return nil
	}
	notes := make([]string, len(information))
	for i, item := range information {
		notes[i] = item.Evidence.Detail
	}
	return notes
}
