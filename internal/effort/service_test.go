package effort

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

// noTopology is the answer set of a checkout that carries no managed worktree
// at all, which is the precondition finish requires.
func noTopology(deps *Dependencies) {
	deps.Worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
	deps.BranchExists = func(context.Context, string) (bool, error) { return false, nil }
}

func TestUpdateMemoryRefusesInvalidResidentsAndUnrepairedMetadata(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	value := "replacement"
	if err := service.UpdateMemory("bad_slug", MemoryUpdate{Phase: &value}); err == nil || !strings.Contains(err.Error(), "invalid effort slug") {
		t.Fatalf("invalid slug = %v", err)
	}
	if err := service.UpdateMemory("missing-effort", MemoryUpdate{Phase: &value}); err == nil {
		t.Fatal("missing resident accepted")
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "update-faults", Title: "Update faults"}); err != nil {
		t.Fatal(err)
	}
	memory := filepath.Join(root, ".awf", "efforts", "update-faults", "memory.md")
	if err := os.Remove(memory); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateMemory("update-faults", MemoryUpdate{Phase: &value}); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing memory = %v", err)
	}
	if err := os.WriteFile(memory, []byte("---\neffort: update-faults\nphase: old\nnext: old\nupdated: invalid\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateMemory("update-faults", MemoryUpdate{Phase: &value, Next: &value}); err != nil {
		t.Fatalf("canonical metadata refresh = %v", err)
	}
	raw, err := os.ReadFile(memory)
	if err != nil || !strings.Contains(string(raw), "updated: ") || strings.Contains(string(raw), "updated: invalid") {
		t.Fatalf("canonical metadata refresh bytes=%q err=%v", raw, err)
	}
}

func TestRefusalDiagnosticsPreserveErrorIdentityAndSeparateFacts(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, nil)
	_, err := service.Finish(testContext(t), "absent-finish")
	if err == nil || err.Error() != "effort \"absent-finish\" has no active resident or finishing reservation; changed bytes: no; next action: run `awf effort list` and use an active slug" {
		t.Fatalf("absent finish error = %v", err)
	}
	var refused *refusalError
	if !errors.As(err, &refused) {
		t.Fatalf("absent finish error lost refusal identity: %v", err)
	}
	assertRefusalDiagnostic(t, refused, "condition: effort \"absent-finish\" has no active resident or finishing reservation\nstate: resident\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: run `awf effort list` and use an active slug\n")

	_, err = service.New(testContext(t), NewInput{Slug: strings.Repeat("s", 33), Title: "Overlong slug"})
	if err == nil || !strings.Contains(err.Error(), "changed bytes: no") {
		t.Fatalf("overlong slug error = %v", err)
	}
	if !errors.As(err, &refused) {
		t.Fatalf("overlong slug error lost refusal identity: %v", err)
	}
	assertRefusalDiagnostic(t, refused, "condition: explicit effort slug \"sssssssssssssssssssssssssssssssss\" is invalid\nstate: input\ncause: slug must contain 1-32 bytes\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: provide a different canonical value with `--slug`\n")

	refusalCause := errors.New("refusal cause")
	if err := refusal("message", "condition", "state", "cause", nil, refusalCause); !errors.Is(err, refusalCause) {
		t.Fatalf("refusal lost cause identity: %v", err)
	}

	cause := errors.New("resident fault")
	corrupt := &CorruptError{Path: "resident", Err: cause}
	if !errors.Is(corrupt, cause) {
		t.Fatal("corrupt refusal lost cause identity")
	}
	assertRefusalDiagnostic(t, corrupt, "condition: effort resident is unusable\nstate: resident\ncause: resident: resident fault\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: preserve the resident and inspect it for manual cleanup\n")

	invalidMemory := &InvalidMemoryError{Slug: "demo", Err: cause, NextAction: "repair memory.md"}
	if !errors.Is(invalidMemory, cause) {
		t.Fatal("invalid memory refusal lost cause identity")
	}
	assertRefusalDiagnostic(t, invalidMemory, "condition: memory for effort \"demo\" cannot be updated\nstate: memory\ncause: resident fault\n\ndiagnostic:\n  changed:\n    bytes: no\n  steps:\n    step 1: repair memory.md\n")

	if _, err := recoverySteps([]RecoveryAction{{Text: "invalid\naction"}}); err == nil {
		t.Fatal("recovery steps accepted a multiline action")
	}
}

