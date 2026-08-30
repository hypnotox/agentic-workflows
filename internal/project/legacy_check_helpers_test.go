package project

import (
	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/glossary"
	"github.com/hypnotox/agentic-workflows/internal/glossarycheck"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/pitfallcheck"
	"github.com/hypnotox/agentic-workflows/internal/severity"
)

const glossarySidecarPath = glossary.SidecarPath
const glossaryMeaningMax = glossary.MeaningMax

type glossaryRecord = glossary.Record

func checkPitfalls(p renderInputs, corpus adr.Corpus, pitfalls pitfall.Corpus) ([]manifest.Drift, error) {
	result, err := pitfallcheck.Check(p.cfg.Domains, pitfalls, corpus)
	return resultDriftForTest(result), err
}

func checkGlossary(p renderInputs) ([]manifest.Drift, error) {
	input, err := glossaryInputForTest(p)
	if err != nil {
		return nil, err
	}
	result, err := glossarycheck.Evaluate(input)
	return resultDriftForTest(result), err
}

func glossaryTersenessNotes(p renderInputs) ([]string, error) {
	input, err := glossaryInputForTest(p)
	if err != nil {
		return nil, err
	}
	result, err := glossarycheck.Evaluate(input)
	return warningDetailsForTest(result), err
}

func glossaryInputForTest(p renderInputs) (glossarycheck.Input, error) {
	prepared, err := testPublisher(p).Prepare()
	if err != nil {
		return glossarycheck.Input{}, err
	}
	return prepared.Glossary(), nil
}

func mergedGlossaryRecords(sc config.Sidecar) ([]glossaryRecord, error) { return glossary.Merge(sc) }
func glossaryRecords(raw any) ([]glossaryRecord, error)                 { return glossary.Records(raw) }
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
