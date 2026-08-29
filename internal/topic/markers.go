package topic

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

type MarkerKind string

const ProofMarker MarkerKind = "invariant"

type MarkerSite struct {
	Path    string     `json:"path"`
	Line    int        `json:"line"`
	Kind    MarkerKind `json:"kind"`
	ClaimID string     `json:"claimId"`
	Note    string     `json:"note,omitempty"`
}
type MarkerIndex struct{ sites map[string][]MarkerSite }

func (m MarkerIndex) clone() MarkerIndex {
	if m.sites == nil {
		return MarkerIndex{}
	}
	out := MarkerIndex{sites: make(map[string][]MarkerSite, len(m.sites))}
	for claim, sites := range m.sites {
		out.sites[claim] = slices.Clone(sites)
	}
	return out
}

func (m MarkerIndex) ForClaim(id string) []MarkerSite { return slices.Clone(m.sites[id]) }
func (m MarkerIndex) All() []MarkerSite {
	var out []MarkerSite
	for id := range m.sites {
		out = append(out, m.ForClaim(id)...)
	}
	sortSites(out)
	return out
}

const claimIDPattern = `[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*`

// The name group is greedy, so a name containing parentheses captures through
// the payload's final closing parenthesis. Requiring a non-space first and last
// character enforces the no-surrounding-whitespace rule at parse time.
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `) \((\S(?:.*\S)?)\)$`)

// unnamedProofPayloadRE is the diagnostic fallback for a proof marker that
// proofPayloadRE rejected. It deliberately also matches a padded or empty
// parenthetical, so "invariant: <id> ( TestFoo )" and "invariant: <id> ()"
// reach the named diagnostic instead of the generic malformed-marker error.
// Ordering makes this safe: proofPayloadRE is attempted first, so a well-formed
// named marker never reaches it.
var unnamedProofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `)(?: \(.*\))?$`)

// matchingSources returns the configured marker-source families whose globs
// select the repo-relative slash path rel.
func matchingSources(cfg *config.CurrentStateConfig, rel string) []config.CurrentStateSource {
	var sources []config.CurrentStateSource
	for _, src := range cfg.Sources {
		if matchesAny(src.Globs, rel) {
			sources = append(sources, src)
		}
	}
	return sources
}

const maxMarkerLineBytes = 4 << 20

type markerLineSource func(func(string) error) (bool, error)

// scanMarkerBytes adapts already-materialized snapshot bytes to the bounded
// line scanner without copying the complete source into a string and line slice.
func scanMarkerBytes(idx MarkerIndex, rel string, b []byte, sources []config.CurrentStateSource, corpus Corpus, cfg *config.CurrentStateConfig) error {
	return scanMarkerLines(idx, rel, byteLineSource(b), sources, corpus, cfg)
}

func byteLineSource(b []byte) markerLineSource {
	return func(visit func(string) error) (bool, error) {
		for start := 0; start <= len(b); {
			end := start + bytes.IndexByte(b[start:], '\n')
			if end < start {
				end = len(b)
			}
			if end-start > maxMarkerLineBytes {
				return true, fmt.Errorf("marker source line exceeds %d bytes", maxMarkerLineBytes)
			}
			if err := visit(string(b[start:end])); err != nil {
				return true, err
			}
			if end == len(b) {
				break
			}
			start = end + 1
		}
		return true, nil
	}
}

