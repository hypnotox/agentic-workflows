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

// invariant: tooling/quality-gates:gate-tier-cadence (TestGateRunnerModes)
// invariant: tooling/quality-gates:gate-severity-by-protected-property (TestGateRunnerModes)
// invariant: tooling/quality-gates:coverage-raw-identity-ratchet (TestGateRunnerModes)
// TestGateRunnerModes executes the runner against a command-recording fixture so
// the fast composition and additive full composition cannot drift by text shape alone.
func TestGateRunnerModes(t *testing.T) {
	root, logPath := gateRunnerFixture(t)
	shardEnvPath := filepath.Join(root, "shard-environments.log")
	testsupport.WriteFile(t, filepath.Join(root, "fake-bin", "git"), "#!/usr/bin/env bash\nif [[ \"$*\" == *'rev-parse --show-toplevel'* ]]; then pwd; exit 0; fi\nif [[ \"$*\" == *cat-file* ]]; then exit 0; fi\nif [[ \"$*\" == *diff* ]]; then [ -z \"${FAKE_GIT_CHANGED_PATH:-}\" ] || printf '%s\\0' \"$FAKE_GIT_CHANGED_PATH\"; exit 0; fi\nexit 0\n")
	if err := os.Chmod(filepath.Join(root, "fake-bin", "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, int, []string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(shardEnvPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"), "INVOCATION_LOG="+logPath, "SHARD_ENV_LOG="+shardEnvPath)
		out, err := cmd.CombinedOutput()
		status := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			status = exit.ExitCode()
		}
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		lines := []string{}
		if text := strings.TrimSpace(string(data)); text != "" {
			lines = strings.Split(text, "\n")
		}
		return string(out), status, lines
	}
	fastWant := []string{
		"goos=|goarch=|pi=|run ./cmd/versioncheck",
		"goos=|goarch=|pi=|build ./...",
		"goos=|goarch=|pi=|tool golangci-lint run",
		"goos=|goarch=|pi=|run ./cmd/pincheck",
	}
	fullExtra := []string{
		"go-shard-0-0", "go-shard-0-1", "go-shard-0-2", "go-shard-0-3",
		"go-shard-1-0", "go-shard-1-1", "go-shard-1-2", "go-shard-1-3",
		"go-shard-2-0", "go-shard-2-1", "go-shard-2-2",
		"go-shard-3-0",
		"coverage-merge",
		"goos=|goarch=|pi=|run ./cmd/covercheck --policy coverage.out coverage-baseline.json",
		"goos=|goarch=|pi=1|test -json ./internal/publisher -run ^TestPiRealRuntimeSmoke$ -count=1",
		"goos=|goarch=|pi=|vet ./...",
		"goos=|goarch=|pi=|tool golangci-lint run --config .golangci-advisory.yml --issues-exit-code 0",
		"goos=|goarch=|pi=|tool deadcode -json ./...",
		"goos=|goarch=|pi=|run ./cmd/deadcodecheck",
		"goos=linux|goarch=amd64|pi=|build ./...",
		"goos=linux|goarch=arm64|pi=|build ./...",
		"goos=darwin|goarch=amd64|pi=|build ./...",
		"goos=darwin|goarch=arm64|pi=|build ./...",
	}
	for _, tc := range []struct {
		name       string
		args, want []string
	}{
		{"fast", []string{"gate"}, fastWant},
		{"full", []string{"gate", "full", "--range", "base", "head"}, append(slices.Clone(fastWant), fullExtra...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, status, got := run(tc.args...)
			if status != 0 {
				t.Fatalf("status=%d output=%q", status, out)
			}
			if tc.name == "full" {
				assertShardContracts(t, got, shardEnvPath)
			}
			got = normalizeGateInvocations(got)
			if !slices.Equal(sortedStrings(got), sortedStrings(tc.want)) {
				t.Fatalf("invocations=%q, want %q", got, tc.want)
			}
			if tc.name == "full" {
				assertCoverageStageOrder(t, got)
			}
		})
	}
	_, _, fast := run("gate")
	_, _, full := run("gate", "full", "--range", "base", "head")
	fast = normalizeGateInvocations(fast)
	full = normalizeGateInvocations(full)
	for _, stage := range fullExtra {
		if slices.Contains(fast, stage) {
			t.Errorf("fast gate ran full stage %q", stage)
		}
	}
	for _, stage := range fastWant {
		if !slices.Contains(full, stage) {
			t.Errorf("full gate omitted fast stage %q", stage)
		}
	}

	out, status, _ := run("gate", "full", "timings", "--range", "base", "head")
	if status != 0 {
		t.Fatalf("timed full status=%d output=%q", status, out)
	}
	assertTimingLines(t, out, []string{"versioncheck", "build", "lint", "pincheck", "go-test", "coverage-merge", "covercheck", "pi-runtime-smoke", "vet", "advisory-lint", "deadcode", "platform-builds"})

	t.Run("affected feedback remains separate from the fast gate", func(t *testing.T) {
		out, status, got := run("test-affected", "--staged")
		if status != 0 {
			t.Fatalf("status=%d output=%q", status, out)
		}
		want := []string{"goos=|goarch=|pi=|run ./cmd/testselection --execute --staged"}
		if !slices.Equal(got, want) {
			t.Fatalf("invocations=%q, want %q", got, want)
		}
	})

	t.Run("owned range runs mutation last and preserves failure", func(t *testing.T) {
		t.Setenv("FAKE_GIT_CHANGED_PATH", "cmd/covercheck/main.go")
		out, status, lines := run("gate", "full", "timings", "--range", "base", "head")
		if status != 0 {
			t.Fatalf("owned full status=%d output=%q", status, out)
		}
		if len(lines) == 0 || !strings.HasPrefix(lines[len(lines)-1], "timeout 1800s bash ./x __covercheck-mutants-inner ") {
			t.Fatalf("mutation invocation missing or misplaced: %q", lines)
		}
		assertTimingLines(t, out, []string{"versioncheck", "build", "lint", "pincheck", "go-test", "coverage-merge", "covercheck", "pi-runtime-smoke", "vet", "advisory-lint", "deadcode", "platform-builds", "covercheck-mutation-regression"})

		t.Setenv("FAKE_TIMEOUT_STATUS", "23")
		out, status, _ = run("gate", "full", "timings", "--range", "base", "head")
		if status != 23 || strings.Count(out, "gate timing: covercheck-mutation-regression ") != 1 {
			t.Fatalf("mutation failure status=%d output=%q", status, out)
		}
	})
	for _, args := range [][]string{{"gate", "--range", "base", "head"}, {"gate", "full", "full"}, {"gate", "full", "--range", "base"}, {"gate", "unknown"}} {
		out, status, lines := run(args...)
		if status != 2 || !strings.Contains(out, "usage: ./x gate [full] [timings] [--range <base> <head>]") || len(lines) != 0 {
			t.Errorf("x %v: status=%d output=%q invocations=%q", args, status, out, lines)
		}
	}
	t.Run("failed shard prevents dependent coverage and waits for siblings", func(t *testing.T) {
		t.Setenv("FAKE_GO_FAIL_CONTAINS", "./internal/project")
		out, status, lines := run("gate", "full", "timings", "--range", "base", "head")
		normalized := normalizeGateInvocations(lines)
		if status != 17 || !strings.Contains(out, "gate: Go shard 1-0 failed with status 17") || slices.Contains(normalized, "coverage-merge") {
			t.Fatalf("status=%d output=%q invocations=%q", status, out, normalized)
		}
		for _, shard := range []string{"0-0", "0-1", "0-2", "0-3", "1-0", "1-1", "1-2", "1-3", "2-0", "2-1", "2-2", "3-0"} {
			if !slices.Contains(normalized, "go-shard-"+shard) {
				t.Errorf("sibling shard %s did not terminate: %q", shard, normalized)
			}
		}
	})
	t.Run("nonfinal platform failure remains blocking and all targets terminate", func(t *testing.T) {
		t.Setenv("FAKE_GO_FAIL_GOOS", "linux")
		t.Setenv("FAKE_GO_FAIL_GOARCH", "amd64")
		out, status, lines := run("gate", "full", "timings", "--range", "base", "head")
		if status != 17 || !strings.Contains(out, "gate: stage platform-builds failed with status 17") {
			t.Fatalf("status=%d output=%q invocations=%q", status, out, lines)
		}
		joined := strings.Join(lines, "\n")
		for _, target := range []string{"goos=linux|goarch=amd64", "goos=linux|goarch=arm64", "goos=darwin|goarch=amd64", "goos=darwin|goarch=arm64"} {
			if !strings.Contains(joined, target+"|pi=|build ./...") {
				t.Errorf("platform target %s did not terminate: %q", target, lines)
			}
		}
	})
	t.Run("parallel failures all report after every stage terminates", func(t *testing.T) {
		t.Setenv("FAKE_GO_FAIL_CONTAINS", "vet ./...")
		t.Setenv("FAKE_PI_RESULT", "fail")
		out, status, lines := run("gate", "full", "timings", "--range", "base", "head")
		joined := strings.Join(lines, "\n")
		if status != 1 || !strings.Contains(out, "gate: Pi runtime smoke proving units did not run and pass") ||
			!strings.Contains(out, "gate: stage vet failed with status 17") ||
			!strings.Contains(joined, "goos=linux|goarch=amd64|pi=|build ./...") {
			t.Fatalf("status=%d output=%q invocations=%q", status, out, lines)
		}
	})
}

