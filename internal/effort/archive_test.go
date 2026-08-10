package effort

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// invariant: tooling/effort-management:effort-record-authority (TestOpaqueScratchMovesUnchangedWithoutTraversal)
func TestOpaqueScratchMovesUnchangedWithoutTraversal(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	record, err := service.New(testContext(t), NewInput{Slug: "opaque-scratch", Title: "Opaque scratch"})
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(root, ".awf", "efforts", record.Slug, "scratch")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(scratch, "unreadable")
	if err := os.Mkdir(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) })
	if err := os.Symlink("missing-target", filepath.Join(scratch, "opaque-link")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Show(record.Slug); err != nil {
		t.Fatalf("show traversed or rejected opaque scratch: %v", err)
	}
	if listed, err := service.List(); err != nil || len(listed) != 1 {
		t.Fatalf("list traversed or rejected opaque scratch: %#v, %v", listed, err)
	}
	result, err := service.Finish(testContext(t), record.Slug)
	if err != nil || !result.Archived || result.DestinationSyncAvailable != directorySyncAvailable() || result.SourceSyncAvailable != directorySyncAvailable() || result.DestinationSynced != result.DestinationSyncAvailable || result.SourceSynced != result.SourceSyncAvailable {
		t.Fatalf("finish result=%#v err=%v", result, err)
	}
	archivedScratch := filepath.Join(root, filepath.FromSlash(result.ArchivePath), "scratch")
	if _, err := os.Lstat(filepath.Join(archivedScratch, "opaque-link")); err != nil {
		t.Fatalf("opaque symlink was not preserved: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(archivedScratch, "unreadable")); err != nil || info.Mode().Perm() != 0 {
		t.Fatalf("opaque unreadable directory changed: %v, %v", info, err)
	}
	if listed, err := service.List(); err != nil || len(listed) != 0 {
		t.Fatalf("archive leaked into active inventory: %#v, %v", listed, err)
	}
}

func TestScratchBoundaryRejectsUnsafeShapesWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*testing.T, string)
	}{
		{"regular file", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("opaque"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink", func(t *testing.T, path string) {
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) { noTopology(deps); deps.UUID = func() (string, error) { return testIDA, nil } })
			record, err := service.New(testContext(t), NewInput{Slug: "unsafe-scratch", Title: "Unsafe scratch"})
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, ".awf", "efforts", record.Slug)
			tc.build(t, filepath.Join(dir, "scratch"))
			before, err := os.ReadFile(filepath.Join(dir, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Finish(testContext(t), record.Slug); err == nil {
				t.Fatal("unsafe scratch boundary was accepted")
			}
			after, err := os.ReadFile(filepath.Join(dir, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("refusal mutated resident bytes")
			}
		})
	}
}

func TestFinishRefusesActiveArchiveCollisionWithoutMutation(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	record, err := service.New(testContext(t), NewInput{Slug: "active-collision", Title: "Active collision"})
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, ".awf", "effort-archive", record.ID+"-"+record.Slug)
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(testContext(t), record.Slug)
	var partial *PartialFinishError
	wantActions := []RecoveryAction{
		{Text: "inspect " + filepath.Join(root, ".awf", "efforts", record.Slug)},
		{Text: "inspect " + destination},
		{Text: "preserve both residents and resolve the collision manually before retrying"},
	}
	if err == nil || result.State != FinishStateActive || result.Archived || !errors.As(err, &partial) || partial.Result != result || !reflect.DeepEqual(partial.Actions, wantActions) {
		t.Fatalf("collision result=%#v err=%v partial=%#v, want actions %#v", result, err, partial, wantActions)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", record.Slug)); err != nil {
		t.Fatalf("active resident changed: %v", err)
	}
}

func TestFinishReportsExpectedMarkerCompositionFailure(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.ExpectedArchiveMarker = func() ([]byte, error) { return nil, errors.New("render marker") }
	})
	if _, err := service.Finish(testContext(t), "missing"); err == nil || !strings.Contains(err.Error(), "render expected") {
		t.Fatalf("marker renderer error = %v", err)
	}
}

func TestFinishValidatesMarkerAndRootBeforeActiveMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"missing marker", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, ".awf", "effort-archive", ".gitignore")); err != nil {
				t.Fatal(err)
			}
		}},
		{"stale marker", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".awf", "effort-archive", ".gitignore"), []byte("stale\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"regular file root", func(t *testing.T, root string) {
			archive := filepath.Join(root, ".awf", "effort-archive")
			if err := os.RemoveAll(archive); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archive, []byte("unsafe"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsafe root permissions", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, ".awf", "effort-archive"), 0o777); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlinked root", func(t *testing.T, root string) {
			archive := filepath.Join(root, ".awf", "effort-archive")
			if err := os.RemoveAll(archive); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), archive); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := initEffortRepo(t)
			service := openTestService(t, root, func(deps *Dependencies) { noTopology(deps); deps.UUID = func() (string, error) { return testIDA, nil } })
			if _, err := service.New(testContext(t), NewInput{Slug: "marker-guard", Title: "Marker guard"}); err != nil {
				t.Fatal(err)
			}
			tc.mutate(t, root)
			if result, err := service.Finish(testContext(t), "marker-guard"); err == nil || result != (FinishResult{}) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if _, err := os.Stat(filepath.Join(root, ".awf", "efforts", "marker-guard")); err != nil {
				t.Fatalf("active resident changed: %v", err)
			}
		})
	}
}

