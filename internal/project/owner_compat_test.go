package project

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/checkresult"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/referencecheck"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/severity"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// These test-only projections retain old fixtures while asserting the focused
// owners. Production composition has no project-owned policy implementation.
func ownerPlan(files []RenderedFile) outputplan.Plan {
	nodes := make([]outputplan.Node, 0, len(files))
	for _, file := range files {
		out := outputplan.NewOutput(outputplan.OutputSpec{Path: file.Path, Content: file.Content, TemplateID: file.TemplateID, TemplateHash: file.TemplateHash, ConfigHash: file.ConfigHash, Policy: file.Policy, Assembled: file.assembled, Kind: file.kind, Artifact: file.artifact, PartVarRefs: file.partVarRefs})
		nodes = append(nodes, outputplan.NewNode(outputplan.NodeSpec{Path: file.Path, Output: &out}))
	}
	return outputplan.New(nodes, nil)
}

func compatibilityDrift(result checkresult.Result) []manifest.Drift {
	findings := result.Findings()
	out := make([]manifest.Drift, len(findings))
	for i, finding := range findings {
		out[i] = manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail}
	}
	return out
}

func generatedAdditionalForTest(p renderInputs, files []RenderedFile) (checkresult.Result, error) {
	return generatedcheck.Additional(generatedcheck.AdditionalInput{Root: p.root(), ResidentRoot: p.residentRoots().Resident, Config: p.cfg, Catalog: projectCatalog(p), Paths: p.read.Paths}, ownerPlan(files))
}
func unusedVarDrift(p renderInputs, files []RenderedFile) []manifest.Drift {
	result, err := generatedAdditionalForTest(p, files)
	if err != nil {
		panic(err)
	}
	var drift []manifest.Drift
	for _, item := range result.Information() {
		if item.Evidence.Kind == "unused-var" {
			drift = append(drift, manifest.Drift{Path: item.Evidence.Path, Kind: item.Evidence.Kind, Detail: item.Evidence.Detail})
		}
	}
	return drift
}
func unusedDataDrift(p renderInputs, files []RenderedFile) ([]manifest.Drift, error) {
	result, err := generatedAdditionalForTest(p, files)
	if err != nil {
		return nil, err
	}
	var drift []manifest.Drift
	for _, item := range result.Information() {
		if item.Evidence.Kind == "unused-data" {
			drift = append(drift, manifest.Drift{Path: item.Evidence.Path, Kind: item.Evidence.Kind, Detail: item.Evidence.Detail})
		}
	}
	return drift, nil
}

func checkDeadRefs(p renderInputs, files []RenderedFile) []manifest.Drift {
	result, err := referencecheck.Check(ownerPlan(files), p.cfg.Prefix, nil, nil, func(path string) bool { _, err := os.Stat(filepath.Join(p.root(), path)); return err == nil })
	if err != nil {
		panic(err)
	}
	return compatibilityDrift(result)
}
func checkDeadSkillRefs(p renderInputs, files []RenderedFile, effective map[string]bool) []manifest.Drift {
	known := map[string]bool{}
	for name := range projectCatalog(p).Skills {
		known[name] = true
	}
	result, err := referencecheck.Check(ownerPlan(files), p.cfg.Prefix, effective, known, func(string) bool { return true })
	if err != nil {
		panic(err)
	}
	return compatibilityDrift(result)
}
func checkADRRelatedLinks(corpus adr.Corpus) []manifest.Drift {
	adrs := corpus.All()
	values := make([]referencecheck.ADR, len(adrs))
	for i, item := range adrs {
		values[i] = referencecheck.ADR{Number: item.Number, Filename: item.Filename, Related: item.Related}
	}
	result, err := referencecheck.ADRRelated(values)
	if err != nil {
		panic(err)
	}
	return compatibilityDrift(result)
}

