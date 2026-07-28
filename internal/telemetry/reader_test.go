package telemetry

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

func telemetryRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "telemetry repository")
	telemetryGit(t, "init", root)
	return root
}

func telemetryGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func telemetryWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func telemetryStream(id string, observations ...[]byte) string {
	lines := []string{string(testHeaderRaw(id))}
	for _, observation := range observations {
		lines = append(lines, string(observation))
	}
	return strings.Join(lines, "\n") + "\n"
}

func findingCodes(values []IntegrityFinding) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.Source + "/" + value.SessionID + "/" + value.Code
	}
	return out
}

// invariant: tooling/workflow-telemetry:privacy-integrity-and-retention
func TestSessionReaderRejectsBadHeader(t *testing.T) {
	if _, err := ValidateHeader([]byte(`{"record":"header","schemaVersion":1,"sessionId":"x","createdAt":"bad"}`)); err == nil {
		t.Fatal("accepted bad header")
	}
}

func TestReadSessionCorruptionFindingsAndOrdering(t *testing.T) {
	dir := t.TempDir()
	id := "session"
	validFirst := testObservationRaw("123e4567-e89b-42d3-a456-426614174001", "2026-07-27T00:00:00Z", "usage", `{"inputTokens":1,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0,"costUsd":0}`)
	validSecond := testObservationRaw("123e4567-e89b-42d3-a456-426614174002", "2026-07-27T00:00:01Z", "tool", `{"tool":"go","outcome":"success","durationMs":1}`)
	validSameTime := testObservationRaw("123e4567-e89b-42d3-a456-426614174000", "2026-07-27T00:00:00Z", "compaction", `{"inputTokensBefore":1,"inputTokensAfter":0}`)
	path := filepath.Join(dir, id+".jsonl")
	telemetryWrite(t, path, telemetryStream(id, validSecond, validFirst, validSameTime, validFirst, []byte{}, []byte("broken")))
	read := readSession(path, id)
	if read.Header.SessionID != id || len(read.Records) != 4 || len(read.Observations) != 3 {
		t.Fatalf("read session = %#v", read)
	}
	if got := []string{read.Observations[0].ObservationID, read.Observations[1].ObservationID, read.Observations[2].ObservationID}; !reflect.DeepEqual(got, []string{"123e4567-e89b-42d3-a456-426614174000", "123e4567-e89b-42d3-a456-426614174001", "123e4567-e89b-42d3-a456-426614174002"}) {
		t.Fatalf("observation order = %v", got)
	}
	if got := findingCodes(read.Findings); !reflect.DeepEqual(got, []string{"session-v1/session/duplicate-observation-id", "session-v1/session/malformed-record", "session-v1/session/malformed-record"}) {
		t.Fatalf("session findings = %v", got)
	}
	for _, tc := range []struct {
		name, body string
		code       string
	}{
		{"missing-final-lf", string(testHeaderRaw(id)), "missing-final-lf"},
		{"missing-header", "\n", "missing-header"},
		{"invalid-header", "bad\n", "invalid-header"},
		{"identity-mismatch", telemetryStream("other"), "header-identity-mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := filepath.Join(dir, tc.name+".jsonl")
			telemetryWrite(t, candidate, tc.body)
			got := readSession(candidate, id)
			if len(got.Findings) != 1 || got.Findings[0].Code != tc.code {
				t.Fatalf("%s = %#v", tc.name, got)
			}
		})
	}
	if got := readSession(filepath.Join(dir, "missing.jsonl"), id); len(got.Findings) != 1 || got.Findings[0].Code != "unsafe-stream-path" {
		t.Fatalf("missing path = %#v", got)
	}
	if got := readSession("/proc/self/mem", id); len(got.Findings) != 1 || got.Findings[0].Code != "read-failure" {
		t.Fatalf("unreadable regular path = %#v", got)
	}
}

