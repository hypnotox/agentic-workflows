package project

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// invariant: tooling/context-and-topic:context-spill-observability (TestContextSpillObservabilityContract)
func TestContextSpillObservabilityContract(t *testing.T) {
	helper := buildContextSpillHelper(t)
	t.Run("wrapper byte and status preservation", func(t *testing.T) {
		testContextRunnerPreservesOutputStatusAndObservesSpills(t, helper)
	})
	t.Run("logging failure warning degradation", func(t *testing.T) {
		testContextRunnerLoggingFailureWarnsWithoutChangingSuccess(t, helper)
	})
	t.Run("concurrent wrapper records", func(t *testing.T) {
		testContextRunnerConcurrentRecordsDoNotInterleave(t, helper)
	})
	t.Run("safe check advisory", func(t *testing.T) {
		testCheckRunnerSpillAdvisoryTracksNonemptySafeLog(t, helper)
	})
}

func testContextRunnerPreservesOutputStatusAndObservesSpills(t *testing.T, helper string) {
	root := contextRunnerFixture(t, helper)
	run := func(mode string) (string, string, int) {
		t.Helper()
		command := exec.Command("bash", "./x", "context", "a b")
		command.Dir = root
		command.Env = append(os.Environ(), "FAKE_MODE="+mode)
		stdout, stderr := new(strings.Builder), new(strings.Builder)
		command.Stdout, command.Stderr = stdout, stderr
		err := command.Run()
		status := 0
		if err != nil {
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) {
				t.Fatalf("run context: %v", err)
			}
			status = exitError.ExitCode()
		}
		return stdout.String(), stderr.String(), status
	}

	stdout, stderr, status := run("normal")
	if stdout != "context: normal\n\n" || stderr != "" || status != 0 {
		t.Fatalf("normal stdout=%q stderr=%q status=%d", stdout, stderr, status)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "local", "context-spills.log")); !os.IsNotExist(err) {
		t.Fatalf("normal output created spill log: %v", err)
	}

	stdout, stderr, status = run("spill")
	spillPath := filepath.Join(root, "spill.txt")
	wantNotice := fmt.Sprintf("AWF_CONTEXT_SPILL_V1 bytes=9000 format=text\n%s\n", spillPath)
	if stdout != wantNotice || stderr != "" || status != 0 {
		t.Fatalf("spill stdout=%q stderr=%q status=%d", stdout, stderr, status)
	}
	logged, err := os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logged), spillPath) || !strings.Contains(string(logged), "invocation='./x' 'context' 'a b'") {
		t.Fatalf("unexpected spill log %q", logged)
	}

	before := string(logged)
	stdout, _, status = run("near")
	if stdout != "AWF_CONTEXT_SPILL bytes=9000 format=text\n/tmp/nope\n" || status != 0 {
		t.Fatalf("near miss stdout=%q status=%d", stdout, status)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if string(after) != before {
		t.Fatal("near miss was logged")
	}

	stdout, stderr, status = run("malformed")
	if stdout != "AWF_CONTEXT_SPILL_V1 bytes=bad format=text\n/tmp/nope\n" || !strings.Contains(stderr, "local observability logging failed") || status != 0 {
		t.Fatalf("malformed stdout=%q stderr=%q status=%d", stdout, stderr, status)
	}
	after, _ = os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if string(after) != before {
		t.Fatal("malformed notice was logged")
	}

	stdout, _, status = run("failure")
	if stdout != "partial\n\n" || status != 7 {
		t.Fatalf("failure stdout=%q status=%d", stdout, status)
	}
	after, _ = os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if string(after) != before {
		t.Fatal("failed child was logged")
	}
}

func testContextRunnerLoggingFailureWarnsWithoutChangingSuccess(t *testing.T, helper string) {
	root := contextRunnerFixture(t, helper)
	local := filepath.Join(root, ".awf", "local")
	if err := os.Mkdir(local, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "./x", "context")
	command.Dir = root
	command.Env = append(os.Environ(), "FAKE_MODE=spill")
	stdout, stderr := new(strings.Builder), new(strings.Builder)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		t.Fatalf("context failed: %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "AWF_CONTEXT_SPILL_V1 bytes=9000 format=text\n") || !strings.Contains(stderr.String(), "local observability logging failed") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func testContextRunnerConcurrentRecordsDoNotInterleave(t *testing.T, helper string) {
	root := contextRunnerFixture(t, helper)
	const count = 8
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command := exec.Command("bash", "./x", "context", "concurrent")
			command.Dir = root
			command.Env = append(os.Environ(), "FAKE_MODE=spill")
			errs <- command.Run()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != count {
		t.Fatalf("records=%d, want %d: %q", len(lines), count, data)
	}
	for _, line := range lines {
		if strings.Count(line, "\t") != 2 {
			t.Fatalf("interleaved record %q", line)
		}
	}
}

func testCheckRunnerSpillAdvisoryTracksNonemptySafeLog(t *testing.T, helper string) {
	root := contextRunnerFixture(t, helper)
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	goScript := "#!/usr/bin/env bash\nset -e\nif [ \"${1:-}\" = build ]; then\n  while [ $# -gt 0 ]; do\n    if [ \"$1\" = -o ]; then cp ./awf \"$2\"; chmod +x \"$2\"; exit 0; fi\n    shift\n  done\nfi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "go"), []byte(goScript), 0o755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, ".awf", "local")
	if err := os.Mkdir(local, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(local, "context-spills.log")
	if err := os.WriteFile(logPath, []byte("record\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() string {
		command := exec.Command("bash", "./x", "check")
		command.Dir = root
		command.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "FAKE_MODE=normal")
		stderr := new(strings.Builder)
		command.Stderr = stderr
		if err := command.Run(); err != nil {
			t.Fatalf("check: %v, stderr %q", err, stderr.String())
		}
		return stderr.String()
	}
	if stderr := run(); !strings.Contains(stderr, "resolve or promote the issue") {
		t.Fatalf("missing advisory: %q", stderr)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if stderr := run(); strings.Contains(stderr, "resolve or promote the issue") {
		t.Fatalf("empty log advisory: %q", stderr)
	}
	if err := os.Chmod(logPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if stderr := run(); !strings.Contains(stderr, "advisory inspection failed") || strings.Contains(stderr, "check: advisory:") {
		t.Fatalf("unsafe log inspection was not warning-only: %q", stderr)
	}
}

func buildContextSpillHelper(t *testing.T) string {
	t.Helper()
	helper := filepath.Join(t.TempDir(), "contextspilllog")
	build := exec.Command("go", "build", "-o", helper, "./cmd/contextspilllog")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, output)
	}
	return helper
}

func contextRunnerFixture(t *testing.T, helper string) string {
	t.Helper()
	root := t.TempDir()
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(runner), "go run ./cmd/contextspilllog", "'"+helper+"'")
	if err := os.WriteFile(filepath.Join(root, "x"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeAwf := `#!/usr/bin/env bash
case "${FAKE_MODE:-normal}" in
  normal) printf 'context: normal\n\n' ;;
  spill)
    : >"$PWD/spill.txt"
    printf 'AWF_CONTEXT_SPILL_V1 bytes=9000 format=text\n%s\n' "$PWD/spill.txt"
    ;;
  near) printf 'AWF_CONTEXT_SPILL bytes=9000 format=text\n/tmp/nope\n' ;;
  malformed) printf 'AWF_CONTEXT_SPILL_V1 bytes=bad format=text\n/tmp/nope\n' ;;
  failure) printf 'partial\n\n'; exit 7 ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "awf"), []byte(fakeAwf), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
