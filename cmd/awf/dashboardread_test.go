package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestDashboardReadClosedArgvGrammar(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "project")
	allowed := [][]string{
		{"--project-root", root, "--", "metrics", "protocol", "--json"},
		{"--project-root", root, "--", "metrics", "--json"},
		{"--project-root", root, "--", "metrics", "--json", "--effort", "e", "--session", "s", "--phase", "implementation", "--since", "2026-01-01T00:00:00Z", "--until", "2026-01-02T00:00:00Z"},
		{"--project-root", root, "--", "metrics", "export", "--format", "json", "--until", "2026-01-02T00:00:00Z"},
		{"--project-root", root, "--", "doctor", "--json", "--phase", "implementation"},
	}
	for _, args := range allowed {
		if _, _, _, err := parseDashboardReadArgs(args, io.Discard); err != nil {
			t.Errorf("allowed %v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"--project-root", root, "metrics", "--json"},
		{"--project-root", root, "--", "metrics", "export", "--format", "jsonl"},
		{"--project-root", root, "--", "metrics", "--json", "--session", "s", "--effort", "e"},
		{"--project-root", root, "--", "metrics", "--json", "--effort", "e", "--effort", "other"},
		{"--project-root", root, "--", "metrics", "retain", "--json"},
		{"--project-root", root, "--", "metrics", "purge", "--effort", "e", "--confirm"},
		{"--project-root", root, "--", "metrics", "lifecycle", "--request", "-"},
		{"--project-root", root, "--", "doctor", "-j"},
		{"--project-root", root, "--", "doctor", "--json", "positional"},
	} {
		if _, _, _, err := parseDashboardReadArgs(args, io.Discard); err == nil {
			t.Errorf("forbidden argv accepted: %v", args)
		}
	}
}

// invariant: tooling/cli:version-compat-gate
// invariant: tooling/cli:metrics-and-doctor-command-contract
func TestDashboardReadForbiddenArgvCannotReachMutationDispatch(t *testing.T) {
	originalHandler := handlers["metrics"]
	t.Cleanup(func() { handlers["metrics"] = originalHandler })
	mutationCalled := false
	handlers["metrics"] = func(*cmdCtx) error { mutationCalled = true; return nil }
	originalExecutable := dashboardExecutable
	t.Cleanup(func() { dashboardExecutable = originalExecutable })
	executableCalled := false
	dashboardExecutable = func() (string, error) { executableCalled = true; return "", errors.New("unexpected") }

	var stdout, stderr bytes.Buffer
	code := run([]string{"awf", "dashboard-read", "--project-root", "/project", "--", "metrics", "purge", "--effort", "e", "--confirm"}, &stdout, &stderr)
	if code != 1 || mutationCalled || executableCalled || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "dashboard-read: argv:") {
		t.Fatalf("exit=%d mutation=%v executable=%v stdout=%q stderr=%q", code, mutationCalled, executableCalled, stdout.String(), stderr.String())
	}
}

func TestDashboardReadIsRecognizedBeforeWorkingDirectoryAndProjectGuard(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { t.Fatal("dashboard-read reached cwd resolution"); return "", nil })
	var stderr bytes.Buffer
	if code := run([]string{"awf", "dashboard-read"}, io.Discard, &stderr); code != 1 || !strings.HasPrefix(stderr.String(), "dashboard-read: argv:") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}

func TestSnapshotBackedReadDoesNotLoadAdvancedLiveSchema(t *testing.T) {
	root := telemetryProject(t)
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("schemaGeneration: 999999\nunknownFutureAuthority: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalNow := telemetryNow
	telemetryNow = func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { telemetryNow = originalNow })
	policy := config.WorkflowTelemetryConfig{
		Retention: config.TelemetryRetentionConfig{MaxCompletedEffortAgeDays: 17, MaxCompletedEffortCount: 23},
		Diagnostics: config.TelemetryDiagnosticsConfig{
			HeuristicsEnabled: true, MinimumBaselineSamples: 7, BaselinePercentile: 91,
			Thresholds: config.TelemetryThresholdsConfig{PhaseReentryCount: 2, PhaseDurationSeconds: 301, PhaseTokens: 401, CompactionCount: 3, HandoffCount: 4, ToolFailureCount: 5, GateFailureCount: 6, CacheReadPercentBelow: 12, SubagentQueueWaitSeconds: 31, ImplementationReworkCount: 8},
		},
	}
	var output bytes.Buffer
	context := &cmdCtx{inv: invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}, stdout: &output}
	if err := runMetricsQueryWith(context, metricsReadDeps{Root: root, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"maxAgeDays":17,"maxCount":23`) {
		t.Fatalf("snapshot retention absent: %s", output.String())
	}
	output.Reset()
	if err := runDoctorWith(context, metricsReadDeps{Root: root, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"findings":[]`) {
		t.Fatalf("snapshot doctor output = %s", output.String())
	}
}

