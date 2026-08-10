package effort

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// Dependencies is everything the service reaches the outside world through:
// the clock, the identity allocator, the three Git questions it asks, and tree
// removal. Every one is supplied by the composition root, because a service
// that quietly substituted its own would make what a caller composed
// unverifiable from the call site.
//
// The Git dependencies are stated as this package's own questions rather than
// as a repository object, so the service depends on what it asks and not on who
// answers. Each is bound to the checkout the service was opened against, so
// none of them names a root.
type Dependencies struct {
	Clock                 func() time.Time
	UUID                  func() (string, error)
	Worktrees             func(context.Context) ([]awfgit.WorktreeRegistration, error)
	BranchExists          func(context.Context, string) (bool, error)
	ValidateRef           func(context.Context, string) (bool, error)
	RemoveTree            func(string) error
	ExpectedArchiveMarker func() ([]byte, error)
	// Fault is the durability-boundary hook the restartable-finish tests
	// interrupt the service at. It is the one optional member: a nil Fault
	// injects nothing, which is what production wants and what "no fault
	// injection configured" means.
	Fault func(stage string) error
}

// Service owns immutable effort residents and restartable finish.
type Service struct {
	paths                 paths
	store                 store
	clock                 func() time.Time
	uuid                  func() (string, error)
	worktrees             func(context.Context) ([]awfgit.WorktreeRegistration, error)
	branchExists          func(context.Context, string) (bool, error)
	validateRef           func(context.Context, string) (bool, error)
	removeTree            func(string) error
	expectedArchiveMarker func() ([]byte, error)
}

// Open resolves the resident paths owned by roots and composes the service over
// the dependencies it is given. The control roots arrive already resolved so
// one command resolves them once, and so the service and the worktree manager
// provably reason about the same repository identity.
func Open(roots awfgit.ControlRoots, deps Dependencies) (*Service, error) {
	switch {
	case deps.Clock == nil:
		panic("effort Service: missing clock dependency")
	case deps.UUID == nil:
		panic("effort Service: missing UUID allocator dependency")
	case deps.Worktrees == nil:
		panic("effort Service: missing worktree registration dependency")
	case deps.BranchExists == nil:
		panic("effort Service: missing branch probe dependency")
	case deps.ValidateRef == nil:
		panic("effort Service: missing reference validation dependency")
	case deps.RemoveTree == nil:
		panic("effort Service: missing tree removal dependency")
	case deps.ExpectedArchiveMarker == nil:
		panic("effort Service: missing archive marker dependency")
	}
	resolved, err := resolvePaths(roots)
	if err != nil {
		return nil, fmt.Errorf("resolve effort resident paths from %s: %w", roots.PrimaryRoot, err)
	}
	return &Service{
		paths: resolved, store: store{paths: resolved, fault: deps.Fault},
		clock: deps.Clock, uuid: deps.UUID, worktrees: deps.Worktrees,
		branchExists: deps.BranchExists, validateRef: deps.ValidateRef, removeTree: deps.RemoveTree,
		expectedArchiveMarker: deps.ExpectedArchiveMarker,
	}, nil
}

