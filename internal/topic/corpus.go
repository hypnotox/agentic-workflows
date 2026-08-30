package topic

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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

// assembleCorpus builds a Corpus without its marker index from already-read
// topic metadata and parts, the configured domains, and their ownership globs.
// It performs identity pairing, domain-ownership, claim-uniqueness, and
// reference-graph validation, so the filesystem and snapshot loaders share one
// authority over every rule that does not depend on how the bytes were read.
func assembleCorpus(metadata map[string]metaEntry, parts map[string]partEntry, domains []string, domainPaths map[string][]string) (Corpus, error) {
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
			if _, ok := c.byClaim[cl.ID]; ok {
				return Corpus{}, fmt.Errorf("duplicate full claim ID %q", cl.ID)
			}
			c.byClaim[cl.ID] = cl
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

// Clone returns a fully independent semantic corpus projection.
func (c Corpus) Clone() Corpus {
	out := Corpus{
		all:         make([]Topic, len(c.all)),
		byTopic:     make(map[string]*Topic, len(c.byTopic)),
		byClaim:     make(map[string]*Claim, len(c.byClaim)),
		incoming:    cloneStringSlices(c.incoming),
		outgoing:    cloneStringSlices(c.outgoing),
		DomainPaths: cloneStringSlices(c.DomainPaths),
		Markers:     c.Markers.clone(),
	}
	for i, value := range c.all {
		out.all[i] = cloneTopic(value)
		t := &out.all[i]
		out.byTopic[t.ID.String()] = t
		for j := range t.Claims {
			out.byClaim[t.Claims[j].ID] = &t.Claims[j]
		}
	}
	return out
}

func cloneStringSlices(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = slices.Clone(value)
	}
	return out
}

func cloneTopic(value Topic) Topic {
	value.Metadata.Paths = slices.Clone(value.Metadata.Paths)
	value.Claims = slices.Clone(value.Claims)
	for i := range value.Claims {
		value.Claims[i].References = slices.Clone(value.Claims[i].References)
	}
	return value
}

func (c Corpus) All() []Topic {
	out := make([]Topic, len(c.all))
	for i, value := range c.all {
		out[i] = cloneTopic(value)
	}
	return out
}
func (c Corpus) ByTopicID(id string) (Topic, bool) {
	t, ok := c.byTopic[id]
	if !ok {
		return Topic{}, false
	}
	return cloneTopic(*t), true
}
func (c Corpus) ByClaimID(id string) (Claim, bool) {
	cl, ok := c.byClaim[id]
	if !ok {
		return Claim{}, false
	}
	value := *cl
	value.References = slices.Clone(value.References)
	return value, true
}
func (c Corpus) ForDomain(domain string) []Topic {
	var out []Topic
	for _, t := range c.all {
		if t.ID.Domain == domain {
			out = append(out, cloneTopic(t))
		}
	}
	return out
}
func (c Corpus) Incoming(id string) []string { return slices.Clone(c.incoming[id]) }
func (c Corpus) Outgoing(id string) []string { return slices.Clone(c.outgoing[id]) }
