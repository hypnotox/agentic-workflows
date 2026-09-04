package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// noTopology is the answer set of a checkout that carries no managed worktree
// at all, which is the precondition finish requires.
func noTopology(deps *Dependencies) {
	deps.Worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
	deps.BranchExists = func(context.Context, string) (bool, error) { return false, nil }
}
func TestRefusalDiagnosticsPreserveErrorIdentityAndSeparateFacts(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	_, err := service.Finish(testContext(t), "absent-finish")
	if err == nil || err.Error() != "effort \"absent-finish\" has no active resident; changed bytes: no; next action: run `awf effort list` and use an active slug" {
		t.Fatalf("absent finish error = %v", err)
	}
	var refused *refusalError
	if !errors.As(err, &refused) {
		t.Fatalf("absent finish error lost refusal identity: %v", err)
	}
	assertDiagnosticInfo(t, refused, "effort \"absent-finish\" has no active resident", "resident", "", "bytes", "run `awf effort list` and use an active slug")

	_, err = service.New(testContext(t), NewInput{Slug: strings.Repeat("s", 33), Title: "Overlong slug"})
	if err == nil || !strings.Contains(err.Error(), "changed bytes: no") {
		t.Fatalf("overlong slug error = %v", err)
	}
	if !errors.As(err, &refused) {
		t.Fatalf("overlong slug error lost refusal identity: %v", err)
	}
	assertDiagnosticInfo(t, refused, "explicit effort slug \"sssssssssssssssssssssssssssssssss\" is invalid", "input", "slug must contain 1-32 bytes", "bytes", "provide a different canonical value with `--slug`")

	refusalCause := errors.New("refusal cause")
	if err := refusal("message", "condition", "state", "cause", nil, refusalCause); !errors.Is(err, refusalCause) {
		t.Fatalf("refusal lost cause identity: %v", err)
	}

	cause := errors.New("resident fault")
	corrupt := &CorruptError{Path: "resident", Err: cause}
	if !errors.Is(corrupt, cause) {
		t.Fatal("corrupt refusal lost cause identity")
	}
	assertDiagnosticInfo(t, corrupt, "effort resident is unusable", "resident", "resident: resident fault", "bytes", "preserve the resident and inspect it for manual cleanup")
}

func assertDiagnosticInfo(t *testing.T, err error, condition, state, cause, changed, action string) {
	t.Helper()
	info, ok := DiagnosticFor(err)
	if !ok {
		t.Fatalf("no effort diagnostic for %T", err)
	}
	if info.Condition != condition || info.State != state || info.Cause != cause || len(info.Changed) == 0 || info.Changed[0].Label != changed || len(info.Actions) == 0 || info.Actions[0].Text != action {
		t.Fatalf("diagnostic info = %#v", info)
	}
}

func TestFinishMoveFailureLeavesActiveResidentAndRetryConverges(t *testing.T) {
	// invariant: tooling/effort-management:effort-record-authority (TestFinishMoveFailureLeavesActiveResidentAndRetryConverges)
	root := initEffortRepo(t)
	failMove := true
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.Clock = func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) }
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if stage == "finish.move" && failMove {
				failMove = false
				return errors.New("interrupted")
			}
			return nil
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "restartable-finish", Title: "Restartable finish"}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(testContext(t), "restartable-finish")
	if err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "changed bytes: no") {
		t.Fatalf("first finish result=%#v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", "restartable-finish")); err != nil {
		t.Fatalf("active resident was not retained: %v", err)
	}
	result, err = service.Finish(testContext(t), "restartable-finish")
	if err != nil || result.ArchivePath == "" {
		t.Fatalf("retry result=%#v err=%v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", "restartable-finish")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active resident remains after success: %v", err)
	}
}

