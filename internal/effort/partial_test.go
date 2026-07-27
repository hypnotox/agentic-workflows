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

func TestPartialEvidenceFaultSeams(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	base, err := resolvePartial(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	e := PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: "awf/" + idA, Path: filepath.Clean(s.paths.managedWorktree(idA)), CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
	if err := s.RecordPartial(e); err != nil {
		t.Fatal(err)
	}
	oldIdentity := partialIdentity
	partialIdentity = func(string, fileIdentity) error { return errors.New("identity race") }
	if err := s.ClearPartial(idA, "worktree"); err == nil {
		t.Fatal("identity fault hidden")
	}
	partialIdentity = oldIdentity
	_ = os.Remove(s.paths.partial(idA, "worktree"))
	oldResolve := partialResolveGit
	partialResolveGit = func(context.Context, string, ...string) ([]byte, error) { return []byte("not-an-object\n"), nil }
	if _, err := resolvePartial(t.Context(), root, "HEAD"); err == nil {
		t.Fatal("invalid resolver output accepted")
	}
	partialResolveGit = oldResolve
}

func TestPartialEvidenceValidationAndHelpers(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Date(2026, 7, 27, 5, 0, 0, 0, time.UTC))
	base, err := resolvePartial(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	valid := PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: "awf/" + idA, Path: filepath.Clean(s.paths.managedWorktree(idA)), CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
	invalid := []PartialEvidence{
		{EffortID: "bad", Action: "worktree", Branch: valid.Branch, CommonDir: valid.CommonDir},
		{SchemaVersion: 1, EffortID: idA, Action: "unknown", Branch: valid.Branch, CommonDir: valid.CommonDir},
		{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: "bad", Branch: valid.Branch, Path: valid.Path, CommonDir: valid.CommonDir},
		{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: valid.Branch, Path: "relative", CommonDir: valid.CommonDir},
		{SchemaVersion: 1, EffortID: idA, Action: "integration", Tip: "bad", Branch: valid.Branch, TargetPath: root, TargetBranch: "main", Integration: IntegrationMerge, CommonDir: valid.CommonDir},
		{SchemaVersion: 1, EffortID: idA, Action: "removal", Branch: valid.Branch, BranchTip: "bad", CommonDir: valid.CommonDir},
	}
	for _, evidence := range invalid {
		if err := s.RecordPartial(evidence); err == nil {
			t.Fatalf("invalid evidence accepted: %#v", evidence)
		}
	}
	if err := s.RecordPartial(valid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.store.getPartial(idA, "integration"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing evidence error = %v", err)
	}
	if err := s.ClearPartial(idA, "worktree"); err != nil {
		t.Fatal(err)
	}
	if ok, err := branchExistsPartial(t.Context(), root, "missing"); err != nil || ok {
		t.Fatalf("missing branch probe = %v, %v", ok, err)
	}
	if ok, err := ancestorPartial(t.Context(), root, base, base); err != nil || !ok {
		t.Fatalf("ancestor probe = %v, %v", ok, err)
	}
	link := s.paths.partial(idA, "worktree")
	if err := os.Symlink(filepath.Join(root, "other"), link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.store.getPartial(idA, "worktree"); err == nil {
		t.Fatal("symlink evidence accepted")
	}
}

func TestPartialEvidenceValidationAndProbeFailures(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	base, err := resolvePartial(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Clean(s.paths.roots.CommonDir)
	valid := PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: "awf/" + idA, Path: filepath.Clean(s.paths.managedWorktree(idA)), CommonDir: common}
	cases := []PartialEvidence{
		{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: valid.Branch, Path: valid.Path, CommonDir: common + string(filepath.Separator)},
		{SchemaVersion: 2, EffortID: idA, Action: "worktree", Base: base, Branch: valid.Branch, Path: valid.Path, CommonDir: common},
		{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: valid.Branch, Path: valid.Path + string(filepath.Separator) + ".", CommonDir: common},
		{SchemaVersion: 1, EffortID: idA, Action: "integration", Tip: base, Branch: valid.Branch, TargetPath: root, TargetBranch: "main", Integration: IntegrationNone, CommonDir: common},
		{SchemaVersion: 1, EffortID: idA, Action: "removal", Branch: valid.Branch, BranchTip: "bad", CommonDir: common},
	}
	for _, tc := range cases {
		if err := s.store.putPartialValidation(tc); err == nil {
			t.Fatalf("invalid validation accepted: %#v", tc)
		}
	}
	if err := s.store.putPartialValidation(PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "integration", Tip: base, Branch: valid.Branch, TargetPath: root, TargetBranch: "main", Integration: IntegrationMerge, CommonDir: common}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.putPartialValidation(PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "removal", Branch: valid.Branch, BranchTip: base, CommonDir: common}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPartial(valid); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPartial(valid); err == nil {
		t.Fatal("duplicate evidence accepted")
	}
	if _, err := resolvePartial(t.Context(), root, "missing"); err == nil {
		t.Fatal("missing revision accepted")
	}
	if _, err := resolvePartial(t.Context(), root, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := ancestorPartialWithRunnerForTest(t); err == nil {
		t.Fatal("ancestor probe fault hidden")
	}
	if _, err := branchExistsPartialWithRunnerForTest(t); err == nil {
		t.Fatal("branch probe fault hidden")
	}
	if err := s.ClearPartial(idA, "worktree"); err != nil {
		t.Fatal(err)
	}
}

func ancestorPartialWithRunnerForTest(t *testing.T) (bool, error) {
	t.Helper()
	// The production helper deliberately shells out; use a cancelled command to
	// reach its non-exit-error branch without adding a second Git abstraction.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ancestorPartial(ctx, t.TempDir(), "a", "b")
}
func branchExistsPartialWithRunnerForTest(t *testing.T) (bool, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return branchExistsPartial(ctx, t.TempDir(), "missing")
}

func TestPartialRecoveryFaultBranches(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Service, *Record, string) PartialEvidence
	}{
		{"path mismatch", func(s *Service, r *Record, base string) PartialEvidence {
			return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "worktree", Base: base, Branch: "awf/" + r.ID, Path: "/foreign", CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
		}},
		{"listing error", func(s *Service, r *Record, base string) PartialEvidence {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, errors.New("list") }
			return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "worktree", Base: base, Branch: "awf/" + r.ID, Path: s.paths.managedWorktree(r.ID), CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
		}},
		{"managed roots error", func(s *Service, r *Record, base string) PartialEvidence {
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: s.paths.managedWorktree(r.ID), Branch: "refs/heads/awf/" + r.ID, HEAD: base}}, nil
			}
			return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "worktree", Base: base, Branch: "awf/" + r.ID, Path: s.paths.managedWorktree(r.ID), CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
		}},
		{"integration no record", func(s *Service, r *Record, base string) PartialEvidence {
			return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "integration", Base: base, Tip: base, Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), TargetPath: filepath.Clean(s.paths.roots.InvokingRoot), TargetBranch: "main", Integration: IntegrationMerge}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			s := openEffortService(t, root, time.Now())
			r, err := s.New("fault", false)
			if err != nil {
				t.Fatal(err)
			}
			base, err := resolvePartial(t.Context(), root, "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			e := tc.setup(s, &r, base)
			if err := s.RecordPartial(e); err != nil {
				t.Fatal(err)
			}
			var result RepairResult
			if err := s.recoverPartial(&r, &result); err == nil {
				t.Fatal("fault accepted")
			}
		})
	}

	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	r, err := s.New("faults", false)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := resolvePartial(t.Context(), root, "HEAD")
	path := s.paths.managedWorktree(r.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+r.ID, path, base)
	e := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "worktree", Base: base, Branch: "awf/" + r.ID, Path: path, CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
	if err := s.RecordPartial(e); err != nil {
		t.Fatal(err)
	}
	s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
		return []awfgit.WorktreeRegistration{{Path: path, Branch: "refs/heads/awf/" + r.ID, HEAD: base}}, nil
	}
	var result RepairResult
	if err := s.recoverPartial(&r, &result); err != nil {
		t.Fatal(err)
	}
	if r.Worktree == nil {
		t.Fatal("worktree not recovered")
	}
	r.Worktree.Branch = "awf/other"
	if err := s.RecordPartial(e); err != nil {
		t.Fatal(err)
	}
	var mismatch RepairResult
	if err := s.recoverPartial(&r, &mismatch); err == nil {
		t.Fatal("record mismatch accepted")
	}
	_ = os.Remove(s.paths.partial(r.ID, "worktree"))

	// Exercise the repository identity and persistence faults after the evidence is valid.
	root2 := initEffortRepo(t)
	s2 := openEffortService(t, root2, time.Now())
	r2, _ := s2.New("faults", false)
	b2, _ := resolvePartial(t.Context(), root2, "HEAD")
	p2 := s2.paths.managedWorktree(r2.ID)
	if err := os.MkdirAll(filepath.Dir(p2), 0o700); err != nil {
		t.Fatal(err)
	}
	runEffortGit(t, "-C", root2, "worktree", "add", "-b", "awf/"+r2.ID, p2, b2)
	e2 := PartialEvidence{SchemaVersion: 1, EffortID: r2.ID, Action: "worktree", Base: b2, Branch: "awf/" + r2.ID, Path: p2, CommonDir: filepath.Join(root2, "foreign")}
	if err := s2.RecordPartial(e2); err != nil {
		t.Fatal(err)
	}
	s2.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
		return []awfgit.WorktreeRegistration{{Path: p2, Branch: "refs/heads/awf/" + r2.ID, HEAD: b2}}, nil
	}
	s2.paths.roots.CommonDir = filepath.Join(root2, "foreign")
	var rr RepairResult
	if err := s2.recoverPartial(&r2, &rr); err == nil {
		t.Fatal("common identity fault hidden")
	}
}

