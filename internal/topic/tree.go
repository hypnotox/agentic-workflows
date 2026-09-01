package topic

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"gopkg.in/yaml.v3"
)

// TreeReader is the neutral selected-tree contract used by operation loaders.
type TreeReader interface {
	ReadFile(path string) ([]byte, bool, error)
	Paths(prefix string) ([]string, error)
}

type markerLineReader interface {
	ReadLines(path string, maxLineBytes int, visit func(string) error) (bool, error)
}

const (
	treeMetadataRoot   = config.DirName + "/topics/metadata"
	treeMetadataPrefix = treeMetadataRoot + "/"
	treePartsPrefix    = config.DirName + "/topics/parts/"
	treePartSuffix     = "/current-state.md"
)

// LoadCorpusFromReader loads retained topic authority first, then streams each
// selected marker source through the scanner so aggregate source bytes never
// become part of one materialized snapshot.
func LoadCorpusFromReader(read TreeReader, cfg *config.Config) (Corpus, error) {
	paths, err := read.Paths("")
	if err != nil {
		return Corpus{}, err
	}
	domainSidecars := make(map[string]bool, len(cfg.Domains))
	for _, domain := range cfg.Domains {
		domainSidecars[config.DirName+"/domains/"+domain+".yaml"] = true
	}
	files := make([]snapshot.File, 0, len(paths))
	for _, path := range paths {
		if !authorityInputPath(path, domainSidecars) {
			continue
		}
		data, found, err := read.ReadFile(path)
		if err != nil {
			return Corpus{}, err
		}
		if found {
			files = append(files, snapshot.File{Path: path, Mode: snapshot.Regular, Bytes: data})
		}
	}
	tree, err := snapshot.NewTree(files)
	if err != nil {
		return Corpus{}, err
	}
	corpus, err := corpusFromTreeFiles(tree, scannableTreeFiles(tree), cfg)
	if err != nil {
		return Corpus{}, err
	}
	markers, err := markerIndexFromReader(read, paths, corpus, cfg.CurrentState)
	if err != nil {
		return Corpus{}, err
	}
	corpus.Markers = markers
	return corpus, nil
}

func authorityInputPath(path string, domainSidecars map[string]bool) bool {
	return !resident.IsResidentPath(path) && (domainSidecars[path] ||
		(strings.HasPrefix(path, treeMetadataPrefix) && strings.HasSuffix(path, ".yaml")) ||
		(strings.HasPrefix(path, treePartsPrefix) && strings.HasSuffix(path, treePartSuffix)))
}

// LoadCorpusFromTree parses the complete current-state topic corpus from an
// immutable snapshot. It retains domain ownership and marker validation for
// callers that need the complete repository projection.
func LoadCorpusFromTree(tree *snapshot.Tree, cfg *config.Config) (Corpus, error) {
	files := scannableTreeFiles(tree)
	c, err := corpusFromTreeFiles(tree, files, cfg)
	if err != nil {
		return Corpus{}, err
	}
	markers, err := markerIndexFromTreeFiles(files, c, cfg.CurrentState)
	if err != nil {
		return Corpus{}, err
	}
	c.Markers = markers
	return c, nil
}

func corpusFromTreeFiles(tree *snapshot.Tree, files []snapshot.File, cfg *config.Config) (Corpus, error) {
	metadata, parts, err := authorityEntriesFromTreeFiles(files)
	if err != nil {
		return Corpus{}, err
	}
	domainPaths := map[string][]string{}
	for _, d := range cfg.Domains {
		paths, err := domainPathsFromTree(tree, d)
		if err != nil {
			return Corpus{}, err
		}
		domainPaths[d] = paths
	}
	return assembleCorpus(metadata, parts, cfg.Domains, domainPaths)
}

func scannableTreeFiles(tree *snapshot.Tree) []snapshot.File {
	allFiles := tree.List()
	files := make([]snapshot.File, 0, len(allFiles))
	for _, f := range allFiles {
		if f.Scannable() {
			files = append(files, f)
		}
	}
	return files
}

