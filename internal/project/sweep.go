package project

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// claimedModel is the ADR-0086 Decision 1 allowlist: every path under .awf/
// is either claimed here or drift. files holds claimed file paths
// (project-relative, slash-separated); dirs holds structural directories
// legal even when empty; artifacts/singletons index the catalog facts needed
// to classify unknown sidecars and convention-part directories.
type claimedModel struct {
	files      map[string]bool
	dirs       map[string]bool
	artifacts  map[string]map[string]bool // kind -> name -> catalog member
	singletons map[string]bool            // known singleton kinds
}

// claimedDir reports whether dir may exist: a structural dir or an ancestor
// of a claimed file.
func (m *claimedModel) claimedDir(dir string) bool {
	if m.dirs[dir] {
		return true
	}
	pre := dir + "/"
	for f := range m.files {
		if strings.HasPrefix(f, pre) {
			return true
		}
	}
	return false
}

// buildClaimedModel computes the claimed-path model from config, catalog,
// and the plan write files (whose .awf/-prefixed paths are exactly the
// enabled config-tree render units - the model derives from the same code
// path that writes them, per the ADR's dual-bookkeeping consequence).
func (p *Project) buildClaimedModel(files []RenderedFile, topics topic.Corpus) *claimedModel {
	m := &claimedModel{
		files: map[string]bool{
			config.DirName + "/config.yaml":                   true,
			config.DirName + "/awf.lock":                      true,
			config.DirName + "/current-state-upgrade.journal": true,
		},
		dirs: map[string]bool{
			config.DirName:             true,
			config.DirName + "/parts":  true,
			config.DirName + "/memory": true,
		},
		artifacts:  map[string]map[string]bool{},
		singletons: map[string]bool{},
	}
	// A resident root is a structural directory even when its dynamic tree is
	// empty. The names come from the resident table, never re-spelled here.
	for _, name := range resident.RootNames() {
		m.dirs[config.DirName+"/"+name] = true
	}
	for _, f := range files {
		if strings.HasPrefix(f.Path, config.DirName+"/") {
			m.files[f.Path] = true
		}
	}
	for _, d := range kindDescriptors {
		kind := d.Plural
		m.dirs[config.DirName+"/"+kind] = true
		m.dirs[config.DirName+"/"+kind+"/parts"] = true
		m.artifacts[kind] = map[string]bool{}
		names := p.Cfg.Domains
		if d.poolNames != nil {
			names = d.poolNames(p.Cat)
		}
		for _, name := range names {
			m.artifacts[kind][name] = true
			m.files[config.DirName+"/"+kind+"/"+name+".yaml"] = true
			m.dirs[config.DirName+"/"+kind+"/parts/"+name] = true
			for _, sec := range p.declaredSections(kind, name) {
				m.files[config.DirName+"/"+kind+"/parts/"+name+"/"+sec+".md"] = true
			}
		}
	}
	for _, kind := range catalog.SingletonKinds() {
		m.files[config.DirName+"/"+kind+".yaml"] = true
		m.singletons[kind] = true
		m.dirs[config.DirName+"/parts/"+kind] = true
		for _, sec := range p.Cat.Docs[kind].Sections {
			m.files[config.DirName+"/parts/"+kind+"/"+sec+".md"] = true
		}
	}
	// Topics are a discovered producer family rather than an enable-list kind.
	m.dirs[config.DirName+"/topics"] = true
	m.dirs[config.DirName+"/topics/metadata"] = true
	m.dirs[config.DirName+"/topics/parts"] = true
	for _, domain := range p.Cfg.Domains {
		m.dirs[config.DirName+"/topics/metadata/"+domain] = true
		m.dirs[config.DirName+"/topics/parts/"+domain] = true
	}
	for _, t := range topics.All() {
		metadataDir := config.DirName + "/topics/metadata/" + t.ID.Domain
		partsDomain := config.DirName + "/topics/parts/" + t.ID.Domain
		partsTopic := partsDomain + "/" + t.ID.Slug
		m.dirs[metadataDir], m.dirs[partsDomain], m.dirs[partsTopic] = true, true, true
		m.files[metadataDir+"/"+t.ID.Slug+".yaml"] = true
		m.files[partsTopic+"/current-state.md"] = true
	}
	// The runner is a section-bearing config-tree unit but not a SingletonKind, so
	// its convention-part territory is always claimed here - the wrapper's
	// single awf-owned section whose `awf:edit ... create <part> to override`
	// pointer invites a part - so render and the closed-tree sweep agree
	// (ADR-0086/0156).
	m.dirs[config.DirName+"/runner/parts"] = true
	for _, sec := range runnerSections {
		m.files[config.DirName+"/runner/parts/"+sec+".md"] = true
	}
	return m
}

