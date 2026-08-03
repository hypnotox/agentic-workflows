package effort

import (
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
	Clock        func() time.Time
	UUID         func() (string, error)
	Worktrees    func(context.Context) ([]awfgit.WorktreeRegistration, error)
	BranchExists func(context.Context, string) (bool, error)
	ValidateRef  func(context.Context, string) (bool, error)
	RemoveTree   func(string) error
	// Fault is the durability-boundary hook the restartable-finish tests
	// interrupt the service at. It is the one optional member: a nil Fault
	// injects nothing, which is what production wants and what "no fault
	// injection configured" means.
	Fault func(stage string) error
}

// Service owns immutable effort residents and restartable finish.
type Service struct {
	paths        paths
	store        store
	clock        func() time.Time
	uuid         func() (string, error)
	worktrees    func(context.Context) ([]awfgit.WorktreeRegistration, error)
	branchExists func(context.Context, string) (bool, error)
	validateRef  func(context.Context, string) (bool, error)
	removeTree   func(string) error
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
	}
	resolved, err := resolvePaths(roots)
	if err != nil {
		return nil, fmt.Errorf("resolve effort resident paths from %s: %w", roots.PrimaryRoot, err)
	}
	return &Service{
		paths: resolved, store: store{paths: resolved, fault: deps.Fault},
		clock: deps.Clock, uuid: deps.UUID, worktrees: deps.Worktrees,
		branchExists: deps.BranchExists, validateRef: deps.ValidateRef, removeTree: deps.RemoveTree,
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
		return Record{}, fmt.Errorf("invalid outcome title: %w; changed bytes: no; next action: provide a nonblank valid UTF-8 outcome title", err)
	}
	if err := validateNewSlug(ctx, s.validateRef, input.Slug); err != nil {
		return Record{}, err
	}
	id, err := s.uuid()
	if err != nil {
		return Record{}, fmt.Errorf("allocate internal effort UUID: %w; changed bytes: no; next action: retry `awf effort new --slug %q %q`", err, input.Slug, normalized)
	}
	if !uuidV4Pattern.MatchString(id) {
		return Record{}, fmt.Errorf("allocator returned invalid UUIDv4 %q; changed bytes: no; next action: repair the awf installation and retry `awf effort new --slug %q %q`", id, input.Slug, normalized)
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

// UpdateMemory changes selected checkpoint fields and migrates a safe legacy
// header to the canonical closed metadata representation on its first write.
func (s *Service) UpdateMemory(slug string, update MemoryUpdate) error {
	if update.Phase == nil && update.Next == nil {
		return errors.New("memory update requires --phase or --next; changed bytes: no; next action: supply at least one replacement field")
	}
	if err := validateSlug(slug); err != nil {
		return fmt.Errorf("invalid effort slug %q: %w; changed bytes: no; next action: use the exact slug from `awf effort list`", slug, err)
	}
	if update.Phase != nil {
		if err := validateMemoryMutable(*update.Phase); err != nil {
			return fmt.Errorf("invalid memory phase: %w; changed bytes: no", err)
		}
	}
	if update.Next != nil {
		if err := validateMemoryMutable(*update.Next); err != nil {
			return fmt.Errorf("invalid memory next: %w; changed bytes: no", err)
		}
	}
	// Load state separately from memory metadata: an invalid mutable memory is
	// precisely the repair case, but its resident and immutable identity remain
	// subject to the normal ownership checks.
	if _, err := s.store.loadDirectory(s.paths.effort(slug), slug, false); err != nil {
		return err
	}
	path := s.paths.memoryFile(slug)
	raw, err := readRegularNoFollowBounded(path, maxMemoryBytes)
	if err != nil {
		return &CorruptError{Path: path, Err: err}
	}
	doc := inspectMemory(raw, slug)
	if !doc.boundary || doc.identity != slug {
		return &InvalidMemoryError{Slug: slug, Err: doc.err, NextAction: "repair memory.md manually with a matching bounded canonical or legacy identity"}
	}
	if doc.err != nil {
		if doc.invalid["phase"] && update.Phase == nil || doc.invalid["next"] && update.Next == nil {
			return &InvalidMemoryError{Slug: slug, Err: doc.err, NextAction: memoryUpdateCommand(slug, doc.invalid)}
		}
		// A recognized header with only mutable validation failures is safe to
		// replace. Unknown syntax and identity failures were rejected above.
		if len(doc.invalid) == 0 { // coverage-ignore: every boundary-preserving parse error with matching identity either marks a mutable field invalid or was rejected before this repair path
			return &InvalidMemoryError{Slug: slug, Err: doc.err, NextAction: "repair memory.md manually with a matching bounded canonical or legacy identity"}
		}
	}
	metadata := doc.metadata
	metadata.Effort = slug
	if update.Phase != nil {
		metadata.Phase = *update.Phase
	}
	if update.Next != nil {
		metadata.Next = *update.Next
	}
	metadata.Updated = formatMemoryTime(s.now())
	if err := validateMemoryMutable(metadata.Phase); err != nil || validateMemoryMutable(metadata.Next) != nil { // coverage-ignore: supplied replacements are validated and any unrepaired invalid mutable field returned above
		return &InvalidMemoryError{Slug: slug, Err: errors.New("missing invalid mutable replacement"), NextAction: memoryUpdateCommand(slug, doc.invalid)}
	}
	encoded, err := encodeMemory(metadata, doc.body)
	if err != nil { // coverage-ignore: encodeMemory marshals a fixed YAML node of already-valid strings
		return fmt.Errorf("encode updated memory: %w", err)
	}
	return s.store.replaceMemory(path, encoded)
}

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
		return FinishResult{}, fmt.Errorf("invalid effort slug %q: %w; changed bytes: no; next action: use the exact slug from `awf effort list`", slug, err)
	}
	active := s.paths.effort(slug)
	_, err := os.Lstat(active)
	if err == nil {
		record, loadErr := s.store.load(slug)
		if loadErr != nil {
			return FinishResult{}, loadErr
		}
		if topologyErr := s.requireNoManagedTopology(ctx, slug); topologyErr != nil {
			return FinishResult{}, topologyErr
		}
		tombstone := filepath.Join(s.paths.efforts, tombstoneName(record))
		if err := s.store.hit("finish.rename"); err != nil {
			return FinishResult{}, err
		}
		if err := os.Rename(active, tombstone); err != nil { // coverage-ignore: the validated owned active directory and absent UUID tombstone make rename failure a concurrent namespace or storage fault
			return FinishResult{}, fmt.Errorf("rename effort %s to finishing reservation: %w", slug, err)
		}
		if err := s.store.hit("finish.root-fsync"); err != nil {
			return FinishResult{Renamed: true}, fmt.Errorf("effort became inactive but finishing root sync failed: %w; changed bytes: yes; next action: retry `awf effort finish %s`", err, slug)
		}
		if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: injected root-fsync covers the ordered boundary; actual sync failure requires a kernel or storage fault
			return FinishResult{Renamed: true}, fmt.Errorf("fsync efforts root after finishing rename: %w; changed bytes: yes; next action: retry `awf effort finish %s`", err, slug)
		}
		return s.cleanTombstone(slug, tombstone, true)
	}
	if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat returns an inode or os.ErrNotExist absent a kernel fault
		return FinishResult{}, fmt.Errorf("inspect active effort %s: %w", active, err)
	}
	tombstones, err := s.store.findTombstones(slug)
	if err != nil {
		return FinishResult{}, err
	}
	if len(tombstones) == 0 {
		return FinishResult{}, fmt.Errorf("effort %q has no active resident or finishing reservation; changed bytes: no; next action: run `awf effort list` and use an active slug", slug)
	}
	if len(tombstones) != 1 {
		return FinishResult{}, &CorruptError{Path: s.paths.efforts, Err: fmt.Errorf("multiple finishing reservations match slug %q", slug)}
	}
	return s.cleanTombstone(slug, tombstones[0], false)
}