func assertShardContracts(t *testing.T, invocations []string, environmentPath string) {
	t.Helper()
	provingUnit := regexp.MustCompile(` -run \^\(([^)]*)\)\$ `)
	for group, slices := range map[int]int{0: 4, 1: 4, 2: 3} {
		counts := map[string]int{}
		seenSlices := 0
		for _, invocation := range invocations {
			if !strings.Contains(invocation, "/shard"+strconv.Itoa(group)+"-") {
				continue
			}
			seenSlices++
			match := provingUnit.FindStringSubmatch(invocation)
			if len(match) != 2 {
				t.Fatalf("shard group %d omitted canonical proving-unit regex: %q", group, invocation)
			}
			for _, name := range strings.Split(match[1], "|") {
				counts[name]++
			}
		}
		if seenSlices != slices {
			t.Fatalf("shard group %d slices = %d, want %d", group, seenSlices, slices)
		}
		for _, name := range []string{"TestAlpha", "TestBeta", "TestGamma", "TestDelta"} {
			if counts[name] != 1 {
				t.Errorf("shard group %d proving unit %s count = %d, want 1", group, name, counts[name])
			}
		}
		if len(counts) != 4 {
			t.Errorf("shard group %d proving units = %#v", group, counts)
		}
	}

	data, err := os.ReadFile(environmentPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 12 {
		t.Fatalf("shard environment records = %d, want 12: %q", len(lines), data)
	}
	roots := map[string]bool{}
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			t.Fatalf("malformed shard environment record %q", line)
		}
		home, tmp, gotmp, gmp, invocation := parts[0], parts[1], parts[2], parts[3], parts[4]
		if home == "" || home != tmp || home != gotmp {
			t.Errorf("shard mutable roots not isolated together: HOME=%q TMPDIR=%q GOTMPDIR=%q", home, tmp, gotmp)
		}
		if roots[home] {
			t.Errorf("shard mutable root reused: %q", home)
		}
		roots[home] = true
		wantGMP := "1"
		if strings.Contains(invocation, "/shard3-0.out") {
			wantGMP = "2"
		}
		if gmp != wantGMP {
			t.Errorf("shard processor bound for %q = %s, want %s", invocation, gmp, wantGMP)
		}
	}
}

