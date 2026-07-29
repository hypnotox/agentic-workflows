package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

// Legacy schema-1 resident layout. This package is the only current-state
// reader of it: protocol 2 keeps each effort in a `.awf/efforts/<slug>/`
// directory and owns no `.awf/memory/` root at all, so every shape named here
// is historical. The upgrade discards these residents rather than migrating
// them (ADR-0175 Decision 13), which is exactly why nothing is discarded until
// this preflight has proven, read-only, that it is obsolete.
const (
	legacyEffortsRel   = ".awf/efforts"
	legacyMemoryRel    = ".awf/memory"
	legacyWorktreesRel = ".awf/worktrees"
	legacyLockName     = ".lock"
	legacyBranchPrefix = "awf/"
)

// legacyUUIDPattern matches the UUIDv4 that schema-1 used as an effort's public
// identity. Protocol 2 retains a UUID internally but never names a path with it.
var legacyUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// legacyPartialActions is the closed set of schema-1 partial-mutation evidence
// kinds. Each names a Git mutation that had begun when the writer died, so each
// must be checked against live Git facts before it can be called obsolete.
var legacyPartialActions = map[string]bool{"worktree": true, "integration": true, "removal": true}

// LegacyResidents is the complete read-only classification of the schema-1
// residents a project still carries.
type LegacyResidents struct {
	// PrimaryRoot is the checkout that owns the resident roots. Residents are
	// repository-wide, so they live here even when another linked checkout
	// invoked the upgrade.
	PrimaryRoot string
	// Quarantine lists the PrimaryRoot-relative forward-slash paths proven
	// obsolete, sorted bytewise. Each is quarantined whole: a single legacy
	// leaf under the efforts root, or the entire standalone memory root.
	Quarantine []string
}

// legacyRecord is the subset of a schema-1 effort record this preflight reads.
// It deliberately does not model the retired lifecycle fields: proving the
// record is schema-1 legacy is the only question being asked of it.
type legacyRecord struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
}

// legacyPartial is the subset of schema-1 partial-mutation evidence that names
// Git facts. Path is the managed checkout a worktree mutation was creating.
type legacyPartial struct {
	SchemaVersion int    `json:"schemaVersion"`
	EffortID      string `json:"effortId"`
	Action        string `json:"action"`
	Branch        string `json:"branch"`
	Path          string `json:"path"`
}

// residentRefusal reports a resident the upgrade must not touch. Every refusal
// happens before the journal exists, so it can always state that no byte moved.
func residentRefusal(condition, nextAction string) error {
	return fmt.Errorf("unified-effort-resident-migration refused: %s; changed bytes: no; next action: %s", condition, nextAction)
}

// preserveManually is the next action for a resident whose bytes awf cannot
// prove are its own to discard.
const preserveManually = "preserve the resident, inspect it by hand, and rerun `awf upgrade` once it is resolved or removed"

// legacyWorktreeNextAction names the pre-upgrade commands that clear a managed
// worktree. The current binary cannot do it: protocol 2 derives worktrees from
// Git topology and knows nothing about UUID-named managed resources.
func legacyWorktreeNextAction(id string) string {
	return fmt.Sprintf("use the pre-upgrade awf release to run `awf effort integrated %s --commit HEAD` and `awf effort worktree remove %s`, then rerun `awf upgrade`", id, id)
}

// applyUnifiedEffortResidents performs the schema-22 reset as one journaled
// transaction: a complete read-only preflight first, then quarantine of every
// proven legacy resident, then the lock replacement that makes the new
// generation authoritative. The lock is the last operation, so the discarded
// residents and the new generation become true together or not at all. It
// stamps that lock itself, which is what OwnsSchemaStamp declares.
//
// The reset is announced rather than performed silently because it is a
// breaking change: protocol-1 records and standalone memory are discarded, not
// ported. Nothing here invents a slug for the efforts that are lost; protocol 2
// derives a slug from an outcome title a person supplies.
func applyUnifiedEffortResidents(root string, out io.Writer) error {
	classified, err := ClassifyLegacyResidents(root)
	if err != nil {
		return err
	}
	// Residents are repository-wide and live in the primary checkout, while the
	// lock this transaction commits belongs to the invoking one. One journal
	// spans one root, so a split like that is refused rather than half-applied.
	if len(classified.Quarantine) > 0 && classified.PrimaryRoot != filepath.Clean(root) {
		return residentRefusal(
			fmt.Sprintf("the legacy residents belong to the primary checkout %s, not the invoking checkout %s", classified.PrimaryRoot, root),
			"run `awf upgrade` from "+classified.PrimaryRoot,
		)
	}
	fmt.Fprintln(out, "unified-effort-residents: breaking change: protocol-1 effort records and standalone .awf/memory/ content are reset, not migrated")
	fmt.Fprintln(out, "unified-effort-residents: protocol 2 keeps each effort at .awf/efforts/<slug>/ with its own memory.md; recreate the ones you still need with `awf effort new \"<outcome>\"`")
	return upgrade.ResetLegacyResidents(root, classified.Quarantine, 22, out)
}