func TestFinishRefusesEveryManagedTopologyFact(t *testing.T) {
	tests := map[string]struct {
		setup         func(*testing.T, string)
		registrations []awfgit.WorktreeRegistration
		branch        bool
	}{
		"path": {
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".awf", "worktrees", "guarded-finish"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		"registration": {
			registrations: []awfgit.WorktreeRegistration{{Path: "/foreign", Branch: "refs/heads/awf/guarded-finish"}},
		},
		"branch": {branch: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) {
				deps.UUID = func() (string, error) { return testIDA, nil }
				deps.Worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
					return test.registrations, nil
				}
				deps.BranchExists = func(context.Context, string) (bool, error) { return test.branch, nil }
			})
			if _, err := service.New(testContext(t), NewInput{Slug: "guarded-finish", Title: "Guarded finish"}); err != nil {
				t.Fatal(err)
			}
			if test.setup != nil {
				test.setup(t, root)
			}
			result, err := service.Finish(testContext(t), "guarded-finish")
			if err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "changed bytes: no") || !strings.Contains(err.Error(), "worktree remove") {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if !errors.Is(err, ErrManagedTopologyPresent) {
				t.Fatalf("refusal %v is not classified as managed topology", err)
			}
			var refusal *managedTopologyError
			if !errors.As(err, &refusal) {
				t.Fatalf("refusal %v does not retain its typed recovery model", err)
			}
			wantActions := []RecoveryAction{{Text: "run `awf effort worktree remove guarded-finish`"}, {Text: "retry `awf effort finish guarded-finish`"}}
			if !slices.Equal(refusal.actions, wantActions) {
				t.Fatalf("%s recovery actions = %#v, want %#v", name, refusal.actions, wantActions)
			}
		})
	}
}

func TestFinishFaultAndTopologyErrorBranches(t *testing.T) {
	for _, stage := range []string{"finish.move"} {
		t.Run(stage, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) {
				noTopology(deps)
				deps.UUID = func() (string, error) { return testIDA, nil }
				deps.Fault = func(got string) error {
					if got == stage {
						return errors.New("injected")
					}
					return nil
				}
			})
			if _, err := service.New(testContext(t), NewInput{Slug: "finish-faults", Title: "Finish faults"}); err != nil {
				t.Fatal(err)
			}
			result, err := service.Finish(testContext(t), "finish-faults")
			if err == nil || result != (FinishResult{}) {
				t.Fatalf("fault result=%#v err=%v", result, err)
			}
		})
	}

	root := initEffortRepo(t)
	probeErr := func(err error) error {
		t.Helper()
		if errors.Is(err, ErrManagedTopologyPresent) {
			t.Fatalf("inspection failure %v was classified as a managed-topology refusal", err)
		}
		return err
	}
	registrationFault := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
			return nil, errors.New("registration probe")
		}
	})
	if _, err := registrationFault.New(testContext(t), NewInput{Slug: "probe-errors", Title: "Probe errors"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registrationFault.Finish(testContext(t), "probe-errors"); err == nil || !strings.Contains(probeErr(err).Error(), "registration") {
		t.Fatalf("registration error = %v", err)
	}
	branchFault := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
		deps.BranchExists = func(context.Context, string) (bool, error) { return false, errors.New("branch probe") }
	})
	if _, err := branchFault.Finish(testContext(t), "probe-errors"); err == nil || !strings.Contains(probeErr(err).Error(), "branch") {
		t.Fatalf("branch error = %v", err)
	}
	if _, err := branchFault.Finish(testContext(t), "bad_slug"); err == nil {
		t.Fatal("invalid finish slug accepted")
	}
	if _, err := branchFault.Finish(testContext(t), "missing-effort"); err == nil || !strings.Contains(err.Error(), "no active resident") {
		t.Fatalf("missing finish error = %v", err)
	}
	if yesNo(false) != "no" || yesNo(true) != "yes" {
		t.Fatal("yes/no rendering mismatch")
	}
}

func TestServiceResidentAndCorruptFinishBranches(t *testing.T) {
	root := initEffortRepo(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".awf", "worktrees")); err != nil {
		t.Fatal(err)
	}
	roots, deps := testWiring(t, root)
	if _, err := Open(roots, deps); err == nil {
		t.Fatal("symlinked worktrees root accepted")
	}

	root = initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o111); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish(testContext(t), "unreadable-finish"); err == nil {
		t.Fatal("unreadable finishing root accepted")
	}
	if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "corrupt-finish", Title: "Corrupt finish"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", "corrupt-finish", "memory.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish(testContext(t), "corrupt-finish"); err == nil {
		t.Fatal("corrupt active effort finished")
	}
}

