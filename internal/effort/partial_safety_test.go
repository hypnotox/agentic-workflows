package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// invariant: tooling/effort-management:effort-record-authority
func TestPartialAbsentDistinguishesExistingPath(t *testing.T) {
	path := t.TempDir()
	absent, err := partialAbsent(path)
	if err != nil || absent {
		t.Fatalf("existing path classified as absent: absent=%v err=%v", absent, err)
	}
	file := path + "/file"
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if absent, err := partialAbsent(file); err != nil || absent {
		t.Fatalf("existing file classified as absent: absent=%v err=%v", absent, err)
	}
	old := partialLstat
	partialLstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
	defer func() { partialLstat = old }()
	if absent, err := partialAbsent(file); err == nil || absent {
		t.Fatalf("stat error classified as absent: absent=%v err=%v", absent, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestRecoveryRemovalFaultBranches(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Service, string)
	}{
		{"registration lookup", func(s *Service, _ string) {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return nil, errors.New("registration fault")
			}
		}},
		{"branch registered elsewhere", func(s *Service, id string) {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: filepath.Join(s.paths.worktrees, "other"), Branch: "refs/heads/awf/" + id}}, nil
			}
		}},
		{"invalid registration", func(s *Service, _ string) {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: s.paths.managedWorktree(idA), Branch: "refs/heads/other", Detached: true}}, nil
			}
		}},
		{"managed symlink", func(s *Service, _ string) {
			_ = os.RemoveAll(s.paths.managedWorktree(idA))
			_ = os.Symlink(t.TempDir(), s.paths.managedWorktree(idA))
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: s.paths.managedWorktree(idA), Branch: "refs/heads/awf/" + idA}}, nil
			}
		}},
		{"foreign checkout", func(s *Service, _ string) {
			_ = os.RemoveAll(s.paths.managedWorktree(idA))
			runEffortGit(t, "init", s.paths.managedWorktree(idA))
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: s.paths.managedWorktree(idA), Branch: "refs/heads/awf/" + idA}}, nil
			}
		}},
		{"managed truth", func(s *Service, _ string) {
			path := s.paths.managedWorktree(idA)
			runEffortGit(t, "-C", s.paths.roots.InvokingRoot, "worktree", "add", "-b", "awf/"+idA, path, "HEAD")
			oldOwner := residentOwner
			t.Cleanup(func() { residentOwner = oldOwner })
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				residentOwner = func(string, os.FileInfo) error { return errors.New("managed truth fault") }
				return []awfgit.WorktreeRegistration{{Path: path, Branch: "refs/heads/awf/" + idA}}, nil
			}
		}},
		{"absent stat", func(s *Service, _ string) {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := openEffortService(t, initEffortRepo(t), time.Now())
			if _, err := service.New("recovery", false); err != nil {
				t.Fatal(err)
			}
			evidence := PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "removal", Branch: "awf/" + idA, CommonDir: filepath.Clean(service.paths.roots.CommonDir), BranchTip: strings.Repeat("a", 40)}
			if err := service.RecordPartial(evidence); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(service.paths.worktrees, 0o700); err != nil {
				t.Fatal(err)
			}
			tc.setup(service, idA)
			if _, _, err := service.store.getPartial(idA, "removal"); err != nil {
				t.Fatalf("evidence missing: %v", err)
			}
			oldLstat := partialLstat
			if tc.name == "absent stat" {
				managed := service.paths.managedWorktree(idA)
				partialLstat = func(path string) (os.FileInfo, error) {
					if path == managed {
						return nil, errors.New("stat fault")
					}
					return oldLstat(path)
				}
				defer func() { partialLstat = oldLstat }()
			}
			record, err := service.Show(idA)
			if err != nil {
				t.Fatal(err)
			}
			result := RepairResult{SchemaVersion: SchemaVersion}
			if err := service.recoverPartial(&record, &result); err == nil {
				t.Fatal("fault was not surfaced")
			} else {
				t.Logf("recovery refusal: %v", err)
			}
		})
	}
}

func TestRecoveryRemovalPostProbeFaults(t *testing.T) {
	oldBranches, oldResolve, oldGit := partialBranches, partialResolveGit, partialGit
	defer func() { partialBranches, partialResolveGit, partialGit = oldBranches, oldResolve, oldGit }()
	for _, tc := range []struct {
		name      string
		configure func()
		mutate    func(*Record)
	}{
		{"branch probe", func() {
			partialBranches = func(context.Context, string, string) (bool, error) { return false, errors.New("branch probe") }
		}, nil},
		{"tip probe", func() {
			partialBranches = func(context.Context, string, string) (bool, error) { return true, nil }
			partialResolveGit = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("tip probe") }
		}, nil},
		{"branch delete", func() {
			partialBranches = func(context.Context, string, string) (bool, error) { return true, nil }
			partialResolveGit = func(context.Context, string, ...string) ([]byte, error) {
				return []byte(strings.Repeat("a", 40)), nil
			}
			partialGit = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("branch delete") }
		}, nil},
		{"record publish", func() {
			partialBranches = func(context.Context, string, string) (bool, error) { return false, nil }
		}, func(r *Record) { r.Title = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := openEffortService(t, initEffortRepo(t), time.Now())
			if _, err := service.New("recovery", false); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(service.paths.worktrees, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := service.RecordPartial(PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "removal", Branch: "awf/" + idA, CommonDir: filepath.Clean(service.paths.roots.CommonDir), BranchTip: strings.Repeat("a", 40)}); err != nil {
				t.Fatal(err)
			}
			record, err := service.Show(idA)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(&record)
			}
			tc.configure()
			result := RepairResult{SchemaVersion: SchemaVersion}
			if err := service.recoverPartial(&record, &result); err == nil {
				t.Fatal("fault was not surfaced")
			}
		})
	}
}

func TestSafeRecoveryPathRejectsSymlinkAndOwnerFaults(t *testing.T) {
	if err := safeRecoveryPath(string(filepath.Separator)); err != nil {
		t.Fatalf("root path: %v", err)
	}
	root := t.TempDir()
	leaf := filepath.Join(root, "resident")
	if err := os.Mkdir(leaf, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := safeRecoveryPath(leaf); err != nil {
		t.Fatalf("safe path: %v", err)
	}
	old := residentOwner
	residentOwner = func(string, os.FileInfo) error { return os.ErrPermission }
	if err := safeRecoveryPath(leaf); err == nil {
		t.Fatal("foreign owner accepted")
	}
	residentOwner = old
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if err := safeRecoveryPath(filepath.Join(link, "child")); err == nil {
		t.Fatal("symlink ancestor accepted")
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestManagedDirectoryTruthRejectsInjectedForeignOwner(t *testing.T) {
	path := t.TempDir()
	old := residentOwner
	residentOwner = func(string, os.FileInfo) error { return os.ErrPermission }
	defer func() { residentOwner = old }()
	if present, err := managedDirectoryTruth(path); err == nil || present {
		t.Fatalf("foreign owner accepted: present=%v err=%v", present, err)
	}
}

// invariant: tooling/effort-management:effort-record-authority
func TestValidateCurrentOwnerAcceptsCurrentDirectory(t *testing.T) {
	path := t.TempDir()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrentOwner(path, info); err != nil {
		t.Fatal(err)
	}
}
