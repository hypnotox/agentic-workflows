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
)

// claimedModel is the ADR-0086 Decision 1 allowlist: every path under .awf/
// is either claimed here or drift. files holds claimed file paths
// (project-relative, slash-separated); dirs holds structural directories
// legal even when empty; enabled/singletons index the artifact facts the
// classifier needs to keep the pre-ADR-0086 detail strings - locality is
// never stored, because an enabled-but-unclaimed parts dir already implies
// local: true (buildClaimedModel claims every non-local artifact's parts).
type claimedModel struct {
	files      map[string]bool
	dirs       map[string]bool
	enabled    map[string]map[string]bool // kind → name → enabled
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
// and the RenderAll output (whose .awf/-prefixed paths are exactly the
// enabled config-tree render units - the model derives from the same code
// path that writes them, per the ADR's dual-bookkeeping consequence).
func (p *Project) buildClaimedModel(files []RenderedFile) (*claimedModel, error) {
	m := &claimedModel{
		files: map[string]bool{
			config.DirName + "/config.yaml":                   true,
			config.DirName + "/awf.lock":                      true,
			config.DirName + "/current-state-migration.yaml":  true,
			config.DirName + "/current-state-upgrade.journal": true,
		},
		dirs: map[string]bool{
			config.DirName:             true,
			config.DirName + "/parts":  true,
			config.DirName + "/memory": true,
		},
		enabled:    map[string]map[string]bool{},
		singletons: map[string]bool{},
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
		m.enabled[kind] = map[string]bool{}
		for _, name := range d.enable(p.Cfg) {
			m.enabled[kind][name] = true
			m.files[config.DirName+"/"+kind+"/"+name+".yaml"] = true
			sc, err := p.Cfg.Sidecar(kind, name)
			if err != nil { // coverage-ignore: RenderAll read this sidecar earlier in the same Check pass
				return nil, err
			}
			// A local: true artifact renders nothing, so its parts are
			// dead weight - deliberately unclaimed (ADR-0086 Decision 1).
			// A local: true domain sidecar cannot reach here: open-time
			// validation rejects any non-paths: domain field (Decision 5).
			if sc.Local {
				continue
			}
			m.dirs[config.DirName+"/"+kind+"/parts/"+name] = true
			for _, sec := range p.declaredSections(kind, name) {
				m.files[config.DirName+"/"+kind+"/parts/"+name+"/"+sec+".md"] = true
			}
		}
	}
	for _, kind := range catalog.SingletonKinds() {
		m.files[config.DirName+"/"+kind+".yaml"] = true
		m.singletons[kind] = true
		sc, err := p.Cfg.Sidecar(kind, "")
		if err != nil { // coverage-ignore: RenderAll read the singleton sidecars earlier in the same Check pass
			return nil, err
		}
		if sc.Local {
			continue
		}
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
	topics, err := p.Topics()
	if err != nil { // coverage-ignore: Check builds a valid OutputPlan from the same cached topic corpus before sweeping
		return nil, err
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
	// its convention-part territory is claimed here when enabled - the two awf-owned
	// sections whose `awf:edit ... create <part> to override` pointer invites a part
	// (the two in-place sections instead error via section-source-exclusive if a part
	// appears), so render and the closed-tree sweep agree (ADR-0086/0101).
	if p.Cfg.Runner != nil && p.Cfg.Runner.Enabled {
		m.dirs[config.DirName+"/runner/parts"] = true
		for _, sec := range runnerSections {
			m.files[config.DirName+"/runner/parts/"+sec+".md"] = true
		}
	}
	return m, nil
}

var awfBakRE = regexp.MustCompile(`\.awf-bak(\.\d+)?$`)

// classify labels one unclaimed entry: the pre-ADR-0086 orphan shapes keep
// their ADR-0011 detail strings byte-identical, sync-written backups get
// the stale-backup detail (inv: awf-bak-flagged), local-managed artifacts'
// parts their own, and everything else is unclaimed.
// touches-invariant: awf-bak-flagged - stale awf-bak backup classification; proof in sweep_test.go
func (m *claimedModel) classify(rel string, isDir bool) manifest.Drift {
	const localDetail = "convention parts for a local-managed artifact (local: true renders nothing)"
	d := manifest.Drift{Path: rel, Kind: "orphaned"}
	segs := strings.Split(rel, "/") // segs[0] is always ".awf"
	switch {
	case !isDir && awfBakRE.MatchString(rel):
		d.Detail = "stale awf-bak backup: review and delete"
	// Singleton parts tree: .awf/parts/<kind>[/<section>.md].
	case len(segs) == 3 && segs[1] == "parts" && isDir && !m.singletons[segs[2]]:
		d.Detail = "convention parts for an unknown singleton kind"
	case len(segs) == 3 && segs[1] == "parts" && isDir:
		d.Detail = localDetail // known singleton, unclaimed dir ⇒ local: true
	case len(segs) == 4 && segs[1] == "parts" && !isDir && strings.HasSuffix(segs[3], ".md"):
		d.Detail = "convention part for a section not in the singleton's declared set"
	// Kind trees: .awf/<kind>/<name>.yaml and .awf/<kind>/parts/<name>[/<sec>.md].
	case len(segs) == 3 && !isDir && strings.HasSuffix(segs[2], ".yaml") && m.enabled[segs[1]] != nil:
		d.Detail = "sidecar for an artifact not in the enable list"
	case len(segs) == 4 && segs[2] == "parts" && isDir && m.enabled[segs[1]] != nil && !m.enabled[segs[1]][segs[3]]:
		d.Detail = "convention parts for an artifact not in the enable list"
	case len(segs) == 4 && segs[2] == "parts" && isDir && m.enabled[segs[1]] != nil:
		d.Detail = localDetail // enabled name, unclaimed dir ⇒ local: true
	case len(segs) == 5 && segs[2] == "parts" && !isDir && strings.HasSuffix(segs[4], ".md") && m.enabled[segs[1]] != nil && m.enabled[segs[1]][segs[3]]:
		d.Detail = "convention part for a section not in the target's declared set"
	default:
		d.Detail = "unclaimed file or directory: not part of the .awf config tree; delete it or move it out"
	}
	return d
}

// sweepConfigTree walks .awf/ and reports every entry outside the
// claimed-path model (ADR-0086 Decision 1), collapsing to the highest
// fully-unclaimed directory. memory/** is session scratch and wholly exempt
// (ADR-0069). It subsumes the pre-ADR-0086 orphan sweep: wrong-name
// sidecars/parts and undeclared sections keep their detail strings
// (inv: drift-source-set; ADR-0011 section-orphan-flagged).
// invariant: closed-config-tree
// invariant: drift-source-set
// invariant: section-orphan-flagged
func (p *Project) sweepConfigTree(files []RenderedFile) ([]manifest.Drift, error) {
	m, err := p.buildClaimedModel(files)
	if err != nil { // coverage-ignore: see buildClaimedModel's sidecar coverage-ignores
		return nil, err
	}
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
			if rel == config.DirName+"/memory" {
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
