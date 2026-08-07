package migrate

import "github.com/hypnotox/agentic-workflows/internal/catalog"

// historicalNode and historicalRequiresOf preserve the requirement closure used
// by frozen migrations. The live renderer no longer has a requirement graph.
type historicalNode struct {
	Kind string
	Name string
}

func historicalRequiresOf(cat *catalog.Catalog, n historicalNode) []historicalNode {
	var out []historicalNode
	switch n.Kind {
	case "skill":
		spec := cat.Skills[n.Name]
		for _, name := range spec.RequiresSkills {
			out = append(out, historicalNode{Kind: "skill", Name: name})
		}
		if spec.RequiresAgent != "" {
			out = append(out, historicalNode{Kind: "agent", Name: spec.RequiresAgent})
		}
		if spec.RequiresDoc != "" {
			out = append(out, historicalNode{Kind: "doc", Name: spec.RequiresDoc})
		}
	case "agent":
		for _, name := range cat.Agents[n.Name].RequiresSkills {
			out = append(out, historicalNode{Kind: "skill", Name: name})
		}
	}
	return out
}
