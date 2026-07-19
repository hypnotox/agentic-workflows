package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// ValidateLegacySnapshot closes the migration-only legacy fact set against
// HEAD. Authored migration history may change disposition, but declarations,
// backing classes, carriers, markers, cutoff facts, and identities may not drift.
func ValidateLegacySnapshot(root string, preparedCorpus adr.Corpus, prepared Inventory, mappings []Mapping, mutations []Mutation) error {
	configBlobs, err := awfgit.HeadBlobsUnder(root, ".awf")
	if err != nil {
		return err
	}
	var headConfig []byte
	for _, blob := range configBlobs {
		if blob.Path == ".awf/config.yaml" {
			headConfig = blob.Bytes
			break
		}
	}
	if headConfig == nil {
		return errors.New("legacy HEAD lacks .awf/config.yaml")
	}
	cfg, err := config.Parse("", headConfig)
	if err != nil {
		return err
	}
	blobs, err := awfgit.HeadBlobsUnder(root, strings.TrimRight(cfg.DocsDir, "/")+"/decisions")
	if err != nil { // coverage-ignore: the same HEAD repository was opened successfully immediately above
		return err
	}
	tmp, err := os.MkdirTemp("", "awf-bridge-head-")
	if err != nil { // coverage-ignore: the test-controlled system temporary directory is writable
		return err
	}
	defer os.RemoveAll(tmp)
	decisions := filepath.Join(tmp, cfg.DocsDir, "decisions")
	if err := os.MkdirAll(decisions, 0o755); err != nil { // coverage-ignore: the temporary root was created writable above
		return err
	}
	for _, blob := range blobs {
		if !adr.FilenameRe.MatchString(filepath.Base(blob.Path)) {
			continue
		}
		if err := os.WriteFile(filepath.Join(decisions, filepath.Base(blob.Path)), blob.Bytes, os.FileMode(blob.Mode)); err != nil { // coverage-ignore: temporary decisions dir is writable
			return err
		}
	}
	headCorpus, err := adr.LoadCorpus(decisions)
	if err != nil {
		return err
	}
	head, err := BuildInventory(headCorpus)
	if err != nil {
		return err
	}
	if len(head.Entries) != len(prepared.Entries) {
		return fmt.Errorf("legacy HEAD inventory has %d entries; prepared tree has %d", len(head.Entries), len(prepared.Entries))
	}
	for i := range head.Entries {
		a, b := head.Entries[i], prepared.Entries[i]
		if a.Key != b.Key || a.Declarer != b.Declarer || a.Backing != b.Backing || a.Carrier != b.Carrier || a.CarrierDecisionItem != b.CarrierDecisionItem {
			return fmt.Errorf("legacy HEAD/prepared inventory mismatch at %s", a.Key)
		}
	}
	headIDs, preparedIDs := adrIDs(headCorpus), adrIDs(preparedCorpus)
	if !slices.Equal(headIDs, preparedIDs) {
		return errors.New("legacy HEAD/prepared ADR identity set mismatch")
	}
	for _, old := range headCorpus.All() {
		current, _ := preparedCorpus.ByNumber(old.Number)
		if old.IsLegacyShipped() && !old.HasSameStatus(current) {
			return fmt.Errorf("legacy HEAD/prepared shipped status mismatch at ADR-%s", old.Number)
		}
	}
	headCutoff, headGaps := cutoffFacts(headIDs)
	preparedCutoff, preparedGaps := cutoffFacts(preparedIDs)
	if headCutoff != preparedCutoff || !slices.Equal(headGaps, preparedGaps) { // coverage-ignore: identical sorted identity sets necessarily derive identical cutoff facts
		return errors.New("legacy HEAD/prepared cutoff baseline mismatch")
	}
	allBlobs, err := awfgit.HeadBlobsUnder(root, "")
	if err != nil { // coverage-ignore: the same HEAD repository was read successfully above
		return err
	}
	return validateHeadMarkers(root, cfg.Invariants, allBlobs, mappings, mutations)
}

func adrIDs(corpus adr.Corpus) []string {
	ids := make([]string, 0, len(corpus.All()))
	for _, a := range corpus.All() {
		ids = append(ids, a.Number)
	}
	slices.Sort(ids)
	return ids
}
func cutoffFacts(ids []string) (int, []int) {
	max := 0
	present := map[int]bool{}
	for _, id := range ids {
		n, _ := strconv.Atoi(id)
		present[n] = true
		if n > max {
			max = n
		}
	}
	var gaps []int
	for n := 1; n <= max; n++ {
		if !present[n] {
			gaps = append(gaps, n)
		}
	}
	return max + 1, gaps
}

func validateHeadMarkers(root string, cfg *config.InvariantConfig, blobs []awfgit.HeadBlob, mappings []Mapping, mutations []Mutation) error {
	if cfg == nil {
		return nil
	}
	bySlug := map[string]string{}
	for _, m := range mappings {
		slug := m.Key[strings.IndexByte(m.Key, '#')+1:]
		bySlug[slug] = m.Destination
	}
	after := map[string][]byte{}
	for _, m := range mutations {
		if m.AfterPresent {
			after[m.Path] = m.After
		}
	}
	for _, blob := range blobs {
		var sources []config.InvariantSource
		for _, src := range cfg.Sources {
			for _, glob := range src.Globs {
				if pathglob.Match(glob, blob.Path) {
					sources = append(sources, src)
					break
				}
			}
		}
		if len(sources) == 0 {
			continue
		}
		for n, line := range strings.Split(string(blob.Bytes), "\n") {
			trim := strings.TrimSpace(line)
			for _, src := range sources {
				if !strings.HasPrefix(trim, src.Marker) {
					continue
				}
				payload := strings.TrimSpace(strings.TrimPrefix(trim, src.Marker))
				if src.Close != "" {
					if !strings.HasSuffix(payload, src.Close) {
						continue
					}
					payload = strings.TrimSpace(strings.TrimSuffix(payload, src.Close))
				}
				kind, slug, note := "", "", ""
				if strings.HasPrefix(payload, "invariant: ") {
					kind = "invariant"
					slug = strings.TrimSpace(strings.TrimPrefix(payload, "invariant: "))
				} else if strings.HasPrefix(payload, "touches-invariant: ") {
					kind = "touches-state"
					rest := strings.TrimPrefix(payload, "touches-invariant: ")
					parts := strings.SplitN(rest, " - ", 2)
					slug = strings.TrimSpace(parts[0])
					if len(parts) == 2 {
						note = strings.TrimSpace(parts[1])
					}
				}
				dest := bySlug[slug]
				if kind == "" || dest == "" {
					continue
				}
				markerPath := blob.Path
				final, ok := after[markerPath]
				if !ok {
					var readErr error
					final, readErr = os.ReadFile(filepath.Join(root, filepath.FromSlash(markerPath)))
					if readErr != nil {
						return fmt.Errorf("legacy marker %s:%d disappeared from prepared tree", blob.Path, n+1)
					}
				}
				want := kind + ": " + dest
				if note != "" {
					want += " - " + note
				}
				if !strings.Contains(string(final), want) {
					return fmt.Errorf("legacy marker %s:%d was not preserved as %s", blob.Path, n+1, want)
				}
				break
			}
		}
	}
	return nil
}
