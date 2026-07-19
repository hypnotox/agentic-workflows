package bridge

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

type Mapping struct {
	Key, Destination, Origin, Backing string
	Approved                          bool
}

type Mutation struct {
	Path          string `json:"path"`
	BeforePresent bool   `json:"beforePresent"`
	BeforeMode    uint32 `json:"beforeMode"`
	BeforeSHA256  string `json:"beforeSHA256"`
	AfterPresent  bool   `json:"afterPresent"`
	AfterMode     uint32 `json:"afterMode"`
	AfterSHA256   string `json:"afterSHA256"`
	Before, After []byte `json:"-"`
}

func imageHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func newMutation(root, path string, after []byte, afterPresent bool, afterMode uint32) (Mutation, error) {
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	present, mode := true, uint32(0)
	if os.IsNotExist(err) {
		before, present, err = nil, false, nil
	} else if err == nil {
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if statErr != nil { // coverage-ignore: ReadFile just succeeded for the same path; failure requires a concurrent filesystem race
			return Mutation{}, statErr
		}
		mode = uint32(info.Mode().Perm())
	}
	if err != nil { // coverage-ignore: only a non-NotExist filesystem fault reaches this branch
		return Mutation{}, err
	}
	if !afterPresent {
		after, afterMode = nil, 0
	}
	return Mutation{Path: path, BeforePresent: present, BeforeMode: mode, BeforeSHA256: imageHash(before), AfterPresent: afterPresent, AfterMode: afterMode, AfterSHA256: imageHash(after), Before: before, After: after}, nil
}

func DeriveMappings(inventory Inventory, topics []topic.Topic) ([]Mapping, error) {
	var mappings []Mapping
	for _, legacy := range inventory.Entries {
		if !legacy.Active {
			continue
		}
		var exact []topic.Claim
		classMismatch := false
		for _, t := range topics {
			for _, claim := range t.Claims {
				if claim.Type != topic.Invariant || claim.Slug != legacy.Slug || claim.Origin != legacy.Declarer {
					continue
				}
				backing := string(claim.Backing)
				if backing != legacy.Backing {
					classMismatch = true
					continue
				}
				exact = append(exact, claim)
			}
		}
		if len(exact) != 1 {
			if classMismatch && len(exact) == 0 {
				return nil, fmt.Errorf("legacy invariant %s changes backing class", legacy.Key)
			}
			return nil, fmt.Errorf("legacy invariant %s maps to %d current-state claims; require exactly one", legacy.Key, len(exact))
		}
		mappings = append(mappings, Mapping{Key: string(legacy.Key), Destination: exact[0].ID, Origin: "ADR-" + legacy.Declarer, Backing: legacy.Backing})
	}
	slices.SortFunc(mappings, func(a, b Mapping) int { return strings.Compare(a.Key, b.Key) })
	return mappings, nil
}

func ApplyApprovals(inventory Inventory, mappings []Mapping, approvals Approvals) ([]Mapping, error) {
	if !approvals.Present {
		return mappings, fmt.Errorf("%s is required", ApprovalPath)
	}
	byKey := map[string]int{}
	for i := range mappings {
		byKey[mappings[i].Key] = i
	}
	retired := map[string]bool{}
	for _, entry := range inventory.Entries {
		if !entry.Active {
			retired[string(entry.Key)] = true
			if entry.History == nil {
				return mappings, fmt.Errorf("retired invariant %s lacks migration history", entry.Key)
			}
		}
	}
	seen := map[string]bool{}
	for _, approval := range approvals.Entries {
		if retired[approval.Key] {
			return mappings, fmt.Errorf("retired invariant %s forbids an approval", approval.Key)
		}
		i, ok := byKey[approval.Key]
		if !ok {
			return mappings, fmt.Errorf("approval names unknown live invariant %s", approval.Key)
		}
		if seen[approval.Key] {
			return mappings, fmt.Errorf("duplicate approval for %s", approval.Key)
		}
		seen[approval.Key] = true
		if mappings[i].Destination != approval.Destination {
			return mappings, fmt.Errorf("approval destination for %s is %s; derived destination is %s", approval.Key, approval.Destination, mappings[i].Destination)
		}
		mappings[i].Approved = true
	}
	for _, mapping := range mappings {
		if !mapping.Approved {
			return mappings, fmt.Errorf("missing approval for %s", mapping.Key)
		}
	}
	return mappings, nil
}