func scanMarkerLines(idx MarkerIndex, rel string, lines markerLineSource, sources []config.CurrentStateSource, corpus Corpus, cfg *config.CurrentStateConfig) error {
	lineNumber := 0
	_, err := lines(func(line string) error {
		lineNumber++
		trimmed := strings.TrimSpace(line)
		for _, src := range sources {
			if !strings.HasPrefix(trimmed, src.Marker) {
				continue
			}
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, src.Marker))
			payload, ok := markerPayload(trimmed, src)
			if !ok {
				if markerCandidate(raw) {
					return fmt.Errorf("%s:%d: current-state marker is missing closing token %q", rel, lineNumber, src.Close)
				}
				continue
			}
			if !markerCandidate(payload) {
				continue
			}
			site, name, err := resolveMarker(rel, lineNumber, payload, corpus, cfg)
			if err != nil {
				return err
			}
			// Verify at the resolving marker to preserve first-failure line order.
			occurs, err := proofNameOccurs(lines, name, src.Marker)
			if err != nil {
				return err
			}
			if site.Kind == ProofMarker && !occurs {
				return fmt.Errorf("%s:%d: proof marker for %s names %q, which does not occur in this file; the test was deleted, renamed, or moved", rel, lineNumber, site.ClaimID, name)
			}
			idx.sites[site.ClaimID] = append(idx.sites[site.ClaimID], site)
			break
		}
		return nil
	})
	return err
}

// finalizeMarkerIndex validates each claim's backing contract against the
// scanned sites (test-backed invariants require a proof marker; unbacked ones
// forbid one) and path-sorts every claim's sites. It runs once after all source
// files are scanned, regardless of how they were collected.
func finalizeMarkerIndex(idx MarkerIndex, corpus Corpus) error {
	for id, claim := range corpus.byClaim {
		sites := idx.sites[id]
		proofs := 0
		for _, s := range sites {
			if s.Kind == ProofMarker {
				proofs++
			}
		}
		if claim.Type == Invariant && claim.Backing == TestBacking && proofs == 0 {
			return fmt.Errorf("test-backed invariant %s has no proof marker", id)
		}
		if claim.Type == Invariant && claim.Backing == Unbacked && proofs > 0 { // coverage-ignore: resolveMarker rejects an unbacked proof before it can enter the index
			return fmt.Errorf("unbacked invariant %s must not have a proof marker", id)
		}
		sortSites(sites)
		idx.sites[id] = sites
	}
	return nil
}

func markerPayload(line string, src config.CurrentStateSource) (string, bool) {
	if !strings.HasPrefix(line, src.Marker) {
		return "", false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, src.Marker))
	if src.Close != "" {
		if !strings.HasSuffix(payload, src.Close) {
			return "", false
		}
		payload = strings.TrimSpace(strings.TrimSuffix(payload, src.Close))
	}
	return payload, true
}
func markerCandidate(payload string) bool { return strings.HasPrefix(payload, "invariant:") }

// resolveMarker parses one proof marker payload into a site and proving-unit name.
func resolveMarker(path string, line int, payload string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerSite, string, error) {
	s := MarkerSite{Path: path, Line: line, Kind: ProofMarker}
	match := proofPayloadRE.FindStringSubmatch(payload)
	if match != nil {
		s.ClaimID = match[1]
	} else if unnamed := unnamedProofPayloadRE.FindStringSubmatch(payload); unnamed != nil {
		return s, "", fmt.Errorf("%s:%d: proof marker for %s does not name a proving unit", path, line, unnamed[1])
	} else {
		return s, "", fmt.Errorf("%s:%d: malformed current-state marker %q", path, line, payload)
	}
	name := match[2]
	claim, ok := corpus.byClaim[s.ClaimID]
	if !ok {
		return s, "", fmt.Errorf("%s:%d: unknown claim ID %s", path, line, s.ClaimID)
	}
	if claim.Type != Invariant || claim.Backing != TestBacking {
		return s, "", fmt.Errorf("%s:%d: proof marker targets non-test-backed invariant %s", path, line, s.ClaimID)
	}
	if !matchesAny(cfg.TestGlobs, path) {
		return s, "", fmt.Errorf("%s:%d: proof marker is outside currentState.testGlobs", path, line)
	}
	return s, name, nil
}