var awfBakRE = regexp.MustCompile(`\.awf-bak(\.\d+)?$`)

// classify labels one unclaimed entry: the pre-ADR-0086 orphan shapes keep
// specific repair hints, sync-written backups get the stale-backup detail
// (inv: awf-bak-flagged), and everything else is unclaimed.
// touches-state: rendering/sync-and-drift:awf-bak-flagged - stale awf-bak backup classification; proof in sweep_test.go
func (m *claimedModel) classify(rel string, isDir bool) manifest.Drift {
	d := manifest.Drift{Path: rel, Kind: "orphaned"}
	segs := strings.Split(rel, "/") // segs[0] is always ".awf"
	switch {
	case !isDir && awfBakRE.MatchString(rel):
		d.Detail = "stale awf-bak backup: review and delete"
	// Singleton parts tree: .awf/parts/<kind>[/<section>.md].
	case len(segs) == 3 && segs[1] == "parts" && isDir && !m.singletons[segs[2]]:
		d.Detail = "convention parts for an unknown singleton kind"
	case len(segs) == 4 && segs[1] == "parts" && !isDir && strings.HasSuffix(segs[3], ".md"):
		d.Detail = "convention part for a section not in the singleton's declared set"
	// Kind trees: .awf/<kind>/<name>.yaml and .awf/<kind>/parts/<name>[/<sec>.md].
	case len(segs) == 3 && !isDir && strings.HasSuffix(segs[2], ".yaml") && m.artifacts[segs[1]] != nil:
		d.Detail = "sidecar for an artifact not in the catalog"
	case len(segs) == 4 && segs[2] == "parts" && isDir && m.artifacts[segs[1]] != nil && !m.artifacts[segs[1]][segs[3]]:
		d.Detail = "convention parts for an artifact not in the catalog"
	case len(segs) == 5 && segs[2] == "parts" && !isDir && strings.HasSuffix(segs[4], ".md") && m.artifacts[segs[1]] != nil && m.artifacts[segs[1]][segs[3]]:
		d.Detail = "convention part for a section not in the target's declared set"
	default:
		d.Detail = "unclaimed file or directory: not part of the .awf config tree; delete it or move it out"
	}
	return d
}

// sweepConfigTree walks .awf/ and reports every entry outside the
// claimed-path model (ADR-0086 Decision 1), collapsing to the highest
// fully-unclaimed directory. The resident roots are dynamic local state and
// are wholly exempt. It subsumes the pre-ADR-0086 orphan sweep: wrong-name
// sidecars/parts and undeclared sections keep their detail strings
// (inv: drift-source-set; ADR-0011 section-orphan-flagged).
func (p *Project) sweepConfigTree(files []RenderedFile, topics topic.Corpus) ([]manifest.Drift, error) {
	m := p.buildClaimedModel(files, topics)
	var drift []manifest.Drift
	walkErr := filepath.WalkDir(filepath.Join(p.Root, config.DirName), func(path string, de fs.DirEntry, err error) error {
		if err != nil { // coverage-ignore: Check requires the lock inside .awf, so the tree exists; a mid-walk error is a permission fault a test cannot trigger
			return err
		}
		rel, rerr := filepath.Rel(p.Root, path)
		if rerr != nil { // coverage-ignore: path is always under p.Root
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == config.DirName {
			return nil
		}
		if de.IsDir() {
			// Dynamic residents exist only at the primary control root. A linked
			// checkout's same-named tree is not resident authority and must not be
			// silently exempted from its tracked config sweep.
			if resident.IsResidentPath(rel) && p.Root == p.roots.Resident {
				return filepath.SkipDir
			}
			if m.claimedDir(rel) {
				return nil
			}
			drift = append(drift, m.classify(rel, true))
			return filepath.SkipDir
		}
		if m.files[rel] {
			return nil
		}
		drift = append(drift, m.classify(rel, false))
		return nil
	})
	if walkErr != nil { // coverage-ignore: the callback only returns permission-fault errors (above)
		return nil, walkErr
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].Path < drift[j].Path })
	return drift, nil
}