func TestFinishReleasesSlugAndPreservesActivity(t *testing.T) {
	root := initEffortRepo(t)
	ids := []string{testIDA, testIDB}
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { id := ids[0]; ids = ids[1:]; return id, nil }
	})
	first, err := service.New(testContext(t), NewInput{Slug: "reuse-finished", Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	activity := []byte(`{"schemaVersion":2,"owner":"128f47a0-7b3d-4c52-8f1a-123456789abc","attachedAt":"2026-08-10T00:00:00Z","heartbeatAt":"2026-08-10T00:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(root, ".awf", "efforts", first.Slug, "activity.json"), activity, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(testContext(t), first.Slug)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(result.ArchivePath), "activity.json"))
	if err != nil || string(archived) != string(activity) {
		t.Fatalf("activity=%q err=%v", archived, err)
	}
	second, err := service.New(testContext(t), NewInput{Slug: first.Slug, Title: "Second"})
	if err != nil || second.ID != testIDB {
		t.Fatalf("reused record=%#v err=%v", second, err)
	}
}

func TestFinishReservationCorruptionAndPublicationRacePreserveBytes(t *testing.T) {
	t.Run("missing reservation memory", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openTestService(t, root, func(deps *Dependencies) {
			noTopology(deps)
			deps.UUID = func() (string, error) { return testIDA, nil }
			deps.Fault = func(stage string) error {
				if stage == "finish.archive" {
					return errors.New("retain")
				}
				return nil
			}
		})
		record, err := service.New(testContext(t), NewInput{Slug: "missing-reservation-memory", Title: "Missing memory"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Finish(testContext(t), record.Slug); err == nil {
			t.Fatal("archive fault ignored")
		}
		reservation := filepath.Join(root, ".awf", "efforts", tombstoneName(record))
		if err := os.Remove(filepath.Join(reservation, "memory.md")); err != nil {
			t.Fatal(err)
		}
		service.store.fault = nil
		if result, err := service.Finish(testContext(t), record.Slug); err == nil || result.State != FinishStateReserved {
			t.Fatalf("corrupt reservation result=%#v err=%v", result, err)
		}
	})

	t.Run("reservation name identity mismatch", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openTestService(t, root, func(deps *Dependencies) {
			noTopology(deps)
			deps.UUID = func() (string, error) { return testIDA, nil }
			deps.Fault = func(stage string) error {
				if stage == "finish.archive" {
					return errors.New("retain")
				}
				return nil
			}
		})
		record, err := service.New(testContext(t), NewInput{Slug: "mismatched-reservation", Title: "Mismatch"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Finish(testContext(t), record.Slug); err == nil {
			t.Fatal("archive fault ignored")
		}
		from := filepath.Join(root, ".awf", "efforts", tombstoneName(record))
		to := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-"+record.Slug)
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		service.store.fault = nil
		if result, err := service.Finish(testContext(t), record.Slug); err == nil || result != (FinishResult{}) || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("mismatched reservation discovery result=%#v err=%v", result, err)
		}
		if result, err := service.archiveReservation(record.Slug, to, newFinishResult(FinishStateReserved, false, "")); err == nil || result.State != FinishStateReserved || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("mismatched reservation publication result=%#v err=%v", result, err)
		}
	})

	t.Run("destination appears at publication", func(t *testing.T) {
		root := initEffortRepo(t)
		var destination string
		service := openTestService(t, root, func(deps *Dependencies) {
			noTopology(deps)
			deps.UUID = func() (string, error) { return testIDA, nil }
			deps.Fault = func(stage string) error {
				if stage == "finish.archive" {
					return os.Mkdir(destination, 0o700)
				}
				return nil
			}
		})
		record, err := service.New(testContext(t), NewInput{Slug: "publication-race", Title: "Publication race"})
		if err != nil {
			t.Fatal(err)
		}
		destination = filepath.Join(root, ".awf", "effort-archive", record.ID+"-"+record.Slug)
		result, err := service.Finish(testContext(t), record.Slug)
		var partial *PartialFinishError
		source := filepath.Join(root, ".awf", "efforts", tombstoneName(record))
		wantActions := []RecoveryAction{
			{Text: "inspect " + source},
			{Text: "inspect " + destination},
			{Text: "resolve the destination collision or filesystem boundary before retrying"},
		}
		if err == nil || result.State != FinishStateReserved || result.Archived || result.SourceSynced != directorySyncAvailable() || !errors.As(err, &partial) || partial.Result != result || !reflect.DeepEqual(partial.Actions, wantActions) {
			t.Fatalf("publication race result=%#v err=%v partial=%#v, want actions %#v", result, err, partial, wantActions)
		}
		if _, err := os.Stat(source); err != nil {
			t.Fatalf("reservation changed: %v", err)
		}
	})
}

func TestRollbackCreationRemovalFailureRetainsReservation(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
		deps.RemoveTree = func(string) error { return errors.New("remove") }
	})
	record, err := service.New(testContext(t), NewInput{Slug: "rollback-remove", Title: "Rollback remove"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RollbackCreation(testContext(t), record)
	if err == nil || !result.Reserved || result.Removed {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestRollbackCreationFaultBoundariesRetainTypedState(t *testing.T) {
	for _, stage := range []string{"rollback.rename", "rollback.root-fsync", "rollback.delete", "rollback.delete-fsync"} {
		t.Run(stage, func(t *testing.T) {
			root := initEffortRepo(t)
			fault := errors.New("fault")
			service := openTestService(t, root, func(deps *Dependencies) {
				noTopology(deps)
				deps.UUID = func() (string, error) { return testIDA, nil }
				deps.Fault = func(got string) error {
					if got == stage {
						return fault
					}
					return nil
				}
			})
			record, err := service.New(testContext(t), NewInput{Slug: "rollback-fault", Title: "Rollback fault"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.RollbackCreation(testContext(t), record)
			if err == nil || !errors.Is(err, fault) {
				t.Fatalf("rollback fault identity lost: %v", err)
			}
			if stage == "rollback.root-fsync" && !strings.Contains(err.Error(), "after rollback reservation") {
				t.Fatalf("reservation sync boundary absent: %v", err)
			}
			if stage == "rollback.delete-fsync" && !strings.Contains(err.Error(), "after rollback deletion") {
				t.Fatalf("deletion sync boundary absent: %v", err)
			}
			if stage == "rollback.rename" && result != (RollbackResult{}) {
				t.Fatalf("pre-reservation result=%#v", result)
			}
			if stage != "rollback.rename" && !result.Reserved {
				t.Fatalf("post-reservation result=%#v", result)
			}
			if stage == "rollback.delete-fsync" && !result.Removed {
				t.Fatalf("post-delete result=%#v", result)
			}
		})
	}
}

func TestRollbackCreationRejectsCorruptResident(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) {
		noTopology(deps)
		deps.UUID = func() (string, error) { return testIDA, nil }
	})
	record, err := service.New(testContext(t), NewInput{Slug: "corrupt-rollback", Title: "Corrupt rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".awf", "efforts", record.Slug, "memory.md")); err != nil {
		t.Fatal(err)
	}
	if result, err := service.RollbackCreation(testContext(t), record); err == nil || result != (RollbackResult{}) {
		t.Fatalf("corrupt rollback result=%#v err=%v", result, err)
	}
}

func TestRollbackCreationRequiresExactIdentityAndAbsentTopology(t *testing.T) {
	root := initEffortRepo(t)
	service := openTestService(t, root, func(deps *Dependencies) { noTopology(deps); deps.UUID = func() (string, error) { return testIDA, nil } })
	record, err := service.New(testContext(t), NewInput{Slug: "creation-rollback", Title: "Creation rollback"})
	if err != nil {
		t.Fatal(err)
	}
	foreign := record
	foreign.ID = testIDB
	if result, err := service.RollbackCreation(testContext(t), foreign); err == nil || result != (RollbackResult{}) {
		t.Fatalf("foreign result=%#v err=%v", result, err)
	}
	if _, err := service.Show(record.Slug); err != nil {
		t.Fatalf("identity refusal changed resident: %v", err)
	}
	service.worktrees = func(context.Context) ([]awfgit.WorktreeRegistration, error) {
		return nil, errors.New("ambiguous topology")
	}
	if result, err := service.RollbackCreation(testContext(t), record); err == nil || result != (RollbackResult{}) || !strings.Contains(err.Error(), "registrations") {
		t.Fatalf("ambiguous result=%#v err=%v", result, err)
	}
	if _, err := service.Show(record.Slug); err != nil {
		t.Fatalf("ambiguous topology changed resident: %v", err)
	}
	noTopologyDeps := func(context.Context) ([]awfgit.WorktreeRegistration, error) { return nil, nil }
	service.worktrees = noTopologyDeps
	result, err := service.RollbackCreation(testContext(t), record)
	if err != nil || !result.Reserved || !result.Removed {
		t.Fatalf("rollback=%#v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "effort-archive", record.ID+"-"+record.Slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed creation was archived: %v", err)
	}
}
