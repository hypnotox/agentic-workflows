package project

import (
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type ContextProjection string

const (
	ContextConcise ContextProjection = "concise"
	ContextFull    ContextProjection = "full"
)

type RequestStatus string

const (
	RequestLiteral           RequestStatus = "literal"
	RequestDirectoryExpanded RequestStatus = "directory-expanded"
	RequestDirectoryEmpty    RequestStatus = "directory-empty"
	RequestGitSelected       RequestStatus = "git-selected"
)

type PathClassification string

const (
	PathCovered           PathClassification = "covered"
	PathEligibleUnowned   PathClassification = "eligible-unowned"
	PathContextIgnored    PathClassification = "context-ignored"
	PathGeneratedOutput   PathClassification = "generated-output"
	PathNestedAdopter     PathClassification = "nested-adopter"
	PathSymlink           PathClassification = "symlink"
	PathNotFound          PathClassification = "not-found"
	PathOutsideRepository PathClassification = "outside-repository"
)

type ContextResult struct {
	Projection ContextProjection        `json:"projection"`
	Requests   []ContextRequest         `json:"requests"`
	Topics     []InvocationTopicContext `json:"topics"`
	Paths      []ContextPath            `json:"paths"`
}
type ContextRequest struct {
	Query          string        `json:"query"`
	Status         RequestStatus `json:"status"`
	EffectivePaths []string      `json:"effectivePaths"`
}
type ContextPath struct {
	Path     string   `json:"path"`
	Requests []string `json:"requests"`
	// GlobLiteral is true when a user-typed query containing glob
	// metacharacters resolved to a missing or context-ignored path: attribution
	// was suppressed and globs are known to be unexpanded.
	GlobLiteral            bool                `json:"globLiteral,omitempty"`
	Classification         PathClassification  `json:"classification"`
	TargetInsideRepository *bool               `json:"targetInsideRepository,omitempty"`
	NestedRoot             string              `json:"nestedRoot,omitempty"`
	Domains                []DomainRef         `json:"domains"`
	Topics                 []PathTopicRef      `json:"topics"`
	Artifacts              []ArtifactRecord    `json:"artifacts"`
	ADR                    *ADRArtifactContext `json:"adr,omitempty"`
}

// InvocationTopicContext is one applicable topic rendered exactly once per
// invocation: its selectors reduced to a brief with a matched-path count, the
// uncapped claim-ID roster, and, in the concise projection, the full detail of
// the invocation's marker-selected direct-claim union (ADR-0147). The full
// projection leaves DirectClaims empty and carries every claim under Full.
type InvocationTopicContext struct {
	ID                 string                  `json:"id"`
	Title              string                  `json:"title"`
	Summary            string                  `json:"summary"`
	Applicability      TopicApplicabilityBrief `json:"applicability"`
	ClaimIDs           []string                `json:"claimIDs"`
	DirectClaims       []ClaimDetail           `json:"directClaims"`
	OmittedDetailCount int                     `json:"omittedDetailCount"`
	TopicCommand       string                  `json:"topicCommand"`
	CoverageCommand    string                  `json:"coverageCommand"`
	Full               *FullTopicContext       `json:"full,omitempty"`
}

// TopicApplicabilityBrief is the selectors-plus-count reduction of the shared
// applicability evidence model; the concrete census stays in `awf topic
// --coverage` (ADR-0147).
type TopicApplicabilityBrief struct {
	DomainPaths      []string `json:"domainPaths"`
	TopicPaths       []string `json:"topicPaths"`
	DeclaredGlobal   bool     `json:"declaredGlobal"`
	MatchedPathCount int      `json:"matchedPathCount"`
}

// PathTopicRef attributes one applicable topic to one effective path: the
// topic's domain-qualified ID and the path's own directly marker-selected claim
// IDs; topic authority lives in the invocation-level collection (ADR-0147).
type PathTopicRef struct {
	ID             string   `json:"id"`
	DirectClaimIDs []string `json:"directClaimIDs"`
}

// ClaimReferences is the stable reference shape populated by the projection batch.
type ClaimReferences struct {
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}
type ClaimDetail struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Prose      string             `json:"prose"`
	Backing    string             `json:"backing"`
	Verify     string             `json:"verify,omitempty"`
	Sites      []topic.MarkerSite `json:"sites"`
	References ClaimReferences    `json:"references"`
}
type FullTopicContext struct {
	Claims  []ClaimDetail   `json:"claims"`
	Pending []PendingChange `json:"pending"`
}
type ADROperationDetail struct {
	Current     *ClaimDetail        `json:"current,omitempty"`
	History     *topic.ClaimHistory `json:"history,omitempty"`
	MarkerSites []topic.MarkerSite  `json:"markerSites"`
}
type ADROperationContext struct {
	Operation     string              `json:"operation"`
	Claim         string              `json:"claim"`
	Topic         string              `json:"topic"`
	Progress      string              `json:"progress"`
	StateSequence int                 `json:"stateSequence,omitempty"`
	ClaimState    string              `json:"claimState"`
	Detail        *ADROperationDetail `json:"detail,omitempty"`
}
type ADRArtifactContext struct {
	Number        string                `json:"number"`
	Title         string                `json:"title"`
	Status        string                `json:"status"`
	Mutability    string                `json:"mutability"`
	AuthorityRole string                `json:"authorityRole"`
	Operations    []ADROperationContext `json:"operations"`
}

