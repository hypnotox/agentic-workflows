package topic

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"gopkg.in/yaml.v3"
)

const (
	treeMetadataRoot   = config.DirName + "/topics/metadata"
	treeMetadataPrefix = treeMetadataRoot + "/"
	treePartsPrefix    = config.DirName + "/topics/parts/"
	treePartSuffix     = "/current-state.md"
)

// LoadCorpusFromTree parses the current-state topic corpus from an immutable
// snapshot instead of the live filesystem, so a working-tree, index, or commit
// universe yields exactly the corpus that tree encodes. Topic metadata, parts,
// domain ownership, and marker sources are all read from tree; cfg supplies the
// configured domains and marker-source families (parse it from the same tree
// for a single-universe load). It shares assembleCorpus and the marker
// scan/validate core with the filesystem LoadCorpus, so both loaders enforce
// identical rules over identically shaped bytes.
func LoadCorpusFromTree(tree *snapshot.Tree, cfg *config.Config, adrs adr.Corpus) (Corpus, error) {
	files := tree.List()
	metadata := map[string]metaEntry{}
	parts := map[string]partEntry{}
	for _, f := range files {
		switch {
		case strings.HasPrefix(f.Path, treeMetadataPrefix) && strings.HasSuffix(f.Path, ".yaml"):
			id, m, err := ParseMetadata(treeMetadataRoot, f.Path, f.Bytes)
			if err != nil {
				return Corpus{}, err
			}
			if err := recordMeta(metadata, id, metaEntry{meta: m, path: f.Path}); err != nil { // coverage-ignore: distinct snapshot paths yield distinct topic IDs; recordMeta's duplicate branch is proven by TestRecordMetaRejectsDuplicateID
				return Corpus{}, err
			}
		case strings.HasPrefix(f.Path, treePartsPrefix) && strings.HasSuffix(f.Path, treePartSuffix):
			seg := strings.Split(strings.TrimPrefix(f.Path, treePartsPrefix), "/")
			if len(seg) != 3 || !kebabRE.MatchString(seg[0]) || !kebabRE.MatchString(seg[1]) {
				return Corpus{}, fmt.Errorf("invalid topic part path %q", f.Path)
			}
			parts[(TopicID{seg[0], seg[1]}).String()] = partEntry{data: f.Bytes, path: f.Path}
		}
	}
	domainPaths := map[string][]string{}
	for _, d := range cfg.Domains {
		paths, err := domainPathsFromTree(tree, d)
		if err != nil {
			return Corpus{}, err
		}
		domainPaths[d] = paths
	}
	c, err := assembleCorpus(metadata, parts, cfg.Domains, domainPaths, adrs)
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

// domainPathsFromTree reads one domain sidecar's ownership globs from the
// snapshot, mirroring config.Sidecar's zero-Sidecar-on-missing contract so a
// domain without a sidecar owns no paths rather than failing.
func domainPathsFromTree(tree *snapshot.Tree, domain string) ([]string, error) {
	f, ok := tree.Lookup(config.DirName + "/domains/" + domain + ".yaml")
	if !ok {
		return nil, nil
	}
	var sc config.Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(f.Bytes))
	dec.KnownFields(true)
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("parse domain sidecar %s: %w", domain, err)
	}
	return slices.Clone(sc.Paths), nil
}

// markerIndexFromTreeFiles scans a snapshot's files for current-state markers,
// reusing the byte-fed scan/validate core. The snapshot is already the selected
// Git universe (nested repositories and ignored paths are excluded upstream), so
// marker selection is by configured source globs alone.
func markerIndexFromTreeFiles(files []snapshot.File, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerIndex, error) {
	idx := MarkerIndex{sites: map[string][]MarkerSite{}}
	if cfg != nil {
		for _, f := range files {
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