func (s *Service) requireNoManagedTopology(ctx context.Context, slug string) error {
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	if _, err := os.Lstat(managed); err == nil {
		return managedTopologyRefusal("managed worktree path %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", managed, slug)
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
			return managedTopologyRefusal("managed Git registration for %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
		}
	}
	exists, err := s.branchExists(ctx, "awf/"+slug)
	if err != nil {
		return fmt.Errorf("inspect managed branch for %s: %w", slug, err)
	}
	if exists {
		return managedTopologyRefusal("managed branch awf/%s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
	}
	return nil
}

func (s *Service) cleanTombstone(slug, tombstone string, renamed bool) (FinishResult, error) {
	record, err := s.store.loadDirectory(tombstone, slug, true)
	if err != nil {
		return FinishResult{Renamed: renamed}, err
	}
	want := filepath.Join(s.paths.efforts, tombstoneName(record))
	if filepath.Clean(want) != filepath.Clean(tombstone) {
		return FinishResult{Renamed: renamed}, &CorruptError{Path: tombstone, Err: errors.New("finishing name does not match stored slug and UUID")}
	}
	if err := s.store.hit("finish.delete"); err != nil {
		return FinishResult{Renamed: renamed}, fmt.Errorf("finishing cleanup interrupted: %w; changed bytes: %s; next action: retry `awf effort finish %s`", err, yesNo(renamed), slug)
	}
	if err := s.removeTree(tombstone); err != nil {
		return FinishResult{Renamed: renamed}, fmt.Errorf("delete proven finishing reservation %s: %w; changed bytes: %s; next action: retry `awf effort finish %s`", tombstone, err, yesNo(renamed), slug)
	}
	if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: the owned root remains openable after deleting one proven child; failure requires a kernel or storage fault
		return FinishResult{Renamed: renamed, Cleaned: true}, fmt.Errorf("fsync efforts root after finishing cleanup: %w; changed bytes: yes; next action: verify %s is absent, then retry finish if it remains", err, tombstone)
	}
	return FinishResult{Renamed: renamed, Cleaned: true}, nil
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
