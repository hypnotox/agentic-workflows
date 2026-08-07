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

// Loaded is one immutable current-state view: the intrinsically routed ADR records and
// the topic corpus assembled from the same snapshot. A caller runs Check over
// the records and topics and EvaluateCoverage over the topic corpus, so both
// the static handshake and coverage read a single consistent universe.
type Loaded struct {
	ADRs []adr.ADR
	// Sources owns the parsed ADR bytes keyed by identity for operation-local
	// qualification against merge-parent evidence.
	Sources map[string][]byte
	// Corpus is the identity-indexed view over ADRs, built and validated once
	// here. Consumers take it rather than rebuilding one, so the corpus-wide
	// duplicate-identity refusal (ADR-0202 item 4) has a single evaluation point
	// per load instead of one per consumer.
	Corpus adr.Corpus
	Topics topic.Corpus
}

// LoadFromTree assembles the ADR and topic corpora from a single snapshot Tree,
// so a working-tree, index, or commit universe yields exactly the current-state
// view that tree encodes (ADR-0135). cfg supplies the docs directory, configured
// domains, and marker-source families; parse it from the same tree for a
// single-universe load. It does not run Check or EvaluateCoverage; the command
// layer applies eligibility filters and routes findings.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config) (Loaded, error) {
	records, sources, corpus, err := authorityFromTree(tree)
	if err != nil {
		return Loaded{}, err
	}
	topics, err := topic.LoadCorpusFromTree(tree, cfg, corpus)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{ADRs: records, Sources: sources, Corpus: corpus, Topics: topics}, nil
}

// LoadUniverseFromSelection assembles historical policy authority from a sparse
// selection. It does not materialize that selection as a Tree.
func LoadUniverseFromSelection(selection *snapshot.Selection, cfg *config.Config) (Universe, error) {
	return loadUniverseFromFiles(selection.List(), cfg)
}

func loadUniverseFromFiles(files []snapshot.File, cfg *config.Config) (Universe, error) {
	records, sources, corpus, err := authorityFromFiles(files)
	if err != nil {
		return Universe{}, err
	}
	topics, err := topic.LoadAuthorityCorpusFromFiles(files, cfg, corpus)
	if err != nil {
		return Universe{}, err
	}
	return Universe{ADRs: records, Sources: sources, Topics: topics.All()}, nil
}

func authorityFromTree(tree *snapshot.Tree) ([]adr.ADR, map[string][]byte, adr.Corpus, error) {
	return authorityFromFiles(tree.List())
}

func authorityFromFiles(files []snapshot.File) ([]adr.ADR, map[string][]byte, adr.Corpus, error) {
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
// with the intrinsic router. adr.NewCorpus subsequently enforces corpus-level
// identity uniqueness that a per-file parse cannot see. Per-file legacy and
// governed routing is enforced by adr.ParseRecord, which also rejects a
// non-reserved file that is neither form.
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
