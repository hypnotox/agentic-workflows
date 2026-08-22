package project

import (
	"maps"
	"slices"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/pitfallcheck"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/plancheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/vocabularycheck"
)

const glossarySidecarPath = glossary.SidecarPath
const glossaryMeaningMax = glossary.MeaningMax

type glossaryRecord = glossary.Record

func knownDynamicPlanDiagnosticCategory(category string) bool {
	result, err := plancheck.Diagnostics(&plan.DiagnosticsError{Diagnostics: []*plan.Diagnostic{{Category: category, Path: "example.md", Detail: "example"}}})
	return err == nil && len(result.Findings()) == 1
}

func checkPlans(p renderInputs, corpus adr.Corpus, plans []plan.Plan) []manifest.Drift {
	result, _ := plancheck.Validity(plans, corpus, config.AuditScopes(p.cfg.Audit), fullProfile(p))
	return resultDriftForTest(result)
}

func checkPitfalls(p renderInputs, corpus adr.Corpus, pitfalls pitfall.Corpus) ([]manifest.Drift, error) {
	result, err := pitfallcheck.Check(p.cfg.Domains, pitfalls, corpus)
	return resultDriftForTest(result), err
}

func checkGlossary(p renderInputs) ([]manifest.Drift, error) {
	input, err := vocabularyInputForTest(p, pitfall.Corpus{})
	if err != nil {
		return nil, err
	}
	input.Tags = nil
	results, err := vocabularycheck.Evaluate(input)
	return resultDriftForTest(results.Glossary), err
}

func checkTagVocabulary(p renderInputs, pitfalls pitfall.Corpus) ([]manifest.Drift, error) {
	input := vocabularycheck.Input{Domains: slices.Clone(p.cfg.Domains), Tags: maps.Clone(p.cfg.Tags), Pitfalls: pitfalls}
	results, err := vocabularycheck.Evaluate(input)
	return resultDriftForTest(results.Tags), err
}

func tagHealthNotes(p renderInputs, pitfalls pitfall.Corpus) ([]string, error) {
	input := vocabularycheck.Input{Domains: slices.Clone(p.cfg.Domains), Tags: maps.Clone(p.cfg.Tags), Pitfalls: pitfalls}
	results, err := vocabularycheck.Evaluate(input)
	return warningDetailsForTest(results.Tags), err
}

func glossaryTersenessNotes(p renderInputs) ([]string, error) {
	input, err := vocabularyInputForTest(p, pitfall.Corpus{})
	if err != nil {
		return nil, err
	}
	input.Tags = nil
	results, err := vocabularycheck.Evaluate(input)
	return warningDetailsForTest(results.Glossary), err
}

func vocabularyInputForTest(p renderInputs, pitfalls pitfall.Corpus) (vocabularycheck.Input, error) {
	sc, err := p.cfg.Sidecar("docs", "glossary")
	if err != nil {
		return vocabularycheck.Input{}, err
	}
	authored, err := glossary.Records(sc.Data["terms"])
	if err != nil {
		return vocabularycheck.Input{}, err
	}
	merged, err := glossary.Merge(withDefaultData(sc, projectCatalog(p).Docs["glossary"].Data, specializedListDataKeys("docs", "glossary")...))
	if err != nil {
		return vocabularycheck.Input{}, err
	}
	return vocabularycheck.Input{GlossaryEnabled: fullProfile(p), Authored: authored, Merged: merged, Domains: slices.Clone(p.cfg.Domains), Tags: maps.Clone(p.cfg.Tags), Pitfalls: pitfalls}, nil
}

func mergedGlossaryRecords(sc config.Sidecar) ([]glossaryRecord, error) { return glossary.Merge(sc) }
func glossaryRecords(raw any) ([]glossaryRecord, error)                 { return glossary.Records(raw) }

func selectedRefs(p plan.Plan, selector string) (plan.Phase, plan.Task, error) {
	return plancheck.Select(p, selector)
}

func resolveSelectedPlanDecisions(p plan.Plan, corpus adr.Corpus, phase plan.Phase, task plan.Task) ([]plan.ResolvedDecision, []plan.ResolvedDecision, error) {
	return plancheck.ResolveSelectedDecisions(p, corpus, phase, task)
}

func planArtifactReport(plans []plan.Plan, corpus adr.Corpus) ([]manifest.Drift, []string) {
	result, _ := plancheck.Artifact(plans, corpus)
	var notes []string
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Warn {
			notes = append(notes, finding.Evidence.Detail)
		}
	}
	return resultDriftForTest(result), notes
}

func resultDriftForTest(result checkresult.Result) []manifest.Drift {
	var drift []manifest.Drift
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Error {
			drift = append(drift, manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail})
		}
	}
	return drift
}

func warningDetailsForTest(result checkresult.Result) []string {
	var details []string
	for _, finding := range result.Findings() {
		if finding.Rank == severity.Warn {
			details = append(details, finding.Evidence.Detail)
		}
	}
	return details
}
