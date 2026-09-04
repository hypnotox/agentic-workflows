package project

import "github.com/hypnotox/agentic-workflows/internal/outputplan"

// RenderedFile is the residual checker projection of one neutral planned output.
// It is constructed only at the outputplan-to-check translation point.
type RenderedFile struct {
	Path, Content, TemplateID, TemplateHash, ConfigHash string
	RegenChecked                                        bool
	Policy                                              outputplan.Policy
	Declarer, DeclarerProjection                        string
	assembled                                           string
	stubDefaults, stubParts, markerParts                []string
	kind, artifact                                      string
	partVarRefs                                         []string
}

func checkFile(output outputplan.Output) RenderedFile {
	return RenderedFile{
		Path: output.Path(), Content: output.Content(), TemplateID: output.TemplateID(),
		TemplateHash: output.TemplateHash(), ConfigHash: output.ConfigHash(), RegenChecked: output.RegenChecked(),
		Policy: output.Policy(), Declarer: output.Declarer(), DeclarerProjection: output.DeclarerProjection(),
		assembled: output.Assembled(), stubDefaults: output.StubDefaults(), stubParts: output.StubParts(),
		markerParts: output.MarkerParts(), kind: output.Kind(), artifact: output.Artifact(), partVarRefs: output.PartVarRefs(),
	}
}

func planWriteFiles(plan *outputplan.Plan) []RenderedFile {
	outputs := plan.Outputs()
	files := make([]RenderedFile, len(outputs))
	for i, output := range outputs {
		files[i] = checkFile(output)
	}
	return files
}
