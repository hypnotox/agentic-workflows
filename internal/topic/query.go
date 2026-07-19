package topic

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
)

// QueryOptions selects independent detail projections for a current-state query.
type QueryOptions struct {
	History, References, Coverage bool
}

// QueryResult is the single deterministic semantic model used by human and JSON
// presentation. Optional detail blocks are nil unless their corresponding flag
// was requested.
type QueryResult struct {
	Kind       string            `json:"kind"`
	ID         string            `json:"id"`
	Title      string            `json:"title,omitempty"`
	Summary    string            `json:"summary,omitempty"`
	Claims     []QueryClaim      `json:"claims"`
	History    []ClaimHistory    `json:"history,omitempty"`
	References []ClaimReferences `json:"references,omitempty"`
	Coverage   *QueryCoverage    `json:"coverage,omitempty"`
}

type QueryClaim struct {
	ID      string    `json:"id"`
	Type    ClaimType `json:"type"`
	Prose   string    `json:"prose"`
	Backing Backing   `json:"backing"`
	Verify  string    `json:"verify,omitempty"`
}

type ADRHistory struct {
	Number string `json:"number"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type ClaimHistory struct {
	ClaimID   string       `json:"claimId"`
	Origin    ADRHistory   `json:"origin"`
	RevisedBy []ADRHistory `json:"revisedBy"`
}

type ClaimReferences struct {
	ClaimID  string   `json:"claimId"`
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}

type QueryCoverage struct {
	DeclaredGlobal     bool                `json:"declaredGlobal"`
	DeclaredPaths      []string            `json:"declaredPaths"`
	EffectiveSelectors []EffectiveSelector `json:"effectiveSelectors"`
	MarkerSites        []MarkerSite        `json:"markerSites"`
}

// Query resolves one active topic or claim and assembles only the requested
// direct detail. It never traverses references or constructs removed identities.
func Query(c Corpus, adrs adr.Corpus, selector string, opts QueryOptions) (QueryResult, error) {
	topicID, claimID, err := ParseSelector(selector)
	if err != nil {
		return QueryResult{}, err
	}
	t, ok := c.ByTopicID(topicID)
	if !ok {
		return QueryResult{}, fmt.Errorf("current-state topic %q not found", topicID)
	}
	result := QueryResult{Kind: "topic", ID: topicID, Title: t.Metadata.Title, Summary: t.Metadata.Summary, Claims: []QueryClaim{}}
	claims := t.Claims
	if claimID != "" {
		claim, found := c.ByClaimID(claimID)
		if !found {
			return QueryResult{}, fmt.Errorf("current-state claim %q not found", claimID)
		}
		result.Kind, result.ID, result.Title, result.Summary = "claim", claimID, "", ""
		claims = []Claim{claim}
	}
	for _, claim := range claims {
		backing := claim.Backing
		if backing == NoBacking {
			backing = ExplicitNoBacking
		}
		result.Claims = append(result.Claims, QueryClaim{ID: claim.ID, Type: claim.Type, Prose: claim.Prose, Backing: backing, Verify: claim.Verify})
	}
	if opts.History {
		result.History = make([]ClaimHistory, 0, len(claims))
		for _, claim := range claims {
			origin, _ := adrs.ByNumber(claim.Origin)
			h := ClaimHistory{ClaimID: claim.ID, Origin: historyADR(origin), RevisedBy: []ADRHistory{}}
			for _, number := range claim.RevisedBy {
				revision, _ := adrs.ByNumber(number)
				h.RevisedBy = append(h.RevisedBy, historyADR(revision))
			}
			result.History = append(result.History, h)
		}
	}
	if opts.References {
		result.References = make([]ClaimReferences, 0, len(claims))
		for _, claim := range claims {
			incoming, outgoing := c.Incoming(claim.ID), c.Outgoing(claim.ID)
			if incoming == nil {
				incoming = []string{}
			}
			if outgoing == nil {
				outgoing = []string{}
			}
			result.References = append(result.References, ClaimReferences{ClaimID: claim.ID, Incoming: incoming, Outgoing: outgoing})
		}
	}
	if opts.Coverage {
		coverage := CoverageForTopic(t, c.DomainPaths[t.ID.Domain], c.Markers)
		markerSites := coverage.MarkerSites
		if claimID != "" {
			markerSites = c.Markers.ForClaim(claimID)
		}
		result.Coverage = &QueryCoverage{
			DeclaredGlobal: coverage.DeclaredGlobal, DeclaredPaths: nonNil(coverage.DeclaredPaths),
			EffectiveSelectors: nonNil(coverage.EffectiveSelectors), MarkerSites: nonNil(markerSites),
		}
	}
	return result, nil
}

func ParseSelector(selector string) (topicID, claimID string, err error) {
	if claimIDRE.MatchString(selector) {
		return strings.Split(selector, ":")[0], selector, nil
	}
	parts := strings.Split(selector, "/")
	if len(parts) == 2 && kebabRE.MatchString(parts[0]) && kebabRE.MatchString(parts[1]) {
		return selector, "", nil
	}
	return "", "", fmt.Errorf("invalid topic selector %q: expected <domain>/<topic> or <domain>/<topic>:<claim>", selector)
}

func historyADR(a adr.ADR) ADRHistory {
	return ADRHistory{Number: a.Number, Title: strings.TrimPrefix(a.Title, "ADR-"+a.Number+": "), Status: a.Status}
}

func nonNil[S ~[]E, E any](in S) S {
	if in == nil {
		return make(S, 0)
	}
	return slices.Clone(in)
}