func TestPartialRecoveryRefusesAmbiguousWorktreeEvidence(t *testing.T) {
	cases := []struct {
		name string
		regs func(string, string, string) ([]awfgit.WorktreeRegistration, error)
	}{
		{"listing error", func(string, string, string) ([]awfgit.WorktreeRegistration, error) {
			return nil, errors.New("list fault")
		}},
		{"duplicate path", func(path, branch, base string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{{Path: path, Branch: "refs/heads/other", HEAD: base}, {Path: path, Branch: branch, HEAD: base}}, nil
		}},
		{"branch elsewhere", func(path, branch, base string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{{Path: path, Branch: branch, HEAD: base}, {Path: path + "-other", Branch: branch, HEAD: base}}, nil
		}},
		{"wrong registration", func(path, branch, base string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{{Path: path, Branch: "refs/heads/other", HEAD: base}}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			s := openEffortService(t, root, time.Now())
			r, err := s.New("ambiguous", false)
			if err != nil {
				t.Fatal(err)
			}
			base, err := resolvePartial(t.Context(), root, "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			path := s.paths.managedWorktree(r.ID)
			e := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "worktree", Base: base, Branch: "awf/" + r.ID, Path: path, CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
			if err := s.RecordPartial(e); err != nil {
				t.Fatal(err)
			}
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return tc.regs(path, "refs/heads/"+e.Branch, base)
			}
			var result RepairResult
			if err := s.recoverPartial(&r, &result); err == nil {
				t.Fatal("ambiguous evidence accepted")
			}
		})
	}
}

