package project

import (
	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func hookTID(name string) string { return artifactregistry.HookTemplateID(name) }

type singletonSpec struct {
	kind     string
	tid      string
	outPath  func(Layout) string
	sections func(*catalog.Catalog) []string
}

func plainSingletons(cat *catalog.Catalog) []singletonSpec {
	declarations := artifactregistry.PlainSingletons(cat)
	out := make([]singletonSpec, len(declarations))
	for i, declaration := range declarations {
		declaration := declaration
		out[i] = singletonSpec{
			kind: declaration.Kind,
			tid:  declaration.TemplateID,
			outPath: func(Layout) string {
				return declaration.OutputPath
			},
			sections: func(*catalog.Catalog) []string {
				return append([]string(nil), declaration.Sections...)
			},
		}
	}
	return out
}
