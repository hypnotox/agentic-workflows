package topic

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

type Corpus struct {
	all                []Topic
	byTopic            map[string]*Topic
	byClaim            map[string]*Claim
	incoming, outgoing map[string][]string
	DomainPaths        map[string][]string
	Markers            MarkerIndex
}

// metaEntry is one topic's already-parsed metadata plus the source path it came
// from (an absolute filesystem path or a repo-relative slash path).
type metaEntry struct {
	meta Metadata
	path string
}

// partEntry is one topic's raw current-state part bytes plus its source path.
type partEntry struct {
	data []byte
	path string
}

// recordMeta inserts one topic's metadata, rejecting a second source that
// derives the same topic ID. The duplicate case is unreachable through a real
// filesystem or snapshot walk - each metadata path yields a distinct ID - so it
// is proven by a direct unit test rather than a loader fixture.
func recordMeta(metadata map[string]metaEntry, id TopicID, entry metaEntry) error {
	key := id.String()
	if prior, ok := metadata[key]; ok {
		return fmt.Errorf("duplicate topic ID %q discovered at %q and %q", key, filepath.ToSlash(prior.path), filepath.ToSlash(entry.path))
	}
	metadata[key] = entry
	return nil
}

// LoadCorpus parses the on-disk .awf/topics tree into a Corpus, reading domain
// ownership from cfg and scanning marker sources under root. It reads every
// input into memory and delegates the identity, provenance, reference, and
// marker assembly to assembleCorpus, the byte-fed core the snapshot loader
// shares.
func LoadCorpus(root string, cfg *config.Config, adrs adr.Corpus) (Corpus, error) {
	base := filepath.Join(root, config.DirName, "topics")
	metadataRoot, partsRoot := filepath.Join(base, "metadata"), filepath.Join(base, "parts")
	metadata := map[string]metaEntry{}
	if err := collectFiles(metadataRoot, func(path string) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		id, m, err := readMetadata(metadataRoot, path)
		if err != nil {
			return err
		}
		return recordMeta(metadata, id, metaEntry{meta: m, path: path})
	}); err != nil {
		return Corpus{}, err
	}
	parts := map[string]partEntry{}
	if err := collectFiles(partsRoot, func(path string) error {
		if filepath.Base(path) != "current-state.md" {
			return nil
		}
		rel, err := filepath.Rel(partsRoot, path)
		if err != nil { // coverage-ignore: WalkDir yields a path beneath partsRoot, so Rel cannot fail
			return err
		}
		seg := strings.Split(filepath.ToSlash(rel), "/")
		if len(seg) != 3 || !kebabRE.MatchString(seg[0]) || !kebabRE.MatchString(seg[1]) {
			return fmt.Errorf("invalid topic part path %q", filepath.ToSlash(path))
		}
		b, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: discovery just walked this file; failure requires a concurrent filesystem race
			return err
		}
		parts[(TopicID{seg[0], seg[1]}).String()] = partEntry{data: b, path: path}
		return nil
	}); err != nil {
		return Corpus{}, err
	}
	domainPaths := map[string][]string{}
	for _, d := range cfg.Domains {
		sc, err := cfg.Sidecar("domains", d)
		if err != nil { // coverage-ignore: Project.Open already read and validated every configured domain sidecar
			return Corpus{}, err
		}
		domainPaths[d] = slices.Clone(sc.Paths)
	}
	c, err := assembleCorpus(metadata, parts, cfg.Domains, domainPaths, adrs)
	if err != nil {
		return Corpus{}, err
	}
	markers, err := BuildMarkerIndex(root, c, cfg.CurrentState)
	if err != nil {
		return Corpus{}, err
	}
	c.Markers = markers
	return c, nil
}