func TestPartialRecoveryIntegrationAndRemovalPolicies(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	r, err := s.New("policy", false)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := resolvePartial(t.Context(), root, "HEAD")
	path := s.paths.managedWorktree(r.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+r.ID, path, base)
	r, err = s.AttachWorktree(r.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	writeEffortFile(t, filepath.Join(path, "tip"), "tip\n")
	runEffortGit(t, "-C", path, "add", "tip")
	runEffortGit(t, "-C", path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "tip")
	tip, _ := resolvePartial(t.Context(), path, "HEAD")
	common := filepath.Clean(s.paths.roots.CommonDir)
	bad := []PartialEvidence{
		{SchemaVersion: 1, EffortID: r.ID, Action: "integration", Branch: "awf/" + r.ID, CommonDir: common, Tip: tip, TargetPath: root + "-foreign", TargetBranch: "main", Integration: IntegrationMerge},
		{SchemaVersion: 1, EffortID: r.ID, Action: "integration", Branch: "awf/" + r.ID, CommonDir: common, Tip: tip, TargetPath: root, TargetBranch: "wrong", Integration: IntegrationMerge},
	}
	for _, e := range bad {
		if err := s.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		var result RepairResult
		if err := s.recoverPartial(&r, &result); err == nil {
			t.Fatal("unsafe integration evidence accepted")
		}
		_ = os.Remove(s.paths.partial(r.ID, "integration"))
	}
	branchOut, err := gitPartial(t.Context(), root, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	good := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "integration", Branch: "awf/" + r.ID, CommonDir: common, Tip: tip, TargetPath: root, TargetBranch: strings.TrimSpace(string(branchOut)), Integration: IntegrationMerge}
	if err := s.RecordPartial(good); err != nil {
		t.Fatal(err)
	}
	var divergent RepairResult
	if err := s.recoverPartial(&r, &divergent); err == nil {
		t.Fatal("integration ancestry refusal missing")
	}
	runEffortGit(t, "-C", root, "merge", "--ff-only", "awf/"+r.ID)
	var result RepairResult
	if err := s.recoverPartial(&r, &result); err != nil {
		t.Fatal(err)
	}
	if r.Integration != IntegrationMerge {
		t.Fatalf("disposition=%s", r.Integration)
	}
}

func TestPartialRecoveryRemovalFaults(t *testing.T) {
	makeCase := func(t *testing.T, mutate func(*Service, *Record, string)) {
		t.Helper()
		root := initEffortRepo(t)
		s := openEffortService(t, root, time.Now())
		r, err := s.New("removal fault", false)
		if err != nil {
			t.Fatal(err)
		}
		base, _ := resolvePartial(t.Context(), root, "HEAD")
		e := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "removal", Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), BranchTip: base}
		if err := s.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		oldB, oldR, oldG := partialBranches, partialResolveGit, partialGit
		defer func() { partialBranches, partialResolveGit, partialGit = oldB, oldR, oldG }()
		mutate(s, &r, base)
		var result RepairResult
		if err := s.recoverPartial(&r, &result); err == nil {
			t.Fatal("removal fault hidden")
		}
	}
	makeCase(t, func(s *Service, _ *Record, _ string) {
		s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, errors.New("list") }
	})
	makeCase(t, func(s *Service, _ *Record, _ string) {
		partialBranches = func(context.Context, string, string) (bool, error) { return false, errors.New("branches") }
	})
	makeCase(t, func(s *Service, _ *Record, _ string) { s.store.fs = &faultFileSystem{fail: "create-temp"} })
	makeCase(t, func(s *Service, _ *Record, _ string) { s.store.fs = &faultFileSystem{failRemove: true} })
	oldBranches, oldResolve, oldGit := partialBranches, partialResolveGit, partialGit
	defer func() { partialBranches, partialResolveGit, partialGit = oldBranches, oldResolve, oldGit }()
	makeCase(t, func(_ *Service, _ *Record, _ string) {
		partialBranches = func(context.Context, string, string) (bool, error) { return true, nil }
		partialResolveGit = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("resolve") }
	})
	makeCase(t, func(_ *Service, _ *Record, base string) {
		partialBranches = func(context.Context, string, string) (bool, error) { return true, nil }
		partialResolveGit = func(context.Context, string, ...string) ([]byte, error) { return []byte(base + "\n"), nil }
		partialGit = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("git mutation") }
	})
}