func PlanNormalization(root string, cfg *config.Config, corpus adr.Corpus, inventory Inventory, mappings []Mapping) ([]Mutation, error) {
	var mutations []Mutation
	converted, err := config.ConvertInvariantsToCurrentState(cfg.Source())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(converted, cfg.Source()) {
		m, err := newMutation(root, config.DirName+"/config.yaml", converted, true, 0o644)
		if err != nil { // coverage-ignore: config.Load already read this same fixed path; failure requires a concurrent filesystem race
			return nil, err
		}
		mutations = append(mutations, m)
	}
	for _, a := range corpus.All() {
		raw, err := rawADR(corpus, a.Number)
		if err != nil { // coverage-ignore: corpus loading just read this ADR; failure requires a concurrent filesystem race
			return nil, err
		}
		planned := raw
		var missing []LegacyInvariant
		for _, entry := range inventory.Entries {
			if entry.Carrier == a.Number && entry.History == nil {
				missing = append(missing, entry)
			}
		}
		if len(missing) > 0 {
			if _, err := time.Parse("2006-01-02", a.Date); err != nil {
				return nil, fmt.Errorf("ADR-%s requires a valid frontmatter date for migration history", a.Number)
			}
			slices.SortFunc(missing, func(x, y LegacyInvariant) int { return strings.Compare(string(x.Key), string(y.Key)) })
			var block strings.Builder
			offset, exists := migrationHistoryInsertOffset(planned)
			if offset > 0 && planned[offset-1] != '\n' {
				block.WriteByte('\n')
			}
			if !exists {
				block.WriteString("\n## Migration history\n")
			}
			block.WriteByte('\n')
			for _, entry := range missing {
				fmt.Fprintf(&block, "- %s: retired invariant `%s`; basis: encoded\n", a.Date, entry.Key)
			}
			planned = append(planned[:offset], append([]byte(block.String()), planned[offset:]...)...)
		}
		if a.IsLegacyShipped() && a.IsSuperseded() {
			re := regexp.MustCompile(`(?m)^status: Superseded$`)
			if !re.Match(planned) {
				return nil, fmt.Errorf("ADR-%s Superseded status is not canonical", a.Number)
			}
			planned = re.ReplaceAll(planned, []byte("status: Implemented"))
		}
		if !bytes.Equal(raw, planned) {
			rel, _ := filepath.Rel(root, a.Path)
			m, err := newMutation(root, filepath.ToSlash(rel), planned, true, fileMode(root, filepath.ToSlash(rel)))
			if err != nil { // coverage-ignore: rawADR just read this same path successfully; failure requires a concurrent filesystem race
				return nil, err
			}
			mutations = append(mutations, m)
		}
	}
	markerMutations, err := planMarkerRewrites(root, cfg.Invariants, mappings)
	if err != nil {
		return nil, err
	}
	mutations = mergeMutations(mutations, markerMutations)
	slices.SortFunc(mutations, func(a, b Mutation) int { return strings.Compare(a.Path, b.Path) })
	return mutations, nil
}

func migrationHistoryInsertOffset(data []byte) (int, bool) {
	text := string(data)
	pos, in, fence, fenceLen := 0, false, byte(0), 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trim := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
		if marker, n, ok := historyFence(trim); ok {
			if fence == 0 {
				fence, fenceLen = marker, n
			} else if marker == fence && n >= fenceLen && strings.TrimSpace(trim[n:]) == "" {
				fence, fenceLen = 0, 0
			}
			pos += len(line)
			continue
		}
		if fence == 0 {
			if trim == "## Migration history" {
				in = true
			} else if in && strings.HasPrefix(trim, "## ") {
				return pos, true
			}
		}
		pos += len(line)
	}
	return len(data), in
}

func rawADR(corpus adr.Corpus, number string) ([]byte, error) { return corpus.Raw(number) }

func fileMode(root, path string) uint32 {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return 0o644
	}
	return uint32(info.Mode().Perm())
}