// ClassifyLegacyResidents inspects every schema-1 binary-owned leaf and every
// Git fact those leaves refer to, without mutating a byte. It returns the
// resident paths the journal may quarantine, or refuses. A refusal always
// leaves the tree exactly as it found it, so the caller may report it before a
// journal exists.
func ClassifyLegacyResidents(root string) (LegacyResidents, error) {
	primary, facts, err := legacyTopology(root)
	if err != nil {
		return LegacyResidents{}, err
	}
	quarantine, ids, err := classifyEffortsRoot(primary)
	if err != nil {
		return LegacyResidents{}, err
	}
	memory, err := classifyMemoryRoot(primary)
	if err != nil {
		return LegacyResidents{}, err
	}
	quarantine = append(quarantine, memory...)
	if err := facts.refuseLegacyWorktrees(primary, ids); err != nil {
		return LegacyResidents{}, err
	}
	sort.Strings(quarantine)
	return LegacyResidents{PrimaryRoot: primary, Quarantine: quarantine}, nil
}

// legacyFacts is the live Git topology the preflight consults. A tree with no
// repository at all carries no worktree or branch facts, so the zero value is
// the correct answer there rather than a reason to refuse.
type legacyFacts struct {
	registrations []awfgit.WorktreeRegistration
	branches      map[string]bool
}

// legacyTopology resolves the resident-owning checkout and reads the Git facts
// the preflight needs. Only go-git's canonical not-a-repository error permits
// the plain-directory fallback: a malformed .git or unsafe topology is a
// present checkout whose facts cannot be read, and guessing there would let a
// live managed worktree slip past the refusal below.
func legacyTopology(root string) (string, legacyFacts, error) {
	repo, err := awfgit.OpenRepo(root)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return filepath.Clean(root), legacyFacts{}, nil
		}
		return "", legacyFacts{}, fmt.Errorf("inspect Git checkout at %s: %w", root, err)
	}
	roots, err := awfgit.ResolveControlRoots(context.Background(), root)
	if err != nil {
		return "", legacyFacts{}, fmt.Errorf("resolve Git control roots at %s: %w", root, err)
	}
	registrations, err := awfgit.ListWorktreeRegistrations(context.Background(), roots.InvokingRoot)
	if err != nil { // coverage-ignore: ResolveControlRoots parsed the same porcelain from the same checkout moments earlier
		return "", legacyFacts{}, err
	}
	branches, err := legacyBranches(repo)
	if err != nil { // coverage-ignore: OpenRepo validated this repository; enumerating its refs fails only on a concurrent storage fault
		return "", legacyFacts{}, err
	}
	return roots.PrimaryRoot, legacyFacts{registrations: registrations, branches: branches}, nil
}

// legacyBranches collects the repository's local branch short names.
func legacyBranches(repo *gogit.Repository) (map[string]bool, error) {
	iter, err := repo.Branches()
	if err != nil { // coverage-ignore: go-git returns an iterator over the validated reference storer without a reachable failure
		return nil, err
	}
	defer iter.Close()
	branches := map[string]bool{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		branches[ref.Name().Short()] = true
		return nil
	})
	return branches, err
}

// refuseLegacyWorktrees refuses while any legacy UUID worktree fact remains:
// a registered managed path, a managed branch, or the managed directory itself.
// The set of identifiers is the union of the ones the residents named and the
// ones Git still carries, so a worktree whose record was already deleted is
// still caught.
func (f legacyFacts) refuseLegacyWorktrees(primary string, ids map[string]bool) error {
	known := map[string]bool{}
	for id := range ids {
		known[id] = true
	}
	for branch := range f.branches {
		if id, ok := strings.CutPrefix(branch, legacyBranchPrefix); ok && legacyUUIDPattern.MatchString(id) {
			known[id] = true
		}
	}
	worktrees := filepath.Join(primary, filepath.FromSlash(legacyWorktreesRel))
	for _, registration := range f.registrations {
		if id, ok := legacyManagedID(worktrees, registration.Path); ok {
			known[id] = true
		}
	}
	entries, err := os.ReadDir(worktrees)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect %s: %w", worktrees, err)
	}
	for _, entry := range entries {
		if legacyUUIDPattern.MatchString(entry.Name()) {
			known[entry.Name()] = true
		}
	}
	for _, id := range sortedKeys(known) {
		condition, found := f.legacyWorktreeFact(worktrees, id)
		if !found {
			continue
		}
		return residentRefusal(condition, legacyWorktreeNextAction(id))
	}
	return nil
}

