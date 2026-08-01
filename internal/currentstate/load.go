package currentstate

import (
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
// single-universe load. gaps are the recorded absent lower ADR numbers the
// contiguity check tolerates. It does not run Check or EvaluateCoverage; the
// command layer applies eligibility filters and routes findings.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config, _ ...[]int) (Loaded, error) {
	records, err := adrsFromTree(tree, cfg.DocsDir)
	if err != nil {
		return Loaded{}, err
	}
	corpus, err := adr.NewCorpus(records)
	if err != nil {
		return Loaded{}, err
	}
	topics, err := topic.LoadCorpusFromTree(tree, cfg, corpus)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{ADRs: records, Corpus: corpus, Topics: topics}, nil
}

// adrsFromTree parses every top-level ADR decision file in the snapshot with the
// intrinsic router, then enforces the corpus-level facts a per-file parse cannot
// see: no two files share a number, and the numbers are contiguous from 1 except
// for the recorded legacy gaps (ADR-0135). Per-file legacy and governed routing
// is already enforced by adr.ParseRecord, which also rejects a non-reserved file
// that is neither form. Contiguity stays number-scoped: a pending record has no
// number to be contiguous with (ADR-0202 item 4).
func adrsFromTree(tree *snapshot.Tree, docsDir string) ([]adr.ADR, error) {
	prefix := docsDir + "/decisions/"
	var records []adr.ADR
	for _, f := range tree.List() {
		if !f.Scannable() {
			continue
		}
		rel, ok := strings.CutPrefix(f.Path, prefix)
		if !ok || strings.Contains(rel, "/") || !strings.HasSuffix(rel, ".md") || adr.IsReservedBasename(rel) {
			continue
		}
		rec, err := adr.ParseRecord(rel, f.Bytes)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}
