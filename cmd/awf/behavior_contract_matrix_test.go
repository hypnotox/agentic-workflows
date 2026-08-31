package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/upgrade"
)

type applicationContractCase struct {
	name string
	run  func(*testing.T)
}

func TestApplicationBehaviorContractMatrix(t *testing.T) {
	cases := []applicationContractCase{
		{name: "effort lifecycle", run: contractEffortLifecycle},
		{name: "render and check fixpoint", run: contractRenderCheckFixpoint},
		{name: "upgrade and refusal safety", run: contractUpgradeAndRefusal},
		{name: "journal recovery", run: contractJournalRecovery},
	}
	for _, test := range cases {
		t.Run(test.name, test.run)
	}
}

func contractCommand(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runFrom(root, append([]string{"awf"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func contractEffortLifecycle(t *testing.T) {
	root := commandRepo(t)
	code, stdout, stderr := contractCommand(t, root, "effort", "new", "--slug", "behavior-contract", "Behavior contract")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "managed worktree added") {
		t.Fatalf("new: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	worktreePath := filepath.Join(root, ".awf", "worktrees", "behavior-contract")
	if info, err := os.Stat(worktreePath); err != nil || !info.IsDir() {
		t.Fatalf("managed worktree was not created: %v", err)
	}
	memory := filepath.Join(root, ".awf", "efforts", "behavior-contract", "memory.md")
	if body, err := os.ReadFile(memory); err != nil || !bytes.Contains(body, []byte("## Checkpoint")) {
		t.Fatalf("new memory = %q, %v", body, err)
	}
	for _, args := range [][]string{{"effort", "show", "behavior-contract"}, {"effort", "worktree", "remove", "behavior-contract"}, {"effort", "finish", "behavior-contract"}} {
		code, stdout, stderr = contractCommand(t, root, args...)
		if code != 0 || stderr != "" || stdout == "" {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		if args[1] == "worktree" {
			if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
				t.Fatalf("managed worktree survives removal: %v", err)
			}
		}
	}
	archives, err := filepath.Glob(filepath.Join(root, ".awf", "effort-archive", "*-behavior-contract"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("archives = %v, %v", archives, err)
	}
	if archived, err := os.ReadFile(filepath.Join(archives[0], "memory.md")); err != nil || !bytes.Contains(archived, []byte("## Checkpoint")) {
		t.Fatalf("archived memory = %q, %v", archived, err)
	}
}

func contractRenderCheckFixpoint(t *testing.T) {
	root := scaffoldProject(t)
	if code, stdout, stderr := contractCommand(t, root, "check"); code != 0 || stderr != "" || !strings.Contains(stdout, "status: completed") {
		t.Fatalf("initial check: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := contractCommand(t, root, "check"); code != 1 || stderr != "" || !strings.Contains(stdout, "AGENTS.md") {
		t.Fatalf("dirty check: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if code, stdout, stderr := contractCommand(t, root, "render"); code != 0 || stderr != "" || !strings.Contains(stdout, "status: completed") {
		t.Fatalf("render: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if code, stdout, stderr := contractCommand(t, root, "check"); code != 0 || stderr != "" || !strings.Contains(stdout, "status: completed") {
		t.Fatalf("final check: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func contractUpgradeAndRefusal(t *testing.T) {
	root := scaffoldProject(t)
	if code, stdout, stderr := contractCommand(t, root, "upgrade"); code != 0 || stderr != "" || !strings.Contains(stdout, "status: completed") {
		t.Fatalf("current upgrade: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	lock, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lock.SchemaVersion = 49
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	before := snapshotUpgradeFixture(t, root)
	code, stdout, stderr := contractCommand(t, root, "upgrade")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "schema 49") {
		t.Fatalf("below-floor upgrade: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertUpgradeFixtureUnchanged(t, root, before)
}

func contractJournalRecovery(t *testing.T) {
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	lockBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	finalLock := append(bytes.Clone(lockBytes), []byte("\n# interrupted\n")...)
	prepared := filepath.Join(root, "prepared.txt")
	if err := os.WriteFile(prepared, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	journal := upgrade.Journal{
		Version:         upgrade.JournalVersion,
		Phase:           "applying",
		FinalLockSHA256: fmt.Sprintf("%x", sha256.Sum256(finalLock)),
		Operations: []upgrade.Operation{
			{Path: "prepared.txt", Prior: upgrade.Image{Present: false}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: []byte("new")}},
			{Path: upgrade.LockRel(), Prior: upgrade.Image{Present: true, Mode: uint32(info.Mode().Perm()), Content: lockBytes}, Replacement: upgrade.Image{Present: true, Mode: 0o644, Content: finalLock}},
		},
	}
	encoded, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upgradeJournalPath(root), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := contractCommand(t, root, "upgrade", "--recover"); code != 0 || stderr != "" || !strings.Contains(stdout, "recovered") {
		t.Fatalf("recover: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(prepared); !os.IsNotExist(err) {
		t.Fatalf("prepared file survives recovery: %v", err)
	}
	if journalPresence(t, root) {
		t.Fatal("journal survives recovery")
	}
}