func TestPartialRecoveryInjectedBranches(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	r, err := s.New("injected", false)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := resolvePartial(t.Context(), root, "HEAD")
	oldG, oldA, oldB, oldR := partialGit, partialAncestor, partialBranches, partialResolveGit
	defer func() { partialGit, partialAncestor, partialBranches, partialResolveGit = oldG, oldA, oldB, oldR }()
	partialGit = func(context.Context, string, ...string) ([]byte, error) { return []byte("main\n"), nil }
	integration := func() PartialEvidence {
		return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "integration", Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), Tip: base, TargetPath: filepath.Clean(root), TargetBranch: "main", Integration: IntegrationMerge}
	}
	for _, fault := range []func(){func() {
		partialAncestor = func(context.Context, string, string, string) (bool, error) { return false, errors.New("ancestor") }
	}, func() {
		partialGit = func(context.Context, string, ...string) ([]byte, error) { return []byte("main\n"), nil }
		partialAncestor = func(context.Context, string, string, string) (bool, error) { return true, nil }
	}} {
		e := integration()
		if err := s.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		var rr RepairResult
		fault()
		if err := s.recoverPartial(&r, &rr); err == nil {
			t.Fatal("integration fault hidden")
		}
		_ = os.Remove(s.paths.partial(r.ID, "integration"))
		partialGit = oldG
		partialAncestor = oldA
	}
	remove := func() PartialEvidence {
		return PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "removal", Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), BranchTip: base}
	}
	for _, setup := range []func(){func() {
		s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{{Path: "/other", Branch: "refs/heads/awf/" + r.ID}}, nil
		}
	}, func() {
		s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
			return []awfgit.WorktreeRegistration{{Path: s.paths.managedWorktree(r.ID), Branch: "wrong"}}, nil
		}
		_ = os.WriteFile(s.paths.managedWorktree(r.ID), []byte("x"), 0600)
	}, func() {
		s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		_ = os.MkdirAll(s.paths.managedWorktree(r.ID), 0700)
	}} {
		e := remove()
		if err := s.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		setup()
		var rr RepairResult
		_ = s.recoverPartial(&r, &rr)
		_ = os.Remove(s.paths.partial(r.ID, "removal"))
		_ = os.RemoveAll(s.paths.managedWorktree(r.ID))
	}
}