func checkGeneratedTracking(nested bool, repo *awfgit.Repo, ctx context.Context, op *OutputPlan) (checkBatch, []string, error) {
	var paths generatedcheck.IndexPaths
	if repo != nil {
		paths = repo.IndexPaths
	}
	result, err := generatedcheck.Tracking(ctx, nested, paths, *op)
	if err != nil {
		return checkBatch{}, nil, err
	}
	batch := checkBatch{}
	batch.appendResult(result)
	var notes []string
	for _, item := range result.Information() {
		notes = append(notes, item.Evidence.Detail)
	}
	return batch, notes, nil
}

type lockedFinding struct {
	Drift    manifest.Drift
	Property checkresult.Property
}

func checkLockedFiles(roots resident.Roots, lock *manifest.Lock, rendered map[string]RenderedFile, tracking []manifest.Drift) []lockedFinding {
	files := make([]RenderedFile, 0, len(rendered))
	for _, file := range rendered {
		files = append(files, file)
	}
	findings := make([]checkresult.Finding, 0, len(tracking))
	for _, drift := range tracking {
		if drift.Kind == "untracked" {
			detail := drift.Detail
			if detail == "" {
				detail = "untracked"
			}
			findings = append(findings, checkresult.Finding{Rank: severity.Error, Property: propertyReproducibility, Evidence: checkresult.Evidence{Kind: drift.Kind, Path: drift.Path, Detail: detail}})
		}
	}
	tracked, _ := checkresult.New(findings, nil)
	result, err := generatedcheck.Locked(false, lock, ownerPlan(files), func(path string) ([]byte, error) { return os.ReadFile(roots.ResolveOutput(path)) }, tracked)
	if err != nil {
		panic(err)
	}
	out := make([]lockedFinding, 0, len(result.Findings()))
	for _, finding := range result.Findings() {
		out = append(out, lockedFinding{Drift: manifest.Drift{Path: finding.Evidence.Path, Kind: finding.Evidence.Kind, Detail: finding.Evidence.Detail}, Property: finding.Property})
	}
	return out
}

func checkStagedRenderedFiles(lock *manifest.Lock, rendered map[string]RenderedFile, reader ProjectTreeReader, indexed map[string]bool, includeResident bool) ([]manifest.Drift, error) {
	files := make([]RenderedFile, 0, len(rendered))
	for _, file := range rendered {
		files = append(files, file)
	}
	result, err := generatedcheck.Staged(!includeResident, lock, ownerPlan(files), reader, indexed)
	if err != nil {
		return nil, err
	}
	return compatibilityDrift(result), nil
}

func sweepConfigTree(p renderInputs, files []RenderedFile, topics topic.Corpus) ([]manifest.Drift, error) {
	result, err := generatedcheck.Additional(generatedcheck.AdditionalInput{Root: p.root(), ResidentRoot: p.residentRoots().Resident, Config: p.cfg, Catalog: projectCatalog(p), Topics: topics.All(), Paths: p.read.Paths}, ownerPlan(files))
	if err != nil {
		return nil, err
	}
	return compatibilityDrift(result), nil
}

type claimedModel struct {
	files      map[string]bool
	singletons map[string]bool
}

func buildClaimedModel(p renderInputs, _ []RenderedFile, _ topic.Corpus) *claimedModel {
	model := &claimedModel{files: map[string]bool{}, singletons: map[string]bool{}}
	for _, kind := range catalog.SingletonKindsFor(projectCatalog(p)) {
		model.singletons[kind] = true
		model.files[config.DirName+"/"+kind+".yaml"] = true
	}
	return model
}

func validateArtifact(content []byte, _ AgentDialect) error {
	return generatedcheck.ValidateFrontmatter(content)
}

func declaredSections(p renderInputs, kind, name string) []string {
	switch kind {
	case "skills":
		return projectCatalog(p).Skills[name].Sections
	case "agents":
		return projectCatalog(p).Agents[name].Sections
	case "docs":
		return projectCatalog(p).Docs[name].Sections
	case "domains":
		return projectCatalog(p).DomainDoc.Sections
	}
	return nil
}
