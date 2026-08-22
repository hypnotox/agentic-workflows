package project

import "github.com/hypnotox/agentic-workflows/internal/config"

func layout(p renderInputs) Layout {
	d := config.DocsDir
	dec := d + "/decisions"
	docs := map[string]string{}
	singletons := map[string]string{}
	if projectCatalog(p) != nil {
		for name, entry := range projectCatalog(p).Docs {
			docs[name] = docOutPath(p, name)
			if !entry.AgentsDoc && entry.TemplateKey != "" {
				singletons[entry.TemplateKey] = docOutPath(p, name)
			}
		}
	}
	return Layout{DocsDir: d, ADRDir: dec, IndexMd: dec + "/INDEX.md", PlansDir: d + "/plans", Docs: docs, Singletons: singletons, DomainsDir: d + "/domains"}
}
