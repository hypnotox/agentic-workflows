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

func LoadCorpus(root string, cfg *config.Config, adrs adr.Corpus) (Corpus, error) {
	base := filepath.Join(root, config.DirName, "topics")
	metadataRoot, partsRoot := filepath.Join(base, "metadata"), filepath.Join(base, "parts")
	metadata := map[string]string{}
	parts := map[string]string{}
	if err := collectFiles(metadataRoot, func(path string) error {
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		id, _, err := readMetadata(metadataRoot, path)
		if err != nil {
			return err
		}
		return recordTopicPath(metadata, id, path)
	}); err != nil {
		return Corpus{}, err
	}
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
		parts[(TopicID{seg[0], seg[1]}).String()] = path
		return nil
	}); err != nil {
		return Corpus{}, err
	}
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
	for _, d := range cfg.Domains {
		configured[d] = true
	}
	c := Corpus{byTopic: map[string]*Topic{}, byClaim: map[string]*Claim{}, incoming: map[string][]string{}, outgoing: map[string][]string{}, DomainPaths: map[string][]string{}}
	for _, d := range cfg.Domains {
		sc, err := cfg.Sidecar("domains", d)
		if err != nil { // coverage-ignore: Project.Open already read and validated every configured domain sidecar
			return Corpus{}, err
		}
		c.DomainPaths[d] = slices.Clone(sc.Paths)
	}
	for _, key := range ids {
		mp, mo := metadata[key]
		pp, po := parts[key]
		if !mo {
			return Corpus{}, fmt.Errorf("topic %s has a part but no metadata", key)
		}
		if !po {
			return Corpus{}, fmt.Errorf("topic %s has metadata but no current-state part", key)
		}
		id, m, err := readMetadata(metadataRoot, mp)
		if err != nil { // coverage-ignore: discovery parsed this same metadata file earlier in this call
			return Corpus{}, err
		}
		if !configured[id.Domain] {
			return Corpus{}, fmt.Errorf("topic %s belongs to unconfigured domain %q", id.String(), id.Domain)
		}
		b, err := os.ReadFile(pp)
		if err != nil { // coverage-ignore: discovery just walked this file; failure requires a concurrent filesystem race
			return Corpus{}, err
		}
		t, err := ParsePart(id, pp, b)
		if err != nil {
			return Corpus{}, fmt.Errorf("parse topic part %s: %w", filepath.ToSlash(pp), err)
		}
		t.Metadata, t.MetadataPath = m, mp
		c.all = append(c.all, t)
	}
	for i := range c.all {
		t := &c.all[i]
		c.byTopic[t.ID.String()] = t
		for j := range t.Claims {
			cl := &t.Claims[j]
			if _, ok := c.byClaim[cl.ID]; ok { // coverage-ignore: one canonical filesystem pair owns each path-derived topic ID, and local duplicates fail ParsePart
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
	markers, err := BuildMarkerIndex(root, c, cfg.CurrentState)
	if err != nil {
		return Corpus{}, err
	}
	c.Markers = markers
	return c, nil
}

func recordTopicPath(paths map[string]string, id TopicID, path string) error {
	key := id.String()
	if prior, ok := paths[key]; ok {
		return fmt.Errorf("duplicate topic ID %q discovered at %q and %q", key, filepath.ToSlash(prior), filepath.ToSlash(path))
	}
	paths[key] = path
	return nil
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
