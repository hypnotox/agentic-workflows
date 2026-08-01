package currentstate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Loaded is one immutable current-state view: the cutoff-aware ADR records and
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
// single-universe load. boundaries are the lock's ADR format cutoffs routing
// per-ADR legacy/V1/V2 parsing, and gaps are the recorded absent lower ADR
// numbers the contiguity check tolerates. It does not run Check or
// EvaluateCoverage; the command layer applies eligibility filters and routes
// findings.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config, boundaries adr.FormatBoundaries, gaps []int) (Loaded, error) {
	records, err := adrsFromTree(tree, cfg.DocsDir, boundaries, gaps)
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
// cutoff-aware router, then enforces the corpus-level facts a per-file parse
// cannot see: no two files share a number, and the numbers are contiguous from 1
// except for the recorded legacy gaps (ADR-0135). Per-file legacy, V1, V2, and
// pending-V3 routing is already enforced by adr.ParseRecord, which also rejects
// a non-reserved file that is neither form. Contiguity stays number-scoped: a
// pending record has no number to be contiguous with (ADR-0202 item 4).
func adrsFromTree(tree *snapshot.Tree, docsDir string, boundaries adr.FormatBoundaries, gaps []int) ([]adr.ADR, error) {
	prefix := docsDir + "/decisions/"
	var records []adr.ADR
	var numbers []int
	for _, f := range tree.List() {
		if !f.Scannable() {
			continue
		}
		rel, ok := strings.CutPrefix(f.Path, prefix)
		if !ok || strings.Contains(rel, "/") {
			continue // outside the decisions directory or in a nested subdirectory
		}
		if !strings.HasSuffix(rel, ".md") || adr.IsReservedBasename(rel) {
			continue // README.md, INDEX.md, the template, or a non-Markdown companion file
		}
		rec, err := adr.ParseRecord(rel, f.Bytes, boundaries)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
		if rec.Number == "" {
			continue
		}
		num, _ := strconv.Atoi(rec.Number) // a numbered record carries FilenameRe's four-digit group
		numbers = append(numbers, num)
	}
	if err := checkADRContiguity(numbers, gaps, boundaries.V1From); err != nil {
		return nil, err
	}
	return records, nil
}

// checkADRContiguity verifies the parsed ADR numbers are unique and cover 1..max
// except for the recorded legacy gaps, which must all fall below the cutoff. An
// empty corpus is left to the caller.
func checkADRContiguity(numbers, gaps []int, cutoff int) error {
	if len(numbers) == 0 {
		return nil
	}
	present := map[int]bool{}
	maxNum := 0
	for _, n := range numbers {
		if present[n] {
			return fmt.Errorf("ADR number %04d is declared by more than one file", n)
		}
		present[n] = true
		if n > maxNum {
			maxNum = n
		}
	}
	var absent []int
	for n := 1; n <= maxNum; n++ {
		if !present[n] {
			absent = append(absent, n)
		}
	}
	want := make([]int, len(gaps))
	copy(want, gaps)
	sort.Ints(want)
	for _, g := range want {
		if cutoff > 0 && g >= cutoff {
			return fmt.Errorf("recorded legacy gap %04d is at or above the format cutoff %d", g, cutoff)
		}
	}
	if !equalInts(absent, want) {
		return fmt.Errorf("ADR numbers are not contiguous from 1: missing %v, recorded gaps %v", absent, want)
	}
	return nil
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