// RandomUUIDv4 is the production identity allocator. It is exported because the
// composition root, not this package, decides what allocates an effort's
// internal identity.
func RandomUUIDv4() (string, error) {
	var raw [16]byte
	_, _ = rand.Read(raw[:]) // crypto/rand.Read fills the slice or terminates the process.
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func (s *Service) now() time.Time { return s.clock().UTC() }

func (s *Service) New(ctx context.Context, input NewInput) (Record, error) {
	normalized, err := normalizeTitle(input.Title)
	if err != nil {
		return Record{}, refusal(fmt.Sprintf("invalid outcome title: %v; changed bytes: no; next action: provide a nonblank valid UTF-8 outcome title", err), "outcome title is invalid", "input", err.Error(), []RecoveryAction{{Text: "provide a nonblank valid UTF-8 outcome title"}}, err)
	}
	if err := validateNewSlug(ctx, s.validateRef, input.Slug); err != nil {
		return Record{}, err
	}
	id, err := s.uuid()
	if err != nil {
		return Record{}, refusal(fmt.Sprintf("allocate internal effort UUID: %v; changed bytes: no; next action: retry `awf effort new --slug %q %q`", err, input.Slug, normalized), fmt.Sprintf("internal effort UUID allocation failed for %q", input.Slug), "operation", err.Error(), []RecoveryAction{{Text: fmt.Sprintf("retry `awf effort new --slug %q %q`", input.Slug, normalized)}}, err)
	}
	if !uuidV4Pattern.MatchString(id) {
		return Record{}, refusal(fmt.Sprintf("allocator returned invalid UUIDv4 %q; changed bytes: no; next action: repair the awf installation and retry `awf effort new --slug %q %q`", id, input.Slug, normalized), fmt.Sprintf("internal effort UUID for %q is invalid", input.Slug), "installation", "allocator returned an invalid UUIDv4", []RecoveryAction{{Text: fmt.Sprintf("repair the awf installation and retry `awf effort new --slug %q %q`", input.Slug, normalized)}}, nil)
	}
	record := Record{SchemaVersion: SchemaVersion, ID: id, Slug: input.Slug, Title: normalized, CreatedAt: s.now(), MemoryPath: s.paths.publicMemoryPath(input.Slug)}
	if err := s.store.create(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// InvokingRoot reports the checkout the service was opened from, so callers
// can name where execution continues when no managed worktree is created.
func (s *Service) InvokingRoot() string { return s.paths.roots.InvokingRoot }

func (s *Service) List() ([]Record, error) { return s.store.list() }

func (s *Service) Show(slug string) (Record, error) { return s.store.load(slug) }

func memoryUpdateCommand(slug string, invalid map[string]bool) string {
	command := "./awf effort memory update " + slug
	if invalid["phase"] {
		command += " --phase <replacement-phase>"
	}
	if invalid["next"] {
		command += " --next <replacement-next>"
	}
	return command
}

func (s *Service) Finish(ctx context.Context, slug string) (FinishResult, error) {
	if err := validateSlug(slug); err != nil {
		return FinishResult{}, invalidSlugRefusal(slug, err)
	}
	if err := s.validateArchive(); err != nil {
		return FinishResult{}, err
	}
	active := s.paths.effort(slug)
	if _, err := os.Lstat(active); err == nil {
		record, loadErr := s.store.load(slug)
		if loadErr != nil {
			return FinishResult{}, loadErr
		}
		if topologyErr := s.requireNoManagedTopology(ctx, slug); topologyErr != nil {
			return FinishResult{}, topologyErr
		}
		activeResult := newFinishResult(FinishStateActive, false, s.paths.publicArchivePath(record))
		if err := s.requireArchiveDestinationAbsent(record, activeResult); err != nil {
			return activeResult, err
		}
		tombstone := filepath.Join(s.paths.efforts, tombstoneName(record))
		if err := s.store.hit("finish.rename"); err != nil {
			return FinishResult{}, err
		}
		if err := moveDirectoryNoReplace(active, tombstone); err != nil { // coverage-ignore: the validated owned active directory and absent UUID reservation make failure a concurrent namespace or storage fault
			return FinishResult{}, fmt.Errorf("rename effort %s to finishing reservation: %w", slug, err)
		}
		result := newFinishResult(FinishStateReserved, true, s.paths.publicArchivePath(record))
		if result.SourceSyncAvailable {
			if err := s.store.hit("finish.root-fsync"); err != nil {
				return result, partialFinish(result, fmt.Errorf("effort became reserved but source parent sync failed: %w", err), retryFinish(slug))
			}
			if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: injected root-fsync covers this kernel boundary
				return result, partialFinish(result, fmt.Errorf("sync efforts root after finishing rename: %w", err), retryFinish(slug))
			}
			result.SourceSynced = true
		}
		return s.archiveReservation(slug, tombstone, result)
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
		return FinishResult{}, fmt.Errorf("inspect active effort %s: %w", active, err)
	}
	tombstones, err := s.store.findTombstones(slug)
	if err != nil {
		return FinishResult{}, err
	}
	if len(tombstones) == 0 {
		return FinishResult{}, refusal(fmt.Sprintf("effort %q has no active resident or finishing reservation; changed bytes: no; next action: run `awf effort list` and use an active slug", slug), fmt.Sprintf("effort %q has no active resident or finishing reservation", slug), "resident", "", []RecoveryAction{{Text: "run `awf effort list` and use an active slug"}}, nil)
	}
	if len(tombstones) != 1 {
		return FinishResult{}, &CorruptError{Path: s.paths.efforts, Err: fmt.Errorf("multiple finishing reservations match slug %q", slug)}
	}
	return s.archiveReservation(slug, tombstones[0], newFinishResult(FinishStateReserved, false, ""))
}

func (s *Service) validateArchive() error {
	if err := s.paths.validate(s.paths.effortArchive); err != nil {
		return refusal(fmt.Sprintf("validate effort archive root: %v; changed bytes: no; next action: run `awf render` and inspect the archive root", err), "effort archive root is unsafe", "archive", err.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.effortArchive}}, err)
	}
	if err := validateOwnedDirectory(s.paths.effortArchive); err != nil {
		return refusal(fmt.Sprintf("validate effort archive root: %v; changed bytes: no; next action: run `awf render` and inspect the archive root", err), "effort archive root is unsafe", "archive", err.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.effortArchive}}, err)
	}
	info, err := os.Lstat(s.paths.effortArchive)
	if err != nil { // coverage-ignore: validateOwnedDirectory just inspected the same inode; failure requires a concurrent namespace race
		return fmt.Errorf("reinspect effort archive root permissions: %w", err)
	}
	if info.Mode().Perm()&0o022 != 0 {
		permissionErr := safety("resident-permissions", s.paths.effortArchive, fmt.Errorf("mode is %o, group/world write bits must be clear", info.Mode().Perm()))
		return refusal(fmt.Sprintf("validate effort archive root: %v; changed bytes: no; next action: run `awf render` and inspect the archive root", permissionErr), "effort archive root is unsafe", "archive", permissionErr.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.effortArchive}}, permissionErr)
	}
	raw, err := readRegularNoFollow(s.paths.archiveMarker())
	if err != nil {
		return refusal(fmt.Sprintf("validate effort archive marker: %v; changed bytes: no; next action: run `awf render` and inspect the marker", err), "effort archive marker is unsafe or absent", "archive", err.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.archiveMarker()}}, err)
	}
	expected, err := s.expectedArchiveMarker()
	if err != nil {
		return fmt.Errorf("render expected effort archive marker: %w", err)
	}
	if !bytes.Equal(raw, expected) {
		return refusal("effort archive marker is stale; changed bytes: no; next action: run `awf render` and inspect the marker", "effort archive marker is stale", "archive", "marker bytes do not match the planned output", []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.archiveMarker()}}, nil)
	}
	return nil
}