type contextPathSet struct {
	tree        *snapshot.Tree
	eligible    []string
	nested      []string
	outputs     map[string]bool
	ignores     []string
	domainPaths map[string][]string
}

func safelyMatchablePaths(tree *snapshot.Tree) []string {
	out := []string{}
	for _, f := range tree.List() {
		if f.Scannable() {
			out = append(out, f.Path)
		}
	}
	return out
}

func buildContextRequests(queries []string, selectedByGit bool, set contextPathSet) ([]ContextRequest, map[string][]string) {
	seen := map[string]bool{}
	requests := []ContextRequest{}
	attribution := map[string][]string{}
	files := set.tree.List()
	for _, raw := range queries {
		if raw == "" {
			continue
		}
		q := filepath.ToSlash(filepath.Clean(raw))
		if seen[q] {
			continue
		}
		seen[q] = true
		r := ContextRequest{Query: q, Status: RequestLiteral, EffectivePaths: []string{}}
		if selectedByGit {
			r.Status = RequestGitSelected
		}
		if !outsideContextPath(q) {
			if _, ok := set.tree.Lookup(q); ok || set.outputs[q] {
				r.EffectivePaths = append(r.EffectivePaths, q)
			} else {
				prefix := q + "/"
				if q == "." {
					prefix = ""
				}
				directory := q == "."
				for _, f := range files {
					if strings.HasPrefix(f.Path, prefix) {
						directory = true
						break
					}
				}
				if directory {
					for _, p := range set.eligible {
						if strings.HasPrefix(p, prefix) {
							r.EffectivePaths = append(r.EffectivePaths, p)
						}
					}
					if len(r.EffectivePaths) == 0 {
						r.Status = RequestDirectoryEmpty
					} else {
						r.Status = RequestDirectoryExpanded
					}
				} else {
					r.EffectivePaths = append(r.EffectivePaths, q)
				}
			}
		} else {
			r.EffectivePaths = append(r.EffectivePaths, q)
		}
		slices.Sort(r.EffectivePaths)
		r.EffectivePaths = slices.Compact(r.EffectivePaths)
		for _, p := range r.EffectivePaths {
			attribution[p] = append(attribution[p], q)
		}
		requests = append(requests, r)
	}
	slices.SortFunc(requests, func(a, b ContextRequest) int { return strings.Compare(a.Query, b.Query) })
	for p := range attribution {
		slices.Sort(attribution[p])
		attribution[p] = slices.Compact(attribution[p])
	}
	return requests, attribution
}

func outsideContextPath(p string) bool {
	return filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../")
}

// globLiteralQuery reports whether p contains glob metacharacters, so a missing
// or ignored user-typed query must not be attributed by string-matching globs.
func globLiteralQuery(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

func classifyContextPath(p string, set contextPathSet) (PathClassification, string, *bool) {
	if outsideContextPath(p) {
		return PathOutsideRepository, "", nil
	}
	for _, root := range set.nested {
		if p == root || strings.HasPrefix(p, root+"/") {
			return PathNestedAdopter, root + "/.awf/config.yaml", nil
		}
	}
	if set.outputs[p] {
		return PathGeneratedOutput, "", nil
	}
	if f, ok := set.tree.Lookup(p); ok && f.Mode == snapshot.Symlink {
		target := string(f.Bytes)
		inside := true
		if path.IsAbs(target) {
			inside = false
		} else {
			joined := path.Clean(path.Join(path.Dir(p), target))
			inside = joined != ".." && !strings.HasPrefix(joined, "../")
		}
		return PathSymlink, "", &inside
	}
	if pathMatchesAny(set.ignores, p) {
		return PathContextIgnored, "", nil
	}
	if _, ok := set.tree.Lookup(p); !ok {
		return PathNotFound, "", nil
	}
	for _, globs := range set.domainPaths {
		if pathMatchesAny(globs, p) {
			return PathCovered, "", nil
		}
	}
	return PathEligibleUnowned, "", nil
}