func mergeMutations(a, b []Mutation) []Mutation {
	byPath := map[string]Mutation{}
	for _, m := range a {
		byPath[m.Path] = m
	}
	for _, m := range b {
		if prior, ok := byPath[m.Path]; ok {
			m.Before, m.BeforePresent, m.BeforeMode, m.BeforeSHA256 = prior.Before, prior.BeforePresent, prior.BeforeMode, prior.BeforeSHA256
		}
		byPath[m.Path] = m
	}
	out := make([]Mutation, 0, len(byPath))
	for _, m := range byPath {
		out = append(out, m)
	}
	return out
}

func planMarkerRewrites(root string, cfg *config.InvariantConfig, mappings []Mapping) ([]Mutation, error) {
	if cfg == nil {
		return nil, nil
	}
	bySlug := map[string]string{}
	for _, mapping := range mappings {
		slug := mapping.Key[strings.IndexByte(mapping.Key, '#')+1:]
		if prior := bySlug[slug]; prior != "" && prior != mapping.Destination {
			return nil, fmt.Errorf("unqualified marker slug %s is ambiguous", slug)
		}
		bySlug[slug] = mapping.Destination
	}
	var out []Mutation
	err := filepath.WalkDir(root, func(path string, de os.DirEntry, err error) error {
		if err != nil { // coverage-ignore: requires a permission fault or concurrent source-tree removal
			return err
		}
		if de.IsDir() {
			if path != root && (de.Name() == ".git" || de.Name() == "vendor" || de.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if path != root {
				if _, err := os.Stat(filepath.Join(path, ".awf")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		relSlash := filepath.ToSlash(rel)
		var sources []config.InvariantSource
		for _, src := range cfg.Sources {
			for _, glob := range src.Globs {
				if pathglob.Match(glob, relSlash) {
					sources = append(sources, src)
					break
				}
			}
		}
		if len(sources) == 0 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: WalkDir just returned this regular file; failure requires a concurrent filesystem race
			return err
		}
		lines := strings.SplitAfter(string(raw), "\n")
		changed := false
		for n, line := range lines {
			ending := ""
			body := line
			if strings.HasSuffix(body, "\n") {
				ending, body = "\n", strings.TrimSuffix(body, "\n")
			}
			trim := strings.TrimSpace(body)
			for _, src := range sources {
				if !strings.HasPrefix(trim, src.Marker) {
					continue
				}
				payload := strings.TrimSpace(strings.TrimPrefix(trim, src.Marker))
				closeToken := ""
				if src.Close != "" {
					if !strings.HasSuffix(payload, src.Close) {
						if strings.HasPrefix(payload, "invariant:") || strings.HasPrefix(payload, "touches-invariant:") {
							return fmt.Errorf("%s:%d: unqualified marker is missing closing token %q", relSlash, n+1, src.Close)
						}
						continue
					}
					closeToken, payload = " "+src.Close, strings.TrimSpace(strings.TrimSuffix(payload, src.Close))
				}
				indent := body[:len(body)-len(strings.TrimLeft(body, " \t"))]
				if strings.HasPrefix(payload, "invariant: ") && !strings.Contains(strings.TrimPrefix(payload, "invariant: "), ":") {
					slug := strings.TrimSpace(strings.TrimPrefix(payload, "invariant: "))
					dest := bySlug[slug]
					if dest == "" {
						return fmt.Errorf("%s:%d: unqualified invariant marker %s has no live mapping", relSlash, n+1, slug)
					}
					lines[n], changed = indent+src.Marker+" invariant: "+dest+closeToken+ending, true
				} else if strings.HasPrefix(payload, "touches-invariant: ") {
					rest := strings.TrimPrefix(payload, "touches-invariant: ")
					parts := strings.SplitN(rest, " - ", 2)
					if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
						return fmt.Errorf("%s:%d: touches-invariant marker requires a note", relSlash, n+1)
					}
					dest := bySlug[strings.TrimSpace(parts[0])]
					if dest == "" {
						return fmt.Errorf("%s:%d: touches marker has no live mapping", relSlash, n+1)
					}
					lines[n], changed = indent+src.Marker+" touches-state: "+dest+" - "+strings.TrimSpace(parts[1])+closeToken+ending, true
				}
				break
			}
		}
		if changed {
			after := []byte(strings.Join(lines, ""))
			m, err := newMutation(root, relSlash, after, true, fileMode(root, relSlash))
			if err != nil { // coverage-ignore: newMutation rereads the same source file read successfully above
				return err
			}
			out = append(out, m)
		}
		return nil
	})
	return out, err
}
