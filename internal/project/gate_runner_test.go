package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"gopkg.in/yaml.v3"
)

// invariant: tooling/quality-gates:gate-tier-cadence (TestGateRunnerModes)
// invariant: tooling/quality-gates:gate-severity-by-protected-property (TestGateRunnerModes)
// TestGateRunnerModes keeps the runner contract explicit: the fast composition
// has no behavioural work and full is its additive exhaustive composition.
func TestGateRunnerModes(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "x"))
	if err != nil {
		t.Fatal(err)
	}
	x := string(text)
	for _, want := range []string{"full)", "--range)", "run_gate_step versioncheck", "run_gate_step build go build ./...", "run_gate_step lint go tool golangci-lint run", "run_gate_step pincheck", "run_gate_step go-test env -u AWF_PI_RUNTIME_SMOKE go test -p=1 -timeout=20m ./...", "run_gate_step covercheck", "run_gate_step pi-runtime-smoke", "run_gate_step advisory-lint", "run_gate_step deadcode", "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"} {
		if !strings.Contains(x, want) {
			t.Errorf("runner missing %q", want)
		}
	}
	if strings.Contains(x, "select_gate_tests") || strings.Contains(x, "windows/amd64") {
		t.Error("runner retains obsolete staged selector or Windows target")
	}
	fast := x[strings.Index(x, "run_gate_step versioncheck"):strings.Index(x, "if [ \"$full\" = true ]")]
	for _, forbidden := range []string{"go-test", "covercheck", "pi-runtime", "advisory", "deadcode", "vet", "build-linux"} {
		if strings.Contains(fast, forbidden) {
			t.Errorf("fast gate contains %s", forbidden)
		}
	}
}

func TestGateRunnerGrammar(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "x"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "usage: ./x gate [full] [timings] [--range <base> <head>]") {
		t.Fatal("missing gate grammar")
	}
}

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

// invariant: tooling/quality-gates:covercheck-mutation-regression (TestCovercheckMutantsRunnerContract)
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
	if output, status, _ := run([]string{"REQUIRE_UNIX_SOCKET_BUDGET=1"}, "covercheck-mutants", "--evidence", "socket-budget-evidence", "--baseline", "-"); status != 0 {
		t.Fatalf("mutation temporary root exhausts the Unix socket path budget: status=%d output=%q", status, output)
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

// invariant: tooling/quality-gates:coverage-raw-identity-ratchet (TestCoverageActivationContracts)
// invariant: tooling/quality-gates:covercheck-mutation-regression (TestCoverageActivationContracts)
func TestCoverageActivationContracts(t *testing.T) {
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	text := string(runner)
	for _, required := range []string{
		`run_gate_step covercheck go run ./cmd/covercheck --policy "$prof" coverage-baseline.json`,
		`covercheck_mutants_selected staged`,
		`covercheck_mutants_selected ranges "${ranges[@]}"`,
		`run_gate_step covercheck-mutation-regression run_covercheck_mutants`,
		`timeout 1800s bash "$0" __covercheck-mutants-inner`,
		`evidence="$root/.cache/covercheck-mutants-evidence"`,
	} {
		if !strings.Contains(text, required) {
			t.Errorf("runner missing activation contract %q", required)
		}
	}
	if strings.Contains(text, `run_gate_step covercheck go run ./cmd/covercheck "$prof"`) {
		t.Error("runner retains the percentage-only coverage blocker")
	}
	if strings.Contains(text, `evidence="$root/.awf/efforts/`) {
		t.Error("runner retains mutation evidence in the managed awf config tree")
	}
	if strings.Index(text, `run_gate_step covercheck go run ./cmd/covercheck --policy "$prof" coverage-baseline.json`) > strings.Index(text, `run_gate_step covercheck-mutation-regression run_covercheck_mutants`) {
		t.Error("runner invokes the mutation blocker before policy evaluation")
	}

	workflow, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(workflow)
	for _, required := range []string{
		"fetch-depth: 0",
		"EVENT: ${{ github.event_name }}",
		"PR_BASE: ${{ github.event.pull_request.base.sha }}",
		"PUSH_BASE: ${{ github.event.before }}",
		`./x gate full --range "$base" "$CANDIDATE"`,
	} {
		if !strings.Contains(ci, required) {
			t.Errorf("CI missing mutation contract %q", required)
		}
	}
	if strings.Contains(ci, "covercheck-mutants") {
		t.Error("CI must delegate range-qualified mutation to the full gate")
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
  if [ "${REQUIRE_UNIX_SOCKET_BUDGET:-}" = 1 ]; then
    socket="$TMPDIR/TestFilesystemProjectReaderPathsExcludeUnsupportedEntries1234567890/001/.codegraph/daemon.sock"
    [ "${#socket}" -lt 108 ]
  fi
}
case "$*" in
  *"list -m"*) echo list-module >>"$INVOCATION_LOG"; echo v0.6.0 ;;
  "tool -n gremlins") echo tool-lookup >>"$INVOCATION_LOG"; echo /fake/gremlins ;;
  "version -m /fake/gremlins") echo tool-version >>"$INVOCATION_LOG"; printf '/fake/gremlins\n\tmod\tgithub.com/go-gremlins/gremlins\tv0.6.0\th1:fake\n' ;;
  *"list -f"*"TestGoFiles"*) require_tmp_env; echo list-tests >>"$INVOCATION_LOG"; if [ "${FAKE_TEST_CENSUS:-}" = wrong ]; then echo wrong_test.go; else printf 'main_test.go\npolicy_edge_test.go\n'; fi ;;
  *"list -f"*".Imports"*) require_tmp_env; echo list-imports >>"$INVOCATION_LOG"; echo github.com/hypnotox/agentic-workflows/internal/coverage ;;
  *"list -deps -test"*) require_tmp_env; echo list-deps >>"$INVOCATION_LOG"; [ "${FAKE_NO_DEP:-}" = 1 ] || echo github.com/hypnotox/agentic-workflows/internal/coverage ;;
  *"test -p=1 -timeout=20m -count=1 ./..."*) require_tmp_env; echo test >>"$INVOCATION_LOG" ;;
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
	fakeTimeout := `#!/usr/bin/env bash
set -euo pipefail
printf 'timeout %s\n' "$*" >>"$INVOCATION_LOG"
`
	testsupport.WriteFile(t, filepath.Join(fakeBin, "timeout"), fakeTimeout)
	if err := os.Chmod(filepath.Join(fakeBin, "timeout"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "invocations.log")
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, logPath
}
