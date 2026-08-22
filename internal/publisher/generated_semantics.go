package publisher

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/generatedcheck"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// generatedSemantics captures the generated checker input from this operation's
// selected tree before the output plan leaves Publisher.
func generatedSemantics(p renderInputs, topics topic.Corpus) (generatedcheck.AdditionalInput, error) {
	input := generatedcheck.AdditionalInput{Vars: maps.Clone(p.cfg.Vars), Domains: slices.Clone(p.cfg.Domains)}
	for _, kind := range []string{"skills", "agents", "docs"} {
		for _, name := range artifactNames(p.catalog(), kind) {
			sections := artifactSections(p.catalog(), kind, name)
			input.Artifacts = append(input.Artifacts, generatedcheck.Artifact{Kind: kind, Name: name, Sections: slices.Clone(sections)})
			sidecar, err := p.cfg.Sidecar(kind, name)
			if err != nil { // coverage-ignore: output-plan construction already parsed this bound operation sidecar before generated semantics are captured
				return generatedcheck.AdditionalInput{}, err
			}
			input.Sidecars = append(input.Sidecars, generatedcheck.SidecarData{Kind: kind, Name: name, Path: ".awf/" + kind + "/" + name + ".yaml", Data: maps.Clone(sidecar.Data)})
		}
	}
	for _, domain := range p.cfg.Domains {
		input.Artifacts = append(input.Artifacts, generatedcheck.Artifact{Kind: "domains", Name: domain, Sections: slices.Clone(p.catalog().DomainDoc.Sections)})
	}
	for _, name := range catalog.SingletonKindsFor(p.catalog()) {
		input.Singletons = append(input.Singletons, generatedcheck.Artifact{Kind: name, Sections: slices.Clone(p.catalog().Docs[name].Sections)})
		sidecar, err := p.cfg.Sidecar(name, "")
		if err != nil { // coverage-ignore: output-plan construction already parsed this bound singleton sidecar before generated semantics are captured
			return generatedcheck.AdditionalInput{}, err
		}
		input.Sidecars = append(input.Sidecars, generatedcheck.SidecarData{Kind: name, Path: ".awf/" + name + ".yaml", Data: maps.Clone(sidecar.Data)})
	}
	for _, item := range topics.All() {
		input.Topics = append(input.Topics, generatedcheck.Topic{Domain: item.ID.Domain, Slug: item.ID.Slug})
	}
	if reader, ok := p.read.(interface {
		Entries(string) ([]generatedcheck.TreeEntry, error)
	}); ok {
		entries, err := reader.Entries(".awf/")
		if err != nil {
			return generatedcheck.AdditionalInput{}, err
		}
		input.Entries = entries
	} else {
		paths, err := p.read.Paths(".awf/")
		if err != nil {
			return generatedcheck.AdditionalInput{}, err
		}
		dirs := map[string]bool{}
		for _, source := range paths {
			source = strings.TrimPrefix(source, "./")
			input.Entries = append(input.Entries, generatedcheck.TreeEntry{Path: source})
			for parent := path.Dir(source); parent != "." && parent != "/"; parent = path.Dir(parent) {
				dirs[parent] = true
			}
		}
		for dir := range dirs {
			input.Entries = append(input.Entries, generatedcheck.TreeEntry{Path: dir, Directory: true})
		}
	}
	for _, entry := range input.Entries {
		if !entry.Directory && strings.HasPrefix(entry.Path, pitfallsSourceDir+"/") {
			input.PitfallPaths = append(input.PitfallPaths, entry.Path)
		}
	}
	input.ResidentRoot = p.state.Roots().Tracked == p.state.Roots().Resident
	return input, nil
}

func artifactNames(c *catalog.Catalog, kind string) []string {
	switch kind {
	case "skills":
		return slices.Sorted(maps.Keys(c.Skills))
	case "agents":
		return slices.Sorted(maps.Keys(c.Agents))
	case "docs":
		return catalog.NameDerivedDocNames(c)
	}
	return nil
}
func artifactSections(c *catalog.Catalog, kind, name string) []string {
	switch kind {
	case "skills":
		return c.Skills[name].Sections
	case "agents":
		return c.Agents[name].Sections
	case "docs":
		return c.Docs[name].Sections
	}
	return nil
}

func cloneGeneratedOutput(input generatedcheck.AdditionalInput) generatedcheck.AdditionalInput {
	out := input
	out.Vars, out.Domains, out.PitfallPaths = maps.Clone(input.Vars), slices.Clone(input.Domains), slices.Clone(input.PitfallPaths)
	out.Artifacts, out.Singletons, out.Topics, out.Entries = slices.Clone(input.Artifacts), slices.Clone(input.Singletons), slices.Clone(input.Topics), slices.Clone(input.Entries)
	for i := range out.Artifacts {
		out.Artifacts[i].Sections = slices.Clone(out.Artifacts[i].Sections)
	}
	for i := range out.Singletons {
		out.Singletons[i].Sections = slices.Clone(out.Singletons[i].Sections)
	}
	out.Sidecars = slices.Clone(input.Sidecars)
	for i := range out.Sidecars {
		out.Sidecars[i].Data = maps.Clone(out.Sidecars[i].Data)
	}
	return out
}
