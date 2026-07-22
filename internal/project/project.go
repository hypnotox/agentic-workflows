// Package project ties config, catalog, render, and manifest together to sync rendered files into a project and check them for drift.
package project

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/plan"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// Version is the awf release version - the single version authority
// (ADR-0049): gate comparisons, the lock stamp, the bootstrap pin, and the
// CLI output all read this const.
const Version = "0.22.0"

// BridgeTrancheComplete blocks publication while the two-plan current-state
// bridge tranche is only partially implemented. Plans 1 and 2 have both landed
// (migration readiness, attestation, and ordinary-command refusal are all
// present), so the tranche is complete and publication is unblocked.
const BridgeTrancheComplete = true

// minVersionBySchema maps each config-schema generation to the minimum
// project.Version allowed to render it; adding a migration without an entry
// here (and a matching const bump) fails the gate (ADR-0049 Decision 4).
var minVersionBySchema = map[int]string{
	6:  "0.6.0",
	7:  "0.11.0",
	8:  "0.12.0",
	9:  "0.17.0",
	10: "0.17.0",
	11: "0.17.0",
	12: "0.17.0",
	13: "0.17.0",
	14: "0.18.0",
	15: "0.20.0",
	16: "0.21.0",
	17: "0.22.0",
}

type Project struct {
	Root    string
	Cfg     *config.Config
	Cat     *catalog.Catalog
	Targets []Target
	// effSkills is the effective rendered skill set (enabled minus doc-gate-
	// suppressed, local kept), populated by RenderAll; templates read it as
	// .skills and artifactConfigHash folds it in for .skills-referencing
	// templates (ADR-0046).
	effSkills map[string]bool
	// corpus is the lazily-loaded parsed ADR view (ADR-0130 item 1), threaded
	// to every consumer that needs an ADR fact instead of each one loading.
	corpus *adr.Corpus
	// topics is the lazily loaded current-state producer corpus for one invocation.
	topics *topic.Corpus
}

func Open(root string) (*Project, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	targets, err := resolveTargets(cfg.Targets)
	if err != nil {
		return nil, err
	}
	p := &Project{Root: root, Cfg: cfg, Targets: targets}
	cat, err := p.effectiveCatalog()
	if err != nil {
		return nil, err
	}
	p.Cat = cat
	if err := p.validateAgainstCatalog(); err != nil {
		return nil, err
	}
	return p, nil
}

// Backup records a foreign file preserved before sync overwrote its path.
type Backup struct {
	Path  string // project-relative file that was overwritten
	Bak   string // project-relative backup copy (.awf-bak[.N])
	Index bool   // the file is the generated ADR/domain index (ownership-takeover note)
}

// Change records a sync-written file whose rendered output differs from the
// prior lock's, with the cause the lock's hashes can attribute: "template"
// (the upstream template source moved), "config" (the project's effective
// inputs - vars, sidecar, parts - moved), "template+config" (both),
// "internal" (hashes unmoved: a non-hashed input such as the binary's version
// stamp), "regenerated" (a generated index, which carries no hashes to
// attribute), or "added" (no prior entry). The provenance triage signal for
// reviewing a large sync diff - upstream churn vs the project's own inputs.
type Change struct {
	Path  string
	Cause string
}

// SyncReport renders and writes the project, additionally backing up any
// foreign file (on disk but absent from the start-of-sync lock) before overwriting
// it, and returning those backups (ADR-0035) plus the per-file provenance of
// output that changed against the prior lock and the lock-relative paths of the
// files its prune actually removed (both path-sorted; a file whose output is
// byte-identical, and first-adoption initialization with no prior lock reports
// no change - a routine re-sync stays silent).
func (p *Project) SyncReport() ([]Backup, []Change, []string, error) {
	return p.syncReport(nil)
}

// InitAuthority is the explicit provenance supplied only by first adoption.
type InitAuthority struct {
	InitializedWithVersion string
}

// InitializeReport renders a first adoption while sealing its existing ADR
// identities. It has the same reporting contract as SyncReport.
func (p *Project) InitializeReport(seed InitAuthority) ([]Backup, []Change, []string, error) {
	return p.syncReport(&seed)
}

