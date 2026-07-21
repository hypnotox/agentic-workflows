package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/migrate"
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
	// Direct-call coverage of the version sub-check's corrupt branch. When a
	// config layout exists, gate()'s GateState check errors first; without one
	// (next test) this reader is the first to hit the lock.
	root, _ := corruptLock(t, "garbage")
	if _, _, _, err := lockVsBinary(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want corrupt-lock error, got %v", err)
	}
}

func TestGateCorruptLockWithoutConfigLayout(t *testing.T) {
	// A corrupt lock beside NO config layout: Generation stats no config file
	// and never loads the lock, so gate()'s version sub-check is the first
	// reader to hit it - the error must propagate, not be swallowed (ADR-0076).
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.LockPath(root), corruptions["garbage"], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gate(root); err == nil || !strings.Contains(err.Error(), "unreadable .awf/awf.lock") {
		t.Fatalf("want corrupt-lock error through the version sub-check, got %v", err)
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

func TestUpgradeCorruptLockRefuses(t *testing.T) {
	for variant := range corruptions {
		t.Run(variant, func(t *testing.T) {
			root, want := corruptLock(t, variant)
			var out, errb bytes.Buffer
			code := runAt(t, root, []string{"awf", "upgrade"}, &out, &errb)
			assertRefused(t, root, want, code, out.String()+errb.String())
		})
	}
}

func TestUpgradeReportsBinaryBehind(t *testing.T) {
	root := scaffoldProject(t)
	lockPath := config.LockPath(root)
	l, err := manifest.Load(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	l.SchemaVersion = migrate.Current() + 1
	l.ADRFormatV2From = l.ADRFormatV1From
	if err := l.Save(lockPath); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := runAt(t, root, []string{"awf", "upgrade"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "update your pinned awf") || strings.Contains(all, "already current") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}

func TestUpgradeOutsideProject(t *testing.T) {
	var out, errb bytes.Buffer
	code := runAt(t, t.TempDir(), []string{"awf", "upgrade"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "not an awf project (run `awf init`)") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}

func TestProjectCommandsHintInit(t *testing.T) {
	var out, errb bytes.Buffer
	code := runAt(t, t.TempDir(), []string{"awf", "sync"}, &out, &errb)
	all := out.String() + errb.String()
	if code != 1 || !strings.Contains(all, "not an awf project (run `awf init`)") {
		t.Fatalf("code=%d output:\n%s", code, all)
	}
}

func TestUninstallAndInitRefuseCorruptLock(t *testing.T) {
	for variant := range corruptions {
		for _, cmd := range []string{"uninstall", "init"} {
			t.Run(variant+"/"+cmd, func(t *testing.T) {
				root, want := corruptLock(t, variant)
				var out, errb bytes.Buffer
				code := runAt(t, root, []string{"awf", cmd}, &out, &errb)
				all := out.String() + errb.String()
				if cmd == "init" && strings.Contains(all, "collision") {
					t.Fatalf("init reported collisions instead of the lock error:\n%s", all)
				}
				assertRefused(t, root, want, code, all)
			})
		}
	}
}
