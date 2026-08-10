package adr

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
)

// TestADRLockProcess is the subprocess half of TestScaffoldLockReleasesAfterProcessExit.
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

// TestScaffoldLockReleasesAfterProcessExit proves a held advisory lock is
// released when its holder exits, without deleting the persistent lock file.
func TestScaffoldLockReleasesAfterProcessExit(t *testing.T) {
	dir := t.TempDir()
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
	reader := bufio.NewReader(stdout)
	if line, err := reader.ReadString('\n'); err != nil || line != "attempt\n" {
		t.Fatalf("holder signal = %q, %v", line, err)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "acquired\n" {
		t.Fatalf("acquisition signal = %q, %v", line, err)
	}
	if _, err := io.WriteString(stdin, "exit\n"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	unlock, err := acquireScaffoldLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
}