// assembleCorpus builds a Corpus without its marker index from already-read
// topic metadata and parts, the configured domains, their ownership globs, and
// the ADR corpus for provenance validation. It performs identity pairing,
// domain-ownership, claim-uniqueness, Implemented-provenance, and reference-graph
// validation, so the filesystem and snapshot loaders share one authority over
// every rule that does not depend on how the bytes were read.
func assembleCorpus(metadata map[string]metaEntry, parts map[string]partEntry, domains []string, domainPaths map[string][]string, adrs adr.Corpus) (Corpus, error) {
	ids := make([]string, 0, len(metadata)+len(parts))
	seen := map[string]bool{}
	for id := range metadata {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range parts {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	configured := map[string]bool{}
	for _, d := range domains {
		configured[d] = true
	}
	c := Corpus{byTopic: map[string]*Topic{}, byClaim: map[string]*Claim{}, incoming: map[string][]string{}, outgoing: map[string][]string{}, DomainPaths: domainPaths}
	for _, key := range ids {
		me, mo := metadata[key]
		pe, po := parts[key]
		if !mo {
			return Corpus{}, fmt.Errorf("topic %s has a part but no metadata", key)
		}
		if !po {
			return Corpus{}, fmt.Errorf("topic %s has metadata but no current-state part", key)
		}
		seg := strings.Split(key, "/")
		id := TopicID{seg[0], seg[1]}
		if !configured[id.Domain] {
			return Corpus{}, fmt.Errorf("topic %s belongs to unconfigured domain %q", key, id.Domain)
		}
		t, err := ParsePart(id, pe.path, pe.data)
		if err != nil {
			return Corpus{}, fmt.Errorf("parse topic part %s: %w", filepath.ToSlash(pe.path), err)
		}
		t.Metadata, t.MetadataPath = me.meta, me.path
		c.all = append(c.all, t)
	}
	for i := range c.all {
		t := &c.all[i]
		c.byTopic[t.ID.String()] = t
		for j := range t.Claims {
			cl := &t.Claims[j]
			if _, ok := c.byClaim[cl.ID]; ok { // coverage-ignore: one canonical pair owns each path-derived topic ID, and local duplicates fail ParsePart
				return Corpus{}, fmt.Errorf("duplicate full claim ID %q", cl.ID)
			}
			c.byClaim[cl.ID] = cl
			for _, num := range append([]string{cl.Origin}, cl.RevisedBy...) {
				a, ok := adrs.ByNumber(num)
				if !ok {
					return Corpus{}, fmt.Errorf("claim %s cites missing ADR-%s", cl.ID, num)
				}
				if !a.IsImplemented() {
					return Corpus{}, fmt.Errorf("claim %s cites ADR-%s with status %q; provenance must be Implemented", cl.ID, num, a.Status)
				}
			}
		}
	}
	for _, t := range c.all {
		for _, cl := range t.Claims {
			for _, ref := range cl.References {
				if ref == cl.ID {
					return Corpus{}, fmt.Errorf("claim %s references itself", cl.ID)
				}
				if _, ok := c.byClaim[ref]; !ok {
					return Corpus{}, fmt.Errorf("claim %s has dangling reference %s", cl.ID, ref)
				}
				c.outgoing[cl.ID] = append(c.outgoing[cl.ID], ref)
				c.incoming[ref] = append(c.incoming[ref], cl.ID)
			}
		}
	}
	for k := range c.incoming {
		slices.Sort(c.incoming[k])
	}
	for k := range c.outgoing {
		slices.Sort(c.outgoing[k])
	}
	return c, nil
}

func collectFiles(root string, fn func(string) error) error {
	return filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err // coverage-ignore: after the missing-root case, WalkDir errors require a permission or concurrent filesystem fault
		}
		if de.IsDir() {
			return nil
		}
		return fn(path)
	})
}

func (c Corpus) All() []Topic { return slices.Clone(c.all) }
func (c Corpus) ByTopicID(id string) (Topic, bool) {
	t, ok := c.byTopic[id]
	if !ok {
		return Topic{}, false
	}
	return *t, true
}
func (c Corpus) ByClaimID(id string) (Claim, bool) {
	cl, ok := c.byClaim[id]
	if !ok {
		return Claim{}, false
	}
	return *cl, true
}
func (c Corpus) ForDomain(domain string) []Topic {
	var out []Topic
	for _, t := range c.all {
		if t.ID.Domain == domain {
			out = append(out, t)
		}
	}
	return out
}
func (c Corpus) Incoming(id string) []string { return slices.Clone(c.incoming[id]) }
func (c Corpus) Outgoing(id string) []string { return slices.Clone(c.outgoing[id]) }
