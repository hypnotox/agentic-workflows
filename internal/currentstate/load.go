package currentstate

import (
	"path"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Loaded is one immutable snapshot view. Historical ADR records and their raw
// source bytes remain available only for residual external readers; Topics is
// loaded independently as the current-state authority.
type Loaded struct {
	ADRs []adr.ADR
	// Sources owns parsed historical ADR bytes keyed by identity for residual readers.
	Sources map[string][]byte
	// Corpus is the identity-indexed historical ADR projection retained for
	// residual readers.
	Corpus adr.Corpus
	Topics topic.Corpus
}

// LoadFromTree assembles passive historical ADR projections and the independent
// topic corpus from one snapshot tree. cfg supplies the docs directory,
// configured domains, and marker-source families.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config) (Loaded, error) {
	records, sources, corpus, err := adrRecordsFromTree(tree)
	if err != nil {
		return Loaded{}, err
	}
	topics, err := topic.LoadCorpusFromTree(tree, cfg)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{ADRs: records, Sources: sources, Corpus: corpus, Topics: topics}, nil
}

// LoadUniverseFromSelection assembles a passive historical projection from a
// sparse selection. It does not materialize that selection as a Tree.
func LoadUniverseFromSelection(selection *snapshot.Selection, cfg *config.Config) (Universe, error) {
	return loadUniverseFromFiles(selection.List(), cfg)
}

func loadUniverseFromFiles(files []snapshot.File, cfg *config.Config) (Universe, error) {
	records, sources, _, err := adrRecordsFromFiles(files)
	if err != nil {
		return Universe{}, err
	}
	topics, err := topic.LoadAuthorityCorpusFromFiles(files, cfg)
	if err != nil {
		return Universe{}, err
	}
	return Universe{ADRs: records, Sources: sources, Topics: topics.All()}, nil
}

func adrRecordsFromTree(tree *snapshot.Tree) ([]adr.ADR, map[string][]byte, adr.Corpus, error) {
	return adrRecordsFromFiles(tree.List())
}

func adrRecordsFromFiles(files []snapshot.File) ([]adr.ADR, map[string][]byte, adr.Corpus, error) {
	records, sources, err := adrsFromFiles(files, config.DocsDir)
	if err != nil {
		return nil, nil, adr.Corpus{}, err
	}
	corpus, err := adr.NewCorpus(records)
	if err != nil {
		return nil, nil, adr.Corpus{}, err
	}
	return records, sources, corpus, nil
}

// adrsFromFiles parses every top-level ADR decision file in the supplied files
// for the retained passive projections. adr.NewCorpus subsequently enforces
// corpus-level identity uniqueness that a per-file parse cannot see.
func adrsFromFiles(files []snapshot.File, docsDir string) ([]adr.ADR, map[string][]byte, error) {
	prefix := path.Join(docsDir, "decisions") + "/"
	var records []adr.ADR
	sources := map[string][]byte{}
	for _, f := range files {
		if !f.Scannable() {
			continue
		}
		rel, ok := strings.CutPrefix(f.Path, prefix)
		if !ok || strings.Contains(rel, "/") || !strings.HasSuffix(rel, ".md") || adr.IsReservedBasename(rel) {
			continue
		}
		rec, err := adr.ParseRecord(rel, f.Bytes)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, rec)
		sources[rec.Identity()] = slices.Clone(f.Bytes)
	}
	return records, sources, nil
}
