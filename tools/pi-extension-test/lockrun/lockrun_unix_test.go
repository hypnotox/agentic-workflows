//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLockrunHelperProcess(t *testing.T) {
	_ = t // subprocess entrypoint uses the test binary without a test body.
	if os.Getenv("LOCKRUN_HELPER") != "1" {
		return
	}
	switch os.Args[len(os.Args)-1] {
	case "hold":
		time.Sleep(350 * time.Millisecond)
	case "descendant":
		cmd := exec.Command(os.Args[0], "-test.run=TestLockrunHelperProcess", "--", "hold")
		cmd.Env = append(os.Environ(), "LOCKRUN_HELPER=1")
		if err := cmd.Start(); err != nil {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func lockrunBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "lockrun")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build lockrun: %v\n%s", err, output)
	}
	return binary
}

func lockrun(t *testing.T, binary, lock, mode string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(binary, lock, os.Args[0], "-test.run=TestLockrunHelperProcess", "--", mode)
	cmd.Env = append(os.Environ(), "LOCKRUN_HELPER=1")
	return cmd
}

func mustWait(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestRunExitAndSetupFailures(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "lock")
	if got := run([]string{"lockrun"}); got != 2 {
		t.Fatalf("usage status = %d", got)
	}
	if got := run([]string{"lockrun", t.TempDir(), "true"}); got != 1 {
		t.Fatalf("open failure status = %d", got)
	}
	if got := run([]string{"lockrun", lock, "definitely-not-a-command"}); got != 1 {
		t.Fatalf("start failure status = %d", got)
	}
	if got := run([]string{"lockrun", lock, "sh", "-c", "exit 7"}); got != 7 {
		t.Fatalf("exit status = %d", got)
	}
	if got := run([]string{"lockrun", lock, "true"}); got != 0 {
		t.Fatalf("success status = %d", got)
	}
	if got := run([]string{"lockrun", lock, "sh", "-c", "kill -TERM $$"}); got != 143 {
		t.Fatalf("signal status = %d", got)
	}
	go func() {
		time.Sleep(25 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	if got := run([]string{"lockrun", lock, "sh", "-c", "sleep 1"}); got != 143 {
		t.Fatalf("forwarded signal status = %d", got)
	}
}

func TestLockrunWaitsRecoversAndFollowsDescendants(t *testing.T) {
	binary := lockrunBinary(t)
	lock := filepath.Join(t.TempDir(), ".host-lane.lock")
	holder := lockrun(t, binary, lock, "hold")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(75 * time.Millisecond)
	contender := lockrun(t, binary, lock, "hold")
	if err := contender.Start(); err != nil {
		t.Fatal(err)
	}
	contenderDone := make(chan error, 1)
	go func() { contenderDone <- contender.Wait() }()
	select {
	case err := <-contenderDone:
		t.Fatalf("contender acquired a live holder lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	mustWait(t, holder)
	select {
	case err := <-contenderDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("contender did not recover after holder exited")
	}

	leader := lockrun(t, binary, lock, "descendant")
	if err := leader.Run(); err != nil {
		t.Fatal(err)
	}
	contender = lockrun(t, binary, lock, "hold")
	started := time.Now()
	if err := contender.Start(); err != nil {
		t.Fatal(err)
	}
	mustWait(t, contender)
	if elapsed := time.Since(started); elapsed < 200*time.Millisecond {
		t.Fatalf("contender acquired after leader exit while descendant held fd: %s", elapsed)
	}
}