// TestServiceRefusesAMissingDependency proves the composition contract: each
// dependency is required, so a composition root that forgets one fails loudly
// at construction rather than silently running against a substituted default.
func TestServiceRefusesAMissingDependency(t *testing.T) {
	root := initEffortRepo(t)
	roots, complete := testWiring(t, root)
	for name, drop := range map[string]func(*Dependencies){
		"clock":          func(d *Dependencies) { d.Clock = nil },
		"UUID":           func(d *Dependencies) { d.UUID = nil },
		"worktree":       func(d *Dependencies) { d.Worktrees = nil },
		"branch":         func(d *Dependencies) { d.BranchExists = nil },
		"reference":      func(d *Dependencies) { d.ValidateRef = nil },
		"archive marker": func(d *Dependencies) { d.ExpectedArchiveMarker = nil },
	} {
		t.Run(name, func(t *testing.T) {
			deps := complete
			drop(&deps)
			defer func() {
				recovered := recover()
				message, ok := recovered.(string)
				if !ok || !strings.Contains(message, name) {
					t.Fatalf("panic = %v, want one naming the %s dependency", recovered, name)
				}
			}()
			_, _ = Open(roots, deps)
			t.Fatal("missing dependency accepted")
		})
	}
}

// TestSlugMintingReportsAnUnusableRefName covers both ways the branch-name
// probe can refuse a slug: a broken probe is a fault, an invalid name is a
// repair instruction.
func TestSlugMintingReportsAnUnusableRefName(t *testing.T) {
	root := initEffortRepo(t)
	faulted := openTestService(t, root, func(deps *Dependencies) {
		deps.ValidateRef = func(context.Context, string) (bool, error) { return false, errors.New("probe fault") }
	})
	if _, err := faulted.New(testContext(t), NewInput{Slug: "probe-fault", Title: "Probe fault"}); err == nil || !strings.Contains(err.Error(), "probe fault") {
		t.Fatalf("probe fault error = %v", err)
	}
	refused := openTestService(t, root, func(deps *Dependencies) {
		deps.ValidateRef = func(context.Context, string) (bool, error) { return false, nil }
	})
	if _, err := refused.New(testContext(t), NewInput{Slug: "refused-name", Title: "Refused name"}); err == nil || !strings.Contains(err.Error(), "is not a valid Git ref") {
		t.Fatalf("invalid ref error = %v", err)
	}
}

func TestSlugMintingProbesTheExpectedBranchExactlyOnce(t *testing.T) {
	root := initEffortRepo(t)
	var probed []string
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.ValidateRef = func(_ context.Context, branch string) (bool, error) {
			probed = append(probed, branch)
			return true, nil
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "probe-once", Title: "Probe once"}); err != nil {
		t.Fatal(err)
	}
	if len(probed) != 1 || probed[0] != "awf/probe-once" {
		t.Fatalf("ref probes = %q, want exactly [awf/probe-once]", probed)
	}
}

func TestNewUsesExplicitIndependentIdentity(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	record, err := service.New(testContext(t), NewInput{Slug: "caller-selected", Title: "界🙂 independent title"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Slug != "caller-selected" || record.Title != "界🙂 independent title" || record.MemoryPath != ".awf/efforts/caller-selected/memory.md" {
		t.Fatalf("record = %#v", record)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "caller-selected", "state.json")); err != nil {
		t.Fatal(err)
	}
}

func TestNewRefusesInvalidInputBeforeAllocation(t *testing.T) {
	for _, test := range []struct {
		name  string
		input NewInput
		want  string
	}{
		{name: "blank title", input: NewInput{Slug: "valid-slug", Title: " "}, want: "outcome title"},
		{name: "overlong slug", input: NewInput{Slug: strings.Repeat("a", 33), Title: "Valid title"}, want: "1-32 bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := initEffortRepo(t)
			allocated := false
			service := openTestService(t, root, func(deps *Dependencies) {
				deps.UUID = func() (string, error) { allocated = true; return testIDA, nil }
			})
			_, err := service.New(testContext(t), test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "changed bytes: no") || allocated {
				t.Fatalf("New(%#v) err=%v allocated=%v", test.input, err, allocated)
			}
			entries, readErr := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("refusal changed residents: entries=%v err=%v", entries, readErr)
			}
		})
	}
}

func copyResidentDirectory(source, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"state.json", "memory.md"} {
		raw, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destination, name), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}
