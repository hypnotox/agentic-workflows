package adr

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

const scaffoldTestTemplate = `---
format: current-state-v1
status: Proposed
date: YYYY-MM-DD
---
# ADR-NNNN: Title

## Context

Context.

## Decision

1. Decision.

## State changes

None.

## Consequences

Consequence.

## Alternatives Considered

Alternative.

## Status history

- YYYY-MM-DD: Proposed
`

func writeScaffoldTestTemplate(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(scaffoldTestTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
}

func isolateScaffoldLockCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", root)
	case "darwin":
		t.Setenv("HOME", root)
	default:
		t.Setenv("XDG_CACHE_HOME", root)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(cache, "awf", "adr-locks")
}

// TestADRLockProcess is the subprocess half of TestScaffoldLockReleasesAfterProcessDeath.
func TestADRLockProcess(t *testing.T) {
	if os.Getenv("ADR_LOCK_PROCESS") == "" {
		return
	}
	fmt.Fprintln(os.Stdout, "attempt")
	unlock, err := acquireScaffoldLock(os.Getenv("ADR_LOCK_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	}()
	fmt.Fprintln(os.Stdout, "acquired")
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
}

type lockProcess struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	output  *bufio.Reader
}

func startLockProcess(t *testing.T, dir string) lockProcess {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestADRLockProcess$")
	command.Env = append(os.Environ(), "ADR_LOCK_PROCESS=1", "ADR_LOCK_DIR="+dir)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return lockProcess{command: command, stdin: stdin, output: bufio.NewReader(stdout)}
}

func readLockSignal(t *testing.T, process lockProcess, want string) {
	t.Helper()
	if line, err := process.output.ReadString('\n'); err != nil || line != want+"\n" {
		t.Fatalf("lock process signal = %q, %v; want %q", line, err, want)
	}
}

// TestScaffoldLockReleasesAfterProcessDeath proves that the operating system
// releases a descriptor-held lock even when the holder cannot run cleanup.
func TestScaffoldLockReleasesAfterProcessDeath(t *testing.T) {
	cache := isolateScaffoldLockCache(t)
	dir := t.TempDir()
	holder := startLockProcess(t, dir)
	readLockSignal(t, holder, "attempt")
	readLockSignal(t, holder, "acquired")

	waiter := startLockProcess(t, dir)
	readLockSignal(t, waiter, "attempt")
	if err := holder.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := holder.command.Wait(); err == nil {
		t.Fatal("killed lock holder exited successfully")
	}
	readLockSignal(t, waiter, "acquired")
	if _, err := io.WriteString(waiter.stdin, "exit\n"); err != nil {
		t.Fatal(err)
	}
	if err := waiter.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waiter.command.Wait(); err != nil {
		t.Fatal(err)
	}

	identity, err := canonicalDecisionsDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	lockPath := filepath.Join(cache, key+".lock")
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("persistent lock file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("lock mode = %04o, want 0600", got)
		}
		cacheInfo, err := os.Stat(cache)
		if err != nil {
			t.Fatal(err)
		}
		if got := cacheInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("cache mode = %04o, want 0700", got)
		}
	}
}

// invariant: adr-system/adr-lifecycle:adr-new-no-overwrite (TestScaffoldRecordPublicationCollisionPreservesWinner)
func TestScaffoldRecordPublicationCollisionPreservesWinner(t *testing.T) {
	dir := t.TempDir()
	writeScaffoldTestTemplate(t, dir)
	winner := []byte("concurrent winner\n")
	publish := func(path string, contents []byte, mode fs.FileMode) error {
		if err := os.WriteFile(path, winner, 0o644); err != nil {
			return err
		}
		return filepublication.Publish(path, contents, mode)
	}
	noLock := func(string) (func() error, error) { return func() error { return nil }, nil }
	path, err := scaffoldRecordWith(dir, "Collision", CurrentFormat(), true, noLock, os.ReadFile, publish)
	if path != "" || !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision result = %q, %v; want empty path and destination-exists identity", path, err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "collision.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(winner) {
		t.Fatalf("collision changed winner bytes: %q", got)
	}
}

func TestScaffoldRecordLockSpansPublication(t *testing.T) {
	dir := t.TempDir()
	writeScaffoldTestTemplate(t, dir)
	var mutex sync.Mutex
	locked := false
	acquire := func(string) (func() error, error) {
		mutex.Lock()
		locked = true
		return func() error {
			locked = false
			mutex.Unlock()
			return nil
		}, nil
	}
	publish := func(path string, contents []byte, mode fs.FileMode) error {
		if !locked {
			t.Fatal("publication ran after the corpus lock was released")
		}
		return filepublication.Publish(path, contents, mode)
	}
	if _, err := scaffoldRecordWith(dir, "Lock Span", CurrentFormat(), true, acquire, os.ReadFile, publish); err != nil {
		t.Fatal(err)
	}
	if locked {
		t.Fatal("scaffold returned while the corpus lock remained held")
	}
}

func TestAcquireScaffoldLockRejectsMissingDirectory(t *testing.T) {
	isolateScaffoldLockCache(t)
	_, err := acquireScaffoldLock(filepath.Join(t.TempDir(), "missing"))
	if err == nil || !strings.Contains(err.Error(), "canonicalize decisions directory") {
		t.Fatalf("missing directory error = %v", err)
	}
}

func TestAcquireScaffoldLockRejectsCacheFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cache environment behavior is covered by released-target compilation")
	}
	root := t.TempDir()
	cacheFile := filepath.Join(root, "cache-file")
	if err := os.WriteFile(cacheFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", cacheFile)
	} else {
		t.Setenv("XDG_CACHE_HOME", cacheFile)
	}
	_, err := acquireScaffoldLock(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "create ADR lock cache") {
		t.Fatalf("cache-file error = %v", err)
	}
}
