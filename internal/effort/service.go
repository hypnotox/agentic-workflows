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
		branchExists: deps.BranchExists, validateRef: deps.ValidateRef,
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

func (s *Service) List() ([]Record, error) { return s.store.list() }

func (s *Service) Show(slug string) (Record, error) { return s.store.load(slug) }

func (s *Service) Finish(ctx context.Context, slug string) (FinishResult, error) {
	if err := validateSlug(slug); err != nil {
		return FinishResult{}, invalidSlugRefusal(slug, err)
	}
	record, err := s.store.load(slug)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FinishResult{}, refusal(fmt.Sprintf("effort %q has no active resident; changed bytes: no; next action: run `awf effort list` and use an active slug", slug), fmt.Sprintf("effort %q has no active resident", slug), "resident", "", []RecoveryAction{{Text: "run `awf effort list` and use an active slug"}}, nil)
		}
		return FinishResult{}, err
	}
	if err := s.requireNoManagedTopology(ctx, slug); err != nil {
		return FinishResult{}, err
	}
	if err := s.validateArchive(); err != nil {
		return FinishResult{}, err
	}
	active := s.paths.effort(slug)
	destination := s.paths.archive(record)
	if err := s.requireArchiveDestinationAbsent(active, destination); err != nil {
		return FinishResult{}, err
	}
	if err := s.store.hit("finish.move"); err != nil {
		return FinishResult{}, refusal(fmt.Sprintf("archive move failed before changing the active resident: %v; changed bytes: no; next action: retry `awf effort finish %s` or inspect %s", err, slug, active), "effort archive move failed", "operation", err.Error(), []RecoveryAction{{Text: "retry `awf effort finish " + slug + "`"}, {Text: "inspect " + active}}, err)
	}
	if err := moveDirectoryNoReplace(active, destination); err != nil {
		return FinishResult{}, refusal(fmt.Sprintf("move active effort to archive without replacement: %v; changed bytes: no; next action: inspect the active resident and archive destination, then retry", err), "effort archive move failed", "operation", err.Error(), archiveMoveRefusalActions(active, destination), err)
	}
	return FinishResult{ArchivePath: s.paths.publicArchivePath(record)}, nil
}

func (s *Service) validateArchive() error {
	if err := s.paths.validate(s.paths.effortArchive); err != nil {
		return refusal(fmt.Sprintf("validate effort archive root: %v; changed bytes: no; next action: run `awf render` and inspect the archive root", err), "effort archive root is unsafe", "archive", err.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.effortArchive}}, err)
	}
	if err := validateOwnedDirectory(s.paths.effortArchive); err != nil {
		return refusal(fmt.Sprintf("validate effort archive root: %v; changed bytes: no; next action: run `awf render` and inspect the archive root", err), "effort archive root is unsafe", "archive", err.Error(), []RecoveryAction{{Text: "run `awf render`"}, {Text: "inspect " + s.paths.effortArchive}}, err)
	}
	info, err := os.Lstat(s.paths.effortArchive)
	if err != nil {
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

func (s *Service) requireArchiveDestinationAbsent(source, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return refusal(fmt.Sprintf("effort archive destination already exists: %s; changed bytes: no; next action: inspect both residents and resolve the collision manually", destination), "effort archive destination already exists", "archive", "destination collision", archiveCollisionActions(source, destination), nil)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect archive destination %s: %w", destination, err)
	}
	return nil
}

func archiveCollisionActions(source, destination string) []RecoveryAction {
	return []RecoveryAction{{Text: "inspect " + source}, {Text: "inspect " + destination}, {Text: "preserve both residents and resolve the collision manually before retrying"}}
}

func archiveMoveRefusalActions(source, destination string) []RecoveryAction {
	return []RecoveryAction{{Text: "inspect " + source}, {Text: "inspect " + destination}, {Text: "resolve the destination collision or filesystem boundary before retrying"}}
}

func (s *Service) requireNoManagedTopology(ctx context.Context, slug string) error {
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	if _, err := os.Lstat(managed); err == nil {
		return managedTopologyRefusal([]RecoveryAction{{Text: "run `awf effort worktree remove " + slug + "`"}, {Text: "retry `awf effort finish " + slug + "`"}}, "managed worktree path %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", managed, slug)
	} else if !errors.Is(err, os.ErrNotExist) {
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