func assertRefusalDiagnostic(t *testing.T, err interface {
	Diagnostic() (presentation.Diagnostic, error)
}, want string) {
	t.Helper()
	diagnostic, diagnosticErr := err.Diagnostic()
	if diagnosticErr != nil {
		t.Fatal(diagnosticErr)
	}
	document, documentErr := diagnostic.Document()
	if documentErr != nil {
		t.Fatal(documentErr)
	}
	var out bytes.Buffer
	if renderErr := presentation.Render(&out, document); renderErr != nil {
		t.Fatal(renderErr)
	}
	if got := out.String(); got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

func TestFinishRenamesCleansAndRetries(t *testing.T) {
	// invariant: tooling/effort-management:effort-record-authority (TestFinishRenamesCleansAndRetries)
	root := initEffortRepo(t)
	failDelete := true
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.Clock = func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) }
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if stage == "finish.delete" && failDelete {
				failDelete = false
				return errors.New("interrupted")
			}
			return nil
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "restartable-finish", Title: "Restartable finish"}); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(testContext(t), "restartable-finish")
	var partial *PartialFinishError
	if err == nil || !result.Renamed || result.Cleaned || !errors.As(err, &partial) || !strings.Contains(partial.Cause.Error(), "finishing cleanup interrupted") || len(partial.Actions) != 1 || partial.Actions[0].Text != "retry `awf effort finish restartable-finish`" {
		t.Fatalf("first finish result=%#v err=%v partial=%#v", result, err, partial)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", "restartable-finish")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("active directory remains: %v", err)
	}
	result, err = service.Finish(testContext(t), "restartable-finish")
	if err != nil {
		t.Fatal(err)
	}
	if result.Renamed || !result.Cleaned {
		t.Fatalf("retry result = %#v", result)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("finish residue: %v", entries)
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

func TestFinishPreservesMismatchedAndMultipleTombstones(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if stage == "finish.delete" {
				return errors.New("stop")
			}
			return nil
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "foreign-tombstone", Title: "Foreign tombstone"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish(testContext(t), "foreign-tombstone"); err == nil {
		t.Fatal("finish unexpectedly cleaned tombstone")
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("tombstone entries=%v err=%v", entries, err)
	}
	original := filepath.Join(root, ".awf", "efforts", entries[0].Name())
	duplicate := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-foreign-tombstone")
	if err := copyResidentDirectory(original, duplicate); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(duplicate, "state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), testIDA, testIDB, 1))
	if err := os.WriteFile(statePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = service.Finish(testContext(t), "foreign-tombstone")
	if err == nil || !strings.Contains(err.Error(), "multiple finishing reservations") {
		t.Fatalf("multiple tombstone error = %v", err)
	}
	if _, statErr := os.Stat(original); statErr != nil {
		t.Fatalf("original tombstone changed: %v", statErr)
	}
	if _, statErr := os.Stat(duplicate); statErr != nil {
		t.Fatalf("duplicate tombstone changed: %v", statErr)
	}
}

func TestFinishFaultAndTopologyErrorBranches(t *testing.T) {
	for _, stage := range []string{"finish.rename", "finish.root-fsync", "finish.delete"} {
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
			if _, err := service.Finish(testContext(t), "finish-faults"); err == nil {
				t.Fatal("fault was ignored")
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

func TestFinishCleanupAndReservationBranches(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.Fault = func(stage string) error {
			if stage == "finish.delete" {
				return errors.New("retain tombstone")
			}
			return nil
		}
	})
	if _, err := service.New(testContext(t), NewInput{Slug: "reserved-finish", Title: "Reserved finish"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finish(testContext(t), "reserved-finish"); err == nil {
		t.Fatal("finish fault ignored")
	}
	if _, err := service.New(testContext(t), NewInput{Slug: "reserved-finish", Title: "Reserved finish"}); err == nil || !strings.Contains(err.Error(), "reserved by finishing") {
		t.Fatalf("reservation error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".awf", "efforts"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	tombstone := filepath.Join(root, ".awf", "efforts", entries[0].Name())
	failingRemoval := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.RemoveTree = func(string) error { return errors.New("remove fault") }
	})
	if _, err := failingRemoval.Finish(testContext(t), "reserved-finish"); err == nil || !strings.Contains(err.Error(), "remove fault") {
		t.Fatalf("remove error = %v", err)
	}
	if result, err := failingRemoval.cleanTombstone("reserved-finish", filepath.Join(root, ".awf", "efforts", "wrong"), false); err == nil || result != (FinishResult{}) {
		t.Fatalf("wrong tombstone result=%#v err=%v", result, err)
	}
	mismatched := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-reserved-finish")
	if err := os.Rename(tombstone, mismatched); err != nil {
		t.Fatal(err)
	}
	if result, err := failingRemoval.cleanTombstone("reserved-finish", mismatched, false); err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched tombstone result=%#v err=%v", result, err)
	}
	tombstone = mismatched
	if _, err := os.Stat(tombstone); err != nil {
		t.Fatalf("tombstone changed: %v", err)
	}
}

// TestServiceRefusesAMissingDependency proves the composition contract: each
// dependency is required, so a composition root that forgets one fails loudly
// at construction rather than silently running against a substituted default.
func TestServiceRefusesAMissingDependency(t *testing.T) {
	root := initEffortRepo(t)
	roots, complete := testWiring(t, root)
	for name, drop := range map[string]func(*Dependencies){
		"clock":        func(d *Dependencies) { d.Clock = nil },
		"UUID":         func(d *Dependencies) { d.UUID = nil },
		"worktree":     func(d *Dependencies) { d.Worktrees = nil },
		"branch":       func(d *Dependencies) { d.BranchExists = nil },
		"reference":    func(d *Dependencies) { d.ValidateRef = nil },
		"tree removal": func(d *Dependencies) { d.RemoveTree = nil },
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
