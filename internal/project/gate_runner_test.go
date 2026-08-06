package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestGateRunnerModes(t *testing.T) {
	root, logPath := gateRunnerFixture(t)
	run := func(extraEnv []string, args ...string) (string, int, []string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"),
			"INVOCATION_LOG="+logPath,
			"AWF_PI_RUNTIME_SMOKE=1",
		)
		cmd.Env = append(cmd.Env, extraEnv...)
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

	ordinaryErr, ordinaryStatus, ordinary := run(nil, "gate")
	if ordinaryStatus != 0 {
		t.Fatalf("ordinary gate status=%d stderr=%q", ordinaryStatus, ordinaryErr)
	}
	timedErr, timedStatus, timed := run(nil, "gate", "timings")
	if timedStatus != 0 {
		t.Fatalf("timed gate status=%d stderr=%q", timedStatus, timedErr)
	}
	if !slices.Equal(normalizeDeadcodePipeline(ordinary), normalizeDeadcodePipeline(timed)) {
		t.Fatalf("ordinary and timed commands differ:\nordinary=%q\ntimed=%q", ordinary, timed)
	}
	assertGateInvocations(t, ordinary)
	if strings.Contains(ordinaryErr, "gate timing:") {
		t.Fatalf("ordinary gate printed timings: %q", ordinaryErr)
	}
	wantLabels := []string{
		"go-test", "covercheck", "pi-runtime-smoke", "vet",
		"build-linux-arm64", "build-darwin-amd64", "build-darwin-arm64",
		"build-windows-amd64", "build-windows-arm64", "lint", "deadcode", "pincheck",
	}
	assertTimingLines(t, timedErr, wantLabels)

	for _, args := range [][]string{{"gate", "full"}, {"gate", "unknown"}, {"gate", "timings", "extra"}} {
		stderr, status, lines := run(nil, args...)
		if status != 2 || stderr != "usage: ./x gate [timings]\n" || len(lines) != 0 {
			t.Errorf("x %v: status=%d stderr=%q invocations=%q", args, status, stderr, lines)
		}
	}

	testErr, testStatus, testLines := run(nil, "test", "-run", "TestOne")
	const notice = "test: Pi container skipped; run './x pi-test run' alone or './x gate' to include it\n"
	if testStatus != 0 || testErr != notice {
		t.Errorf("x test: status=%d stderr=%q", testStatus, testErr)
	}
	if want := []string{"goos=|goarch=|pi=|test ./... -run TestOne"}; !slices.Equal(testLines, want) {
		t.Errorf("x test invocations=%q, want %q", testLines, want)
	}

	for _, tc := range []struct {
		name, failure, timing, forbidden string
	}{
		{"vet", "vet ./...", "vet", "build ./..."},
		{"deadcode producer", "tool deadcode -json ./...", "deadcode", "run ./cmd/pincheck"},
		{"deadcode consumer", "run ./cmd/deadcodecheck", "deadcode", "run ./cmd/pincheck"},
	} {
		t.Run(tc.name+" failure preserves status and short-circuits", func(t *testing.T) {
			stderr, status, lines := run([]string{"FAKE_GO_FAIL_CONTAINS=" + tc.failure}, "gate", "timings")
			if status != 17 {
				t.Fatalf("failure status=%d, want 17; stderr=%q", status, stderr)
			}
			if strings.Count(stderr, "gate timing: "+tc.timing+" ") != 1 {
				t.Errorf("failure timing for %s=%q", tc.timing, stderr)
			}
			if strings.Contains(strings.Join(lines, "\n"), tc.forbidden) {
				t.Errorf("later stage %q ran after failure: %q", tc.forbidden, lines)
			}
		})
	}

	t.Run("Pi smoke must run rather than skip", func(t *testing.T) {
		stderr, status, lines := run([]string{"FAKE_PI_RESULT=skip"}, "gate", "timings")
		if status != 1 || !strings.Contains(stderr, "gate: Pi runtime smoke proving units did not run and pass") {
			t.Fatalf("skipped Pi smoke status=%d stderr=%q", status, stderr)
		}
		if strings.Contains(strings.Join(lines, "\n"), "vet ./...") {
			t.Errorf("vet ran after skipped Pi smoke: %q", lines)
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
printf 'goos=%s|goarch=%s|pi=%s|%s\n' "${GOOS:-}" "${GOARCH:-}" "${AWF_PI_RUNTIME_SMOKE:-}" "$*" >>"$INVOCATION_LOG"
if [ -n "${FAKE_GO_FAIL_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_FAIL_CONTAINS"* ]]; then
  exit 17
fi
if [[ "$*" == "test -json ./internal/project -run ^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$ -count=1" ]]; then
  printf '{"Time":"2026-08-02T00:00:00Z","Action":"%s","Package":"example/project","Test":"TestPiEffortMemoryToolContract","Elapsed":0}\n' "${FAKE_PI_RESULT:-pass}"
  printf '{"Time":"2026-08-02T00:00:00Z","Action":"%s","Package":"example/project","Test":"TestPiRealRuntimeSmoke","Elapsed":0}\n' "${FAKE_PI_RESULT:-pass}"
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
		switch {
		case strings.HasSuffix(line, "|tool deadcode -json ./..."):
			tool = i
		case strings.HasSuffix(line, "|run ./cmd/deadcodecheck"):
			checker = i
		}
	}
	if tool >= 0 && checker >= 0 && tool > checker {
		out[tool], out[checker] = out[checker], out[tool]
	}
	return out
}

func assertGateInvocations(t *testing.T, lines []string) {
	t.Helper()
	want := []string{
		"goos=|goarch=|pi=|test ./... -coverpkg=./... -coverprofile=coverage.out",
		"goos=|goarch=|pi=|run ./cmd/covercheck coverage.out",
		"goos=|goarch=|pi=1|test -json ./internal/project -run ^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$ -count=1",
		"goos=|goarch=|pi=|vet ./...",
		"goos=linux|goarch=arm64|pi=|build ./...",
		"goos=darwin|goarch=amd64|pi=|build ./...",
		"goos=darwin|goarch=arm64|pi=|build ./...",
		"goos=windows|goarch=amd64|pi=|build ./...",
		"goos=windows|goarch=arm64|pi=|build ./...",
		"goos=|goarch=|pi=|tool golangci-lint run",
		"goos=|goarch=|pi=|tool deadcode -json ./...",
		"goos=|goarch=|pi=|run ./cmd/deadcodecheck",
		"goos=|goarch=|pi=|run ./cmd/pincheck",
	}
	if got := normalizeDeadcodePipeline(lines); !slices.Equal(got, want) {
		t.Errorf("gate invocations:\n got %q\nwant %q", got, want)
	}
}

func assertTimingLines(t *testing.T, stderr string, wantLabels []string) {
	t.Helper()
	pattern := regexp.MustCompile(`^gate timing: ([a-z0-9-]+) ([0-9]+)s$`)
	var got []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		if !strings.HasPrefix(line, "gate timing:") {
			continue
		}
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			t.Errorf("malformed timing line %q", line)
			continue
		}
		got = append(got, match[1])
	}
	if !slices.Equal(got, wantLabels) {
		t.Errorf("timing labels=%q, want %q; stderr=%q", got, wantLabels, stderr)
	}
}
