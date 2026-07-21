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
	Kind           string            `json:"kind"`
	ID             string            `json:"id"`
	Title          string            `json:"title,omitempty"`
	Summary        string            `json:"summary,omitempty"`
	Claims         []QueryClaim      `json:"claims"`
	History        []ClaimHistory    `json:"history,omitempty"`
	References     []ClaimReferences `json:"references,omitempty"`
	Coverage       *QueryCoverage    `json:"coverage,omitempty"`
	HistoricalOnly bool              `json:"historicalOnly,omitempty"`
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
	ClaimID        string       `json:"claimId"`
	Origin         *ADRHistory  `json:"origin,omitempty"`
	LegacyBaseline bool         `json:"legacyBaseline,omitempty"`
	RevisedBy      []ADRHistory `json:"revisedBy"`
	RemovedBy      *ADRHistory  `json:"removedBy,omitempty"`
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
// direct detail. A qualified removed claim resolves only when History is set;
// it never traverses references or constructs tombstone state.
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
			if opts.History {
				operations, historical := adrs.ClaimOperationHistory(claimID)
				if historical {
					history, complete := presentationHistory(claimID, operations)
					if complete {
						return QueryResult{
							Kind: "claim", ID: claimID, Claims: []QueryClaim{}, HistoricalOnly: true,
							History: []ClaimHistory{history},
						}, nil
					}
				}
			}
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
			if operations, ok := adrs.ClaimOperationHistory(claim.ID); ok {
				if history, complete := presentationHistory(claim.ID, operations); complete {
					result.History = append(result.History, history)
					continue
				}
			}
			origin, _ := adrs.ByNumber(claim.Origin)
			originHistory := historyADR(origin)
			h := ClaimHistory{ClaimID: claim.ID, Origin: &originHistory, RevisedBy: []ADRHistory{}}
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

func presentationHistory(claimID string, history adr.ClaimOperationHistory) (ClaimHistory, bool) {
	result := ClaimHistory{ClaimID: claimID, LegacyBaseline: history.LegacyBaseline, RevisedBy: []ADRHistory{}}
	if history.Origin != nil {
		origin := operationADR(*history.Origin)
		result.Origin = &origin
	} else if !history.LegacyBaseline {
		return ClaimHistory{}, false
	}
	for _, revision := range history.RevisedBy {
		result.RevisedBy = append(result.RevisedBy, operationADR(revision))
	}
	if history.RemovedBy != nil {
		removed := operationADR(*history.RemovedBy)
		result.RemovedBy = &removed
	}
	return result, true
}

func operationADR(record adr.OperationRecord) ADRHistory {
	return ADRHistory{Number: record.Number, Title: strings.TrimPrefix(record.Title, "ADR-"+record.Number+": "), Status: record.Status}
}

func nonNil[S ~[]E, E any](in S) S {
	if in == nil {
		return make(S, 0)
	}
	return slices.Clone(in)
}
