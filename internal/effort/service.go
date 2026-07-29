package effort

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// Options supplies deterministic dependencies to focused tests.
type Options struct {
	Clock        func() time.Time
	UUID         func() (string, error)
	Fault        func(string) error
	Worktrees    func(context.Context, string) ([]awfgit.WorktreeRegistration, error)
	Git          func(context.Context, string, ...string) ([]byte, error)
	BranchExists func(context.Context, string, string) (bool, error)
	RemoveTree   func(string) error
}

// Service owns immutable effort residents and restartable finish.
type Service struct {
	ctx          context.Context
	paths        paths
	store        store
	clock        func() time.Time
	uuid         func() (string, error)
	worktrees    func(context.Context, string) ([]awfgit.WorktreeRegistration, error)
	git          func(context.Context, string, ...string) ([]byte, error)
	branchExists func(context.Context, string, string) (bool, error)
	removeTree   func(string) error
}

func Open(ctx context.Context, invokingRoot string, options Options) (*Service, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invokingRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve effort control roots from %s: %w", invokingRoot, err)
	}
	resolved, err := resolvePaths(roots)
	if err != nil {
		return nil, fmt.Errorf("resolve effort resident paths from %s: %w", roots.PrimaryRoot, err)
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	allocator := options.UUID
	if allocator == nil {
		allocator = randomUUIDv4
	}
	worktrees := options.Worktrees
	if worktrees == nil {
		worktrees = awfgit.ListWorktreeRegistrations
	}
	gitRunner := options.Git
	if gitRunner == nil {
		gitRunner = nativeGit
	}
	branchExists := options.BranchExists
	if branchExists == nil {
		branchExists = nativeBranchExists
	}
	removeTree := options.RemoveTree
	if removeTree == nil {
		removeTree = os.RemoveAll
	}
	return &Service{
		ctx: ctx, paths: resolved, store: store{paths: resolved, fault: options.Fault},
		clock: clock, uuid: allocator, worktrees: worktrees, git: gitRunner,
		branchExists: branchExists, removeTree: removeTree,
	}, nil
}

func randomUUIDv4() (string, error) {
	var raw [16]byte
	_, _ = rand.Read(raw[:]) // crypto/rand.Read fills the slice or terminates the process.
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func nativeGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return output, nil
}

func nativeBranchExists(ctx context.Context, root, branch string) (bool, error) {
	command := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	command.Dir = root
	err := command.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("inspect branch %s: %w", branch, err)
}

func (s *Service) now() time.Time { return s.clock().UTC() }

func (s *Service) New(title string) (Record, error) {
	normalized, err := normalizeTitle(title)
	if err != nil {
		return Record{}, fmt.Errorf("invalid outcome title: %w; changed bytes: no; next action: provide a nonblank valid UTF-8 outcome title", err)
	}
	slug, err := deriveSlug(normalized)
	if err != nil {
		return Record{}, err
	}
	id, err := s.uuid()
	if err != nil {
		return Record{}, fmt.Errorf("allocate internal effort UUID: %w; changed bytes: no; next action: retry `awf effort new %q`", err, normalized)
	}
	if !uuidV4Pattern.MatchString(id) {
		return Record{}, fmt.Errorf("allocator returned invalid UUIDv4 %q; changed bytes: no; next action: repair the awf installation and retry", id)
	}
	record := Record{SchemaVersion: SchemaVersion, ID: id, Slug: slug, Title: normalized, CreatedAt: s.now(), MemoryPath: memoryPublicPath(slug)}
	if err := s.store.create(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) List() ([]Record, error) { return s.store.list() }

func (s *Service) Show(slug string) (Record, error) { return s.store.load(slug) }

func (s *Service) Finish(slug string) (FinishResult, error) {
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
		if topologyErr := s.requireNoManagedTopology(slug); topologyErr != nil {
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

func (s *Service) requireNoManagedTopology(slug string) error {
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	if _, err := os.Lstat(managed); err == nil {
		return fmt.Errorf("managed worktree path %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", managed, slug)
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: local lstat returns an inode or os.ErrNotExist absent a kernel fault
		return fmt.Errorf("inspect managed worktree path %s: %w", managed, err)
	}
	registrations, err := s.worktrees(s.ctx, s.paths.roots.InvokingRoot)
	if err != nil {
		return fmt.Errorf("inspect managed worktree registrations: %w", err)
	}
	wantBranch := "refs/heads/awf/" + slug
	for _, registration := range registrations {
		if filepath.Clean(registration.Path) == managed || registration.Branch == wantBranch {
			return fmt.Errorf("managed Git registration for %s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
		}
	}
	exists, err := s.branchExists(s.ctx, s.paths.roots.InvokingRoot, "awf/"+slug)
	if err != nil {
		return fmt.Errorf("inspect managed branch for %s: %w", slug, err)
	}
	if exists {
		return fmt.Errorf("managed branch awf/%s remains; changed bytes: no; next action: run `awf effort worktree remove %s`", slug, slug)
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
