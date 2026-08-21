package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func testPublisher(p renderInputs) *publisher.Publisher {
	facts, err := config.NewFacts(p.cfg)
	if err != nil {
		panic(err)
	}
	base := p.state.state
	state := projectstate.NewDerivedWithFacts(base.Root(), base.Roots(), base.Nested(), facts, base.Catalog(), base.CompleteCatalog(), base.Targets())
	return publisher.New(state, p.cfg, p.read, Version)
}
func outputPlan(p renderInputs) (*OutputPlan, error) {
	plan, err := testPublisher(p).Plan()
	return &plan, err
}
func deriveOperationStateWithPitfalls(p renderInputs) (adr.Corpus, pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	prepared, err := testPublisher(p).Prepare()
	return prepared.ADRs(), prepared.Pitfalls(), prepared.Topics(), prepared.EffectiveSkills(), err
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
