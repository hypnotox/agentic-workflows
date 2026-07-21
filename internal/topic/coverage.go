package topic

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type EffectiveSelector struct {
	DomainPath string `json:"domainPath"`
	TopicPath  string `json:"topicPath"`
}
type TopicCoverage struct {
	DeclaredGlobal          bool
	DeclaredPaths           []string
	EffectiveSelectors      []EffectiveSelector
	HasClaims               bool
	SatisfiesScopedCoverage bool
	MarkerSites             []MarkerSite
}

func CoverageForTopic(t Topic, domainPaths []string, markers MarkerIndex) TopicCoverage {
	out := TopicCoverage{DeclaredGlobal: t.Metadata.Applies == "global", DeclaredPaths: slices.Clone(t.Metadata.Paths), HasClaims: len(t.Claims) > 0}
	if !out.DeclaredGlobal {
		for _, d := range domainPaths {
			for _, p := range t.Metadata.Paths {
				out.EffectiveSelectors = append(out.EffectiveSelectors, EffectiveSelector{DomainPath: d, TopicPath: p})
			}
		}
		out.SatisfiesScopedCoverage = out.HasClaims && len(out.EffectiveSelectors) > 0
	}
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

// CoverageSeverity is the configured strictness for a coverage or fan-out
// finding: CoverageError fails a gated command, CoverageWarn reports without
// failing, and CoverageOff suppresses the finding entirely (ADR-0134 item 11).
type CoverageSeverity string

const (
	// CoverageError makes a consuming command exit nonzero.
	CoverageError CoverageSeverity = "error"
	// CoverageWarn reports the finding without changing the exit code.
	CoverageWarn CoverageSeverity = "warn"
	// CoverageOff suppresses the finding so the evaluator never emits it.
	CoverageOff CoverageSeverity = "off"
)

// CoverageKind distinguishes a missing-scoped-topic finding from a fan-out one.
type CoverageKind string

const (
	// Uncovered marks a domain-owned path with no claim-bearing scoped topic.
	Uncovered CoverageKind = "uncovered"
	// Fanout marks a path matched by more path-scoped topics than the budget.
	Fanout CoverageKind = "fanout"
)

// CoverageFinding is one deterministic coverage result. Domain names the owning
// domain of an Uncovered finding and is empty for a Fanout finding, which is
// emitted once per path across owners; Topics carries a Fanout finding's
// matching count.
type CoverageFinding struct {
	Path     string           `json:"path"`
	Domain   string           `json:"domain,omitempty"`
	Kind     CoverageKind     `json:"kind"`
	Severity CoverageSeverity `json:"severity"`
	Topics   int              `json:"topics,omitempty"`
}

// CoveragePolicy carries the configured coverage/fan-out severities and the
// per-path fan-out budget.
type CoveragePolicy struct {
	Coverage, Fanout CoverageSeverity
	MaxTopicsPerPath int
}

// ClaimBudgetNotes returns one deterministic advisory for each topic whose
// claim count is strictly above maxClaimsPerTopic.
func ClaimBudgetNotes(c Corpus, maxClaimsPerTopic int) []string {
	var notes []string
	topics := c.All()
	slices.SortFunc(topics, func(a, b Topic) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	for _, t := range topics {
		if len(t.Claims) <= maxClaimsPerTopic {
			continue
		}
		id := t.ID.String()
		notes = append(notes, fmt.Sprintf("topic %s has %d claims, above maxClaimsPerTopic limit %d; consider splitting .awf/topics/metadata/%s.yaml and .awf/topics/parts/%s/current-state.md", id, len(t.Claims), maxClaimsPerTopic, id, id))
	}
	return notes
}

// EvaluateCoverage returns the sorted coverage and fan-out findings for the
// eligible paths (ADR-0134 item 11). Every domain owning a path is evaluated
// independently: a domain with no claim-bearing, path-scoped topic covering the
// path yields one Uncovered finding at the coverage severity, so a topic from
// one owner never satisfies another owner's gap. Global and claimless topics
// never satisfy scoped coverage. Across all owners the distinct path-scoped
// topics matching a path are counted once; exceeding the budget yields a single
// Fanout finding at the fan-out severity. Globals are excluded from the count,
// and a CoverageOff severity suppresses its findings. Unowned paths are the
// context ownership concern and produce no finding here.
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
		if policy.Coverage != CoverageOff {
			for _, d := range owners {
				if !coveredByDomain(c, d, path) {
					findings = append(findings, CoverageFinding{Path: path, Domain: d, Kind: Uncovered, Severity: policy.Coverage})
				}
			}
		}
		if policy.Fanout != CoverageOff {
			if count := matchingScopedTopics(c, path); count > policy.MaxTopicsPerPath {
				findings = append(findings, CoverageFinding{Path: path, Kind: Fanout, Severity: policy.Fanout, Topics: count})
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
// domain's paths intersected with the topic's own selectors) covers the path. A
// topic never applies outside its domain ownership by construction. Results are
// sorted by topic ID, so a caller's per-file selection is deterministic.
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

// coveredByDomain reports whether domain has a claim-bearing, path-scoped topic
// whose effective scope covers path.
func coveredByDomain(c Corpus, domain, path string) bool {
	for _, t := range c.all {
		if t.ID.Domain != domain || t.Metadata.Applies == "global" || len(t.Claims) == 0 {
			continue
		}
		if topicMatchesPath(t, c.DomainPaths[domain], path) {
			return true
		}
	}
	return false
}

// matchingScopedTopics counts the path-scoped topics whose effective scope
// covers path, excluding global topics.
func matchingScopedTopics(c Corpus, path string) int {
	count := 0
	for _, t := range c.all {
		if t.Metadata.Applies == "global" {
			continue
		}
		if topicMatchesPath(t, c.DomainPaths[t.ID.Domain], path) {
			count++
		}
	}
	return count
}