// legacyWorktreeFact reports the first live fact proving a legacy managed
// worktree for id still exists.
func (f legacyFacts) legacyWorktreeFact(worktrees, id string) (string, bool) {
	managed := filepath.Join(worktrees, id)
	if _, err := os.Lstat(managed); err == nil {
		return fmt.Sprintf("legacy managed worktree path %s remains", managed), true
	}
	for _, registration := range f.registrations {
		if registration.Path == managed {
			return fmt.Sprintf("legacy managed worktree %s is still registered with Git", managed), true
		}
		if registration.Branch == plumbing.NewBranchReferenceName(legacyBranchPrefix+id).String() {
			return fmt.Sprintf("legacy managed branch %s%s is checked out at %s", legacyBranchPrefix, id, registration.Path), true
		}
	}
	if f.branches[legacyBranchPrefix+id] {
		return fmt.Sprintf("legacy managed branch %s%s remains", legacyBranchPrefix, id), true
	}
	return "", false
}

// legacyManagedID reports the identifier of a registration that sits directly
// under the legacy managed worktrees root.
func legacyManagedID(worktrees, path string) (string, bool) {
	if filepath.Dir(path) != worktrees {
		return "", false
	}
	name := filepath.Base(path)
	return name, legacyUUIDPattern.MatchString(name)
}

// classifyEffortsRoot classifies every entry directly under the efforts
// resident root. Schema 1 wrote only files there, so a directory is a live
// protocol-2 effort or finishing reservation and is left untouched; a file is
// either a known legacy shape or an unknown leaf that refuses.
func classifyEffortsRoot(primary string) ([]string, map[string]bool, error) {
	dir := filepath.Join(primary, filepath.FromSlash(legacyEffortsRel))
	entries, _, err := readResidentRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	var quarantine []string
	ids := map[string]bool{}
	var partials []legacyPartial
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil { // coverage-ignore: ReadDir returned this entry from the same directory read moments earlier
			return nil, nil, fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			continue // a protocol-2 effort resident or finishing reservation
		}
		if err := effort.ValidateResidentLeaf(path, info); err != nil {
			return nil, nil, residentRefusal(fmt.Sprintf("unsafe resident %s: %v", path, err), preserveManually)
		}
		switch {
		case name == ".gitignore":
			continue // the governed tracked output for this root
		case name == legacyLockName:
			quarantine = append(quarantine, legacyEffortsRel+"/"+name)
		case strings.HasSuffix(name, ".json"):
			id, err := classifyLegacyRecord(path, name)
			if err != nil {
				return nil, nil, err
			}
			ids[id] = true
			quarantine = append(quarantine, legacyEffortsRel+"/"+name)
		case strings.HasSuffix(name, ".partial"):
			partial, err := classifyLegacyPartial(path, name)
			if err != nil {
				return nil, nil, err
			}
			ids[partial.EffortID] = true
			partials = append(partials, partial)
			quarantine = append(quarantine, legacyEffortsRel+"/"+name)
		default:
			return nil, nil, residentRefusal("unknown resident leaf "+path, preserveManually)
		}
	}
	if err := refusePartialGitFacts(partials); err != nil {
		return nil, nil, err
	}
	return quarantine, ids, nil
}

// classifyLegacyRecord proves a `<uuid>.json` leaf is a schema-1 effort record
// and returns its identifier. A file that only looks like one is malformed
// rather than obsolete, and malformed residents are preserved for a human.
func classifyLegacyRecord(path, name string) (string, error) {
	id := strings.TrimSuffix(name, ".json")
	if !legacyUUIDPattern.MatchString(id) {
		return "", residentRefusal("unknown resident leaf "+path, preserveManually)
	}
	var record legacyRecord
	if err := readLegacyJSON(path, &record); err != nil {
		return "", err
	}
	if record.SchemaVersion != 1 || record.ID != id {
		return "", residentRefusal(fmt.Sprintf("malformed legacy effort record %s: schemaVersion %d and id %q do not describe this leaf", path, record.SchemaVersion, record.ID), preserveManually)
	}
	return id, nil
}