func TestDashboardReadValidatesSnapshotAndDispatchesEveryReadShape(t *testing.T) {
	root := telemetryProject(t)
	directory := t.TempDir()
	executable := filepath.Join(directory, dashboardExecutableName("awf"))
	launcher := filepath.Join(directory, dashboardExecutableName("awf-dashboard"))
	policyPath := filepath.Join(directory, "policy.json")
	for path, content := range map[string]string{executable: "awf", launcher: "launcher"} {
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	policy := dashboardPolicySnapshot{SchemaVersion: 1, ProtocolMajor: 1, WorkflowTelemetry: dashboardWorkflowTelemetryPolicy{
		Retention: dashboardRetentionPolicy{MaxCompletedEffortAgeDays: 30, MaxCompletedEffortCount: 100},
		Widget:    dashboardWidgetPolicy{Enabled: true, ShowCost: true},
		Diagnostics: dashboardDiagnosticsPolicy{HeuristicsEnabled: true, MinimumBaselineSamples: 3, BaselinePercentile: 90, Thresholds: dashboardThresholdsPolicy{
			PhaseReentryCount: 1, PhaseDurationSeconds: 1, PhaseTokens: 1, CompactionCount: 1, HandoffCount: 1,
			ToolFailureCount: 1, GateFailureCount: 1, CacheReadPercentBelow: 0, SubagentQueueWaitSeconds: 1, ImplementationReworkCount: 1,
		}},
	}}
	major, err := compiledTelemetryMajor()
	if err != nil {
		t.Fatal(err)
	}
	policy.ProtocolMajor = major
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, policyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	binaryDigest, _ := dashboardFileDigest(executable)
	launcherDigest, _ := dashboardFileDigest(launcher)
	policyDigest, _ := dashboardFileDigest(policyPath)
	repositoryID := filepath.Join(root, ".git")
	if err := os.MkdirAll(repositoryID, 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := dashboardRuntimeMetadata{
		FormatVersion: 1, RepositoryID: repositoryID, ObjectFormat: "sha1", Commit: strings.Repeat("a", 40),
		GoVersion: "go", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoFlags: []string{"-buildvcs=false"},
		PolicySchema: 1, ProtocolMajor: major, BinarySHA256: binaryDigest, LauncherSHA256: launcherDigest, PolicySHA256: policyDigest,
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), metadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	oldExecutable, oldGit, oldRead, oldLstat := dashboardExecutable, dashboardGitOutput, dashboardReadFile, dashboardLstat
	oldMetrics, oldExport, oldQuery, oldDoctor := dashboardRunMetrics, dashboardRunMetricsExport, dashboardRunMetricsQuery, dashboardRunDoctor
	t.Cleanup(func() {
		dashboardExecutable, dashboardGitOutput, dashboardReadFile, dashboardLstat = oldExecutable, oldGit, oldRead, oldLstat
		dashboardRunMetrics, dashboardRunMetricsExport, dashboardRunMetricsQuery, dashboardRunDoctor = oldMetrics, oldExport, oldQuery, oldDoctor
	})
	dashboardExecutable = func() (string, error) { return executable, nil }
	goodGit := func(_ string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if args[len(args)-1] == "--git-common-dir" {
				return metadata.RepositoryID, nil
			}
			return metadata.ObjectFormat, nil
		case "cat-file":
			return "", nil
		default:
			return "", errors.New("unexpected git call")
		}
	}
	dashboardGitOutput = goodGit
	query := func(root string, child ...string) error {
		return runDashboardRead(append([]string{"--project-root", root, "--"}, child...), io.Discard)
	}
	for _, child := range [][]string{
		{"metrics", "protocol", "--json"},
		{"metrics", "--json"},
		{"metrics", "export", "--format", "json"},
		{"doctor", "--json"},
	} {
		var output bytes.Buffer
		args := append([]string{"--project-root", root, "--"}, child...)
		if err := runDashboardRead(args, &output); err != nil {
			t.Fatalf("runDashboardRead(%v): %v", child, err)
		}
		if output.Len() == 0 {
			t.Fatalf("runDashboardRead(%v) produced no output", child)
		}
	}
	assertCategory := func(want string, err error) {
		t.Helper()
		if err == nil || !strings.HasPrefix(err.Error(), "dashboard-read: "+want+":") {
			t.Fatalf("category %s error = %v", want, err)
		}
	}
	var runOut, runErr bytes.Buffer
	if code := run([]string{"awf", "dashboard-read", "--project-root", root, "--", "metrics", "protocol", "--json"}, &runOut, &runErr); code != 0 || runOut.Len() == 0 || runErr.Len() != 0 {
		t.Fatalf("run code=%d stdout=%q stderr=%q", code, runOut.String(), runErr.String())
	}
	dashboardReadFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == "metadata.json" {
			return nil, errors.New("injected")
		}
		return os.ReadFile(path)
	}
	assertCategory("metadata", query(root, "metrics", "--json"))
	dashboardReadFile = os.ReadFile
	dashboardRunMetricsQuery = func(*cmdCtx, metricsReadDeps) error { return errors.New("injected query") }
	assertCategory("query", query(root, "metrics", "--json"))
	dashboardRunMetricsQuery = oldQuery
	dashboardLstat = func(path string) (os.FileInfo, error) {
		if path == filepath.Join(root, ".awf") {
			return nil, errors.New("injected")
		}
		return os.Lstat(path)
	}
	assertCategory("storage", query(root, "metrics", "--json"))
	dashboardLstat = os.Lstat
	assertCategory("project-root", query("relative", "metrics", "--json"))
	dashboardExecutable = func() (string, error) { return "", errors.New("executable failed") }
	assertCategory("path", query(root, "metrics", "--json"))
	dashboardExecutable = func() (string, error) { return executable, nil }
	if err := os.Rename(launcher, launcher+".save"); err != nil {
		t.Fatal(err)
	}
	assertCategory("path", query(root, "metrics", "--json"))
	if err := os.Rename(launcher+".save", launcher); err != nil {
		t.Fatal(err)
	}
	metadata.FormatVersion = 0
	invalidMetadata, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), invalidMetadata, 0o600); err != nil {
		t.Fatal(err)
	}
	assertCategory("metadata", query(root, "metrics", "--json"))
	metadata.FormatVersion = 1
	metadataBytes, _ = json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertCategory("metadata", query(root, "metrics", "--json"))
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), metadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	assertCategory("digest", query(root, "metrics", "--json"))
	if err := os.WriteFile(launcher, []byte("launcher"), 0o700); err != nil {
		t.Fatal(err)
	}
	dashboardGitOutput = func(string, ...string) (string, error) { return "", errors.New("git failed") }
	assertCategory("repository", query(root, "metrics", "--json"))
	dashboardGitOutput = goodGit
	dashboardReadFile = func(path string) ([]byte, error) {
		if filepath.Base(path) == "policy.json" {
			return nil, errors.New("injected")
		}
		return os.ReadFile(path)
	}
	assertCategory("policy", query(root, "metrics", "--json"))
	dashboardReadFile = os.ReadFile
	policy.SchemaVersion = 0
	invalidPolicy, _ := json.Marshal(policy)
	if err := os.WriteFile(policyPath, invalidPolicy, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata.PolicySHA256, _ = dashboardFileDigest(policyPath)
	metadataBytes, _ = json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), metadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	assertCategory("policy", query(root, "metrics", "--json"))
	policy.SchemaVersion = 1
	if err := os.WriteFile(policyPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata.PolicySHA256, _ = dashboardFileDigest(policyPath)
	metadataBytes, _ = json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), metadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	assertCategory("policy", query(root, "metrics", "--json"))
	if err := os.WriteFile(policyPath, policyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	metadata.PolicySHA256, _ = dashboardFileDigest(policyPath)
	metadataBytes, _ = json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), metadataBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, selectorContext, err := parseDashboardReadArgs([]string{"--project-root", root, "--", "doctor", "--json", "--phase", "unknown"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseTelemetrySelector(selectorContext.inv); err == nil {
		t.Fatal("invalid selector accepted")
	}
	assertCategory("selector", query(root, "doctor", "--json", "--phase", "unknown"))
	wrapped := dashboardFailure("test", os.ErrNotExist)
	if !errors.Is(wrapped, os.ErrNotExist) {
		t.Fatal("dashboardReadError did not unwrap its cause")
	}
}

func TestDashboardRootAndSelectorValidationCategories(t *testing.T) {
	if output, err := dashboardGitOutput(filepath.Join("..", ".."), "rev-parse", "--show-toplevel"); err != nil || output == "" {
		t.Fatalf("git output = %q, %v", output, err)
	}
	if _, err := dashboardGitOutput(filepath.Join(t.TempDir(), "missing"), "rev-parse", "HEAD"); err == nil {
		t.Fatal("git output accepted missing repository")
	}
	if _, err := canonicalDashboardRoot("relative"); err == nil {
		t.Fatal("relative root accepted")
	}
	root := t.TempDir()
	for _, bad := range []string{filepath.Join(root, "missing"), filepath.Join(root, "file")} {
		if strings.HasSuffix(bad, "file") {
			if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := canonicalDashboardRoot(bad); err == nil {
			t.Fatalf("bad root %s accepted", bad)
		}
	}
	if err := requireDashboardRegular(root); err == nil {
		t.Fatal("directory accepted as regular file")
	}
	if validDashboardMetadataIdentity(dashboardRuntimeMetadata{}) || validDashboardMetadataIdentity(dashboardRuntimeMetadata{RepositoryID: root, ObjectFormat: "bad", Commit: strings.Repeat("a", 40)}) || validDashboardMetadataIdentity(dashboardRuntimeMetadata{RepositoryID: root, ObjectFormat: "sha1", Commit: "short"}) || validDashboardMetadataIdentity(dashboardRuntimeMetadata{RepositoryID: root, ObjectFormat: "sha1", Commit: strings.Repeat("z", 40)}) {
		t.Fatal("invalid metadata identity accepted")
	}
	if !validDashboardMetadataIdentity(dashboardRuntimeMetadata{RepositoryID: root, ObjectFormat: "sha256", Commit: strings.Repeat("a", 64)}) {
		t.Fatal("valid sha256 identity rejected")
	}
	validPolicy := config.WorkflowTelemetryConfig{Retention: config.TelemetryRetentionConfig{}, Diagnostics: config.TelemetryDiagnosticsConfig{MinimumBaselineSamples: 1, BaselinePercentile: 1, Thresholds: config.TelemetryThresholdsConfig{PhaseReentryCount: 1, PhaseDurationSeconds: 1, PhaseTokens: 1, CompactionCount: 1, HandoffCount: 1, ToolFailureCount: 1, GateFailureCount: 1, CacheReadPercentBelow: 0, SubagentQueueWaitSeconds: 1, ImplementationReworkCount: 1}}}
	if !validDashboardPolicy(validPolicy) {
		t.Fatal("valid policy rejected")
	}
	invalidPolicy := validPolicy
	invalidPolicy.Retention.MaxCompletedEffortAgeDays = -1
	if validDashboardPolicy(invalidPolicy) {
		t.Fatal("negative retention accepted")
	}
	invalidPolicy = validPolicy
	invalidPolicy.Diagnostics.Thresholds.PhaseTokens = 0
	if validDashboardPolicy(invalidPolicy) {
		t.Fatal("zero threshold accepted")
	}
	invalidPolicy = validPolicy
	invalidPolicy.Diagnostics.Thresholds.CacheReadPercentBelow = 101
	if validDashboardPolicy(invalidPolicy) {
		t.Fatal("cache threshold accepted")
	}
	if _, err := dashboardFileDigest(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing digest file accepted")
	}
	if dashboardExecutableNameForOS("awf", "windows") != "awf.exe" || dashboardExecutableNameForOS("awf", "linux") != "awf" {
		t.Fatal("dashboard executable naming mismatch")
	}
	if dashboardPathWithin(root, filepath.Dir(root)) || !dashboardPathWithin(root, root) {
		t.Fatal("path confinement mismatch")
	}
	withDot := root + string(filepath.Separator) + "."
	if _, err := canonicalDashboardRoot(withDot); err == nil {
		t.Fatal("non-canonical root accepted")
	}
	_, _, context, err := parseDashboardReadArgs([]string{"--project-root", root, "--", "doctor", "--json", "--phase", "unknown"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseTelemetrySelector(context.inv); err == nil {
		t.Fatal("invalid selector value accepted")
	}
}
