package topic

import (
	"maps"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/severity"
)

// TopicApplicability preserves separate witnesses for repository-wide authority
// and domain-bounded path ownership. ApplicablePaths and OwnedPaths are from the
// caller's selected universe, not symbolic glob-intersection proofs.
type TopicApplicability struct {
	DeclaredGlobal  bool         `json:"declaredGlobal"`
	DomainPaths     []string     `json:"domainPaths"`
	TopicPaths      []string     `json:"topicPaths"`
	ApplicablePaths []string     `json:"applicablePaths"`
	OwnedPaths      []string     `json:"ownedPaths"`
	MarkerSites     []MarkerSite `json:"markerSites"`
}

func ApplicabilityForTopic(t Topic, domainPaths []string, markers MarkerIndex, currentPaths []string) TopicApplicability {
	out := TopicApplicability{
		DeclaredGlobal: t.Metadata.Applies == "global",
		DomainPaths:    nonNil(slices.Clone(domainPaths)), TopicPaths: nonNil(slices.Clone(t.Metadata.Paths)),
		ApplicablePaths: []string{}, OwnedPaths: []string{}, MarkerSites: []MarkerSite{},
	}
	slices.Sort(out.DomainPaths)
	slices.Sort(out.TopicPaths)
	for _, p := range currentPaths {
		if topicMatchesPath(t, out.DomainPaths, p) {
			out.ApplicablePaths = append(out.ApplicablePaths, p)
		}
		if topicOwnsPath(t, out.DomainPaths, p) {
			out.OwnedPaths = append(out.OwnedPaths, p)
		}
	}
	slices.Sort(out.ApplicablePaths)
	out.ApplicablePaths = slices.Compact(out.ApplicablePaths)
	slices.Sort(out.OwnedPaths)
	out.OwnedPaths = slices.Compact(out.OwnedPaths)
	claimIDs := map[string]bool{}
	for _, cl := range t.Claims {
		claimIDs[cl.ID] = true
	}
	for _, site := range markers.All() {
		if claimIDs[site.ClaimID] {
			out.MarkerSites = append(out.MarkerSites, site)
		}
	}
	return out
}

// CoverageKind distinguishes a missing topic-owner finding from a fan-out one.
type CoverageKind string

const (
	// Uncovered marks a domain-owned path with no claim-bearing topic owner.
	Uncovered CoverageKind = "uncovered"
	// Fanout marks a path owned by more topics than the budget.
	Fanout CoverageKind = "fanout"
)

// CoverageFinding is one deterministic coverage result. Domain names the owning
// domain of an Uncovered finding and is empty for a Fanout finding, which is
// emitted once per path across owners; Topics carries a Fanout finding's
// matching count.
type CoverageFinding struct {
	Path   string       `json:"path"`
	Domain string       `json:"domain,omitempty"`
	Kind   CoverageKind `json:"kind"`
	// The rank is not part of this struct's wire form. Stated rather than left
	// implicit: an untagged exported field still marshals, as a bare 0 or 1.
	Severity severity.Rank `json:"-"`
	Topics   int           `json:"topics,omitempty"`
}

// CoveragePolicy carries which coverage checks a caller wants evaluated. A
// caller that does not want a finding class does not request it; no value
// suppresses a requested check (ADR-0183 items 2 and 8).
type CoveragePolicy struct {
	Coverage, Fanout bool
}

const maxTopicsPerPath = 8

// EvaluateCoverage returns the sorted coverage and fan-out findings for the
// eligible paths (ADR-0134 item 11). Every domain owning a path is evaluated
// independently: a domain with no claim-bearing topic owning the path yields one
// Uncovered finding at error, so a topic from one owner never satisfies another
// owner's gap. A global topic owns only its declared paths bounded by its parent
// domain. Across all owners the distinct owning topics matching a path are
// counted once; exceeding the budget yields a single Fanout finding at warn. The caller selects which checks run
// through the policy, and no value suppresses a requested check. Unowned paths
// are the context ownership concern and produce no finding here.
func EvaluateCoverage(c Corpus, paths []string, policy CoveragePolicy) []CoverageFinding {
	domains := slices.Sorted(maps.Keys(c.DomainPaths))
	findings := []CoverageFinding{}
	for _, path := range paths {
		var owners []string
		for _, d := range domains {
			if matchesAny(c.DomainPaths[d], path) {
				owners = append(owners, d)
			}
		}
		if len(owners) == 0 {
			continue
		}
		if policy.Coverage {
			for _, d := range owners {
				if !coveredByDomain(c, d, path) {
					findings = append(findings, CoverageFinding{Path: path, Domain: d, Kind: Uncovered, Severity: severity.Error})
				}
			}
		}
		if policy.Fanout {
			if count := matchingOwningTopics(c, path); count > maxTopicsPerPath {
				findings = append(findings, CoverageFinding{Path: path, Kind: Fanout, Severity: severity.Warn, Topics: count})
			}
		}
	}
	slices.SortFunc(findings, func(a, b CoverageFinding) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		if a.Kind != b.Kind {
			return strings.Compare(string(a.Kind), string(b.Kind))
		}
		return strings.Compare(a.Domain, b.Domain)
	})
	return findings
}

// TopicsForPath returns the topics applicable to a repo-relative path: every
// global topic plus every path-scoped topic whose effective scope (its owning
// domain's paths intersected with the topic's own selectors) covers the path.
// Scoped topics are domain-bounded; global topics remain applicable outside
// their bounded ownership. Results are sorted by topic ID, so a caller's
// per-file selection is deterministic.
func TopicsForPath(c Corpus, path string) []Topic {
	var out []Topic
	for _, t := range c.all {
		if topicMatchesPath(t, c.DomainPaths[t.ID.Domain], path) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b Topic) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	return out
}

// coveredByDomain reports whether domain has a claim-bearing topic whose
// bounded ownership covers path.
func coveredByDomain(c Corpus, domain, path string) bool {
	for _, t := range c.all {
		if t.ID.Domain != domain || len(t.Claims) == 0 {
			continue
		}
		if topicOwnsPath(t, c.DomainPaths[domain], path) {
			return true
		}
	}
	return false
}

// matchingOwningTopics counts the topics whose bounded ownership covers path.
func matchingOwningTopics(c Corpus, path string) int {
	count := 0
	for _, t := range c.all {
		if topicOwnsPath(t, c.DomainPaths[t.ID.Domain], path) {
			count++
		}
	}
	return count
}
