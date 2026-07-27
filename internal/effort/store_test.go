package effort

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	idA = "11111111-1111-4111-8111-111111111111"
	idB = "22222222-2222-4222-8222-222222222222"
)

// invariant: tooling/effort-management:effort-record-authority
func TestEffortRecordAuthorityLifecycleListingCollisionAndMemory(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC)
	ids := []string{idB, idB, idA}
	service, err := Open(t.Context(), root, Options{
		Clock: func() time.Time { return now },
		UUID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.New("  Useful outcome  ", true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != idB || first.Title != "Useful outcome" || !first.MemoryPresent || first.State != StateActive || first.Integration != IntegrationNone {
		t.Fatalf("first record = %#v", first)
	}
	second, err := service.New("Another outcome", false)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != idA || second.MemoryPresent {
		t.Fatalf("collision retry record = %#v", second)
	}
	listed, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{listed[0].ID, listed[1].ID}; !reflect.DeepEqual(got, []string{idA, idB}) {
		t.Fatalf("list order = %v", got)
	}

	renamed, err := service.Rename(idA, "Renamed display")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != idA || renamed.Title != "Renamed display" || !renamed.UpdatedAt.After(renamed.CreatedAt) {
		t.Fatalf("rename = %#v", renamed)
	}
	completed, err := service.Complete(idA)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != StateCompleted || completed.MemoryPresent || completed.Worktree != nil {
		t.Fatalf("complete = %#v", completed)
	}
	reopened, err := service.Reopen(idA)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State != StateActive {
		t.Fatalf("reopen = %#v", reopened)
	}
	abandoned, err := service.Abandon(idA)
	if err != nil {
		t.Fatal(err)
	}
	if abandoned.State != StateAbandoned {
		t.Fatalf("abandon = %#v", abandoned)
	}
	path, withMemory, err := service.Memory(idA)
	if err != nil {
		t.Fatal(err)
	}
	if !withMemory.MemoryPresent || path != filepath.Join(root, ".awf", "memory", idA+".md") {
		t.Fatalf("memory result path=%q record=%#v", path, withMemory)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "Effort: "+idA+"\nRoute: unselected\nPhase: unselected\nWorkflow: unselected\nNext: ") {
		t.Fatalf("memory skeleton = %q", raw)
	}
}

func TestEffortExactSchemaLogicalAssignmentsAndValidation(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 7, 27, 0, 0, 0, 123, time.UTC)
	service := openEffortService(t, root, now)
	record, err := service.New("Schema contract", false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".awf", "efforts", idA+".json"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schemaVersion":1,"id":"11111111-1111-4111-8111-111111111111","title":"Schema contract","state":"active","createdAt":"2026-07-27T00:00:00.000000123Z","updatedAt":"2026-07-27T00:00:00.000000123Z","memoryPresent":false,"worktree":null,"integration":"none"}`
	if string(raw) != want {
		t.Fatalf("persisted JSON = %s\nwant = %s", raw, want)
	}
	if record.AssignedSessionIDs != nil {
		t.Fatalf("new assignments = %#v, want nil", record.AssignedSessionIDs)
	}
	var persistedKeys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &persistedKeys); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"createdAt", "id", "integration", "memoryPresent", "schemaVersion", "state", "title", "updatedAt", "worktree"}
	gotKeys := make([]string, 0, len(persistedKeys))
	for key := range persistedKeys {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("persisted authority fields = %v, want exactly %v", gotKeys, wantKeys)
	}
	assignmentDir := filepath.Join(root, ".awf", "assignments")
	if err := os.MkdirAll(assignmentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeEffortFile(t, filepath.Join(assignmentDir, "sessions.json"), `{"schemaVersion":1,"sessions":{"z-session":"`+idA+`","a-session":"`+idA+`"}}`)
	shown, err := service.Show(idA)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shown.AssignedSessionIDs, []string{"a-session", "z-session"}) {
		t.Fatalf("logical assignments = %v", shown.AssignedSessionIDs)
	}

	for name, title := range map[string]string{"blank": "   ", "too-long": strings.Repeat("a", 161), "invalid-utf8": string([]byte{0xff})} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeTitle(title); err == nil {
				t.Fatal("invalid title accepted")
			}
		})
	}
}