func (p *Project) syncReport(seed *InitAuthority) ([]Backup, []Change, []string, error) {
	p.beginInvocation()
	// Refuse before rendering or writing anything: a corrupt lock must never
	// produce a backup, skip a prune, or be overwritten (ADR-0076 Decision 2).
	old, found, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, nil, nil, err
	}
	var initCutoff int
	var initGaps []int
	if seed != nil {
		if found {
			return nil, nil, nil, errors.New("first-adoption initialization requires an absent lock")
		}
		initCutoff, initGaps, err = adr.AdoptionBoundary(p.decisionsDir())
		if err != nil {
			return nil, nil, nil, fmt.Errorf("seal first-adoption ADR authority: %w", err)
		}
	} else {
		if !found {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync cannot create lock authority; use the bridge release to attest")
		}
		state, stateErr := old.AuthorityState()
		if stateErr != nil || state != manifest.AuthorityPermanent {
			return nil, nil, nil, errors.New("pre-tracking authority: ordinary sync requires a permanent lock; use the bridge release to attest")
		}
	}
	metricsResident, err := inspectResidentMetrics(p.Root)
	if err != nil {
		return nil, nil, nil, err
	}
	op, err := p.OutputPlan()
	if err != nil {
		return nil, nil, nil, err
	}
	files := op.writeFiles()
	for _, f := range files {
		if f.Policy.ValidateFrontmatter {
			if err := validateArtifact([]byte(f.Content), f.Encoder); err != nil { // coverage-ignore: rendered catalog skill/agent syntax is template-fixed and cannot be invalid at sync time
				return nil, nil, nil, fmt.Errorf("invalid agent artifact in %s: %w", f.Path, err)
			}
		}
	}
	var localErr error
	p.localReservations(op, func(path string, e error) {
		if localErr == nil {
			localErr = fmt.Errorf("local target %s: %w", path, e)
		}
	})
	if localErr != nil {
		return nil, nil, nil, localErr
	}

	// Prior lock, read before any write (top of this func): membership decides
	// foreign (back up) vs awf-managed (overwrite silently), and drives pruning.
	prior := map[string]bool{}
	if old != nil {
		for path := range old.Files {
			prior[path] = true
		}
	}

	var backups []Backup
	lock := &manifest.Lock{AWFVersion: Version, SchemaVersion: migrate.Current(), Files: map[string]manifest.Entry{}}
	if old != nil {
		lock.InitializedWithVersion = old.InitializedWithVersion
		lock.ADRFormatV1From = old.ADRFormatV1From
		lock.ADRFormatV2From = old.ADRFormatV2From
		lock.LegacyADRGaps = slices.Clone(old.LegacyADRGaps)
	} else {
		lock.InitializedWithVersion = seed.InitializedWithVersion
		lock.ADRFormatV1From = initCutoff
		lock.ADRFormatV2From = initCutoff
		lock.LegacyADRGaps = slices.Clone(initGaps)
	}
	want := map[string]bool{}
	for _, f := range files {
		abs := filepath.Join(p.Root, f.Path)
		dir := filepath.Dir(abs)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, nil, err
		}
		if f.Path == config.DirName+"/metrics/.gitignore" {
			if err := os.Chmod(dir, 0o700); err != nil { // coverage-ignore: MkdirAll just established the confined directory; a permission race is not deterministic under the root gate
				return nil, nil, nil, err
			}
		}
		if !prior[f.Path] {
			if _, statErr := os.Stat(abs); statErr == nil {
				// touches-state: rendering/sync-and-drift:sync-backs-up-foreign - foreign-file backup on sync; proof in project_test.go
				bak, err := p.BackupFile(f.Path)
				if err != nil { // coverage-ignore: BackupFile only fails on a copyFile permission fault that root bypasses
					return nil, nil, nil, fmt.Errorf("back up %s: %w", f.Path, err)
				}
				backups = append(backups, Backup{Path: f.Path, Bak: bak, Index: f.RegenChecked})
			} else if !errors.Is(statErr, os.ErrNotExist) { // coverage-ignore: os.Stat returns a non-NotExist error only on a permission/IO fault that root bypasses
				return nil, nil, nil, statErr
			}
		}
		// A rendered #!-shebang script is written executable (ADR-0100 Decision 8),
		// so the runner is runnable as ./x. The mode is enforced on every sync - a
		// pre-existing file's mode is corrected too, since os.WriteFile applies perm
		// only at creation - hence the explicit Chmod.
		perm := os.FileMode(0o644)
		if strings.HasPrefix(f.Content, "#!") {
			perm = 0o755
		}
		if err := os.WriteFile(abs, []byte(f.Content), perm); err != nil {
			return nil, nil, nil, err
		}
		if err := os.Chmod(abs, perm); err != nil { // coverage-ignore: os.Chmod fails only on a permission/ownership fault that root bypasses, right after a successful WriteFile to the same path
			return nil, nil, nil, err
		}
		lock.Files[f.Path] = manifest.Entry{
			TemplateID: f.TemplateID, TemplateHash: f.TemplateHash,
			ConfigHash: f.ConfigHash, OutputHash: manifest.Hash([]byte(f.Content)),
			RegenChecked: f.RegenChecked,
		}
		want[f.Path] = true
	}
	// Plan reservations are non-writing local artifacts. They remain wanted so a
	// managed-to-local transition cannot be pruned.
	for _, node := range op.Nodes {
		if node.Reservation {
			want[node.Path] = true
		}
	}
	// Prune files from the previous lock that are no longer produced, then remove
	// every directory left empty - walking all ancestors deepest-first, not just the
	// immediate parent, so dropping a target clears its whole tree (inv:
	// target-prune-ancestors; reuses Uninstall's idiom).
	var pruned []string
	if old != nil {
		dirs := map[string]bool{}
		for path := range old.Files {
			if want[path] || preserveMetricsRemoval(path, metricsResident) {
				continue
			}
			// A non-local entry (corrupted or malicious lock) would delete outside
			// the root and send the ancestor walk below it, never reaching p.Root.
			if !filepath.IsLocal(filepath.FromSlash(path)) {
				continue
			}
			file := filepath.Join(p.Root, path)
			// Report only an actual removal - a path whose file is already gone
			// must not be claimed pruned.
			if os.Remove(file) == nil {
				pruned = append(pruned, path)
			}
			for d := filepath.Dir(file); d != p.Root; d = filepath.Dir(d) {
				dirs[d] = true
			}
		}
		dirList := slices.Collect(maps.Keys(dirs))
		slices.SortFunc(dirList, func(a, b string) int { return len(b) - len(a) })
		for _, d := range dirList {
			_ = os.Remove(d) // removes only if now empty
		}
	}
	slices.Sort(pruned) // lock-map iteration order is random; output must not be
	// Provenance: classify each written file whose output moved against the
	// prior lock, from the final lock state (one entry per path by
	// construction). A first sync has no baseline - report nothing rather
	// than flood a fresh adoption with "added" lines.
	var changes []Change
	if old != nil {
		for path, e := range lock.Files {
			oldE, ok := old.Files[path]
			if !ok {
				changes = append(changes, Change{Path: path, Cause: "added"})
				continue
			}
			if e.OutputHash == oldE.OutputHash {
				continue
			}
			tMoved, cMoved := e.TemplateHash != oldE.TemplateHash, e.ConfigHash != oldE.ConfigHash
			var cause string
			switch {
			case tMoved && cMoved:
				cause = "template+config"
			case tMoved:
				cause = "template"
			case cMoved:
				cause = "config"
			case e.RegenChecked:
				// Regeneration-checked entries carry no frozen-hash attribution:
				// generated indexes (whose inputs are the scanned decision
				// records) and in-place files (whose read-back body moved).
				cause = "regenerated"
			default:
				// Real hashes, neither moved: a non-hashed input such as
				// the binary's version stamp.
				cause = "internal"
			}
			changes = append(changes, Change{Path: path, Cause: cause})
		}
		slices.SortFunc(changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	}
	return backups, changes, pruned, lock.Save(p.lockPath())
}

