package adr

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// TreeReader is the neutral selected-tree contract used by operation loaders.
type TreeReader interface {
	ReadFile(path string) ([]byte, bool, error)
	Paths(prefix string) ([]string, error)
}

// LoadCorpusFromTree parses ADR authority from one already-selected operation
// tree. It shares the record parser and corpus constructor with LoadCorpus.
func LoadCorpusFromTree(read TreeReader, dir string) (Corpus, error) {
	prefix := strings.TrimSuffix(path.Clean(dir), "/") + "/"
	paths, err := read.Paths(prefix)
	if err != nil {
		return Corpus{}, err
	}
	slices.Sort(paths)
	adrs := make([]ADR, 0, len(paths))
	for _, sourcePath := range paths {
		if path.Dir(sourcePath) != strings.TrimSuffix(prefix, "/") {
			continue
		}
		base := path.Base(sourcePath)
		if IsReservedBasename(base) || !strings.HasSuffix(base, ".md") {
			continue
		}
		data, found, err := read.ReadFile(sourcePath)
		if err != nil {
			return Corpus{}, fmt.Errorf("read %s: %w", base, err)
		}
		if !found {
			continue
		}
		a, err := ParseRecord(base, data)
		if err != nil {
			return Corpus{}, fmt.Errorf("parse %s: %w", base, err)
		}
		if a.Number == "" && a.Format != CurrentFormat() && !a.IsV3() { // coverage-ignore: ParseRecord only returns legacy/v1/v2 records with filename-derived numbers, v3 is explicitly exempt, and current v4 is excluded.
			return Corpus{}, ErrNotADRRecord(base)
		}
		a.Path = sourcePath
		adrs = append(adrs, a)
	}
	return NewCorpus(adrs)
}