func authorityEntriesFromTreeFiles(files []snapshot.File) (map[string]metaEntry, map[string]partEntry, error) {
	metadata := map[string]metaEntry{}
	parts := map[string]partEntry{}
	for _, f := range files {
		switch {
		case strings.HasPrefix(f.Path, treeMetadataPrefix) && strings.HasSuffix(f.Path, ".yaml"):
			id, m, err := ParseMetadata(treeMetadataRoot, f.Path, f.Bytes)
			if err != nil {
				return nil, nil, err
			}
			if err := recordMeta(metadata, id, metaEntry{meta: m, path: f.Path}); err != nil {
				return nil, nil, err
			}
		case strings.HasPrefix(f.Path, treePartsPrefix) && strings.HasSuffix(f.Path, treePartSuffix):
			seg := strings.Split(strings.TrimPrefix(f.Path, treePartsPrefix), "/")
			if len(seg) != 3 || !kebabRE.MatchString(seg[0]) || !kebabRE.MatchString(seg[1]) {
				return nil, nil, fmt.Errorf("invalid topic part path %q", f.Path)
			}
			parts[(TopicID{seg[0], seg[1]}).String()] = partEntry{data: f.Bytes, path: f.Path}
		}
	}
	return metadata, parts, nil
}

// domainPathsFromTree reads one domain sidecar's ownership globs from the
// snapshot, mirroring config.Sidecar's zero-Sidecar-on-missing contract so a
// domain without a sidecar owns no paths rather than failing.
func domainPathsFromTree(tree *snapshot.Tree, domain string) ([]string, error) {
	f, ok := tree.Lookup(config.DirName + "/domains/" + domain + ".yaml")
	if !ok || !f.Scannable() {
		return nil, nil
	}
	var sc config.Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(f.Bytes))
	dec.KnownFields(true)
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("parse domain sidecar %s: %w", domain, err)
	}
	if err := config.ValidatePathGlobs(sc.Paths); err != nil {
		return nil, fmt.Errorf("domain sidecar %s paths: %w", domain, err)
	}
	return slices.Clone(sc.Paths), nil
}

func markerIndexFromReader(read TreeReader, paths []string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerIndex, error) {
	idx := MarkerIndex{sites: map[string][]MarkerSite{}}
	if cfg != nil {
		nested := nestedProjectRootsFromPaths(paths)
		for _, path := range paths {
			if resident.IsResidentPath(path) || belowAnyRoot(path, nested) {
				continue
			}
			sources := matchingSources(cfg, path)
			if len(sources) == 0 {
				continue
			}
			var err error
			if lineReader, ok := read.(markerLineReader); ok {
				lines := func(visit func(string) error) (bool, error) {
					return lineReader.ReadLines(path, maxMarkerLineBytes, visit)
				}
				err = scanMarkerLines(idx, path, lines, sources, corpus, cfg)
			} else {
				var data []byte
				var found bool
				data, found, err = read.ReadFile(path)
				if err == nil && found {
					err = scanMarkerBytes(idx, path, data, sources, corpus, cfg)
				}
			}
			if err != nil {
				return MarkerIndex{}, fmt.Errorf("scan current-state markers: %w", err)
			}
		}
	}
	if err := finalizeMarkerIndex(idx, corpus); err != nil {
		return MarkerIndex{}, err
	}
	return idx, nil
}

// markerIndexFromTreeFiles scans a snapshot's files for current-state markers,
// reusing the byte-fed scan/validate core. The snapshot is already the selected
// Git universe (nested repositories and ignored paths are excluded upstream), so
// marker selection is by configured source globs alone.
func markerIndexFromTreeFiles(files []snapshot.File, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerIndex, error) {
	idx := MarkerIndex{sites: map[string][]MarkerSite{}}
	if cfg != nil {
		nested := nestedProjectRoots(files)
		for _, f := range files {
			if resident.IsResidentPath(f.Path) || belowAnyRoot(f.Path, nested) {
				continue
			}
			sources := matchingSources(cfg, f.Path)
			if len(sources) == 0 {
				continue
			}
			if err := scanMarkerBytes(idx, f.Path, f.Bytes, sources, corpus, cfg); err != nil {
				return MarkerIndex{}, fmt.Errorf("scan current-state markers: %w", err)
			}
		}
	}
	if err := finalizeMarkerIndex(idx, corpus); err != nil {
		return MarkerIndex{}, err
	}
	return idx, nil
}

// nestedProjectRoots returns every non-root directory that carries its own awf
// config. A Git snapshot of a monorepo includes nested adopted projects even
// though they are not nested Git repositories, so snapshot marker scans must
// reproduce the filesystem scanner's .awf boundary explicitly.
func nestedProjectRoots(files []snapshot.File) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return nestedProjectRootsFromPaths(paths)
}

func nestedProjectRootsFromPaths(paths []string) []string {
	const suffix = "/" + config.DirName + "/config.yaml"
	var roots []string
	for _, path := range paths {
		if strings.HasSuffix(path, suffix) {
			roots = append(roots, strings.TrimSuffix(path, suffix))
		}
	}
	slices.Sort(roots)
	return roots
}

func belowAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}