func normalizeGateInvocations(lines []string) []string {
	out := slices.Clone(lines)
	for i, line := range out {
		for _, shard := range []string{"0-0", "0-1", "0-2", "0-3", "1-0", "1-1", "1-2", "1-3", "2-0", "2-1", "2-2", "3-0"} {
			if strings.Contains(line, "|test -p=1 -timeout=20m -count=1 ") && strings.Contains(line, "/shard"+shard+".out") {
				out[i] = "go-shard-" + shard
			}
		}
		if strings.Contains(line, "|run ./cmd/covercheck --merge ") {
			out[i] = "coverage-merge"
		}
	}
	return out
}

func sortedStrings(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}

func assertCoverageStageOrder(t *testing.T, invocations []string) {
	t.Helper()
	merge := slices.Index(invocations, "coverage-merge")
	policy := slices.Index(invocations, "goos=|goarch=|pi=|run ./cmd/covercheck --policy coverage.out coverage-baseline.json")
	if merge < 0 || policy <= merge {
		t.Fatalf("coverage dependency order = %q", invocations)
	}
	for _, shard := range []string{"0-0", "0-1", "0-2", "0-3", "1-0", "1-1", "1-2", "1-3", "2-0", "2-1", "2-2", "3-0"} {
		if index := slices.Index(invocations, "go-shard-"+shard); index < 0 || index >= merge {
			t.Fatalf("shard %s did not complete before merge: %q", shard, invocations)
		}
	}
}

