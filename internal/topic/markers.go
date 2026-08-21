package topic

import (
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

const (
	StateMarker   MarkerKind = "state"
	ProofMarker   MarkerKind = "invariant"
	TouchesMarker MarkerKind = "touches-state"
)

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

// Each marker kind gets its own expression so a trailing name is grammatically
// available only to a proof marker (ADR-0199). The name group is greedy, so a
// name containing parentheses captures through to the payload's final closing
// parenthesis. Requiring a non-space first and last character enforces the
// no-surrounding-whitespace rule here, at parse time, rather than letting
// " TestFoo " reach the occurrence check and be reported as a missing unit.
var statePayloadRE = regexp.MustCompile(`^state: (` + claimIDPattern + `)$`)
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `) \((\S(?:.*\S)?)\)$`)
var touchesPayloadRE = regexp.MustCompile(`^touches-state: (` + claimIDPattern + `) - (.+)$`)

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

// scanMarkerBytes scans one source file's bytes for the marker families that
// select it, resolving each valid marker into a site on idx. It is the byte-fed
// scan core shared by the filesystem walker and the snapshot loader.
func scanMarkerBytes(idx MarkerIndex, rel string, b []byte, sources []config.CurrentStateSource, corpus Corpus, cfg *config.CurrentStateConfig) error {
	lines := strings.Split(string(b), "\n")
	for n, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, src := range sources {
			if !strings.HasPrefix(trimmed, src.Marker) {
				continue
			}
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, src.Marker))
			payload, ok := markerPayload(trimmed, src)
			if !ok {
				if markerCandidate(raw) {
					return fmt.Errorf("%s:%d: current-state marker is missing closing token %q", rel, n+1, src.Close)
				}
				continue
			}
			if !markerCandidate(payload) {
				continue
			}
			site, name, err := resolveMarker(rel, n+1, payload, corpus, cfg)
			if err != nil {
				return err
			}
			// Verified here, at the point the site resolves, rather than after
			// the loop: deferring would report every resolve error in the file
			// ahead of every occurrence error, and the scan must keep reporting
			// the first failure in line order (ADR-0199 item 3).
			if site.Kind == ProofMarker && !proofNameOccurs(lines, name, src.Marker) {
				return fmt.Errorf("%s:%d: proof marker for %s names %q, which does not occur in this file; the test was deleted, renamed, or moved", rel, n+1, site.ClaimID, name)
			}
			idx.sites[site.ClaimID] = append(idx.sites[site.ClaimID], site)
			break
		}
	}
	return nil
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
func markerCandidate(payload string) bool {
	for _, prefix := range []string{"state:", "invariant:", "touches-state:"} {
		if strings.HasPrefix(payload, prefix) {
			return true
		}
	}
	return false
}

// resolveMarker parses one marker payload into a site, returning the proof
// name alongside it. The name is empty for a state or touches marker, which
// have no grammatical slot for one.
func resolveMarker(path string, line int, payload string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerSite, string, error) {
	s := MarkerSite{Path: path, Line: line}
	name := ""
	if m := statePayloadRE.FindStringSubmatch(payload); m != nil {
		s.Kind = StateMarker
		s.ClaimID = m[1]
	} else if m := proofPayloadRE.FindStringSubmatch(payload); m != nil {
		s.Kind = ProofMarker
		s.ClaimID = m[1]
		name = m[2]
	} else if m := unnamedProofPayloadRE.FindStringSubmatch(payload); m != nil {
		return s, "", fmt.Errorf("%s:%d: proof marker for %s does not name a proving unit", path, line, m[1])
	} else if m := touchesPayloadRE.FindStringSubmatch(payload); m != nil {
		s.Kind = TouchesMarker
		s.ClaimID = m[1]
		s.Note = strings.TrimSpace(m[2])
	} else {
		return s, "", fmt.Errorf("%s:%d: malformed current-state marker %q", path, line, payload)
	}
	claim, ok := corpus.byClaim[s.ClaimID]
	if !ok {
		return s, "", fmt.Errorf("%s:%d: unknown claim ID %s", path, line, s.ClaimID)
	}
	t := corpus.byTopic[strings.Split(s.ClaimID, ":")[0]]
	if s.Kind == ProofMarker {
		if claim.Type != Invariant || claim.Backing != TestBacking {
			return s, "", fmt.Errorf("%s:%d: proof marker targets non-test-backed invariant %s", path, line, s.ClaimID)
		}
		if !matchesAny(cfg.TestGlobs, path) {
			return s, "", fmt.Errorf("%s:%d: proof marker is outside currentState.testGlobs", path, line)
		}
	} else if !topicMatchesPath(*t, corpus.DomainPaths[t.ID.Domain], path) {
		return s, "", fmt.Errorf("%s:%d: marker for %s is outside effective topic scope", path, line, s.ClaimID)
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
func proofNameOccurs(lines []string, name, marker string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), marker) {
			continue
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
				return true
			}
			off = start + 1
		}
	}
	return false
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
