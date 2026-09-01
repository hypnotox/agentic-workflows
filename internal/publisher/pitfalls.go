package publisher

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
)

const pitfallsSourceDir = config.DirName + "/docs/pitfalls"

func loadPitfallCorpus(p renderInputs) (pitfall.Corpus, error) {
	return loadPitfallCorpusFrom(projectTreeReader(p))
}

func loadPitfallCorpusFrom(reader outputplan.TreeReader) (pitfall.Corpus, error) {
	paths, err := reader.Paths(pitfallsSourceDir + "/")
	if err != nil {
		return pitfall.Corpus{}, err
	}
	files := make([]pitfall.SourceFile, 0, len(paths))
	for _, source := range paths {
		regular := true
		if current, ok := reader.(filesystemProjectReader); ok {
			info, err := os.Lstat(filepath.Join(current.root, filepath.FromSlash(source)))
			if err != nil {
				return pitfall.Corpus{}, fmt.Errorf("inspect pitfall source %s: %w", source, err)
			}
			regular = info.Mode().IsRegular()
		}
		b, ok, err := reader.ReadFile(source)
		if err != nil {
			return pitfall.Corpus{}, fmt.Errorf("read pitfall source %s: %w", source, err)
		}
		if !ok {
			regular = false
		}
		files = append(files, pitfall.SourceFile{Path: filepath.ToSlash(source), Bytes: b, Regular: regular})
	}
	return pitfall.Load(files)
}

type pitfallIndexEntry struct {
	Slug, Title, LinkTitle, TableTitle string
	Domains                            []string
	DomainsText                        string
}
type pitfallDomainGroup struct {
	Name, Heading string
	Entries       []pitfallIndexEntry
}
type pitfallIndexModel struct {
	Entries    []pitfallIndexEntry
	Domains    []pitfallDomainGroup
	Unassigned []pitfallIndexEntry
}

type pitfallLeafModel struct {
	Slug, Title, Heading, Source, Body string
	Domains                            []string
	DomainsText                        string
}

func buildPitfallIndex(corpus pitfall.Corpus) pitfallIndexModel {
	entries := corpus.All()
	slices.SortFunc(entries, func(a, b pitfall.Entry) int {
		if n := strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)); n != 0 {
			return n
		}
		return strings.Compare(a.Slug, b.Slug)
	})
	model := pitfallIndexModel{}
	groups := map[string][]pitfallIndexEntry{}
	for _, e := range entries {
		item := pitfallIndexEntry{Slug: e.Slug, Title: e.Title, LinkTitle: pitfall.EscapeLinkLabel(e.Title), TableTitle: pitfall.EscapeTableCell(e.Title), Domains: e.Domains, DomainsText: pitfall.EscapeTableCell(strings.Join(e.Domains, ", "))}
		model.Entries = append(model.Entries, item)
		if len(e.Domains) == 0 {
			model.Unassigned = append(model.Unassigned, item)
		}
		for _, domain := range e.Domains {
			groups[domain] = append(groups[domain], item)
		}
	}
	names := mapsKeys(groups)
	slices.Sort(names)
	for _, name := range names {
		model.Domains = append(model.Domains, pitfallDomainGroup{Name: name, Heading: pitfall.EscapeHeading(name), Entries: groups[name]})
	}
	return model
}

func mapsKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func pitfallIndexSidecar(sc config.Sidecar, corpus pitfall.Corpus) config.Sidecar {
	out := sc
	out.Data = map[string]any{"pitfalls": buildPitfallIndex(corpus)}
	return out
}

func pitfallLeafData(e pitfall.Entry) map[string]any {
	return map[string]any{"pitfall": pitfallLeafModel{Slug: e.Slug, Title: e.Title, Heading: pitfall.EscapeHeading(e.Title), Source: e.SourcePath, Body: e.Body, Domains: e.Domains, DomainsText: strings.Join(e.Domains, ", ")}}
}

// pitfallSourcePaths returns stable provenance inputs for the complete corpus.
func pitfallSourcePaths(corpus pitfall.Corpus) []string {
	out := make([]string, 0, corpus.Len())
	for _, e := range corpus.All() {
		out = append(out, e.SourcePath)
	}
	slices.Sort(out)
	return out
}
