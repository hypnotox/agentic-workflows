package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

func testPublisher(p renderInputs) *publisher.Publisher {
	session, err := newSession(p.root(), p.residentRoots(), p.isNested(), p.cfg, p.catalog(), p.session.Targets(), p.session.Repository(), p.read)
	if err != nil {
		panic(err)
	}
	return publisher.New(session, Version)
}
func outputPlan(p renderInputs) (*outputplan.Plan, error) {
	plan, err := testPublisher(p).Plan()
	return &plan, err
}
func deriveOperationStateWithPitfalls(p renderInputs) (pitfall.Corpus, topic.Corpus, error) {
	operation := testPublisher(p)
	pitfalls, err := operation.Pitfalls()
	if err != nil {
		return pitfall.Corpus{}, topic.Corpus{}, err
	}
	topics, err := topic.LoadCorpusFromReader(p.read, p.cfg)
	return pitfalls, topics, err
}
func outputPlanWithPitfalls(p renderInputs, _ pitfall.Corpus, _ topic.Corpus) (*outputplan.Plan, error) {
	return outputPlan(p)
}
func generateDomainDocs(p renderInputs, _ topic.Corpus) ([]RenderedFile, error) {
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
func generateConfigReference(p renderInputs, _ []RenderedFile) (*RenderedFile, bool, error) {
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

var _ = context.Background
var _ = config.DirName
