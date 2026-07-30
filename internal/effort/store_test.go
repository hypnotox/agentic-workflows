package effort

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testIDA = "018f47a0-7b3d-4c52-8f1a-123456789abc"
	testIDB = "128f47a0-7b3d-4c52-8f1a-123456789abc"
)

func TestEffortProtocol2CreateShowListAndCollision(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	ids := []string{testIDA, testIDB}
	service, err := Open(context.Background(), root, Options{
		Clock: func() time.Time { return now },
		UUID: func() (string, error) {
			if len(ids) == 0 {
				return testIDB, nil
			}
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	zeta, err := service.New("Zeta result")
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := service.New("Alpha result")
	if err != nil {
		t.Fatal(err)
	}
	if zeta.Slug != "zeta-result" || alpha.Slug != "alpha-result" {
		t.Fatalf("unexpected slugs: %#v %#v", zeta, alpha)
	}
	shown, err := service.Show("zeta-result")
	if err != nil {
		t.Fatal(err)
	}
	if shown != zeta {
		t.Fatalf("show = %#v, want %#v", shown, zeta)
	}
	listed, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Slug != "alpha-result" || listed[1].Slug != "zeta-result" {
		t.Fatalf("list is not sorted by slug: %#v", listed)
	}
	resident := filepath.Join(root, ".awf", "efforts", "zeta-result")
	entries, err := os.ReadDir(resident)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "memory.md,state.json" {
		t.Fatalf("resident leaves = %v", names)
	}
	memory, err := os.ReadFile(filepath.Join(resident, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"Effort: zeta-result\n", "Phase:", "Next:", "Updated:", "## Brief", "## Decision log", "## Observations", "## Handoff log"} {
		if !strings.Contains(string(memory), phrase) {
			t.Fatalf("memory skeleton missing %q:\n%s", phrase, memory)
		}
	}
	if _, err := service.New("Zeta result"); err == nil || !strings.Contains(err.Error(), "collides") || !strings.Contains(err.Error(), "changed bytes: no") {
		t.Fatalf("collision error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".awf", "efforts", ".lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("effort lock exists: %v", err)
	}
}

func TestCreationPublicationFaultOrderAndIncompleteEnumeration(t *testing.T) {
	stages := []string{
		"reserve.directory",
		"memory.write", "memory.fsync", "memory.rename", "memory.directory-fsync",
		"state.write", "state.fsync", "state.rename", "state.directory-fsync",
		"efforts-root.fsync",
	}
	for _, failStage := range stages {
		t.Run(failStage, func(t *testing.T) {
			root := initEffortRepo(t)
			var seen []string
			service, err := Open(context.Background(), root, Options{
				Clock: func() time.Time { return time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC) },
				UUID:  func() (string, error) { return testIDA, nil },
				Fault: func(stage string) error {
					seen = append(seen, stage)
					if stage == failStage {
						return errors.New("stop")
					}
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.New("Fault matrix"); err == nil {
				t.Fatal("creation succeeded at injected failure")
			}
			wantPrefix := stages[:indexOfStage(t, stages, failStage)+1]
			if strings.Join(seen, ",") != strings.Join(wantPrefix, ",") {
				t.Fatalf("stages = %v, want prefix %v", seen, wantPrefix)
			}
			listed, listErr := service.List()
			statePublished := indexOfStage(t, stages, failStage) >= indexOfStage(t, stages, "state.directory-fsync")
			if statePublished {
				if listErr != nil || len(listed) != 1 {
					t.Fatalf("published state must enumerate: list=%#v err=%v", listed, listErr)
				}
			} else if listErr != nil || len(listed) != 0 {
				t.Fatalf("incomplete directory must be ignored: list=%#v err=%v", listed, listErr)
			}
			assertNoEffortTemporaries(t, filepath.Join(root, ".awf", "efforts", "fault-matrix"))

			// Recreating the same slug must name which reservation blocks it: an
			// incomplete one is a different condition, and a different repair,
			// from an active effort.
			_, retryErr := service.New("Fault matrix")
			if retryErr == nil {
				t.Fatal("recreation over an existing reservation succeeded")
			}
			wantCondition := "an incomplete reservation exists"
			if statePublished {
				wantCondition = "an active effort already exists"
			}
			if !strings.Contains(retryErr.Error(), wantCondition) {
				t.Fatalf("recreation error = %v, want condition %q", retryErr, wantCondition)
			}
		})
	}
}

func TestConcurrentSameSlugCreationHasOneWinner(t *testing.T) {
	root := initEffortRepo(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{testIDA, testIDB} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			service, err := Open(context.Background(), root, Options{UUID: func() (string, error) { return id, nil }})
			if err == nil {
				_, err = service.New("One winner")
			}
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	var successes, failures int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case strings.Contains(err.Error(), "collides"):
			failures++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("successes=%d failures=%d", successes, failures)
	}
}

func TestEnumerationPreservesAndDiagnosesForeignResidents(t *testing.T) {
	tests := map[string]func(*testing.T, string){
		"invalid entry": func(t *testing.T, root string) {
			writeEffortFile(t, filepath.Join(root, ".awf", "efforts", "foreign.json"), "foreign")
		},
		"symlink": func(t *testing.T, root string) {
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(root, ".awf", "efforts", "linked-effort")); err != nil {
				t.Fatal(err)
			}
		},
		"non-directory fifo": func(t *testing.T, root string) {
			// A fifo names a well-formed slug, so only the leaf type rejects it.
			// Enumeration must diagnose it without opening the pipe and blocking.
			if err := testMkfifo(filepath.Join(root, ".awf", "efforts", "fifo-effort"), 0o600); err != nil {
				t.Skipf("fifo fixture unavailable: %v", err)
			}
		},
		"published state missing memory": func(t *testing.T, root string) {
			dir := filepath.Join(root, ".awf", "efforts", "missing-memory")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeEffortFile(t, filepath.Join(dir, "state.json"), `{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"missing-memory","title":"Missing memory","createdAt":"2026-07-29T12:00:00Z"}`)
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			root := initEffortRepo(t)
			setup(t, root)
			service, err := Open(context.Background(), root, Options{})
			if err != nil {
				t.Fatal(err)
			}
			// The message clauses are template text, so preservation is asserted
			// against the real bytes rather than against the wording.
			before := snapshotEffortsTree(t, root)
			_, err = service.List()
			if err == nil || !strings.Contains(err.Error(), "changed bytes: no") || !strings.Contains(err.Error(), "preserve") {
				t.Fatalf("diagnostic = %v", err)
			}
			if after := snapshotEffortsTree(t, root); !reflect.DeepEqual(before, after) {
				t.Fatalf("refused enumeration changed bytes:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

func TestProtocol2ValidationAndEnumerationBranches(t *testing.T) {
	if got := (&CorruptError{Err: os.ErrInvalid}).Unwrap(); !errors.Is(got, os.ErrInvalid) {
		t.Fatalf("unwrap = %v", got)
	}
	if _, err := normalizeTitle(string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8 title accepted")
	}
	for _, slug := range []string{"", strings.Repeat("a", 64), "bad_slug", "bad..slug"} {
		if err := validateSlug(slug); err == nil {
			t.Errorf("invalid slug %q accepted", slug)
		}
	}
	base := persistedRecord{SchemaVersion: 2, ID: testIDA, Slug: "valid-slug", Title: "Valid slug", CreatedAt: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)}
	invalid := []persistedRecord{
		{SchemaVersion: 1, ID: base.ID, Slug: base.Slug, Title: base.Title, CreatedAt: base.CreatedAt},
		{SchemaVersion: 2, ID: "bad", Slug: base.Slug, Title: base.Title, CreatedAt: base.CreatedAt},
		{SchemaVersion: 2, ID: base.ID, Slug: "other", Title: base.Title, CreatedAt: base.CreatedAt},
		{SchemaVersion: 2, ID: base.ID, Slug: "bad_slug", Title: base.Title, CreatedAt: base.CreatedAt},
		{SchemaVersion: 2, ID: base.ID, Slug: base.Slug, Title: " padded ", CreatedAt: base.CreatedAt},
		{SchemaVersion: 2, ID: base.ID, Slug: base.Slug, Title: base.Title},
	}
	invalidSlug := base
	invalidSlug.Slug = "bad_slug"
	if err := validatePersisted(invalidSlug, "bad_slug"); err == nil {
		t.Fatal("persisted invalid slug accepted")
	}
	for index, record := range invalid {
		if err := validatePersisted(record, base.Slug); err == nil {
			t.Errorf("invalid persisted record %d accepted: %#v", index, record)
		}
	}
	decoder := json.NewDecoder(strings.NewReader(`{} {}`))
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := requireJSONEOF(decoder); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
	decoder = json.NewDecoder(strings.NewReader(`{} nope`))
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := requireJSONEOF(decoder); err == nil {
		t.Fatal("malformed trailing JSON accepted")
	}
	for _, name := range []string{"plain", finishingPrefix + "bad", finishingPrefix + testIDA + "-bad_slug"} {
		if _, _, ok := parseTombstoneName(name); ok {
			t.Errorf("malformed tombstone %q accepted", name)
		}
	}

	t.Run("empty list without resident root", func(t *testing.T) {
		root := initEffortRepo(t)
		if err := os.RemoveAll(filepath.Join(root, ".awf", "efforts")); err != nil {
			t.Fatal(err)
		}
		service, err := Open(context.Background(), root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		listed, err := service.List()
		if err != nil || len(listed) != 0 {
			t.Fatalf("list=%v err=%v", listed, err)
		}
	})

	for name, setup := range map[string]func(*testing.T, string){
		"foreign leaf": func(t *testing.T, root string) {
			service := openEffortService(t, root, time.Now().UTC())
			if _, err := service.New("Foreign leaf"); err != nil {
				t.Fatal(err)
			}
			writeEffortFile(t, filepath.Join(root, ".awf", "efforts", "foreign-leaf", "extra"), "x")
		},
		"mismatched memory": func(t *testing.T, root string) {
			service := openEffortService(t, root, time.Now().UTC())
			if _, err := service.New("Wrong memory"); err != nil {
				t.Fatal(err)
			}
			writeEffortFile(t, filepath.Join(root, ".awf", "efforts", "wrong-memory", "memory.md"), "Effort: other\n")
		},
		"non-directory effort": func(t *testing.T, root string) {
			writeEffortFile(t, filepath.Join(root, ".awf", "efforts", "regular-file"), "x")
		},
		"trailing state JSON": func(t *testing.T, root string) {
			dir := filepath.Join(root, ".awf", "efforts", "trailing-state")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeEffortFile(t, filepath.Join(dir, "state.json"), `{"schemaVersion":2,"id":"018f47a0-7b3d-4c52-8f1a-123456789abc","slug":"trailing-state","title":"Trailing state","createdAt":"2026-07-29T12:00:00Z"} {}`)
			writeEffortFile(t, filepath.Join(dir, "memory.md"), "Effort: trailing-state\n")
		},
		"malformed finishing name": func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, ".awf", "efforts", ".finishing-bad"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := initEffortRepo(t)
			setup(t, root)
			service, err := Open(context.Background(), root, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.List(); err == nil {
				t.Fatal("corrupt resident accepted")
			}
		})
	}

	t.Run("list skips tracked marker and valid tombstone", func(t *testing.T) {
		root := initEffortRepo(t)
		writeEffortFile(t, filepath.Join(root, ".awf", "efforts", ".gitignore"), "*")
		service := openEffortService(t, root, time.Now().UTC())
		if _, err := service.New("Listed tombstone"); err != nil {
			t.Fatal(err)
		}
		active := filepath.Join(root, ".awf", "efforts", "listed-tombstone")
		tombstone := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDA+"-listed-tombstone")
		if err := os.Rename(active, tombstone); err != nil {
			t.Fatal(err)
		}
		listed, err := service.List()
		if err != nil || len(listed) != 0 {
			t.Fatalf("list=%v err=%v", listed, err)
		}
	})

	t.Run("unsafe reserve and list authority", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o000); err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Unreadable reserve"); err == nil {
			t.Fatal("unreadable reserve accepted")
		}
		if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o777); err != nil {
			t.Fatal(err)
		}
		if _, err := service.New("Unsafe reserve"); err == nil {
			t.Fatal("unsafe reserve accepted")
		}
		if err := os.Chmod(filepath.Join(root, ".awf", "efforts"), 0o700); err != nil {
			t.Fatal(err)
		}
		service.store.paths.roots.PrimaryRoot = "relative"
		if _, err := service.List(); err == nil {
			t.Fatal("invalid list authority accepted")
		}
	})

	t.Run("invalid finishing resident", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		name := finishingPrefix + testIDA + "-bad-finish"
		if err := os.Symlink(t.TempDir(), filepath.Join(root, ".awf", "efforts", name)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.List(); err == nil {
			t.Fatal("symlinked finishing resident accepted")
		}
	})

	t.Run("mismatched finishing state", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		if _, err := service.New("Mismatched finish"); err != nil {
			t.Fatal(err)
		}
		active := filepath.Join(root, ".awf", "efforts", "mismatched-finish")
		mismatch := filepath.Join(root, ".awf", "efforts", finishingPrefix+testIDB+"-mismatched-finish")
		if err := os.Rename(active, mismatch); err != nil {
			t.Fatal(err)
		}
		if _, err := service.store.findTombstones("mismatched-finish"); err == nil {
			t.Fatal("mismatched finishing state accepted")
		}
	})

	t.Run("direct missing state and tombstone root", func(t *testing.T) {
		root := initEffortRepo(t)
		service := openEffortService(t, root, time.Now().UTC())
		dir := filepath.Join(root, ".awf", "efforts", "missing-state")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := service.store.loadDirectory(dir, "missing-state", true); err == nil {
			t.Fatal("missing state accepted")
		}
		if err := os.RemoveAll(filepath.Join(root, ".awf", "efforts")); err != nil {
			t.Fatal(err)
		}
		if found, err := service.store.findTombstones("missing-state"); err != nil || len(found) != 0 {
			t.Fatalf("tombstones=%v err=%v", found, err)
		}
	})
}

func indexOfStage(t *testing.T, stages []string, want string) int {
	t.Helper()
	for index, stage := range stages {
		if stage == want {
			return index
		}
	}
	t.Fatalf("stage %q not found", want)
	return -1
}

// snapshotEffortsTree records every resident leaf under the efforts root by
// mode, with contents for regular files. Non-regular leaves are recorded by
// mode alone so a fifo fixture is never opened.
func snapshotEffortsTree(t *testing.T, root string) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".awf", "efforts")
	out := map[string]string{}
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil { // coverage-ignore: every walked path is rooted at base by construction
			return err
		}
		if !info.Mode().IsRegular() {
			out[rel] = info.Mode().String()
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil { // coverage-ignore: the walk just proved this readable regular leaf exists
			return err
		}
		out[rel] = info.Mode().String() + ":" + string(content)
		return nil
	})
	if err != nil { // coverage-ignore: the efforts root is an owned readable directory in every fixture
		t.Fatalf("snapshot efforts tree: %v", err)
	}
	return out
}

// assertNoEffortTemporaries proves publication cleaned up after itself: a
// leaked staging file would otherwise sit undetected inside a reserved
// directory that enumeration deliberately ignores.
func assertNoEffortTemporaries(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil { // coverage-ignore: the reservation created this owned readable directory
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("publication left temporary %s", filepath.Join(dir, entry.Name()))
		}
	}
}

func openEffortService(t *testing.T, root string, now time.Time) *Service {
	t.Helper()
	service, err := Open(context.Background(), root, Options{Clock: func() time.Time { return now }, UUID: func() (string, error) { return testIDA, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func initEffortRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runEffortGit(t, "init", root)
	writeEffortFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	runEffortGit(t, "-C", root, "add", "tracked.txt")
	runEffortGit(t, "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	if err := os.MkdirAll(filepath.Join(root, ".awf", "efforts"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func runEffortGit(t *testing.T, args ...string) {
	t.Helper()
	_ = runEffortGitOutput(t, args...)
}

func runEffortGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeEffortFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
