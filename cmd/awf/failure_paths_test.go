package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// corruptions is the ADR-0076 Decision 6 corrupt-lock variant matrix.
var corruptions = map[string][]byte{
	"truncated": []byte(`{"awfVersion":"0.9.0","schemaVersion":6,"files":{`),
	"garbage":   {0x00, 0xff, 0x13, 0x37},
	"conflict":  []byte("<<<<<<< HEAD\n{\"awfVersion\":\"0.9.0\"}\n=======\n{\"awfVersion\":\"0.8.0\"}\n>>>>>>> theirs\n"),
}

// corruptLock scaffolds a synced project, overwrites its lock with the named
// corruption, and returns root plus the corrupt bytes for the untouched check.
func corruptLock(t *testing.T, variant string) (string, []byte) {
	t.Helper()
	root := scaffoldProject(t)
	body := corruptions[variant]
	if err := os.WriteFile(config.LockPath(root), body, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, body
}

// runAt drives the full CLI dispatch with the process cwd at root.
func runAt(t *testing.T, root string, args []string, stdout, stderr *bytes.Buffer) int {
	t.Helper()
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	return run(args, stdout, stderr)
}

// assertRefused asserts the run refused with the recovery hint, created no
// .awf-bak backup anywhere under root, and left the corrupt lock byte-identical.
func assertRefused(t *testing.T, root string, wantBytes []byte, code int, out string) {
	t.Helper()
	if code != 1 {
		t.Fatalf("exit = %d, want 1; output:\n%s", code, out)
	}
	if !strings.Contains(out, "unreadable .awf/awf.lock") || !strings.Contains(out, "restore it from version control") {
		t.Fatalf("missing recovery hint:\n%s", out)
	}
	var baks []string
	err := filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err == nil && strings.Contains(filepath.Base(p), ".awf-bak") {
			baks = append(baks, p)
		}
		return nil
	})
	if err != nil || len(baks) != 0 {
		t.Fatalf("backup storm: %v (err %v)", baks, err)
	}
	got, err := os.ReadFile(config.LockPath(root))
	if err != nil || !bytes.Equal(got, wantBytes) {
		t.Fatalf("corrupt lock was modified (err %v)", err)
	}
}

func TestLockVsBinaryCorruptLockErrors(t *testing.T) {
	// Direct-call coverage of the defense-in-depth branch: via gate() the
	// GateState check errors first, so only a direct caller reaches it.
	root, _ := corruptLock(t, "garbage")
	if _, _, _, err := lockVsBinary(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want corrupt-lock error, got %v", err)
	}
}

func TestGatedCommandsRefuseCorruptLock(t *testing.T) {
	for variant := range corruptions {
		for _, cmd := range []string{"sync", "check", "invariants", "audit", "list"} {
			t.Run(variant+"/"+cmd, func(t *testing.T) {
				root, want := corruptLock(t, variant)
				var out, errb bytes.Buffer
				code := runAt(t, root, []string{"awf", cmd}, &out, &errb)
				assertRefused(t, root, want, code, out.String()+errb.String())
			})
		}
	}
}
