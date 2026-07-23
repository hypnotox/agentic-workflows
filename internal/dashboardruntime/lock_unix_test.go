//go:build !windows

package dashboardruntime

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestAdvisoryLockFaults(t *testing.T) {
	oldLstat, oldOpenFile, oldFlock := lockLstat, lockOpenFile, lockFlock
	t.Cleanup(func() { lockLstat, lockOpenFile, lockFlock = oldLstat, oldOpenFile, oldFlock })
	boom := errors.New("injected")
	if _, err := acquireAdvisoryLock(t.TempDir()); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("unsafe shape error = %v", err)
	}
	lockLstat = func(string) (os.FileInfo, error) { return nil, boom }
	if _, err := acquireAdvisoryLock("lock"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("inspect error = %v", err)
	}
	lockLstat = os.Lstat
	lockOpenFile = func(string, int, os.FileMode) (*os.File, error) { return nil, boom }
	if _, err := acquireAdvisoryLock("lock"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("open error = %v", err)
	}
	lockOpenFile = os.OpenFile
	lockFlock = func(int, int) error { return boom }
	if _, err := acquireAdvisoryLock(filepath.Join(t.TempDir(), "lock")); !errors.Is(err, ErrBuild) {
		t.Fatalf("flock error = %v", err)
	}
	if ownedByCurrentUser(fakeFileInfo{}) {
		t.Fatal("FileInfo without syscall stat reported as owned")
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func TestResolveAdvisoryLockIsReleasedOnProcessDeath(t *testing.T) {
	fixture := newRuntimeFixture(t)
	wrapper, reached, _ := synchronizedGoWrapper(t, 1)
	command := exec.Command(os.Args[0], "-test.run=^TestResolveLockProcessHelper$")
	command.Env = append(os.Environ(),
		"AWF_DASHBOARD_LOCK_HELPER=1",
		"AWF_DASHBOARD_LOCK_ROOT="+fixture.root,
		"AWF_DASHBOARD_LOCK_CACHE="+fixture.cache,
		"AWF_DASHBOARD_LOCK_GO="+wrapper,
	)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFileLines(t, reached, 1)
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()

	done := make(chan error, 1)
	go func() {
		_, err := Resolve(fixture.root, fixture.env)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Resolve after lock owner death: %v; helper output: %s", err, output.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Resolve remained blocked after lock owner process died")
	}
}

func TestResolveLockProcessHelper(t *testing.T) {
	if os.Getenv("AWF_DASHBOARD_LOCK_HELPER") != "1" {
		return
	}
	env := BuildEnvironment{
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		GoBinary:     os.Getenv("AWF_DASHBOARD_LOCK_GO"),
		XDGCacheHome: os.Getenv("AWF_DASHBOARD_LOCK_CACHE"),
		Stderr:       os.Stderr,
	}
	if _, err := Resolve(os.Getenv("AWF_DASHBOARD_LOCK_ROOT"), env); err != nil {
		t.Fatal(err)
	}
}
