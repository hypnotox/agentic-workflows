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
	ADRs   []adr.ADR
	Topics topic.Corpus
}

// LoadFromTree assembles the ADR and topic corpora from a single snapshot Tree,
// so a working-tree, index, or commit universe yields exactly the current-state
// view that tree encodes (ADR-0135). cfg supplies the docs directory, configured
// domains, and marker-source families; parse it from the same tree for a
// single-universe load. cutoff is the lock's adrFormatV1From boundary routing
// per-ADR legacy/format-v1 parsing, and gaps are the recorded absent lower ADR
// numbers the contiguity check tolerates. It does not run Check or
// EvaluateCoverage; the command layer applies eligibility filters and routes
// findings.
func LoadFromTree(tree *snapshot.Tree, cfg *config.Config, cutoff int, gaps []int) (Loaded, error) {
	records, err := adrsFromTree(tree, cfg.DocsDir, cutoff, gaps)
	if err != nil {
		return Loaded{}, err
	}
	topics, err := topic.LoadCorpusFromTree(tree, cfg, adr.NewCorpus(records))
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{ADRs: records, Topics: topics}, nil
}

// adrsFromTree parses every top-level ADR decision file in the snapshot with the
// cutoff-aware router, then enforces the corpus-level facts a per-file parse
// cannot see: no two files share a number, and the numbers are contiguous from 1
// except for the recorded legacy gaps (ADR-0135). Per-file format-v1 versus
// legacy routing is already enforced by adr.ParseRecord.
func adrsFromTree(tree *snapshot.Tree, docsDir string, cutoff int, gaps []int) ([]adr.ADR, error) {
	prefix := docsDir + "/decisions/"
	var records []adr.ADR
	var numbers []int
	for _, f := range tree.List() {
		rel, ok := strings.CutPrefix(f.Path, prefix)
		if !ok || strings.Contains(rel, "/") {
			continue // outside the decisions directory or in a nested subdirectory
		}
		m := adr.FilenameRe.FindStringSubmatch(rel)
		if m == nil {
			continue // README.md, INDEX.md, a template, or another non-ADR file
		}
		rec, err := adr.ParseRecord(rel, f.Bytes, cutoff)
		if err != nil {
			return nil, err
		}
		num, _ := strconv.Atoi(m[1]) // the regex admits only four digits
		records = append(records, rec)
		numbers = append(numbers, num)
	}
	if err := checkADRContiguity(numbers, gaps, cutoff); err != nil {
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
