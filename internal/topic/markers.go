package topic

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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

func (m MarkerIndex) ForClaim(id string) []MarkerSite { return slices.Clone(m.sites[id]) }
func (m MarkerIndex) All() []MarkerSite {
	var out []MarkerSite
	for id := range m.sites {
		out = append(out, m.ForClaim(id)...)
	}
	sortSites(out)
	return out
}

var markerPayloadRE = regexp.MustCompile(`^(state|invariant): ([a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*)$`)
var touchesPayloadRE = regexp.MustCompile(`^touches-state: ([a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*) - (.+)$`)

type markerWalkDir func(string, fs.WalkDirFunc) error

func BuildMarkerIndex(root string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerIndex, error) {
	return buildMarkerIndex(root, corpus, cfg, filepath.WalkDir)
}

func buildMarkerIndex(root string, corpus Corpus, cfg *config.CurrentStateConfig, walk markerWalkDir) (MarkerIndex, error) {
	idx := MarkerIndex{sites: map[string][]MarkerSite{}}
	if cfg != nil {
		err := walk(root, func(path string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				switch de.Name() {
				case ".git", "vendor", "node_modules":
					return filepath.SkipDir
				}
				if path != root {
					if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
						return filepath.SkipDir
					}
					if _, err := os.Lstat(filepath.Join(path, config.DirName)); err == nil {
						return filepath.SkipDir
					}
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil { // coverage-ignore: WalkDir yields paths beneath root, so Rel cannot fail
				return err
			}
			rel = filepath.ToSlash(rel)
			var sources []config.CurrentStateSource
			for _, src := range cfg.Sources {
				if matchesAny(src.Globs, rel) {
					sources = append(sources, src)
				}
			}
			if len(sources) == 0 {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil { // coverage-ignore: WalkDir just returned this file; failure requires a concurrent filesystem race
				return err
			}
			for n, line := range strings.Split(string(b), "\n") {
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
					site, err := resolveMarker(rel, n+1, payload, corpus, cfg)
					if err != nil {
						return err
					}
					idx.sites[site.ClaimID] = append(idx.sites[site.ClaimID], site)
					break
				}
			}
			return nil
		})
		if err != nil {
			return MarkerIndex{}, fmt.Errorf("scan current-state markers under %s: %w", filepath.ToSlash(root), err)
		}
	}
	for id, claim := range corpus.byClaim {
		sites := idx.sites[id]
		proofs := 0
		for _, s := range sites {
			if s.Kind == ProofMarker {
				proofs++
			}
		}
		if claim.Type == Invariant && claim.Backing == TestBacking && proofs == 0 {
			return MarkerIndex{}, fmt.Errorf("test-backed invariant %s has no proof marker", id)
		}
		if claim.Type == Invariant && claim.Backing == Unbacked && proofs > 0 { // coverage-ignore: resolveMarker rejects an unbacked proof before it can enter the index
			return MarkerIndex{}, fmt.Errorf("unbacked invariant %s must not have a proof marker", id)
		}
		sortSites(sites)
		idx.sites[id] = sites
	}
	return idx, nil
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
func resolveMarker(path string, line int, payload string, corpus Corpus, cfg *config.CurrentStateConfig) (MarkerSite, error) {
	s := MarkerSite{Path: path, Line: line}
	if m := markerPayloadRE.FindStringSubmatch(payload); m != nil {
		s.Kind = MarkerKind(m[1])
		s.ClaimID = m[2]
	} else if m := touchesPayloadRE.FindStringSubmatch(payload); m != nil {
		s.Kind = TouchesMarker
		s.ClaimID = m[1]
		s.Note = strings.TrimSpace(m[2])
	} else {
		return s, fmt.Errorf("%s:%d: malformed current-state marker %q", path, line, payload)
	}
	claim, ok := corpus.byClaim[s.ClaimID]
	if !ok {
		return s, fmt.Errorf("%s:%d: unknown claim ID %s", path, line, s.ClaimID)
	}
	t := corpus.byTopic[strings.Split(s.ClaimID, ":")[0]]
	if s.Kind == ProofMarker {
		if claim.Type != Invariant || claim.Backing != TestBacking {
			return s, fmt.Errorf("%s:%d: proof marker targets non-test-backed invariant %s", path, line, s.ClaimID)
		}
		if !matchesAny(cfg.TestGlobs, path) {
			return s, fmt.Errorf("%s:%d: proof marker is outside currentState.testGlobs", path, line)
		}
	} else if !topicMatchesPath(*t, corpus.DomainPaths[t.ID.Domain], path) {
		return s, fmt.Errorf("%s:%d: marker for %s is outside effective topic scope", path, line, s.ClaimID)
	}
	return s, nil
}
func topicMatchesPath(t Topic, domainPaths []string, path string) bool {
	if t.Metadata.Applies == "global" {
		return true
	}
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
