package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"gopkg.in/yaml.v3"
)

// invariant: tooling/quality-gates:gate-severity-by-protected-property (TestGateRunnerModes)
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
		"versioncheck", "go-test", "covercheck", "pi-runtime-smoke", "vet",
		"build-linux-arm64", "build-darwin-amd64", "build-darwin-arm64",
		"build-windows-amd64", "build-windows-arm64", "lint", "advisory-lint", "deadcode", "pincheck",
	}
	assertTimingLines(t, timedErr, wantLabels)

	t.Run("advisory findings warn and continue", func(t *testing.T) {
		stderr, status, lines := run([]string{
			"FAKE_GO_OUTPUT_CONTAINS=.golangci-advisory.yml",
			"FAKE_GO_OUTPUT=advisory-finding-sentinel",
		}, "gate")
		if status != 0 {
			t.Fatalf("advisory finding status=%d stderr=%q", status, stderr)
		}
		if !strings.Contains(stderr, "warning: advisory lint findings\nadvisory-finding-sentinel\n") {
			t.Errorf("advisory finding was not visibly warned: %q", stderr)
		}
		if !strings.Contains(strings.Join(lines, "\n"), "run ./cmd/pincheck") {
			t.Errorf("later gate stages did not run after advisory finding: %q", lines)
		}
	})

	t.Run("advisory execution failure blocks and preserves status", func(t *testing.T) {
		stderr, status, lines := run([]string{
			"FAKE_GO_OUTPUT_CONTAINS=.golangci-advisory.yml",
			"FAKE_GO_OUTPUT=advisory-failure-sentinel",
			"FAKE_GO_FAIL_CONTAINS=.golangci-advisory.yml",
		}, "gate")
		if status != 17 || !strings.Contains(stderr, "advisory-failure-sentinel") {
			t.Fatalf("advisory failure status=%d stderr=%q", status, stderr)
		}
		if strings.Contains(strings.Join(lines, "\n"), "tool deadcode -json ./...") {
			t.Errorf("deadcode ran after advisory execution failure: %q", lines)
		}
	})

	for _, args := range [][]string{{"gate", "full"}, {"gate", "unknown"}, {"gate", "timings", "extra"}} {
		stderr, status, lines := run(nil, args...)
		if status != 2 || stderr != "usage: ./x gate [timings]\n" || len(lines) != 0 {
			t.Errorf("x %v: status=%d stderr=%q invocations=%q", args, status, stderr, lines)
		}
	}

	testErr, testStatus, testLines := run(nil, "test", "-run", "TestOne")
	const notice = "test: Pi host lane skipped; run './x pi-test run' alone or './x gate' to include it\n"
	if testStatus != 0 || testErr != notice {
		t.Errorf("x test: status=%d stderr=%q", testStatus, testErr)
	}
	if want := []string{"goos=|goarch=|pi=|test ./... -run TestOne"}; !slices.Equal(testLines, want) {
		t.Errorf("x test invocations=%q, want %q", testLines, want)
	}

	for _, tc := range []struct {
		name, failure, timing, forbidden string
	}{
		{"versioncheck", "run ./cmd/versioncheck", "versioncheck", "test ./..."},
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

// invariant: tooling/quality-gates:gate-severity-by-protected-property (TestGateLintRuleInventory)
func TestGateLintRuleInventory(t *testing.T) {
	type lintConfig struct {
		Linters struct {
			Enable   []string `yaml:"enable"`
			Settings struct {
				Staticcheck struct {
					Checks []string `yaml:"checks"`
				} `yaml:"staticcheck"`
			} `yaml:"settings"`
		} `yaml:"linters"`
		Formatters struct {
			Enable []string `yaml:"enable"`
		} `yaml:"formatters"`
	}
	readConfig := func(path string) lintConfig {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var config lintConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatal(err)
		}
		return config
	}

	blocking := readConfig("../../.golangci.yml")
	wantBlocking := []string{
		"govet", "staticcheck", "errcheck", "ineffassign", "nilerr", "bodyclose", "errorlint",
		"durationcheck", "asasalint", "nilnesserr", "gocheckcompilerdirectives", "makezero",
		"exhaustive", "wastedassign",
	}
	wantBlockingStaticcheck := []string{"SA0*", "SA1*", "SA2*", "SA3*", "SA4*", "SA5*", "SA9*"}
	if !slices.Equal(blocking.Linters.Enable, wantBlocking) ||
		!slices.Equal(blocking.Linters.Settings.Staticcheck.Checks, wantBlockingStaticcheck) ||
		len(blocking.Formatters.Enable) != 0 {
		t.Fatalf("blocking lint inventory = linters %q, staticcheck %q, formatters %q", blocking.Linters.Enable, blocking.Linters.Settings.Staticcheck.Checks, blocking.Formatters.Enable)
	}

	advisory := readConfig("../../.golangci-advisory.yml")
	wantAdvisory := []string{
		"staticcheck", "nilnil", "unused", "unconvert", "unparam", "predeclared", "gocritic",
		"dupword", "perfsprint", "intrange", "usestdlibvars", "usetesting", "misspell", "revive", "whitespace",
	}
	wantAdvisoryStaticcheck := []string{"SA6*", "S*", "ST*", "QF*"}
	wantFormatters := []string{"gofmt", "goimports"}
	if !slices.Equal(advisory.Linters.Enable, wantAdvisory) ||
		!slices.Equal(advisory.Linters.Settings.Staticcheck.Checks, wantAdvisoryStaticcheck) ||
		!slices.Equal(advisory.Formatters.Enable, wantFormatters) {
		t.Fatalf("advisory lint inventory = linters %q, staticcheck %q, formatters %q", advisory.Linters.Enable, advisory.Linters.Settings.Staticcheck.Checks, advisory.Formatters.Enable)
	}
}

