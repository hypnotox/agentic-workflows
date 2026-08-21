package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type OutputNode = outputplan.Node
type OutputDeclaration = publisher.OutputDeclaration
type OutputInput = publisher.OutputInput
type OutputRecipe = publisher.OutputRecipe

type ConfigReference = publisher.ConfigReference
type ConfigKeyRow = publisher.ConfigKeyRow
type VarRow = publisher.VarRow
type DataKeyRow = publisher.DataKeyRow

func testPublisher(p renderInputs) *publisher.Publisher {
	return publisher.New(p.state.state, p.cfg, p.read, Version)
}
func outputPlan(p renderInputs) (*OutputPlan, error) {
	plan, err := testPublisher(p).Plan()
	return &plan, err
}
func outputPlanWithPitfalls(p renderInputs, _ adr.Corpus, _ pitfall.Corpus, _ topic.Corpus, _ map[string]bool) (*OutputPlan, error) {
	return outputPlan(p)
}
func syncReportWithPitfalls(p renderInputs, seed *InitAuthority, filesystems syncFilesystems, _ adr.Corpus, _ pitfall.Corpus, _ topic.Corpus, _ map[string]bool) ([]Backup, []Change, []string, error) {
	plan, err := outputPlan(p)
	if err != nil {
		return nil, nil, nil, err
	}
	return syncReportWithPlan(p, seed, filesystems, plan)
}
func generateIndexMD(p renderInputs, _ adr.Corpus) RenderedFile {
	plan, err := outputPlan(p)
	if err != nil {
		return RenderedFile{}
	}
	for _, file := range planWriteFiles(plan) {
		if file.Path == layout(p).IndexMd {
			return file
		}
	}
	return RenderedFile{}
}
func generateDomainDocs(p renderInputs, _ topic.Corpus, _ map[string]bool) ([]RenderedFile, error) {
	plan, err := outputPlan(p)
	if err != nil {
		return nil, err
	}
	prefix := layout(p).DomainsDir + "/"
	var files []RenderedFile
	for _, file := range planWriteFiles(plan) {
		if len(file.Path) > len(prefix) && file.Path[:len(prefix)] == prefix {
			files = append(files, file)
		}
	}
	return files, nil
}
func generateConfigReference(p renderInputs, _ []RenderedFile, _ map[string]bool) (*RenderedFile, bool, error) {
	plan, err := outputPlan(p)
	if err != nil {
		return nil, false, err
	}
	want := layout(p).Docs["config-reference"]
	for _, file := range planWriteFiles(plan) {
		if file.Path == want {
			return &file, true, nil
		}
	}
	return nil, false, nil
}
func plannedOutputs(p renderInputs) ([]string, error) {
	plan, err := outputPlan(p)
	if err != nil {
		return nil, err
	}
	return plan.Paths(), nil
}
func plannedOutputPaths(plan *OutputPlan) []string { return plan.Paths() }
func BuildOutputDeclarations(cfg *config.Config, cat *catalog.Catalog, targets []Target, read ProjectTreeReader, corpus adr.Corpus) ([]OutputDeclaration, error) {
	return publisher.BuildOutputDeclarations(cfg, cat, targets, read, corpus)
}
func configReferenceModel(p renderInputs) (ConfigReference, error) {
	return testPublisher(p).BuildConfigReference()
}
func PotentialVarConsumers() (map[string][]string, error) { return publisher.PotentialVarConsumers() }
func ConfigReferencePresentation(key string, model *ConfigReference, status string) (presentation.Document, error) {
	return publisher.ConfigReferencePresentation(key, model, status)
}
func StaticConfigReference() (ConfigReference, error) { return publisher.StaticConfigReference() }
func preflightLocalDoc(p renderInputs, doc config.LocalDoc) error {
	return testPublisher(p).PreflightLocalDoc(doc)
}
func renderResidentMarkerOperation(p renderInputs, name string) (RenderedFile, error) {
	output, err := testPublisher(p).RenderResidentMarker(name)
	if err != nil {
		return RenderedFile{}, err
	}
	return checkFile(output), nil
}
func validateLiveTemplates(p renderInputs) error { _, err := testPublisher(p).Plan(); return err }
func CheckStagedDriftRoot(ctx context.Context, root string) ([]manifest.Drift, error) {
	prep, err := PrepareStagedContextState(ctx, root)
	if err != nil {
		return nil, err
	}
	plan, err := publisher.New(prep.State, prep.Config, prep.Reader, Version).Plan()
	if err != nil {
		return nil, err
	}
	return CheckStagedDrift(prep, plan)
}