func (p *Project) lockPath() string {
	return config.LockPath(p.Root)
}

// beginInvocation drops any per-invocation cached state, so the operation that
// follows observes the decisions directory as it is on disk now. Every public
// operation that reads ADRs calls it before its first Corpus use.
func (p *Project) beginInvocation() { p.corpus, p.topics = nil, nil }

// Corpus returns the project's parsed ADR corpus, loading it on first use
// within the current invocation and reusing it for the rest of that invocation
// (ADR-0130 item 1). Threading one view is what collapses the eight-or-so
// per-check parses a single awf check used to perform.
//
// The cache is per-INVOCATION, not per-Project: every public operation that
// reads ADRs calls beginInvocation first. A Project outlives a single call, and
// Check's whole contract is to compare rendered output against the decisions
// directory as it is on disk right now - so a corpus held across calls would
// make a Check following a Sync miss an ADR written in between, silently
// blinding the drift oracle rather than merely serving a stale read.
func (p *Project) Corpus() (adr.Corpus, error) {
	if p.corpus != nil {
		return *p.corpus, nil
	}
	c, err := adr.LoadCorpus(p.decisionsDir())
	if err != nil {
		return adr.Corpus{}, err
	}
	p.corpus = &c
	return c, nil
}

// Audit runs the process-conformance audit (ADR-0017) over the caller-supplied
// commit range. No config key supplies a base: the range is always explicit
// (ADR-0127 Decision 3).
func (p *Project) Audit(base, head string) ([]audit.Finding, int, error) {
	s := audit.Resolve(p.Cfg.Audit)
	lay := p.layout()
	generated := map[string]bool{}
	lock, _, err := manifest.LoadOptional(p.lockPath())
	if err != nil {
		return nil, 0, err
	}
	if lock != nil {
		for path := range lock.Files {
			generated[path] = true
		}
	}
	domainPaths := map[string][]string{}
	for _, d := range p.Cfg.Domains {
		sc, err := p.Cfg.Sidecar("domains", d)
		if err != nil {
			return nil, 0, err
		}
		for _, g := range sc.Paths {
			if err := pathglob.Validate(g); err != nil {
				return nil, 0, fmt.Errorf("domain %q paths: %w", d, err)
			}
		}
		if len(sc.Paths) > 0 {
			domainPaths[d] = sc.Paths
		}
	}
	findings, commits, err := audit.Run(p.Root, base, head, audit.Inputs{
		Settings:          s,
		GeneratedPaths:    generated,
		ADRDir:            lay.ADRDir,
		DocsDir:           lay.DocsDir,
		IndexMd:           lay.IndexMd,
		PlansDir:          lay.PlansDir,
		ConfiguredDomains: p.Cfg.Domains,
		DomainsPartsDir:   config.DirName + "/domains/parts",
		DomainPaths:       domainPaths,
	})
	if err != nil {
		return nil, 0, err
	}
	// The snapshot-diff transition check rides the same range (ADR-0135): each
	// commit's ADR/claim mutations must match its ADR operations. It is advisory
	// like the rest of the audit and derives boundaries from each commit snapshot.
	trans, err := p.auditTransitions(base, head)
	if err != nil { // coverage-ignore: audit.Run above validated this exact range through its own Collect, so auditTransitions' only error source (a re-Collect of base..head) cannot newly fail here
		return nil, 0, err
	}
	return append(findings, trans...), commits, nil
}

// NewADR scaffolds a new ADR file under the project's decisions dir: the next
// sequential number, the rendered template with its title/date filled in and
// marker comments stripped, refusing to overwrite an existing file. Mirrors
// the CheckInvariants/Audit pattern - cmd/awf reaches this only through this
// exported method, never internal/project.Layout directly.
func (p *Project) NewADR(title string) (string, error) {
	lock, err := manifest.Load(p.lockPath())
	if err != nil {
		return "", err
	}
	number, err := adr.NextNumber(p.decisionsDir())
	if err != nil {
		return "", err
	}
	n, _ := strconv.Atoi(number)
	format := adr.CurrentStateV1
	if lock.ADRFormatV2From > 0 && n >= lock.ADRFormatV2From {
		format = adr.CurrentStateV2
	}
	return adr.NewFile(p.decisionsDir(), title, format)
}

// NewPlan scaffolds a new plan under docsDir/plans from the rendered plans
// template. Mirrors NewADR minus sequential numbering (ADR-0098).
func (p *Project) NewPlan(title string) (string, error) {
	return plan.NewFile(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"), title)
}
