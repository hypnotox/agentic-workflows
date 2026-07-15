package catalog

// Node is one artifact in the Requires* dependency graph (ADR-0081).
// Docs are pure sinks: DocEntry declares no requirements.
type Node struct {
	Kind string // "skill", "agent", or "doc"
	Name string
}

// RequiresOf enumerates n's direct requirement edges declared in cat - the
// single source of edge truth (ADR-0081 Decision 1). An unknown name yields
// a zero-value spec and therefore no edges: project-local artifacts
// (ADR-0068) are leaves.
func RequiresOf(cat *Catalog, n Node) []Node {
	var out []Node
	switch n.Kind {
	case "skill":
		spec := cat.Skills[n.Name]
		for _, s := range spec.RequiresSkills {
			out = append(out, Node{Kind: "skill", Name: s})
		}
		if spec.RequiresAgent != "" {
			out = append(out, Node{Kind: "agent", Name: spec.RequiresAgent})
		}
		if spec.RequiresDoc != "" {
			out = append(out, Node{Kind: "doc", Name: spec.RequiresDoc})
		}
	case "agent":
		for _, s := range cat.Agents[n.Name].RequiresSkills {
			out = append(out, Node{Kind: "skill", Name: s})
		}
	}
	return out
}

// Closure returns the forward closure of seeds under RequiresOf, seeds
// included, breadth-first with edges in declaration order (deterministic).
func Closure(cat *Catalog, seeds []Node) []Node {
	seen := map[Node]bool{}
	var out []Node
	queue := append([]Node(nil), seeds...)
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
		queue = append(queue, RequiresOf(cat, n)...)
	}
	return out
}