func TestReadLegacyDiscoveryAndFindings(t *testing.T) {
	absent, findings, err := readLegacy(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(absent) != 0 || len(findings) != 0 {
		t.Fatalf("absent legacy = %#v %#v %v", absent, findings, err)
	}
	file := filepath.Join(t.TempDir(), "file")
	telemetryWrite(t, file, "not a directory")
	if _, _, err := readLegacy(file); err == nil {
		t.Fatal("readLegacy accepted a file root")
	}
	root := filepath.Join(t.TempDir(), "legacy")
	telemetryWrite(t, filepath.Join(root, "effort-b", "effort.json"), "{\"effort\":\"b\"}\n")
	telemetryWrite(t, filepath.Join(root, "effort-b", "sessions", "session-b.jsonl"), "{\"b\":1}\n\n{\"b\":2}\n")
	telemetryWrite(t, filepath.Join(root, "effort-b", "sessions", "session-a.jsonl"), "{\"a\":1}\n")
	if err := os.MkdirAll(filepath.Join(root, "effort-b", "sessions", "bad.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/proc/self/mem", filepath.Join(root, "effort-b", "sessions", "unreadable.jsonl")); err != nil {
		t.Fatal(err)
	}
	telemetryWrite(t, filepath.Join(root, "effort-a", "sessions", "stream.jsonl"), "{\"a\":2}\n")
	telemetryWrite(t, filepath.Join(root, "not-a-directory"), "x")
	reads, findings, err := readLegacy(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{reads[0].EffortID, reads[1].EffortID}; !reflect.DeepEqual(got, []string{"effort-a", "effort-b"}) {
		t.Fatalf("legacy order = %v", got)
	}
	if got := []string{string(reads[1].Records[0]), string(reads[1].Records[1]), string(reads[1].Records[2]), string(reads[1].Records[3])}; !reflect.DeepEqual(got, []string{"{\"effort\":\"b\"}", "{\"a\":1}", "{\"b\":1}", "{\"b\":2}"}) {
		t.Fatalf("legacy records = %v", got)
	}
	if got := findingCodes(findings); !reflect.DeepEqual(got, []string{"legacy-protocol-1/bad/unsafe-legacy-stream", "legacy-protocol-1/unreadable/read-failure", "legacy/not-a-directory/unsafe-legacy-entry"}) {
		t.Fatalf("legacy findings = %v", got)
	}
}

func TestReadDiscoversResidentDataAndReadErrors(t *testing.T) {
	if _, err := Read(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Read accepted a non-repository")
	}
	root := telemetryRepo(t)
	svc, err := effort.Open(t.Context(), root, effort.Options{})
	if err != nil {
		t.Fatal(err)
	}
	record, err := svc.New("Telemetry discovery", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Assign(record.ID, "session-a"); err != nil {
		t.Fatal(err)
	}
	metrics := filepath.Join(root, ".awf", "metrics")
	telemetryWrite(t, filepath.Join(metrics, "sessions", "session-a.jsonl"), telemetryStream("session-a", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":1,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0,"costUsd":0}`)))
	if err := os.Mkdir(filepath.Join(metrics, "sessions", "directory.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	telemetryWrite(t, filepath.Join(metrics, "sessions", ".jsonl"), "bad\n")
	if err := os.Symlink(filepath.Join(metrics, "sessions", "session-a.jsonl"), filepath.Join(metrics, "sessions", "unsafe.jsonl")); err != nil {
		t.Fatal(err)
	}
	telemetryWrite(t, filepath.Join(metrics, "efforts", record.ID, "effort.json"), "{\"legacy\":true}\n")
	telemetryWrite(t, filepath.Join(metrics, "efforts", record.ID, "sessions", "legacy-session.jsonl"), "{\"kind\":\"usage_observed\",\"payload\":{\"inputTokens\":2}}\n")
	reads, err := Read(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if reads.Records[record.ID].Title != "Telemetry discovery" || reads.Assignments["session-a"] != record.ID {
		t.Fatalf("joined effort state = %#v", reads)
	}
	if got := []string{reads.Sessions[0].SessionID, reads.Sessions[1].SessionID}; !reflect.DeepEqual(got, []string{"session-a", "unsafe"}) {
		t.Fatalf("session discovery = %v", got)
	}
	if len(reads.Legacy) != 1 || reads.Legacy[0].EffortID != record.ID {
		t.Fatalf("legacy discovery = %#v", reads.Legacy)
	}
	wantFindings := []string{
		"session-v1//unsafe-stream-entry",
		"session-v1/directory/unsafe-stream-entry",
		"session-v1/unsafe/unsafe-stream-path",
	}
	if got := findingCodes(reads.Findings); !reflect.DeepEqual(got, wantFindings) {
		t.Fatalf("discovery findings = %v", got)
	}

	for _, tc := range []struct {
		name string
		make func(string)
	}{
		{"effort-open", func(root string) {
			if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), filepath.Join(root, ".awf", "memory")); err != nil {
				t.Fatal(err)
			}
		}},
		{"effort-list", func(root string) {
			telemetryWrite(t, filepath.Join(root, ".awf", "efforts", "not-a-record.json"), "{}")
		}},
		{"assignments", func(root string) {
			svc, err := effort.Open(t.Context(), root, effort.Options{})
			if err != nil {
				t.Fatal(err)
			}
			created, err := svc.New("Assignment failure", false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Assign(created.ID, "session"); err != nil {
				t.Fatal(err)
			}
			telemetryWrite(t, filepath.Join(root, ".awf", "assignments", "sessions.json"), "{")
		}},
		{"sessions", func(root string) {
			telemetryWrite(t, filepath.Join(root, ".awf", "metrics", "sessions"), "not a directory")
		}},
		{"legacy", func(root string) {
			telemetryWrite(t, filepath.Join(root, ".awf", "metrics", "efforts"), "not a directory")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := telemetryRepo(t)
			tc.make(candidate)
			if _, err := Read(t.Context(), candidate); err == nil {
				t.Fatal("Read accepted injected resident failure")
			}
		})
	}
}

func TestReaderServiceAndLegacySafetySeams(t *testing.T) {
	root := telemetryRepo(t)
	listFailure := errors.New("list failure")
	t.Run("list", func(t *testing.T) {
		original := readEffortList
		readEffortList = func(*effort.Service) ([]effort.Record, error) { return nil, listFailure }
		t.Cleanup(func() { readEffortList = original })
		if _, err := Read(t.Context(), root); !errors.Is(err, listFailure) {
			t.Fatalf("list seam error = %v", err)
		}
	})
	assignmentFailure := errors.New("assignment failure")
	t.Run("assignments", func(t *testing.T) {
		original := readEffortAssignments
		readEffortAssignments = func(*effort.Service) ([]effort.Assignment, error) { return nil, assignmentFailure }
		t.Cleanup(func() { readEffortAssignments = original })
		if _, err := Read(t.Context(), root); !errors.Is(err, assignmentFailure) {
			t.Fatalf("assignment seam error = %v", err)
		}
	})
	t.Run("legacy-directory", func(t *testing.T) {
		metrics := filepath.Join(root, ".awf", "metrics", "efforts", "legacy")
		if err := os.MkdirAll(metrics, 0o700); err != nil {
			t.Fatal(err)
		}
		original := inspectLegacyDirectory
		inspectLegacyDirectory = func(string) (os.FileInfo, error) { return nil, errors.New("unsafe directory") }
		t.Cleanup(func() { inspectLegacyDirectory = original })
		read, err := Read(t.Context(), root)
		if err != nil || !reflect.DeepEqual(findingCodes(read.Findings), []string{"legacy/legacy/unsafe-legacy-path"}) {
			t.Fatalf("legacy directory seam = %#v, %v", read, err)
		}
	})
}

func TestFindingSort(t *testing.T) {
	findings := []IntegrityFinding{{Source: "z", SessionID: "a", Code: "a"}, {Source: "a", SessionID: "z", Code: "a"}, {Source: "a", SessionID: "a", Code: "z"}, {Source: "a", SessionID: "a", Code: "a"}}
	sortFindings(findings)
	if got := findingCodes(findings); !reflect.DeepEqual(got, []string{"a/a/a", "a/a/z", "a/z/a", "z/a/a"}) {
		t.Fatalf("finding order = %v", got)
	}
	if got := finding("source", "session", "code"); got != (IntegrityFinding{Source: "source", SessionID: "session", Code: "code"}) {
		t.Fatalf("finding = %#v", got)
	}
	if !errors.Is(nil, nil) {
		t.Fatal("sanity")
	}
}