func TestRunnerFmtRestoresMissingImport(t *testing.T) {
	root := t.TempDir()
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "x"), string(runner))
	if err := os.Chmod(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("../../.golangci-advisory.yml")
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, ".golangci-advisory.yml"), string(config))
	testsupport.WriteFile(t, filepath.Join(root, "go.mod"), "module example.com/formatter\n\ngo 1.25.0\n")
	goFile := filepath.Join(root, "value.go")
	testsupport.WriteFile(t, goFile, "package formatter\n\nfunc Value() string {\n\treturn strings.TrimSpace(\" value \")\n}\n")

	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	toolPath, err := exec.Command(realGo, "tool", "-n", "golangci-lint").Output()
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := "#!/usr/bin/env bash\nset -euo pipefail\nif [ \"${1:-}\" = tool ] && [ \"${2:-}\" = golangci-lint ]; then\n  shift 2\n  exec " + strconv.Quote(strings.TrimSpace(string(toolPath))) + " \"$@\"\nfi\nexec " + strconv.Quote(realGo) + " \"$@\"\n"
	testsupport.WriteFile(t, filepath.Join(fakeBin, "go"), fakeGo)
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "fixture", nil)

	cmd := exec.Command("bash", "./x", "fmt")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("x fmt: %v: %s", err, output)
	}
	formatted, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(formatted), "\"strings\"") {
		t.Fatalf("x fmt did not restore the missing import:\n%s", formatted)
	}
	compile := exec.Command("go", "test", "./...")
	compile.Dir = root
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("formatted fixture does not compile: %v: %s", err, output)
	}
}