// proofNameOccurs reports whether name appears verbatim on some line whose
// trimmed form does not open with the family's marker token, and not as part of
// a longer identifier. The flanking condition is what catches a rename, in both
// directions: a marker naming TestFoo is satisfied by neither a surviving
// TestFooBar nor a surviving XTestFoo.
//
// Recognition is syntactic and line-local. One condition does the excluding: for
// a family whose marker token is its comment leader, which covers // and #, it
// skips WHOLE-LINE comments, and every marker line is a special case of one, so
// a stack of markers naming one unit cannot satisfy itself and the marker's own
// line needs no separate case (ADR-0199 item 3). The token is a parameter
// because the exclusion is per-family; it cannot be hardcoded.
//
// Two forms deliberately stay searchable, because the test is where a line
// OPENS rather than what it contains. A trailing comment on a code line is
// searched, so a cross-reference like `continue // see TestFoo` can satisfy a
// marker naming TestFoo. And a family whose token is a prefixed or block-comment
// form excludes only lines opening with that exact token. Both are accepted
// false negatives: narrowing them needs per-language comment parsing, which the
// check refuses (ADR-0199 item 9).
func proofNameOccurs(lines markerLineSource, name, marker string) (bool, error) {
	found := false
	_, err := lines(func(line string) error {
		if found || strings.HasPrefix(strings.TrimSpace(line), marker) {
			return nil
		}
		// Padding removes the bounds cases from the flanking test: a match can
		// never sit at either end of the padded line.
		padded := " " + line + " "
		for off := 1; ; {
			j := strings.Index(padded[off:], name)
			if j < 0 {
				break
			}
			start := off + j
			// Continue past a flanked hit rather than giving up on the line: the
			// same name can occur twice on one line, once as part of a longer
			// identifier and once on its own, as in a wrapper calling the test.
			if !identBefore(padded[:start]) && !identAt(padded[start+len(name):]) {
				found = true
				break
			}
			off = start + 1
		}
		return nil
	})
	return found, err
}

// identBefore and identAt decode a whole rune rather than testing a byte, so an
// adopter whose identifiers or test labels carry non-ASCII letters gets the same
// rename protection as an ASCII one (ADR-0199 item 2 defines the name as free
// text, not a Go identifier).
func identBefore(s string) bool {
	r, size := utf8.DecodeLastRuneInString(s)
	return size > 0 && identRune(r)
}

func identAt(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	return size > 0 && identRune(r)
}

func identRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// topicMatchesPath answers repository-wide applicability. A global topic is
// authoritative everywhere, including outside the bounded paths it may own.
// PathAuthority returns deterministic owning domains and applicable topics for a lexical repository path. Global topics are applicable outside bounded domain ownership but do not create a domain owner.
func PathAuthority(c Corpus, path string) ([]string, []string) {
	domains := []string{}
	for domain, selectors := range c.DomainPaths {
		if pathglob.MatchAny(selectors, path) {
			domains = append(domains, domain)
		}
	}
	slices.Sort(domains)
	topics := []string{}
	for _, candidate := range c.All() {
		if topicMatchesPath(candidate, c.DomainPaths[candidate.ID.Domain], path) {
			topics = append(topics, candidate.ID.String())
		}
	}
	slices.Sort(topics)
	return domains, topics
}

func topicMatchesPath(t Topic, domainPaths []string, path string) bool {
	if t.Metadata.Applies == "global" {
		return true
	}
	return topicOwnsPath(t, domainPaths, path)
}

// topicOwnsPath answers domain-bounded path ownership. Selectors are the only
// ownership declaration, so a global topic without paths owns nothing.
func topicOwnsPath(t Topic, domainPaths []string, path string) bool {
	return matchesAny(t.Metadata.Paths, path) && matchesAny(domainPaths, path)
}
func matchesAny(globs []string, path string) bool {
	for _, g := range globs {
		if pathglob.Match(g, path) {
			return true
		}
	}
	return false
}
func sortSites(s []MarkerSite) {
	slices.SortFunc(s, func(a, b MarkerSite) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
}