func (s *Service) requireArchiveDestinationAbsent(record Record, result FinishResult) error {
	destination := s.paths.archive(record)
	if _, err := os.Lstat(destination); err == nil {
		source := s.paths.effort(record.Slug)
		if result.State == FinishStateReserved {
			source = filepath.Join(s.paths.efforts, tombstoneName(record))
		}
		return partialFinish(result, fmt.Errorf("effort archive destination already exists: %s", destination), archiveCollisionActions(source, destination))
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat reports an inode or os.ErrNotExist absent a kernel fault
		return fmt.Errorf("inspect archive destination %s: %w", destination, err)
	}
	return nil
}

func retryFinish(slug string) []RecoveryAction {
	return []RecoveryAction{{Text: "retry `awf effort finish " + slug + "`"}}
}

func partialFinish(result FinishResult, cause error, actions []RecoveryAction) error {
	return &PartialFinishError{Result: result, Cause: cause, Actions: actions}
}

func newFinishResult(state FinishResidentState, reserved bool, archivePath string) FinishResult {
	available := directorySyncAvailable()
	return FinishResult{
		State: state, Reserved: reserved, ArchivePath: archivePath,
		DestinationSyncAvailable: available, SourceSyncAvailable: available,
	}
}

func (s *Service) archiveReservation(slug, tombstone string, result FinishResult) (FinishResult, error) {
	record, err := s.store.loadDirectory(tombstone, slug, true)
	if err != nil {
		return result, err
	}
	want := filepath.Join(s.paths.efforts, tombstoneName(record))
	if filepath.Clean(want) != filepath.Clean(tombstone) {
		return result, &CorruptError{Path: tombstone, Err: errors.New("finishing name does not match stored slug and UUID")}
	}
	destination := s.paths.archive(record)
	result.ArchivePath = s.paths.publicArchivePath(record)
	if err := s.requireArchiveDestinationAbsent(record, result); err != nil {
		return result, err
	}
	if err := s.store.hit("finish.archive"); err != nil {
		return result, partialFinish(result, fmt.Errorf("archive move interrupted before completion: %w", err), retryFinish(slug))
	}
	if err := moveDirectoryNoReplace(tombstone, destination); err != nil {
		return result, partialFinish(result, fmt.Errorf("move finishing reservation to archive without replacement: %w", err), archiveMoveRefusalActions(tombstone, destination))
	}
	result.State = FinishStateArchived
	result.Archived = true
	result.SourceSynced = false
	if result.DestinationSyncAvailable {
		if err := s.store.hit("finish.archive-parent-fsync"); err != nil {
			return result, partialFinish(result, fmt.Errorf("archive destination parent sync failed after move: %w", err), archiveInspectionActions(tombstone, destination))
		}
		if err := syncDirectory(s.paths.effortArchive); err != nil { // coverage-ignore: injected archive-parent-fsync covers this kernel boundary
			return result, partialFinish(result, fmt.Errorf("sync archive parent after move: %w", err), archiveInspectionActions(tombstone, destination))
		}
		result.DestinationSynced = true
	}
	if result.SourceSyncAvailable {
		if err := s.store.hit("finish.source-parent-fsync"); err != nil {
			return result, partialFinish(result, fmt.Errorf("efforts source parent sync failed after archive move: %w", err), archiveInspectionActions(tombstone, destination))
		}
		if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: injected source-parent-fsync covers this kernel boundary
			return result, partialFinish(result, fmt.Errorf("sync efforts parent after archive move: %w", err), archiveInspectionActions(tombstone, destination))
		}
		result.SourceSynced = true
	}
	return result, nil
}

