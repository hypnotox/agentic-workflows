package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/testselection"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const fixturePolicy = `{
  "version":2,
  "lanes":[
    {"name":"go","patterns":["**/*.go","**/testdata/**","go.mod"]},
    {"name":"pi-runtime","patterns":["templates/pi/**"]},
    {"name":"render-template","patterns":["templates/**"]},
    {"name":"platform-sensitive","patterns":["**/*_linux.go"]},
    {"name":"release-archive","patterns":["cmd/releasecheck/**"]}
  ],
  "shared_path_patterns":["go.mod","test-selection.json","x"],
  "generated_go_patterns":["**/*_generated.go"]
}`

func selectionRepository(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitRepo(t)
	root := repo.Root()
	for filename, contents := range map[string]string{
		"go.mod":                "module example.test/selection\ngo 1.27\n",
		"test-selection.json":   fixturePolicy,
		"internal/leaf/leaf.go": "package leaf\n",
		"internal/user/user.go": "package user\nimport _ \"example.test/selection/internal/leaf\"\n",
		"cmd/meta/main.go":      "package main\nimport _ \"example.test/selection/internal/user\"\nfunc main() {}\n",
	} {
		full := filepath.Join(root, filename)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitfixture.AddAll(t, repo)
	gitfixture.Merge(t, repo, "base")
	return root
}

func decodeResult(t *testing.T, output []byte) testselection.Result {
	t.Helper()
	var result testselection.Result
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("machine output %q: %v", output, err)
	}
	return result
}

func TestRunEmitsStableMachineOnlyWorkingTreeSelection(t *testing.T) {
	root := selectionRepository(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "leaf", "leaf.go"), []byte("package leaf\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	result := decodeResult(t, stdout.Bytes())
	if result.Version != 2 || result.Outcome != "selected" || !reflect.DeepEqual(laneNames(result.Lanes), []string{"go"}) {
		t.Fatalf("result = %#v", result)
	}
	if got, want := packageNames(result.Packages), []string{"./cmd/meta", "./internal/leaf", "./internal/user"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("stdout is not one JSON document: %q", stdout.String())
	}
}

func laneNames(lanes []testselection.Lane) []string {
	names := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		names = append(names, lane.Name)
	}
	return names
}

func packageNames(packages []testselection.Package) []string {
	names := make([]string, 0, len(packages))
	for _, selected := range packages {
		names = append(names, selected.Path)
	}
	return names
}

// invariant: tooling/quality-gates:affected-package-feedback (TestExecuteUsesOneSortedNormallyScheduledGoInvocation)
func TestExecuteUsesOneSortedNormallyScheduledGoInvocation(t *testing.T) {
	oldRunner, oldCaches := runSelectedGoTests, readGoCaches
	t.Cleanup(func() { runSelectedGoTests, readGoCaches = oldRunner, oldCaches })
	readGoCaches = func(context.Context, string) (string, string, error) {
		return "/shared/go-build", "/shared/go-mod", nil
	}
	calls := 0
	var gotArgs, gotEnv []string
	runSelectedGoTests = func(_ context.Context, _ string, args, environment []string, _ io.Writer) error {
		calls++
		gotArgs = append([]string(nil), args...)
		gotEnv = append([]string(nil), environment...)
		return nil
	}
	result := testselection.Result{Packages: []testselection.Package{{Path: "./z"}, {Path: "./a"}, {Path: "./m"}}}
	if err := executeSelection(context.Background(), t.TempDir(), result, io.Discard); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("Go test calls = %d, want 1", calls)
	}
	want := []string{"test", "-count=1", "./a", "./m", "./z"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %v, want %v", gotArgs, want)
	}
	joined := strings.Join(gotEnv, "\n")
	if strings.Contains(joined, "GOMAXPROCS=") || strings.Contains(strings.Join(gotArgs, " "), "-p=1") {
		t.Fatalf("forced serialization remains: args=%v env=%v", gotArgs, gotEnv)
	}
	if !strings.Contains(joined, "GOCACHE=/shared/go-build") || !strings.Contains(joined, "GOMODCACHE=/shared/go-mod") {
		t.Fatalf("shared Go caches missing: %v", gotEnv)
	}
}

func TestGoCachePathsPreserveSpaces(t *testing.T) {
	goCache, moduleCache, err := parseGoCaches([]byte(`{"GOCACHE":"/home/a user/.cache/go-build","GOMODCACHE":"/home/a user/go/pkg/mod"}`))
	if err != nil || goCache != "/home/a user/.cache/go-build" || moduleCache != "/home/a user/go/pkg/mod" {
		t.Fatalf("GOCACHE=%q GOMODCACHE=%q err=%v", goCache, moduleCache, err)
	}
}

func TestRunExecuteKeepsStdoutMachineOnlyAndGoOutputOnStderr(t *testing.T) {
	root := selectionRepository(t)
	if err := os.WriteFile(filepath.Join(root, "internal", "leaf", "leaf.go"), []byte("package leaf\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--execute"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	result := decodeResult(t, stdout.Bytes())
	if result.Outcome != "selected" || !strings.Contains(stderr.String(), "example.test/selection/internal/leaf") {
		t.Fatalf("result=%#v stderr=%s", result, stderr.String())
	}
	if strings.Contains(stdout.String(), "ok\t") {
		t.Fatalf("Go output leaked into machine result: %q", stdout.String())
	}
}

func TestValidInvocationFailuresEmitARefusedMachineResult(t *testing.T) {
	root := selectionRepository(t)
	for _, args := range [][]string{
		{"testselection", "--root", root, "--policy", filepath.Join(root, "missing-policy.json")},
		{"testselection", "--root", filepath.Join(root, "missing-repository")},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("args=%v code=%d stderr=%s", args, code, stderr.String())
		}
		result := decodeResult(t, stdout.Bytes())
		if result.Version != 2 || result.Outcome != "refused" || len(result.Diagnostics) != 1 || result.Lanes == nil || result.Packages == nil {
			t.Fatalf("args=%v result=%#v", args, result)
		}
	}
}

func TestRunSupportsStagedAndRejectsConflictingModes(t *testing.T) {
	root := selectionRepository(t)
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{"internal/leaf/leaf.go": "package leaf\n// staged\n"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--staged"}, &stdout, &stderr); code != 0 {
		t.Fatalf("staged code=%d stderr=%s", code, stderr.String())
	}
	if result := decodeResult(t, stdout.Bytes()); result.Outcome != "selected" {
		t.Fatalf("staged result=%#v", result)
	}
	if code := run([]string{"testselection", "--staged", "--range", "HEAD..HEAD"}, &stdout, &stderr); code != 2 {
		t.Fatalf("usage code=%d", code)
	}
}

func TestRangeUsesRequestedHeadPolicyGraphAndSourceTree(t *testing.T) {
	root := selectionRepository(t)
	gitfixture.Stage(t, gitfixture.At(root), map[string]string{
		"templates/pi/new.tmpl":  "head template\n",
		"internal/newpkg/new.go": "package newpkg\n",
	})
	gitfixture.Merge(t, gitfixture.At(root), "head")
	// Corrupt the invoking checkout after the commit. Selection must clone and
	// evaluate the requested head, including its policy and newly added package.
	if err := os.WriteFile(filepath.Join(root, "test-selection.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "internal", "newpkg")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"testselection", "--root", root, "--range", "HEAD~1..HEAD"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	result := decodeResult(t, stdout.Bytes())
	if got, want := laneNames(result.Lanes), []string{"go", "pi-runtime", "render-template"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("lanes = %v, want %v; result=%#v", got, want, result)
	}
	if got, want := packageNames(result.Packages), []string{"./internal/newpkg"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v; result=%#v", got, want, result)
	}
}

func TestAffectedAndFullCommandsUseTheSameGoTestSemanticsForSelectedPackages(t *testing.T) {
	result := testselection.Result{Packages: []testselection.Package{{Path: "./internal/a"}, {Path: "./internal/b"}}}
	oldRunner, oldCaches := runSelectedGoTests, readGoCaches
	t.Cleanup(func() { runSelectedGoTests, readGoCaches = oldRunner, oldCaches })
	readGoCaches = func(context.Context, string) (string, string, error) { return "/go-cache", "/mod-cache", nil }
	var affected []string
	runSelectedGoTests = func(_ context.Context, _ string, args, _ []string, _ io.Writer) error {
		affected = append([]string(nil), args...)
		return nil
	}
	if err := executeSelection(context.Background(), t.TempDir(), result, io.Discard); err != nil {
		t.Fatal(err)
	}
	full := goTestArgs([]string{"./..."})
	if len(affected) < 2 || !reflect.DeepEqual(affected[:2], full[:2]) {
		t.Fatalf("affected command %v and full command %v have different Go test semantics", affected, full)
	}
}

func TestFullSuiteEnvironmentExcludesPiRuntimeSmoke(t *testing.T) {
	environment := environmentWithout([]string{"PATH=/bin", "AWF_PI_RUNTIME_SMOKE=1", "HOME=/tmp/home"}, "AWF_PI_RUNTIME_SMOKE")
	if got, want := environment, []string{"PATH=/bin", "HOME=/tmp/home"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("environment = %v, want %v", got, want)
	}
}

func withLinuxPlatform(t *testing.T) {
	t.Helper()
	old := currentPlatform
	currentPlatform = func() (string, string) { return "linux", "amd64" }
	t.Cleanup(func() { currentPlatform = old })
}

func isolateFullLinuxTestEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AWF_FULL_LINUX_CEILING", "")
	t.Setenv("AWF_FULL_LINUX_TIMING_ARTIFACT", "")
}

func TestFullLinuxCalibrationRecordsRawEvidenceWithoutInventingCeiling(t *testing.T) {
	isolateFullLinuxTestEnvironment(t)
	withLinuxPlatform(t)
	old := runFullSuite
	t.Cleanup(func() { runFullSuite = old })
	var gotArgs []string
	runFullSuite = func(_ context.Context, _ string, args []string, _ io.Writer) (time.Duration, error) {
		gotArgs = append([]string(nil), args...)
		return 187*time.Second + 250*time.Millisecond, nil
	}
	artifactPath := filepath.Join(t.TempDir(), "timing.json")
	var stdout, stderr bytes.Buffer
	if code := runFullLinux([]string{"--mode", "calibrate", "--artifact", artifactPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	artifact := readTimingArtifact(t, artifactPath)
	if artifact.Status != "passed" || artifact.WarningCeilingMS != 0 || artifact.DurationMS != 187250 || artifact.HardTimeoutMS != (10*time.Minute).Milliseconds() {
		t.Fatalf("artifact = %#v", artifact)
	}
	if want := []string{"test", "-count=1", "./..."}; !reflect.DeepEqual(gotArgs, want) || !reflect.DeepEqual(artifact.Command, append([]string{"go"}, want...)) {
		t.Fatalf("command args=%v artifact=%v", gotArgs, artifact.Command)
	}
}

func TestFullLinuxBudgetWarnsWithoutFailureAndRecordsBudget(t *testing.T) {
	isolateFullLinuxTestEnvironment(t)
	withLinuxPlatform(t)
	old := runFullSuite
	t.Cleanup(func() { runFullSuite = old })
	runFullSuite = func(_ context.Context, _ string, _ []string, _ io.Writer) (time.Duration, error) {
		return 4*time.Minute + time.Second, nil
	}
	artifactPath := filepath.Join(t.TempDir(), "timing.json")
	var stdout, stderr bytes.Buffer
	if code := runFullLinux([]string{"--mode", "budget", "--ceiling", "4m", "--artifact", artifactPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("warning failed healthy suite: code=%d stderr=%s", code, stderr.String())
	}
	artifact := readTimingArtifact(t, artifactPath)
	if artifact.Status != "warning" || artifact.WarningCeilingMS != (4*time.Minute).Milliseconds() || artifact.HardTimeoutMS != (12*time.Minute).Milliseconds() || !strings.Contains(stderr.String(), "WARNING:") {
		t.Fatalf("artifact=%#v stderr=%s", artifact, stderr.String())
	}
}

func TestFullLinuxHardTimeoutAndFailureWriteArtifactsAndFail(t *testing.T) {
	isolateFullLinuxTestEnvironment(t)
	withLinuxPlatform(t)
	oldRunner, oldContext := runFullSuite, fullLinuxBudgetContext
	t.Cleanup(func() { runFullSuite, fullLinuxBudgetContext = oldRunner, oldContext })
	fullLinuxBudgetContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		return context.WithDeadline(parent, time.Now().Add(-time.Nanosecond))
	}
	runFullSuite = func(ctx context.Context, _ string, _ []string, _ io.Writer) (time.Duration, error) {
		<-ctx.Done()
		return time.Second, ctx.Err()
	}
	artifactPath := filepath.Join(t.TempDir(), "timeout.json")
	var stdout, stderr bytes.Buffer
	if code := runFullLinux([]string{"--mode", "budget", "--ceiling", "1m", "--artifact", artifactPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("timeout code=%d stderr=%s", code, stderr.String())
	}
	if artifact := readTimingArtifact(t, artifactPath); artifact.Status != "timeout" || artifact.HardTimeoutMS != (10*time.Minute).Milliseconds() {
		t.Fatalf("timeout artifact=%#v", artifact)
	}

	fullLinuxBudgetContext = context.WithTimeout
	runFullSuite = func(context.Context, string, []string, io.Writer) (time.Duration, error) {
		return time.Second, errors.New("suite failed")
	}
	artifactPath = filepath.Join(t.TempDir(), "failed.json")
	stderr.Reset()
	if code := runFullLinux([]string{"--mode", "calibrate", "--artifact", artifactPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("failure code=%d stderr=%s", code, stderr.String())
	}
	if artifact := readTimingArtifact(t, artifactPath); artifact.Status != "failed" {
		t.Fatalf("failure artifact=%#v", artifact)
	}
}

func TestFullLinuxRequiresHostedEvidenceCeilingAndExactPlatform(t *testing.T) {
	// Reproduce the hosted outer command's timing environment before applying
	// the missing-ceiling test's isolated environment contract.
	t.Setenv("AWF_FULL_LINUX_CEILING", "4m")
	t.Setenv("AWF_FULL_LINUX_TIMING_ARTIFACT", filepath.Join(t.TempDir(), "ambient.json"))
	isolateFullLinuxTestEnvironment(t)
	oldPlatform, oldRunner := currentPlatform, runFullSuite
	t.Cleanup(func() { currentPlatform, runFullSuite = oldPlatform, oldRunner })
	currentPlatform = func() (string, string) { return "linux", "amd64" }
	runFullSuite = func(context.Context, string, []string, io.Writer) (time.Duration, error) {
		return 0, errors.New("full suite must not run while validating arguments")
	}
	var stdout, stderr bytes.Buffer
	if code := runFullLinux([]string{"--mode", "budget", "--artifact", filepath.Join(t.TempDir(), "x")}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "evidence-derived") {
		t.Fatalf("missing ceiling code=%d stderr=%s", code, stderr.String())
	}
	currentPlatform = func() (string, string) { return "darwin", "arm64" }
	stderr.Reset()
	if code := runFullLinux([]string{"--mode", "calibrate", "--artifact", filepath.Join(t.TempDir(), "x")}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "requires linux/amd64") {
		t.Fatalf("platform code=%d stderr=%s", code, stderr.String())
	}
}

func TestXFullLinuxBudgetRefusesMissingReviewedCeilingBeforeExecution(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", filepath.Join(root, "x"), "test-full-linux", "budget")
	cmd.Dir = root
	cmd.Env = removeEnvironment(os.Environ(), "AWF_FULL_LINUX_CEILING", "AWF_FULL_LINUX_TIMING_ARTIFACT")
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 || !bytes.Contains(output, []byte("reviewed hosted evidence")) {
		t.Fatalf("err=%v output=%s", err, output)
	}
}

func removeEnvironment(environment []string, names ...string) []string {
	removed := make(map[string]bool, len(names))
	for _, name := range names {
		removed[name] = true
	}
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if !removed[name] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func readTimingArtifact(t *testing.T, filename string) timingArtifact {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var artifact timingArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	return artifact
}
