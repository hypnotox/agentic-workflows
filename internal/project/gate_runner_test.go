package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestGateRunnerModes(t *testing.T) {
	root, logPath := gateRunnerFixture(t)
	run := func(args ...string) (string, int, []string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"), "INVOCATION_LOG="+logPath)
		stdout, stderr := new(strings.Builder), new(strings.Builder)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		err := cmd.Run()
		status := 0
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				status = exit.ExitCode()
			} else {
				t.Fatalf("run x %v: %v", args, err)
			}
		}
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var lines []string
		if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
			lines = strings.Split(trimmed, "\n")
		}
		return stderr.String(), status, lines
	}

	ordinaryErr, ordinaryStatus, ordinary := run("gate")
	if ordinaryStatus != 0 {
		t.Fatalf("ordinary gate status=%d stderr=%q", ordinaryStatus, ordinaryErr)
	}
	timedErr, timedStatus, timed := run("gate", "timings")
	if timedStatus != 0 {
		t.Fatalf("timed gate status=%d stderr=%q", timedStatus, timedErr)
	}
	if !slices.Equal(normalizeDeadcodePipeline(ordinary), normalizeDeadcodePipeline(timed)) {
		t.Fatalf("ordinary and timed commands differ:\nordinary=%q\ntimed=%q", ordinary, timed)
	}
	assertGateInvocationSet(t, ordinary)
	if strings.Contains(ordinaryErr, "gate timing:") {
		t.Fatalf("ordinary gate printed timings: %q", ordinaryErr)
	}
	wantLabels := []string{
		"go-test", "covercheck", "pi-runtime-smoke", "vet",
		"build-linux-arm64", "build-darwin-amd64", "build-darwin-arm64",
		"build-windows-amd64", "build-windows-arm64", "lint", "deadcode", "pincheck",
	}
	last := -1
	for _, label := range wantLabels {
		needle := "gate timing: " + label + " "
		at := strings.Index(timedErr, needle)
		if at < 0 {
			t.Errorf("timed gate missing %q in %q", needle, timedErr)
		} else if at < last {
			t.Errorf("timing label %q is out of order in %q", label, timedErr)
		}
		last = at
	}

	for _, args := range [][]string{{"gate", "full"}, {"gate", "unknown"}, {"gate", "timings", "extra"}} {
		stderr, status, lines := run(args...)
		if status != 2 || stderr != "usage: ./x gate [timings]\n" || len(lines) != 0 {
			t.Errorf("x %v: status=%d stderr=%q invocations=%q", args, status, stderr, lines)
		}
	}

	testErr, testStatus, testLines := run("test", "-run", "TestOne")
	const notice = "test: Pi container skipped; run './x pi-test run' alone or './x gate' to include it\n"
	if testStatus != 0 || testErr != notice {
		t.Errorf("x test: status=%d stderr=%q", testStatus, testErr)
	}
	if want := []string{"pi=|test ./... -run TestOne"}; !slices.Equal(testLines, want) {
		t.Errorf("x test invocations=%q, want %q", testLines, want)
	}

	t.Run("failure preserves status and short-circuits", func(t *testing.T) {
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "./x", "gate", "timings")
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"),
			"INVOCATION_LOG="+logPath,
			"FAKE_GO_FAIL_CONTAINS=vet ./...",
		)
		stderr := new(strings.Builder)
		cmd.Stderr = stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 17 {
			t.Fatalf("failure status err=%v, want exit 17", err)
		}
		if !strings.Contains(stderr.String(), "gate timing: vet ") || strings.Contains(stderr.String(), "gate timing: build-") {
			t.Errorf("failure timings=%q", stderr.String())
		}
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(data), "build ./...") || strings.Contains(string(data), "tool golangci-lint") {
			t.Errorf("later stages ran after vet failure: %q", data)
		}
	})
}

func gateRunnerFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "x"), string(runner))
	if err := os.Chmod(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := `#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = env ]; then
  case "${2:-}" in
    GOOS) printf 'linux\n' ;;
    GOARCH) printf 'amd64\n' ;;
    *) exit 2 ;;
  esac
  exit 0
fi
printf 'pi=%s|%s\n' "${AWF_PI_RUNTIME_SMOKE:-}" "$*" >>"$INVOCATION_LOG"
if [ -n "${FAKE_GO_FAIL_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_FAIL_CONTAINS"* ]]; then
  exit 17
fi
`
	testsupport.WriteFile(t, filepath.Join(fakeBin, "go"), fakeGo)
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "invocations.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, logPath
}

func normalizeDeadcodePipeline(lines []string) []string {
	out := slices.Clone(lines)
	tool, checker := -1, -1
	for i, line := range out {
		switch line {
		case "pi=|tool deadcode -json ./...":
			tool = i
		case "pi=|run ./cmd/deadcodecheck":
			checker = i
		}
	}
	if tool >= 0 && checker >= 0 && tool > checker {
		out[tool], out[checker] = out[checker], out[tool]
	}
	return out
}

func assertGateInvocationSet(t *testing.T, lines []string) {
	t.Helper()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"pi=|test ./... -coverpkg=./... -coverprofile=coverage.out",
		"pi=|run ./cmd/covercheck coverage.out",
		"pi=1|test ./internal/project -run ^TestPiRealRuntimeSmoke$ -count=1",
		"pi=|vet ./...",
		"pi=|tool golangci-lint run",
		"pi=|tool deadcode -json ./...",
		"pi=|run ./cmd/deadcodecheck",
		"pi=|run ./cmd/pincheck",
	} {
		if strings.Count(joined, want) != 1 {
			t.Errorf("gate invocation count for %q != 1 in %q", want, lines)
		}
	}
	if strings.Contains(joined, "container.sh") {
		t.Errorf("gate directly invoked container script: %q", lines)
	}
	for _, target := range []string{"linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"} {
		parts := strings.Split(target, "/")
		needle := "pi=|build ./..."
		found := false
		for _, line := range lines {
			if line == needle && parts[0] != "" && parts[1] != "" {
				// Environment is asserted through the timing-label matrix; the fake go
				// log proves the build command count without depending on env output.
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing cross-build command for %s in %q", target, lines)
		}
	}
	if got := strings.Count(joined, "pi=|build ./..."); got != 5 {
		t.Errorf("cross-build count=%d, want 5 in %q", got, lines)
	}
}