// classifyLegacyPartial proves a `.<uuid>.<action>.partial` leaf is schema-1
// partial-mutation evidence and returns the Git facts it names.
func classifyLegacyPartial(path, name string) (legacyPartial, error) {
	unknown := residentRefusal("unknown resident leaf "+path, preserveManually)
	rest, ok := strings.CutPrefix(strings.TrimSuffix(name, ".partial"), ".")
	if !ok {
		return legacyPartial{}, unknown
	}
	id, action, ok := strings.Cut(rest, ".")
	if !ok || !legacyUUIDPattern.MatchString(id) || !legacyPartialActions[action] {
		return legacyPartial{}, unknown
	}
	var partial legacyPartial
	if err := readLegacyJSON(path, &partial); err != nil {
		return legacyPartial{}, err
	}
	if partial.SchemaVersion != 1 || partial.EffortID != id || partial.Action != action || partial.Branch != legacyBranchPrefix+id {
		return legacyPartial{}, residentRefusal(fmt.Sprintf("malformed legacy partial-mutation evidence %s: it does not describe effort %s action %q", path, id, action), preserveManually)
	}
	return partial, nil
}

// refusePartialGitFacts inspects the Git facts each partial-mutation evidence
// file names before that evidence may be called obsolete. The managed branch
// and path are already covered by the repository-wide worktree refusal; what
// remains is the arbitrary absolute checkout path a worktree mutation had
// begun creating, which need not sit under the managed root at all.
func refusePartialGitFacts(partials []legacyPartial) error {
	for _, partial := range partials {
		if partial.Path == "" {
			continue
		}
		if _, err := os.Lstat(partial.Path); err == nil {
			return residentRefusal(
				fmt.Sprintf("legacy partial-mutation evidence for effort %s names a checkout at %s that still exists", partial.EffortID, partial.Path),
				legacyWorktreeNextAction(partial.EffortID),
			)
		}
	}
	return nil
}

// classifyMemoryRoot proves the standalone memory root holds nothing awf cannot
// discard, then classifies the whole root for quarantine. Its contents are
// unbounded authored prose rather than a known leaf set, so every descendant is
// checked for safety and none is checked for shape.
func classifyMemoryRoot(primary string) ([]string, error) {
	dir := filepath.Join(primary, filepath.FromSlash(legacyMemoryRel))
	_, present, err := readResidentRoot(dir)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil { // coverage-ignore: the walk root was just validated and every refusal below stops the walk first
			return residentRefusal(fmt.Sprintf("unreadable standalone memory resident %s: %v", path, err), preserveManually)
		}
		if path == dir {
			return nil
		}
		info, err := entry.Info()
		if err != nil { // coverage-ignore: WalkDir returned this entry from a directory read moments earlier
			return residentRefusal(fmt.Sprintf("unreadable standalone memory resident %s: %v", path, err), preserveManually)
		}
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return validateResidentDir(path, info)
		}
		if err := effort.ValidateResidentLeaf(path, info); err != nil {
			return residentRefusal(fmt.Sprintf("unsafe standalone memory resident %s: %v", path, err), preserveManually)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return []string{legacyMemoryRel}, nil
}

// readResidentRoot lists a resident root after proving the root itself is a
// current-owned real directory, and reports whether it exists at all. An absent
// root is empty, not a refusal; an existing but empty root is still present, so
// a root awf no longer owns is quarantined even when nothing is left inside it.
func readResidentRoot(dir string) ([]os.DirEntry, bool, error) {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil { // coverage-ignore: an existing path under the validated primary checkout lstats cleanly absent a concurrent race
		return nil, false, fmt.Errorf("inspect %s: %w", dir, err)
	}
	if err := validateResidentDir(dir, info); err != nil {
		return nil, false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil { // coverage-ignore: the directory was just proven a readable current-owned directory
		return nil, false, fmt.Errorf("read %s: %w", dir, err)
	}
	return entries, true, nil
}

// validateResidentDir refuses a resident directory awf cannot prove is its own
// to move: a symlink, a non-directory, or another user's bytes.
func validateResidentDir(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return residentRefusal("symlinked resident "+path, preserveManually)
	}
	if !info.IsDir() {
		return residentRefusal(fmt.Sprintf("resident %s is not a directory (mode %s)", path, info.Mode()), preserveManually)
	}
	if err := effort.ValidateCurrentOwner(path, info); err != nil { // coverage-ignore: requires a foreign-owned fixture created by a privileged test process
		return residentRefusal(fmt.Sprintf("foreign-owned resident %s: %v", path, err), preserveManually)
	}
	return nil
}

// readLegacyJSON reads a legacy resident leaf without following links and
// decodes it strictly. Unparseable bytes are malformed, never obsolete.
func readLegacyJSON(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil { // coverage-ignore: the leaf was proven a readable current-owned regular file moments earlier
		return residentRefusal(fmt.Sprintf("unreadable resident %s: %v", path, err), preserveManually)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return residentRefusal(fmt.Sprintf("malformed resident %s: %v", path, err), preserveManually)
	}
	return nil
}

// sortedKeys returns a set's members in deterministic order so a tree carrying
// several legacy facts always refuses on the same one.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
