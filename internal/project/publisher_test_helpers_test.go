package project

import (
	"context"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type Backup = publisher.Backup
type Change = publisher.Change
type InitAuthority = publisher.InitAuthority

func testPublisher(p renderInputs) *publisher.Publisher {
	session, err := newSession(p.root(), p.residentRoots(), p.isNested(), p.cfg, p.catalog(), p.session.Targets(), p.session.Repository(), p.read)
	if err != nil {
		panic(err)
	}
	return publisher.New(session, Version)
}
func outputPlan(p renderInputs) (*OutputPlan, error) {
	plan, err := testPublisher(p).Plan()
	return &plan, err
}
func deriveOperationStateWithPitfalls(p renderInputs) (pitfall.Corpus, topic.Corpus, map[string]bool, error) {
	prepared, err := testPublisher(p).Prepare()
	if err != nil {
		return pitfall.Corpus{}, topic.Corpus{}, nil, err
	}
	return prepared.Pitfalls(), prepared.Topics(), prepared.EffectiveSkills(), nil
}
func outputPlanWithPitfalls(p renderInputs, _ pitfall.Corpus, _ topic.Corpus, _ map[string]bool) (*OutputPlan, error) {
	return outputPlan(p)
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

var _ = context.Background
var _ = config.DirName
