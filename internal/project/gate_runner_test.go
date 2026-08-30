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

// invariant: tooling/quality-gates:gate-tier-cadence (TestGateRunnerComposition)
func TestGateRunnerComposition(t *testing.T) {
	root := t.TempDir()
	runner, err := os.ReadFile("../../x")
	if err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(root, "x"), string(runner))
	if err := os.Chmod(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "calls")
	fakeBin := filepath.Join(root, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	testsupport.WriteFile(t, filepath.Join(fakeBin, "go"), "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> \"$INVOCATION_LOG\"\n")
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, int, []string) {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", append([]string{"./x"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "INVOCATION_LOG="+logPath)
		out, runErr := cmd.CombinedOutput()
		status := 0
		if runErr != nil {
			var exit *exec.ExitError
			if !errors.As(runErr, &exit) {
				t.Fatal(runErr)
			}
			status = exit.ExitCode()
		}
		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		var calls []string
		if text := strings.TrimSpace(string(raw)); text != "" {
			calls = strings.Split(text, "\n")
		}
		return string(out), status, calls
	}
	want := []string{"run ./cmd/versioncheck", "build ./...", "tool golangci-lint run", "run ./cmd/pincheck"}
	out, status, calls := run("gate", "timings")
	if status != 0 || !slices.Equal(calls, want) {
		t.Fatalf("status=%d output=%q calls=%q want=%q", status, out, calls, want)
	}
	for _, label := range []string{"versioncheck", "build", "lint", "pincheck"} {
		if !strings.Contains(out, "gate timing: "+label+" ") {
			t.Errorf("missing timing for %s: %q", label, out)
		}
	}
	out, status, calls = run("gate", "full")
	if status != 2 || len(calls) != 0 || !strings.Contains(out, "usage: ./x gate [timings]") {
		t.Fatalf("retired full gate accepted: status=%d output=%q calls=%q", status, out, calls)
	}

	// invariant: tooling/quality-gates:affected-package-feedback (TestGateRunnerComposition)
	_, status, calls = run("test-affected", "--staged")
	if status != 0 || !slices.Equal(calls, []string{"run ./cmd/testselection --execute --staged"}) {
		t.Fatalf("focused feedback status=%d calls=%q", status, calls)
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
