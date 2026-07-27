package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/effort"
)

func TestEffortCommandsFromPrimaryAndLinkedWorktrees(t *testing.T) {
	primary := filepath.Join(t.TempDir(), "primary with spaces")
	commandGit(t, "init", primary)
	if err := os.WriteFile(filepath.Join(primary, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commandGit(t, "-C", primary, "add", "tracked.txt")
	commandGit(t, "-C", primary, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	linked := filepath.Join(filepath.Dir(primary), "linked with spaces")
	commandGit(t, "-C", primary, "worktree", "add", "--detach", linked, "HEAD")

	created := runEffortCommand(t, primary, "new", []string{"CLI outcome"}, map[string]bool{})
	fields := strings.Fields(created)
	if len(fields) < 2 || fields[0] != "effort" {
		t.Fatalf("new output = %q", created)
	}
	id := fields[1]
	if _, err := os.Stat(filepath.Join(primary, ".awf", "memory", id+".md")); err != nil {
		t.Fatalf("default memory: %v", err)
	}
	listed := runEffortCommand(t, linked, "list", nil, map[string]bool{})
	if !strings.Contains(listed, "effort "+id+" ") {
		t.Fatalf("linked list = %q", listed)
	}

	shownJSON := runEffortCommand(t, linked, "show", []string{id}, map[string]bool{"--json": true})
	var shown effort.Record
	if err := json.Unmarshal([]byte(shownJSON), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.SchemaVersion != 1 || shown.ID != id || shown.Title != "CLI outcome" {
		t.Fatalf("show JSON = %#v", shown)
	}
	renamed := runEffortCommand(t, linked, "rename", []string{id, "Renamed CLI outcome"}, nil)
	if !strings.Contains(renamed, `title="Renamed CLI outcome"`) {
		t.Fatalf("rename output = %q", renamed)
	}
	if completed := runEffortCommand(t, primary, "complete", []string{id}, nil); !strings.Contains(completed, "state=completed") {
		t.Fatalf("complete = %q", completed)
	}
	if reopened := runEffortCommand(t, linked, "reopen", []string{id}, nil); !strings.Contains(reopened, "state=active") {
		t.Fatalf("reopen = %q", reopened)
	}
	if abandoned := runEffortCommand(t, primary, "abandon", []string{id}, nil); !strings.Contains(abandoned, "state=abandoned") {
		t.Fatalf("abandon = %q", abandoned)
	}
	if repaired := runEffortCommand(t, linked, "repair", []string{id}, map[string]bool{"--json": true}); !strings.Contains(repaired, `"schemaVersion":1`) {
		t.Fatalf("repair JSON = %q", repaired)
	}
}

func TestEffortNoMemoryMemoryListJSONAndReservedWorktree(t *testing.T) {
	root := commandRepo(t)
	created := runEffortCommand(t, root, "new", []string{"Without memory"}, map[string]bool{"--no-memory": true})
	id := strings.Fields(created)[1]
	if strings.Contains(created, "memory=true") {
		t.Fatalf("--no-memory output = %q", created)
	}
	memory := runEffortCommand(t, root, "memory", []string{id}, nil)
	if memory != "effort "+id+" memory=.awf/memory/"+id+".md\n" {
		t.Fatalf("memory output = %q", memory)
	}
	listed := runEffortCommand(t, root, "list", nil, map[string]bool{"--json": true})
	var envelope struct {
		SchemaVersion int             `json:"schemaVersion"`
		Efforts       []effort.Record `json:"efforts"`
	}
	if err := json.Unmarshal([]byte(listed), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != 1 || len(envelope.Efforts) != 1 || envelope.Efforts[0].ID != id {
		t.Fatalf("list JSON = %#v", envelope)
	}
	ctx := &cmdCtx{root: root, sub: "new", inv: invocation{positionals: []string{"Rejected"}, bools: map[string]bool{"--worktree": true}}, stdout: &bytes.Buffer{}}
	if err := runEffort(ctx); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("--worktree error = %v", err)
	}
}

type effortErrorWriter struct{}

func (effortErrorWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestEffortCommandRefusalsAndOutputErrors(t *testing.T) {
	root := commandRepo(t)
	if err := runEffort(&cmdCtx{root: filepath.Join(root, "missing"), sub: "list", inv: invocation{bools: map[string]bool{}}, stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("invalid repository accepted")
	}
	if err := runEffort(&cmdCtx{root: root, sub: "new", inv: invocation{positionals: []string{" "}, bools: map[string]bool{}}, stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("blank title accepted")
	}
	created := runEffortCommand(t, root, "new", []string{"Errors"}, map[string]bool{"--no-memory": true})
	id := strings.Fields(created)[1]
	unknown := "33333333-3333-4333-8333-333333333333"
	for _, tc := range []struct {
		sub string
		pos []string
	}{
		{"show", []string{unknown}}, {"rename", []string{unknown, "Name"}}, {"memory", []string{unknown}},
		{"complete", []string{unknown}}, {"abandon", []string{unknown}}, {"reopen", []string{unknown}}, {"repair", []string{unknown}},
	} {
		if err := runEffort(&cmdCtx{root: root, sub: tc.sub, inv: invocation{positionals: tc.pos, bools: map[string]bool{}, values: map[string]string{}}, stdout: &bytes.Buffer{}}); err == nil {
			t.Errorf("%s unknown effort accepted", tc.sub)
		}
	}
	memoryPath := filepath.Join(root, ".awf", "memory", id+".md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memoryPath, []byte("Effort: "+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if repaired := runEffortCommand(t, root, "repair", []string{id}, nil); !strings.Contains(repaired, "change memoryPresent") {
		t.Fatalf("repair change output = %q", repaired)
	}
	badPath := filepath.Join(root, ".awf", "efforts", "bad.json")
	if err := os.WriteFile(badPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runEffort(&cmdCtx{root: root, sub: "list", inv: invocation{bools: map[string]bool{}}, stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("corrupt list accepted")
	}
	if err := os.Remove(badPath); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		sub   string
		pos   []string
		flags map[string]bool
	}{
		{"show", []string{id}, nil}, {"show", []string{id}, map[string]bool{"--json": true}},
		{"list", nil, nil}, {"list", nil, map[string]bool{"--json": true}},
		{"memory", []string{id}, nil}, {"repair", []string{id}, nil}, {"repair", []string{id}, map[string]bool{"--json": true}},
	} {
		if tc.flags == nil {
			tc.flags = map[string]bool{}
		}
		err := runEffort(&cmdCtx{root: root, sub: tc.sub, inv: invocation{positionals: tc.pos, bools: tc.flags, values: map[string]string{}}, stdout: effortErrorWriter{}})
		if err == nil {
			t.Errorf("%s output error ignored", tc.sub)
		}
	}
	if err := runEffort(&cmdCtx{root: root, sub: "unknown", inv: invocation{bools: map[string]bool{}}, stdout: &bytes.Buffer{}}); err == nil {
		t.Fatal("unknown effort child accepted")
	}
}

func runEffortCommand(t *testing.T, root, sub string, positionals []string, bools map[string]bool) string {
	t.Helper()
	if bools == nil {
		bools = map[string]bool{}
	}
	var out bytes.Buffer
	ctx := &cmdCtx{root: root, sub: sub, inv: invocation{positionals: positionals, bools: bools, values: map[string]string{}}, stdout: &out}
	if err := runEffort(ctx); err != nil {
		t.Fatalf("awf effort %s: %v", sub, err)
	}
	return out.String()
}

func commandRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "command repo")
	commandGit(t, "init", root)
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commandGit(t, "-C", root, "add", "tracked.txt")
	commandGit(t, "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	return root
}

func commandGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