func archiveCollisionActions(source, destination string) []RecoveryAction {
	return []RecoveryAction{{Text: "inspect " + source}, {Text: "inspect " + destination}, {Text: "preserve both residents and resolve the collision manually before retrying"}}
}

func archiveMoveRefusalActions(source, destination string) []RecoveryAction {
	return []RecoveryAction{{Text: "inspect " + source}, {Text: "inspect " + destination}, {Text: "resolve the destination collision or filesystem boundary before retrying"}}
}

func archiveInspectionActions(source, destination string) []RecoveryAction {
	return []RecoveryAction{{Text: "inspect " + source}, {Text: "inspect " + destination}, {Text: "do not blindly retry finish after the archive move"}}
}

func (s *Service) requireNoManagedTopology(ctx context.Context, slug string) error {
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	if _, err := os.Lstat(managed); err == nil {
		return managedTopologyRefusal([]RecoveryAction{{Text: "run `awf effort worktree remove " + slug + "`"}, {Text: "retry `awf effort finish " + slug + "`"}}, "managed worktree path %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", managed, slug)
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat returns an inode or os.ErrNotExist absent a kernel fault
		return fmt.Errorf("inspect managed worktree path %s: %w", managed, err)
	}
	registrations, err := s.worktrees(ctx)
	if err != nil {
		return fmt.Errorf("inspect managed worktree registrations: %w", err)
	}
	wantBranch := "refs/heads/awf/" + slug
	for _, registration := range registrations {
		if filepath.Clean(registration.Path) == managed || registration.Branch == wantBranch {
			return managedTopologyRefusal([]RecoveryAction{{Text: "run `awf effort worktree remove " + slug + "`"}, {Text: "retry `awf effort finish " + slug + "`"}}, "managed Git registration for %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
		}
	}
	exists, err := s.branchExists(ctx, "awf/"+slug)
	if err != nil {
		return fmt.Errorf("inspect managed branch for %s: %w", slug, err)
	}
	if exists {
		return managedTopologyRefusal([]RecoveryAction{{Text: "run `awf effort worktree remove " + slug + "`"}, {Text: "retry `awf effort finish " + slug + "`"}}, "managed branch awf/%s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
	}
	return nil
}

// RollbackCreation removes only the immutable resident created by a failed
// default worktree transaction. It is deliberately not a finish variant.
func (s *Service) RollbackCreation(ctx context.Context, identity Record) (RollbackResult, error) {
	if err := s.requireNoManagedTopology(ctx, identity.Slug); err != nil {
		return RollbackResult{}, err
	}
	active := s.paths.effort(identity.Slug)
	record, err := s.store.load(identity.Slug)
	if err != nil {
		return RollbackResult{}, err
	}
	if record.ID != identity.ID || record.Slug != identity.Slug {
		return RollbackResult{}, refusal("failed-creation rollback identity no longer matches; changed bytes: no; next action: retain and inspect the resident", "failed-creation rollback identity changed", "resident", "immutable identity mismatch", []RecoveryAction{{Text: "retain and inspect " + active}}, nil)
	}
	reservation := filepath.Join(s.paths.efforts, tombstoneName(record))
	if err := s.store.hit("rollback.rename"); err != nil {
		return RollbackResult{}, err
	}
	if err := moveDirectoryNoReplace(active, reservation); err != nil { // coverage-ignore: identity was just proven and the UUID reservation is absent
		return RollbackResult{}, fmt.Errorf("reserve failed-creation rollback: %w", err)
	}
	result := RollbackResult{Reserved: true}
	if err := s.store.hit("rollback.root-fsync"); err != nil {
		return result, fmt.Errorf("sync efforts parent after rollback reservation: %w", err)
	}
	if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: injected rollback.root-fsync covers this kernel boundary
		return result, fmt.Errorf("sync efforts parent after rollback reservation: %w", err)
	}
	if err := s.store.hit("rollback.delete"); err != nil {
		return result, err
	}
	if err := s.removeTree(reservation); err != nil {
		return result, fmt.Errorf("delete identity-bound failed-creation reservation: %w", err)
	}
	result.Removed = true
	if err := s.store.hit("rollback.delete-fsync"); err != nil {
		return result, fmt.Errorf("sync efforts parent after rollback deletion: %w", err)
	}
	if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: injected rollback.delete-fsync covers this kernel boundary
		return result, fmt.Errorf("sync efforts parent after rollback deletion: %w", err)
	}
	return result, nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