func assertTimingLines(t *testing.T, output string, want []string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^gate timing: ([a-z0-9-]+) ([0-9]+)s$`)
	var got []string
	for _, match := range pattern.FindAllStringSubmatch(output, -1) {
		got = append(got, match[1])
	}
	if !slices.Equal(got, want) {
		t.Errorf("timing labels=%q, want %q; output=%q", got, want, output)
	}
}

func TestTestPerformanceRunnerComposition(t *testing.T) {
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	text := string(runner)
	if !strings.Contains(text, "test-performance)\n    go run ./cmd/testperformance \"$@\"") {
		t.Fatal("x does not compose the qualification command")
	}
	gateStart := strings.Index(text, "gate)")
	lintStart := strings.Index(text, "lint)")
	if gateStart < 0 || lintStart <= gateStart {
		t.Fatal("x gate and lint case boundaries are missing or reordered")
	}
	if strings.Contains(text[gateStart:lintStart], "testperformance") {
		t.Fatal("fast gate must not run qualification reporting")
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
    GOCACHE) printf '/tmp/fake-go-cache\n/tmp/fake-mod-cache\n' ;;
    *) exit 2 ;;
  esac
  exit 0
fi
if [ "$*" = "list -f {{.ImportPath}} ./..." ]; then
  printf '%s\n' \
    github.com/hypnotox/agentic-workflows/cmd/awf \
    github.com/hypnotox/agentic-workflows/internal/project \
    github.com/hypnotox/agentic-workflows/internal/publisher \
    github.com/hypnotox/agentic-workflows/cmd/covercheck
  exit 0
fi
if [[ "$*" == test\ -list* ]]; then
  printf '%s\n' TestAlpha TestBeta TestGamma TestDelta
  exit 0
fi
printf 'goos=%s|goarch=%s|pi=%s|%s\n' "${GOOS:-}" "${GOARCH:-}" "${AWF_PI_RUNTIME_SMOKE:-}" "$*" >>"$INVOCATION_LOG"
if [[ "$*" == test\ -p=1* && "$*" == *"/shard"*".out"* ]]; then
  printf '%s|%s|%s|%s|%s\n' "$HOME" "$TMPDIR" "$GOTMPDIR" "$GOMAXPROCS" "$*" >>"$SHARD_ENV_LOG"
fi
if [ -n "${FAKE_GO_OUTPUT_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_OUTPUT_CONTAINS"* ]]; then
  printf '%s\n' "${FAKE_GO_OUTPUT:-}"
fi
if [ -n "${FAKE_GO_FAIL_CONTAINS:-}" ] && [[ "$*" == *"$FAKE_GO_FAIL_CONTAINS"* ]]; then
  exit 17
fi
if [ -n "${FAKE_GO_FAIL_GOOS:-}" ] && [ "${GOOS:-}" = "$FAKE_GO_FAIL_GOOS" ] && [ "${GOARCH:-}" = "${FAKE_GO_FAIL_GOARCH:-}" ]; then
  exit 17
fi
if [[ "$*" == "test -json ./internal/publisher -run ^TestPiRealRuntimeSmoke$ -count=1" ]]; then
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
exit "${FAKE_TIMEOUT_STATUS:-0}"
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