func TestCovercheckMutantsRunnerContract(t *testing.T) {
	root, logPath := mutationRunnerFixture(t)
	run := func(env []string, args ...string) (string, int, []string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), append([]string{"PATH=" + filepath.Join(root, "fake-bin") + ":" + os.Getenv("PATH"), "INVOCATION_LOG=" + logPath, "MUTATION_ARGS=" + filepath.Join(root, "gremlins.args")}, env...)...)
		out, err := cmd.CombinedOutput()
		status := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			status = exit.ExitCode()
		}
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return string(out), status, strings.Fields(string(data))
	}

	output, status, lines := run(nil, "covercheck-mutants", "--evidence", "evidence", "--baseline", "baseline.json")
	if status != 0 {
		t.Fatalf("default blocker status=%d output=%q", status, output)
	}
	wantOrder := []string{"list-module", "tool-lookup", "tool-version", "df", "df", "git-isolation", "list-tests", "list-imports", "list-deps", "test", "gremlins-dry", "gremlins-actual", "validate", "lsof"}
	if !slices.Equal(lines, wantOrder) {
		t.Fatalf("orchestration order=%q, want %q", lines, wantOrder)
	}
	if _, err := os.Stat(filepath.Join(root, "evidence", "dry.json")); err != nil {
		t.Fatalf("retained dry evidence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "evidence", "actual.json")); err != nil {
		t.Fatalf("retained actual evidence: %v", err)
	}
	gremlinsArgs, err := os.ReadFile(filepath.Join(root, "gremlins.args"))
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--integration=false", "--workers=1", "--test-cpu=1", "--timeout-coefficient=20", "--threshold-efficacy=0", "--threshold-mcover=0", "--tags=", "--coverpkg=", "--diff=", "--arithmetic-base=true", "--conditionals-boundary=true", "--conditionals-negation=true", "--increment-decrement=true", "--invert-negatives=true", "--invert-assignments=false", "--invert-bitwise=false", "--invert-bwassign=false", "--invert-logical=false", "--invert-loopctrl=false", "--remove-self-assignments=false"} {
		if !strings.Contains(string(gremlinsArgs), flag) {
			t.Errorf("missing required gremlins argument %q in %q", flag, gremlinsArgs)
		}
	}

	for _, tc := range []struct {
		name, gitBody string
		args          []string
		wantRun       bool
	}{
		{"staged owned addition", "printf 'cmd/covercheck/new.go\\0'", []string{"--select-staged"}, true},
		{"staged owned modification", "printf 'cmd/covercheck/main.go\\0'", []string{"--select-staged"}, true},
		{"staged owned deletion", "printf 'cmd/covercheck/old.go\\0'", []string{"--select-staged"}, true},
		{"range rename pair selects destination", "printf 'cmd/other.go\\0cmd/covercheck/new.go\\0'", []string{"--select-range", "base", "head"}, true},
		{"nonowned skips", "printf 'cmd/mutants/main.go\\0'", []string{"--select-staged"}, false},
		{"empty skips", "", []string{"--select-staged"}, false},
		{"malformed fails closed", "printf 'cmd/covercheck/main.go'", []string{"--select-staged"}, true},
		{"unavailable fails closed", "if [[ \"$*\" == diff* ]]; then exit 17; fi\nexit 0", []string{"--select-staged"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\nset -e\nif [[ \"$*\" == *-C* ]]; then echo git-isolation >>\"$INVOCATION_LOG\"; [ \"${FAKE_GIT_INSIDE:-}\" = 1 ] && exit 0 || exit 1; fi\nif [[ \"$*\" == *'rev-parse --show-toplevel'* ]]; then pwd; exit 0; fi\nif [[ \"$*\" == *cat-file* ]]; then [ \"${FAKE_GIT_MISSING:-}\" = 1 ] && exit 17 || exit 0; fi\n"+tc.gitBody+"\n")
			if err := os.Chmod(filepath.Join(root, "fake-bin", "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			_, got, logged := run(nil, append([]string{"covercheck-mutants", "--evidence", "evidence"}, tc.args...)...)
			if tc.wantRun && (got != 0 || !slices.Contains(logged, "validate")) {
				t.Fatalf("status=%d log=%q", got, logged)
			}
			if !tc.wantRun && (got != 0 || slices.Contains(logged, "validate")) {
				t.Fatalf("status=%d log=%q", got, logged)
			}
		})
	}
	staleEvidence := filepath.Join(root, "stale-evidence")
	if err := os.Mkdir(staleEvidence, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dry.json", "actual.json"} {
		testsupport.WriteFile(t, filepath.Join(staleEvidence, name), "stale report must not survive\n")
	}
	if output, status, _ := run([]string{"FAKE_GREMLINS_FAIL=1"}, "covercheck-mutants", "--evidence", staleEvidence, "--baseline", "-"); status == 0 {
		t.Fatalf("failed Gremlins command succeeded: %q", output)
	}
	for _, name := range []string{"dry.json", "actual.json"} {
		if data, err := os.ReadFile(filepath.Join(staleEvidence, name)); err == nil && strings.Contains(string(data), "stale report") {
			t.Errorf("%s retained stale report", name)
		}
	}

	if output, status, _ := run([]string{"REQUIRE_TMP_ENV=1"}, "covercheck-mutants", "--evidence", "tmp-env-evidence", "--baseline", "-"); status != 0 {
		t.Fatalf("post-root command lacks exported temporary environment: status=%d output=%q", status, output)
	}

	injected := filepath.Join(root, "trap-injected")
	trapEvidence := "trap-evidence'; touch '" + injected + "'; #"
	if output, status, _ := run(nil, "covercheck-mutants", "--evidence", trapEvidence, "--baseline", "-"); status != 0 {
		t.Fatalf("quoted evidence path failed: status=%d output=%q", status, output)
	}
	if _, err := os.Stat(injected); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("caller-controlled evidence path executed by trap: %v", err)
	}

	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"GREMLINS override rejected", []string{"GREMLINS_WORKERS=99"}, "GREMLINS_"},
		{"missing range endpoint selects", []string{"FAKE_GIT_MISSING=1"}, ""},
		{"census refusal", []string{"FAKE_TEST_CENSUS=wrong"}, "test census"},
		{"dependency refusal", []string{"FAKE_NO_DEP=1"}, "dependency"},
		{"capacity refusal", []string{"FAKE_DF_LOW=1"}, "capacity"},
		{"Git isolation refusal", []string{"FAKE_GIT_INSIDE=1"}, "Git"},
		{"incomplete report propagates", []string{"FAKE_INCOMPLETE=1"}, "untrusted"},
		{"aggregate timeout propagates", []string{"FAKE_TIMEOUT=1"}, ""},
		{"owned cleanup refusal propagates", []string{"FAKE_LSOF_STATUS=0"}, "temporary root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Restore a known non-repository fake before each refusal case; the
			// selection matrix deliberately replaces this binary.
			testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\nif [[ \"$*\" == *-C* ]]; then [ \"${FAKE_GIT_INSIDE:-}\" = 1 && exit 0 || exit 1; fi\nif [[ \"$*\" == *'rev-parse --show-toplevel'* ]]; then pwd; exit 0; fi\n")
			if err := os.Chmod(filepath.Join(root, "fake-bin", "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			args := []string{"covercheck-mutants", "--evidence", "evidence"}
			if tc.name == "missing range endpoint selects" {
				testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\nif [[ \"$*\" == *-C* ]]; then exit 1; fi\nif [[ \"$*\" == *'rev-parse --show-toplevel'* ]]; then pwd; exit 0; fi\nif [[ \"$*\" == *cat-file* ]]; then exit 17; fi\n")
				if err := os.Chmod(filepath.Join(root, "fake-bin", "git"), 0o755); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--select-range", "base", "head")
			}
			if tc.name == "Git isolation refusal" {
				testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\nif [[ \"$*\" == *-C* ]]; then exit 0; fi\nif [[ \"$*\" == *'rev-parse --show-toplevel'* ]]; then pwd; fi\n")
				if err := os.Chmod(filepath.Join(root, "fake-bin", "git"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			output, status, _ := run(tc.env, args...)
			if tc.name == "missing range endpoint selects" && status == 0 {
				return
			}
			if status == 0 || (tc.want != "" && !strings.Contains(output, tc.want)) {
				t.Fatalf("status=%d output=%q", status, output)
			}
		})
	}
}

func mutationRunnerFixture(t *testing.T) (string, string) {
	t.Helper()
	root, _ := gateRunnerFixture(t)
	testsupport.WriteFile(t, filepath.Join(root, "baseline.json"), "{}\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "covercheck", "main_test.go"), "package covercheck\n")
	testsupport.WriteFile(t, filepath.Join(root, "cmd", "covercheck", "policy_edge_test.go"), "package covercheck\n")
	if err := os.Mkdir(filepath.Join(root, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\n[[ \"$*\" == *-C* ]] && { echo git-isolation >>\"$INVOCATION_LOG\"; exit 1; }\n[[ \"$*\" == *'rev-parse --show-toplevel'* ]] && { pwd; exit 0; }\nexit 0\n")
	fake := `#!/usr/bin/env bash
set -euo pipefail
require_tmp_env() {
  [ "${REQUIRE_TMP_ENV:-}" != 1 ] || { [ -n "${TMPDIR:-}" ] && [ "$TMPDIR" = "${GOTMPDIR:-}" ] && [ "$TMPDIR" = "${TMP:-}" ] && [ "$TMPDIR" = "${TEMP:-}" ]; }
}
case "$*" in
  *"list -m"*) echo list-module >>"$INVOCATION_LOG"; echo v0.6.0 ;;
  "tool -n gremlins") echo tool-lookup >>"$INVOCATION_LOG"; echo /fake/gremlins ;;
  "version -m /fake/gremlins") echo tool-version >>"$INVOCATION_LOG"; printf '/fake/gremlins\n\tmod\tgithub.com/go-gremlins/gremlins\tv0.6.0\th1:fake\n' ;;
  *"list -f"*"TestGoFiles"*) require_tmp_env; echo list-tests >>"$INVOCATION_LOG"; if [ "${FAKE_TEST_CENSUS:-}" = wrong ]; then echo wrong_test.go; else printf 'main_test.go\npolicy_edge_test.go\n'; fi ;;
  *"list -f"*".Imports"*) require_tmp_env; echo list-imports >>"$INVOCATION_LOG"; echo github.com/hypnotox/agentic-workflows/internal/coverage ;;
  *"list -deps -test"*) require_tmp_env; echo list-deps >>"$INVOCATION_LOG"; [ "${FAKE_NO_DEP:-}" = 1 ] || echo github.com/hypnotox/agentic-workflows/internal/coverage ;;
  *"test -count=1 ./..."*) require_tmp_env; echo test >>"$INVOCATION_LOG" ;;
  *"tool gremlins"*) require_tmp_env; printf '%s ' "$@" >"$MUTATION_ARGS"; if [[ "$*" == *--dry-run* ]]; then echo gremlins-dry >>"$INVOCATION_LOG"; else echo gremlins-actual >>"$INVOCATION_LOG"; fi; [ "${FAKE_GREMLINS_FAIL:-}" != 1 ] || exit 42; out=""; eval 'args=("${@}")'; for ((i=0;i<${#args[@]};i++)); do { [ "${args[i]}" = -o ] || [ "${args[i]}" = --output ]; } && out="${args[i+1]}"; done; if [ "${FAKE_INCOMPLETE:-}" = 1 ]; then : >"$out"; else echo '{}' >"$out"; fi ;;
  *"run ./cmd/mutants operators"*) printf '%s\n' '--arithmetic-base=true' '--conditionals-boundary=true' '--conditionals-negation=true' '--increment-decrement=true' '--invert-negatives=true' '--invert-assignments=false' '--invert-bitwise=false' '--invert-bwassign=false' '--invert-logical=false' '--invert-loopctrl=false' '--remove-self-assignments=false' ;;
  *"run ./cmd/mutants validate"*) require_tmp_env; echo validate >>"$INVOCATION_LOG"; if [ "${FAKE_INCOMPLETE:-}" = 1 ]; then echo untrusted >&2; exit 1; fi; echo 'trusted mutation reports: 1 identities; status-sha256=fake' ;;
  *) echo "unexpected go $*" >&2; exit 19 ;;
esac
`
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "go"), fake)
	for _, name := range []string{"go", "git"} {
		if err := os.Chmod(filepath.Join(root, "fake-bin", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "df"), "#!/usr/bin/env bash\necho df >>\"$INVOCATION_LOG\"\nif [ \"${FAKE_DF_LOW:-}\" = 1 ]; then echo 'x 1 1 1 1% /tmp'; else echo 'x 1 1 1048576 1% /tmp'; fi\n")
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "lsof"), "#!/usr/bin/env bash\necho lsof >>\"$INVOCATION_LOG\"\nexit \"${FAKE_LSOF_STATUS:-1}\"\n")
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "timeout"), "#!/usr/bin/env bash\nif [ \"${FAKE_TIMEOUT:-}\" = 1 ]; then exit 124; fi\nshift\nexec \"$@\"\n")
	for _, name := range []string{"df", "lsof", "timeout"} {
		if err := os.Chmod(filepath.Join(root, "fake-bin", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, filepath.Join(root, "invocations.log")
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
if [ -n "${FAKE_GO_OUTPUT_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_OUTPUT_CONTAINS"* ]]; then
  printf '%s\n' "${FAKE_GO_OUTPUT:-}"
fi
if [ -n "${FAKE_GO_FAIL_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_FAIL_CONTAINS"* ]]; then
  exit 17
fi
if [[ "$*" == "test -json ./internal/publisher -run ^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$ -count=1" ]]; then
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

// invariant: tooling/quality-gates:staged-test-selection (TestGateRunnerSelectsTestsFromStagedChanges)
func TestGateRunnerSelectsTestsFromStagedChanges(t *testing.T) {
	goTests := []string{"test ./... -coverpkg=./... -coverprofile=coverage.out", "run ./cmd/covercheck coverage.out"}
	piTests := []string{"TestPi(EffortMemoryToolContract|RealRuntimeSmoke)"}
	both := append(slices.Clone(goTests), piTests...)
	for _, tc := range []struct {
		name, path string
		want       []string
		notices    []string
	}{
		{"docs-only skips both suites", "docs/odd name [1].md", nil, []string{"gate: skipping Go tests and coverage for test-free staged changes", "gate: skipping Pi runtime smoke for test-free staged changes"}},
		{"Pi extension is Pi-only", ".pi/extensions/extension.ts", piTests, []string{"gate: skipping Go tests and coverage for Pi-only staged changes"}},
		{"Pi harness input without Go consumer is Pi-only", "tools/pi-extension-test/package.json", piTests, []string{"gate: skipping Go tests and coverage for Pi-only staged changes"}},
		{"ordinary Go is Go-only", "cmd/example/main.go", goTests, []string{"gate: skipping Pi runtime smoke for Go-only staged changes"}},
		{"Claude input is Go-only", ".claude/agents/reviewer.md", goTests, []string{"gate: skipping Pi runtime smoke for Go-only staged changes"}},
		{"unknown paths fail closed", "LICENSE", both, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, logPath := committedGateRunnerFixture(t)
			gitfixture.Stage(t, gitfixture.At(root), map[string]string{tc.path: "changed\n"})
			assertGateSelection(t, root, logPath, tc.want, tc.notices)
		})
	}

	t.Run("documentation allowlist skips both suites", func(t *testing.T) {
		root, logPath := committedGateRunnerFixture(t)
		gitfixture.Stage(t, gitfixture.At(root), map[string]string{
			"docs/guide.md": "changed\n", "README.md": "changed\n", "changelog/CHANGELOG.md": "changed\n",
			".awf/docs/parts/a.md": "changed\n", "templates/docs/a.md": "changed\n",
		})
		assertGateSelection(t, root, logPath, nil, []string{"gate: skipping Go tests and coverage for test-free staged changes", "gate: skipping Pi runtime smoke for test-free staged changes"})
	})

	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"exact version authority", map[string]string{"internal/project/VERSION": "0.40.0\n"}},
		{"exact root lock", map[string]string{".awf/awf.lock": "changed\n"}},
		{"release preparation inputs", map[string]string{
			"internal/project/VERSION": "0.40.0\n",
			".awf/awf.lock":            "changed\n",
			"changelog/CHANGELOG.md":   "changed\n",
		}},
	} {
		t.Run(tc.name+" skips both suites", func(t *testing.T) {
			root, logPath := committedGateRunnerFixture(t)
			gitfixture.Stage(t, gitfixture.At(root), tc.files)
			assertGateSelection(t, root, logPath, nil, []string{"gate: skipping Go tests and coverage for test-free staged changes", "gate: skipping Pi runtime smoke for test-free staged changes"})
		})
	}

	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"version plus unknown", map[string]string{"internal/project/VERSION": "0.40.0\n", "LICENSE": "changed\n"}},
		{"lock plus neighboring awf source", map[string]string{".awf/awf.lock": "changed\n", ".awf/config.yaml": "changed\n"}},
	} {
		t.Run(tc.name+" runs both suites", func(t *testing.T) {
			root, logPath := committedGateRunnerFixture(t)
			gitfixture.Stage(t, gitfixture.At(root), tc.files)
			assertGateSelection(t, root, logPath, both, nil)
		})
	}

	for _, path := range []string{
		"templates/pi/extension.ts.tmpl", ".pi/agents/reviewer.md", ".pi/skills/reviewer/SKILL.md", "x",
		"internal/project/target.go", "internal/project/VERSION.bak", ".awf/config.yaml", "internal/render/template.go", "internal/config/config.go", "internal/catalog/catalog.go", "templates/embed.go",
		".nvmrc", "tools/pi-extension-test/run.sh", "tools/pi-extension-test/tests/index.test.ts", "tools/pi-extension-test/tests/handoff.test.ts",
	} {
		t.Run("overlap "+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			root, logPath := committedGateRunnerFixture(t)
			if path == "x" {
				writeFakeGit(t, root, "printf 'x\\0'\n")
			} else {
				gitfixture.Stage(t, gitfixture.At(root), map[string]string{path: "changed\n"})
			}
			assertGateSelection(t, root, logPath, both, nil)
		})
	}

	t.Run("mixed Pi-only and Go-only changes run both suites", func(t *testing.T) {
		root, logPath := committedGateRunnerFixture(t)
		gitfixture.Stage(t, gitfixture.At(root), map[string]string{".pi/extensions/extension.ts": "changed\n", "cmd/example/main.go": "changed\n"})
		assertGateSelection(t, root, logPath, both, nil)
	})
	t.Run("empty and unborn repositories fail closed", func(t *testing.T) {
		root, logPath := committedGateRunnerFixture(t)
		assertGateSelection(t, root, logPath, both, nil)
		unborn, unbornLog := gateRunnerFixture(t)
		assertGateSelection(t, unborn, unbornLog, both, nil)
	})
	for _, tc := range []struct {
		name, body string
		want       []string
		notices    []string
	}{
		{"Git failure fails closed", "exit 17\n", both, nil},
		{"malformed snapshot fails closed", "printf 'docs/guide.md\\0templates/pi/index.ts'\n", both, nil},
		{"newline filename remains docs-only", "printf 'docs/line\\nbreak.md\\0'\n", nil, []string{"gate: skipping Go tests and coverage for test-free staged changes", "gate: skipping Pi runtime smoke for test-free staged changes"}},
		{"space filename remains Pi-only", "printf '.pi/extensions/with space.ts\\0'\n", piTests, []string{"gate: skipping Go tests and coverage for Pi-only staged changes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, logPath := committedGateRunnerFixture(t)
			writeFakeGit(t, root, tc.body)
			assertGateSelection(t, root, logPath, tc.want, tc.notices)
		})
	}
	t.Run("additions deletions and rename paths are each classified", func(t *testing.T) {
		root, logPath := gateRunnerFixture(t)
		repo := gitfixture.InitRepoAt(t, root)
		gitfixture.AddAll(t, repo)
		gitfixture.Commit(t, repo, "fixture", map[string]string{"docs/delete.md": "old\n", "cmd/old/main.go": "old\n"})
		gitfixture.StageRemoval(t, repo, "docs/delete.md")
		gitfixture.Stage(t, repo, map[string]string{".pi/extensions/added.ts": "new\n"})
		if err := os.Rename(filepath.Join(root, "cmd/old/main.go"), filepath.Join(root, ".pi/extensions/renamed.ts")); err != nil {
			t.Fatal(err)
		}
		gitfixture.AddAll(t, repo)
		assertGateSelection(t, root, logPath, both, nil)
	})
}

func committedGateRunnerFixture(t *testing.T) (string, string) {
	t.Helper()
	root, logPath := gateRunnerFixture(t)
	repo := gitfixture.InitRepoAt(t, root)
	gitfixture.AddAll(t, repo)
	gitfixture.Commit(t, repo, "fixture", nil)
	return root, logPath
}

func writeFakeGit(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, "fake-bin", "git")
	testsupport.WriteFile(t, path, "#!/usr/bin/env bash\n"+body)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertGateSelection(t *testing.T, root, logPath string, wantTests, wantNotices []string) {
	t.Helper()
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "./x", "gate", "timings")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"), "INVOCATION_LOG="+logPath, "AWF_PI_RUNTIME_SMOKE=1")
	stderr := new(strings.Builder)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("gate: %v: %s", err, stderr.String())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(data)
	for _, test := range wantTests {
		if !strings.Contains(joined, test) {
			t.Errorf("missing %q: %q", test, joined)
		}
	}
	for _, test := range []string{"test ./... -coverpkg=./... -coverprofile=coverage.out", "run ./cmd/covercheck coverage.out", "TestPi(EffortMemoryToolContract|RealRuntimeSmoke)"} {
		if !slices.Contains(wantTests, test) && strings.Contains(joined, test) {
			t.Errorf("unexpected %q: %q", test, joined)
		}
	}
	for _, notice := range wantNotices {
		if !strings.Contains(stderr.String(), notice) {
			t.Errorf("missing notice %q: %q", notice, stderr.String())
		}
	}
	for _, stage := range []string{
		"run ./cmd/versioncheck", "vet ./...",
		"goos=linux|goarch=arm64|pi=|build ./...",
		"goos=darwin|goarch=amd64|pi=|build ./...",
		"goos=darwin|goarch=arm64|pi=|build ./...",
		"goos=windows|goarch=amd64|pi=|build ./...",
		"goos=windows|goarch=arm64|pi=|build ./...",
		"golangci-lint run", "deadcode -json", "./cmd/pincheck",
	} {
		if !strings.Contains(joined, stage) {
			t.Errorf("unconditional stage %q did not run: %q", stage, joined)
		}
	}
	wantTimings := []string{"versioncheck"}
	if slices.Contains(wantTests, "test ./... -coverpkg=./... -coverprofile=coverage.out") {
		wantTimings = append(wantTimings, "go-test", "covercheck")
	}
	if slices.Contains(wantTests, "TestPi(EffortMemoryToolContract|RealRuntimeSmoke)") {
		wantTimings = append(wantTimings, "pi-runtime-smoke")
	}
	wantTimings = append(wantTimings, "vet", "build-linux-arm64", "build-darwin-amd64", "build-darwin-arm64", "build-windows-amd64", "build-windows-arm64", "lint", "advisory-lint", "deadcode", "pincheck")
	assertTimingLines(t, stderr.String(), wantTimings)
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
		"goos=|goarch=|pi=|run ./cmd/versioncheck",
		"goos=|goarch=|pi=|test ./... -coverpkg=./... -coverprofile=coverage.out",
		"goos=|goarch=|pi=|run ./cmd/covercheck coverage.out",
		"goos=|goarch=|pi=1|test -json ./internal/publisher -run ^TestPi(EffortMemoryToolContract|RealRuntimeSmoke)$ -count=1",
		"goos=|goarch=|pi=|vet ./...",
		"goos=linux|goarch=arm64|pi=|build ./...",
		"goos=darwin|goarch=amd64|pi=|build ./...",
		"goos=darwin|goarch=arm64|pi=|build ./...",
		"goos=windows|goarch=amd64|pi=|build ./...",
		"goos=windows|goarch=arm64|pi=|build ./...",
		"goos=|goarch=|pi=|tool golangci-lint run",
		"goos=|goarch=|pi=|tool golangci-lint run --config .golangci-advisory.yml --issues-exit-code 0",
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