func TestPartialRecoveryRemovalAndRecordBranches(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Service, string, string)
	}{
		{"directory truth", func(s *Service, path, _ string) {
			_ = os.RemoveAll(path)
			_ = os.WriteFile(path, []byte("x"), 0600)
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) {
				return []awfgit.WorktreeRegistration{{Path: path, Branch: "refs/heads/awf/" + filepath.Base(path)}}, nil
			}
		}},
		{"remove git", func(s *Service, _, _ string) {
			partialGit = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("remove") }
		}},
		{"branch changed", func(s *Service, _, _ string) {
			partialBranches = func(context.Context, string, string) (bool, error) { return true, nil }
			partialResolveGit = func(context.Context, string, ...string) ([]byte, error) {
				return []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"), nil
			}
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		}},
		{"clear", func(s *Service, _, _ string) {
			s.store.fs = nil
			partialClear = func(_ store, _ string, _ string) error { return errors.New("clear") }
			partialBranches = func(context.Context, string, string) (bool, error) { return false, nil }
			s.worktrees = func(context.Context, string) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			s := openEffortService(t, root, time.Now())
			r, err := s.New("removal", false)
			if err != nil {
				t.Fatal(err)
			}
			base, _ := resolvePartial(t.Context(), root, "HEAD")
			path := s.paths.managedWorktree(r.ID)
			_ = os.MkdirAll(filepath.Dir(path), 0700)
			runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+r.ID, path, base)
			r, _ = s.AttachWorktree(r.ID, base)
			if tc.name != "directory truth" && tc.name != "remove git" {
				runEffortGit(t, "-C", root, "worktree", "remove", path)
			}
			e := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "removal", Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), BranchTip: base}
			if err := s.RecordPartial(e); err != nil {
				t.Fatal(err)
			}
			oldG, oldB, oldR, oldC := partialGit, partialBranches, partialResolveGit, partialClear
			defer func() { partialGit, partialBranches, partialResolveGit, partialClear = oldG, oldB, oldR, oldC }()
			tc.mutate(s, path, base)
			var rr RepairResult
			recoveryErr := s.recoverPartial(&r, &rr)
			if tc.name == "clear" && (recoveryErr == nil || !strings.Contains(recoveryErr.Error(), "settle recovered")) {
				t.Fatalf("clear fault = %v", recoveryErr)
			}
			if recoveryErr == nil {
				t.Fatal("fault hidden")
			}
		})
	}
}