func TestEffortCorruptionSchemaPairsRepairAndAtomicReplacement(t *testing.T) {
	root := initEffortRepo(t)
	now := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	service := openEffortService(t, root, now)
	if _, err := service.New("Repair me", true); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(root, ".awf", "efforts", idA+".json")
	original, _ := os.ReadFile(recordPath)
	writeEffortFile(t, recordPath, `{"schemaVersion":2}`)
	if _, err := service.Show(idA); err == nil {
		t.Fatal("unsupported schema accepted")
	} else {
		var corrupt *CorruptError
		if !errors.As(err, &corrupt) {
			t.Fatalf("schema error = %T %v", err, err)
		}
	}
	still, _ := os.ReadFile(recordPath)
	if string(still) != `{"schemaVersion":2}` {
		t.Fatalf("corrupt input changed to %q", still)
	}
	writeEffortFile(t, recordPath, string(original))
	if err := os.Remove(filepath.Join(root, ".awf", "memory", idA+".md")); err != nil {
		t.Fatal(err)
	}
	repaired, err := service.Repair(idA)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired.Changes) != 1 || repaired.Changes[0].Field != "memoryPresent" || repaired.Record.MemoryPresent {
		t.Fatalf("repair = %#v", repaired)
	}

	for name, pair := range map[string]struct{ worktree, integration string }{
		"absent-none":          {"null", "none"},
		"absent-fast-forward":  {"null", "fast-forward"},
		"absent-merge":         {"null", "merge"},
		"absent-manual":        {"null", "manual"},
		"present-pending":      {worktreeJSON(now), "pending"},
		"present-fast-forward": {worktreeJSON(now), "fast-forward"},
		"present-merge":        {worktreeJSON(now), "merge"},
		"present-manual":       {worktreeJSON(now), "manual"},
	} {
		t.Run(name, func(t *testing.T) {
			raw := schemaRecordJSON(now, pair.worktree, pair.integration)
			writeEffortFile(t, recordPath, raw)
			if _, err := service.Show(idA); err != nil {
				t.Fatalf("legal pair refused: %v", err)
			}
		})
	}
	for name, pair := range map[string]struct{ worktree, integration string }{
		"absent-pending": {"null", "pending"},
		"present-none":   {worktreeJSON(now), "none"},
	} {
		t.Run(name, func(t *testing.T) {
			writeEffortFile(t, recordPath, schemaRecordJSON(now, pair.worktree, pair.integration))
			if _, err := service.Show(idA); err == nil {
				t.Fatal("illegal worktree/integration pair accepted")
			}
		})
	}

	writeEffortFile(t, recordPath, schemaRecordJSON(now, worktreeJSON(now), "pending"))
	completed, err := service.Complete(idA)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Worktree == nil || completed.Integration != IntegrationPending {
		t.Fatalf("completion discarded worktree authority: %#v", completed)
	}
	reopened, err := service.Reopen(idA)
	if err != nil {
		t.Fatal(err)
	}
	abandoned, err := service.Abandon(idA)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Worktree == nil || abandoned.Worktree == nil || abandoned.Integration != IntegrationPending {
		t.Fatalf("lifecycle discarded worktree authority: reopened=%#v abandoned=%#v", reopened, abandoned)
	}

	writeEffortFile(t, recordPath, schemaRecordJSON(now, "null", "none"))
	if err := atomicReplaceForTest(recordPath, []byte(schemaRecordJSON(now.Add(time.Second), "null", "none"))); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(filepath.Dir(recordPath), idB+".json")
	if err := atomicReplaceForTest(newPath, []byte("new")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(newPath); err != nil || string(got) != "new" {
		t.Fatalf("new atomic publication = %q, %v", got, err)
	}
	cleanup := filepath.Join(filepath.Dir(recordPath), ".remove-me")
	writeEffortFile(t, cleanup, "temp")
	if err := (osFileSystem{}).Remove(cleanup); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("record mode = %o", info.Mode().Perm())
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(recordPath), ".effort-*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("sibling temps remain: %v", matches)
	}
}

func TestEffortMemoryRefusesForeignExistingFile(t *testing.T) {
	root := initEffortRepo(t)
	service := openEffortService(t, root, time.Now().UTC())
	if _, err := service.New("No memory", false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "memory", idA+".md")
	writeEffortFile(t, path, "foreign\n")
	if _, _, err := service.Memory(idA); err == nil || !strings.Contains(err.Error(), "non-owned") {
		t.Fatalf("foreign memory error = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "foreign\n" {
		t.Fatalf("foreign memory changed to %q", got)
	}
}

func atomicReplaceForTest(path string, raw []byte) error {
	var expected *fileIdentity
	if identity, err := lstatRegular(path); err == nil {
		expected = &identity
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicReplaceFS(osFileSystem{}, path, raw, expected)
}

func openEffortService(t *testing.T, root string, now time.Time) *Service {
	t.Helper()
	service, err := Open(t.Context(), root, Options{Clock: func() time.Time { return now }, UUID: func() (string, error) { return idA, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func schemaRecordJSON(at time.Time, worktree, integration string) string {
	stamp := at.UTC().Format(time.RFC3339Nano)
	return fmt.Sprintf(`{"schemaVersion":1,"id":"%s","title":"Repair me","state":"active","createdAt":"%s","updatedAt":"%s","memoryPresent":false,"worktree":%s,"integration":"%s"}`, idA, stamp, stamp, worktree, integration)
}

func worktreeJSON(at time.Time) string {
	return fmt.Sprintf(`{"branch":"awf/%s","base":"%s","attachedAt":"%s"}`, idA, strings.Repeat("a", 40), at.UTC().Format(time.RFC3339Nano))
}

func initEffortRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "effort repository with spaces")
	runEffortGit(t, "init", root)
	writeEffortFile(t, filepath.Join(root, "tracked.txt"), "base\n")
	runEffortGit(t, "-C", root, "add", "tracked.txt")
	runEffortGit(t, "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	return root
}

func runEffortGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
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