func TestPartialRecoveryForcedRemovalIsDurable(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	r, err := s.New("forced removal", false)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := resolvePartial(t.Context(), root, "HEAD")
	path := s.paths.managedWorktree(r.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+r.ID, path, base)
	r, err = s.AttachWorktree(r.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	writeEffortFile(t, filepath.Join(path, "unmerged"), "x\n")
	runEffortGit(t, "-C", path, "add", "unmerged")
	runEffortGit(t, "-C", path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "unmerged")
	tip, _ := resolvePartial(t.Context(), root, "awf/"+r.ID)
	e := PartialEvidence{SchemaVersion: 1, EffortID: r.ID, Action: "removal", Branch: "awf/" + r.ID, CommonDir: filepath.Clean(s.paths.roots.CommonDir), DeleteForce: true, BranchTip: tip}
	if err := s.RecordPartial(e); err != nil {
		t.Fatal(err)
	}
	var result RepairResult
	if err := s.recoverPartial(&r, &result); err != nil {
		t.Fatal(err)
	}
	if r.Worktree != nil || r.Integration != IntegrationNone {
		t.Fatalf("forced removal = %#v", r)
	}
	if _, err := resolvePartial(t.Context(), root, "awf/"+r.ID); err == nil {
		t.Fatal("forced branch remains")
	}
}

func TestPartialEvidenceDurabilityAndCorruption(t *testing.T) {
	root := initEffortRepo(t)
	s := openEffortService(t, root, time.Now())
	base, err := resolvePartial(t.Context(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	e := PartialEvidence{SchemaVersion: 1, EffortID: idA, Action: "worktree", Base: base, Branch: "awf/" + idA, Path: filepath.Clean(s.paths.managedWorktree(idA)), CommonDir: filepath.Clean(s.paths.roots.CommonDir)}
	s.store.fs = &faultFileSystem{fail: "publish"}
	if err := s.RecordPartial(e); !errors.Is(err, errInjectedDurability) {
		t.Fatalf("publish fault = %v", err)
	}
	s.store.fs = &faultFileSystem{fail: "create-temp"}
	if err := s.RecordPartial(e); !errors.Is(err, errInjectedDurability) {
		t.Fatalf("publish fault = %v", err)
	}
	s.store.fs = nil
	path := s.paths.partial(idA, "worktree")
	for _, raw := range []string{"{} {}", `{"schemaVersion":1,"effortId":"bad"}`} {
		writeEffortFile(t, path, raw)
		if _, _, err := s.store.getPartial(idA, "worktree"); err == nil {
			t.Fatalf("corrupt evidence accepted: %q", raw)
		}
		_ = os.Remove(path)
	}
	if err := s.RecordPartial(e); err != nil {
		t.Fatal(err)
	}
	s.store.fs = &faultFileSystem{failRemove: true}
	if err := s.ClearPartial(idA, "worktree"); !errors.Is(err, errInjectedCleanup) {
		t.Fatalf("cleanup fault = %v", err)
	}
	_ = os.Remove(path)
	for _, stage := range []string{"open-directory", "fsync-directory"} {
		s.store.fs = nil
		if err := s.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		s.store.fs = &faultFileSystem{fail: stage}
		if err := s.ClearPartial(idA, "worktree"); !errors.Is(err, errInjectedDurability) {
			t.Fatalf("%s fault = %v", stage, err)
		}
	}
}

func TestPartialEvidenceRestartRecovery(t *testing.T) {
	now := time.Date(2026, 7, 27, 5, 0, 0, 0, time.UTC)
	fresh := func(t *testing.T, root string) *Service { t.Helper(); return openEffortService(t, root, now) }
	t.Run("worktree attachment", func(t *testing.T) {
		root := initEffortRepo(t)
		service := fresh(t, root)
		record, err := service.New("partial", false)
		if err != nil {
			t.Fatal(err)
		}
		base, err := resolvePartial(t.Context(), root, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		path := service.paths.managedWorktree(record.ID)
		e := PartialEvidence{SchemaVersion: 1, EffortID: record.ID, Action: "worktree", Base: base, Branch: "awf/" + record.ID, Path: path, CommonDir: filepath.Clean(service.paths.roots.CommonDir)}
		if err := service.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", e.Branch, path, base)
		repaired, err := fresh(t, root).Repair(record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if repaired.Record.Worktree == nil || repaired.Record.Worktree.Base != base || repaired.Record.Integration != IntegrationPending {
			t.Fatalf("attachment recovery = %#v", repaired.Record)
		}
		if _, err := os.Lstat(service.paths.partial(record.ID, "worktree")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("attachment evidence remains: %v", err)
		}
	})
	t.Run("integration", func(t *testing.T) {
		root := initEffortRepo(t)
		service := fresh(t, root)
		record, err := service.New("partial", false)
		if err != nil {
			t.Fatal(err)
		}
		base, _ := resolvePartial(t.Context(), root, "HEAD")
		path := service.paths.managedWorktree(record.ID)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+record.ID, path, base)
		if _, err = service.AttachWorktree(record.ID, base); err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, filepath.Join(path, "tip"), "tip\n")
		runEffortGit(t, "-C", path, "add", "tip")
		runEffortGit(t, "-C", path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "tip")
		tip, err := resolvePartial(t.Context(), path, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "merge", "--ff-only", "awf/"+record.ID)
		branchOut, err := gitPartial(t.Context(), root, "symbolic-ref", "-q", "--short", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		e := PartialEvidence{SchemaVersion: 1, EffortID: record.ID, Action: "integration", Branch: "awf/" + record.ID, CommonDir: filepath.Clean(service.paths.roots.CommonDir), Tip: tip, TargetPath: filepath.Clean(root), TargetBranch: strings.TrimSpace(string(branchOut)), Integration: IntegrationFastForward}
		if err := service.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		repaired, err := fresh(t, root).Repair(record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if repaired.Record.Integration != IntegrationFastForward {
			t.Fatalf("integration recovery=%#v", repaired.Record)
		}
		if _, err := os.Lstat(service.paths.partial(record.ID, "integration")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("integration evidence remains: %v", err)
		}
	})
	t.Run("removal", func(t *testing.T) {
		root := initEffortRepo(t)
		service := fresh(t, root)
		record, err := service.New("partial", false)
		if err != nil {
			t.Fatal(err)
		}
		base, _ := resolvePartial(t.Context(), root, "HEAD")
		path := service.paths.managedWorktree(record.ID)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "add", "-b", "awf/"+record.ID, path, base)
		if _, err = service.AttachWorktree(record.ID, base); err != nil {
			t.Fatal(err)
		}
		if _, err = service.SetIntegration(record.ID, IntegrationFastForward); err != nil {
			t.Fatal(err)
		}
		tip, err := resolvePartial(t.Context(), root, "awf/"+record.ID)
		if err != nil {
			t.Fatal(err)
		}
		e := PartialEvidence{SchemaVersion: 1, EffortID: record.ID, Action: "removal", Branch: "awf/" + record.ID, CommonDir: filepath.Clean(service.paths.roots.CommonDir), BranchTip: tip}
		if err := service.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		runEffortGit(t, "-C", root, "worktree", "remove", path)
		freshService := fresh(t, root)
		originalPartialBranches := partialBranches
		foreign := initEffortRepo(t)
		partialBranches = func(ctx context.Context, checkout, branchName string) (bool, error) {
			present, runErr := originalPartialBranches(ctx, checkout, branchName)
			if present {
				freshService.paths.roots.CommonDir = filepath.Join(foreign, ".git")
			}
			return present, runErr
		}
		_, err = freshService.Repair(record.ID)
		partialBranches = originalPartialBranches
		if err == nil || !strings.Contains(err.Error(), "live control-root identity changed") {
			t.Fatalf("swapped branch deletion accepted: %v", err)
		}
		freshService = fresh(t, root)
		freshService = fresh(t, root)
		repaired, err := freshService.Repair(record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if repaired.Record.Worktree != nil || repaired.Record.Integration != IntegrationFastForward {
			t.Fatalf("removal recovery=%#v", repaired.Record)
		}
		if _, err := resolvePartial(t.Context(), root, "awf/"+record.ID); err == nil {
			t.Fatal("managed branch remains")
		}
		if _, err := os.Lstat(service.paths.partial(record.ID, "removal")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removal evidence remains: %v", err)
		}
	})
	t.Run("corrupt and mismatched evidence refuse", func(t *testing.T) {
		root := initEffortRepo(t)
		service := fresh(t, root)
		record, err := service.New("partial", false)
		if err != nil {
			t.Fatal(err)
		}
		writeEffortFile(t, service.paths.partial(record.ID, "worktree"), "{")
		if _, err := fresh(t, root).Repair(record.ID); err == nil {
			t.Fatal("corrupt evidence accepted")
		}
		_ = os.Remove(service.paths.partial(record.ID, "worktree"))
		base, _ := resolvePartial(t.Context(), root, "HEAD")
		e := PartialEvidence{SchemaVersion: 1, EffortID: record.ID, Action: "worktree", Base: base, Branch: "awf/" + record.ID, Path: service.paths.managedWorktree(record.ID), CommonDir: filepath.Join(root, "foreign")}
		if err := service.RecordPartial(e); err != nil {
			t.Fatal(err)
		}
		_, err = fresh(t, root).Repair(record.ID)
		var hard *awfgit.HardSafetyError
		if !errors.As(err, &hard) || hard.Category != "repository-identity" {
			t.Fatalf("mismatched evidence error=%v", err)
		}
	})
}
