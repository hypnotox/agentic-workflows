package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/telemetry"
)

type dashboardRuntimeMetadata struct {
	FormatVersion  int      `json:"formatVersion"`
	RepositoryID   string   `json:"repositoryID"`
	ObjectFormat   string   `json:"objectFormat"`
	Commit         string   `json:"commit"`
	GoVersion      string   `json:"goVersion"`
	GoExperiment   string   `json:"goExperiment"`
	GoToolchain    string   `json:"goToolchain"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	GoFlags        []string `json:"goFlags"`
	PolicySchema   int      `json:"policySchema"`
	ProtocolMajor  int      `json:"protocolMajor"`
	BinarySHA256   string   `json:"binarySHA256"`
	LauncherSHA256 string   `json:"launcherSHA256"`
	PolicySHA256   string   `json:"policySHA256"`
}

type dashboardPolicySnapshot struct {
	SchemaVersion     int                              `json:"schemaVersion"`
	ProtocolMajor     int                              `json:"protocolMajor"`
	WorkflowTelemetry dashboardWorkflowTelemetryPolicy `json:"workflowTelemetry"`
}

type dashboardWorkflowTelemetryPolicy struct {
	Retention   dashboardRetentionPolicy   `json:"retention"`
	Widget      dashboardWidgetPolicy      `json:"widget"`
	Diagnostics dashboardDiagnosticsPolicy `json:"diagnostics"`
}

type dashboardRetentionPolicy struct {
	MaxCompletedEffortAgeDays int `json:"maxCompletedEffortAgeDays"`
	MaxCompletedEffortCount   int `json:"maxCompletedEffortCount"`
}

type dashboardWidgetPolicy struct {
	Enabled  bool `json:"enabled"`
	ShowCost bool `json:"showCost"`
}

type dashboardDiagnosticsPolicy struct {
	HeuristicsEnabled      bool                      `json:"heuristicsEnabled"`
	MinimumBaselineSamples int                       `json:"minimumBaselineSamples"`
	BaselinePercentile     int                       `json:"baselinePercentile"`
	Thresholds             dashboardThresholdsPolicy `json:"thresholds"`
}

type dashboardThresholdsPolicy struct {
	PhaseReentryCount         int `json:"phaseReentryCount"`
	PhaseDurationSeconds      int `json:"phaseDurationSeconds"`
	PhaseTokens               int `json:"phaseTokens"`
	CompactionCount           int `json:"compactionCount"`
	HandoffCount              int `json:"handoffCount"`
	ToolFailureCount          int `json:"toolFailureCount"`
	GateFailureCount          int `json:"gateFailureCount"`
	CacheReadPercentBelow     int `json:"cacheReadPercentBelow"`
	SubagentQueueWaitSeconds  int `json:"subagentQueueWaitSeconds"`
	ImplementationReworkCount int `json:"implementationReworkCount"`
}

type dashboardReadError struct {
	category string
	cause    error
}

func (e *dashboardReadError) Error() string {
	return "dashboard-read: " + e.category + ": " + e.cause.Error()
}
func (e *dashboardReadError) Unwrap() error { return e.cause }

func dashboardFailure(category string, err error) error {
	return &dashboardReadError{category: category, cause: err}
}

var dashboardExecutable = os.Executable
var dashboardReadFile = os.ReadFile
var dashboardLstat = os.Lstat
var dashboardEvalSymlinks = filepath.EvalSymlinks
var dashboardRunMetrics = runMetrics
var dashboardRunMetricsExport = runMetricsExportWith
var dashboardRunMetricsQuery = runMetricsQueryWith
var dashboardRunDoctor = runDoctorWith
var dashboardGitOutput = func(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func runDashboardRead(args []string, stdout io.Writer) error {
	root, child, context, err := parseDashboardReadArgs(args, stdout)
	if err != nil {
		return dashboardFailure("argv", err)
	}
	canonicalRoot, err := canonicalDashboardRoot(root)
	if err != nil {
		return dashboardFailure("project-root", err)
	}
	executable, err := dashboardExecutable()
	if err != nil {
		return dashboardFailure("path", err)
	}
	directory := filepath.Dir(executable)
	launcherPath := filepath.Join(directory, dashboardExecutableName("awf-dashboard"))
	metadataPath := filepath.Join(directory, "metadata.json")
	policyPath := filepath.Join(directory, "policy.json")
	for _, path := range []string{executable, launcherPath, metadataPath, policyPath} {
		if err := requireDashboardRegular(path); err != nil {
			return dashboardFailure("path", err)
		}
	}
	metadataBytes, err := dashboardReadFile(metadataPath)
	if err != nil {
		return dashboardFailure("metadata", err)
	}
	var metadata dashboardRuntimeMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return dashboardFailure("metadata", err)
	}
	canonicalMetadata, marshalErr := json.Marshal(metadata)
	if marshalErr != nil || !bytes.Equal(canonicalMetadata, metadataBytes) || metadata.FormatVersion != 1 || metadata.GOOS != runtime.GOOS || metadata.GOARCH != runtime.GOARCH || !validDashboardMetadataIdentity(metadata) {
		return dashboardFailure("metadata", errors.New("invalid runtime metadata"))
	}
	for _, check := range []struct{ path, digest string }{{executable, metadata.BinarySHA256}, {launcherPath, metadata.LauncherSHA256}, {policyPath, metadata.PolicySHA256}} {
		actual, digestErr := dashboardFileDigest(check.path)
		if digestErr != nil || actual != check.digest {
			return dashboardFailure("digest", errors.New("runtime artifact mismatch"))
		}
	}
	if err := verifyDashboardRepository(canonicalRoot, metadata); err != nil {
		return dashboardFailure("repository", err)
	}
	policyBytes, err := dashboardReadFile(policyPath)
	if err != nil {
		return dashboardFailure("policy", err)
	}
	var policy dashboardPolicySnapshot
	if err := json.Unmarshal(policyBytes, &policy); err != nil {
		return dashboardFailure("policy", err)
	}
	canonicalPolicy, marshalErr := json.Marshal(policy)
	compiledMajor, protocolErr := compiledTelemetryMajor()
	policyConfig := policy.WorkflowTelemetry.config()
	if marshalErr != nil || !bytes.Equal(canonicalPolicy, policyBytes) || policy.SchemaVersion != 1 || metadata.PolicySchema != policy.SchemaVersion || policy.ProtocolMajor != metadata.ProtocolMajor || protocolErr != nil || policy.ProtocolMajor != compiledMajor || !validDashboardPolicy(policyConfig) {
		return dashboardFailure("policy", errors.New("snapshot schema or protocol is incompatible; restart required"))
	}
	if err := verifyMetricsConfinement(canonicalRoot); err != nil {
		return dashboardFailure("storage", err)
	}
	if _, err := parseTelemetrySelector(context.inv); err != nil {
		return dashboardFailure("selector", err)
	}
	deps := metricsReadDeps{Root: canonicalRoot, Policy: policyConfig}
	var queryErr error
	switch {
	case child[0] == "metrics" && len(child) > 1 && child[1] == "protocol":
		queryErr = dashboardRunMetrics(context)
	case child[0] == "metrics" && len(child) > 1 && child[1] == "export":
		queryErr = dashboardRunMetricsExport(context, deps)
	case child[0] == "metrics":
		queryErr = dashboardRunMetricsQuery(context, deps)
	default:
		queryErr = dashboardRunDoctor(context, deps)
	}
	if queryErr != nil {
		return dashboardFailure("query", queryErr)
	}
	return nil
}

func parseDashboardReadArgs(args []string, stdout io.Writer) (string, []string, *cmdCtx, error) {
	if len(args) < 4 || args[0] != "--project-root" || args[2] != "--" {
		return "", nil, nil, errors.New("expected --project-root <root> -- <query>")
	}
	root, child := args[1], args[3:]
	if err := validateDashboardChildGrammar(child); err != nil {
		return "", nil, nil, err
	}
	inv := invocation{bools: map[string]bool{"--json": true}, values: map[string]string{}}
	var start int
	sub := ""
	switch {
	case child[0] == "metrics" && child[1] == "protocol":
		sub, start = "protocol", len(child)
	case child[0] == "metrics" && child[1] == "export":
		sub, inv.values["--format"], start = "export", "json", 4
	default:
		start = 2
	}
	for i := start; i < len(child); i += 2 {
		inv.values[child[i]] = child[i+1]
	}
	return root, child, &cmdCtx{root: root, sub: sub, inv: inv, stdout: stdout}, nil
}

func validateDashboardChildGrammar(args []string) error {
	if equalDashboardArgs(args, []string{"metrics", "protocol", "--json"}) {
		return nil
	}
	var start int
	switch {
	case len(args) >= 2 && args[0] == "metrics" && args[1] == "--json":
		start = 2
	case len(args) >= 4 && args[0] == "metrics" && args[1] == "export" && args[2] == "--format" && args[3] == "json":
		start = 4
	case len(args) >= 2 && args[0] == "doctor" && args[1] == "--json":
		start = 2
	default:
		return errors.New("query shape is not allowed")
	}
	order := map[string]int{"--effort": 0, "--session": 1, "--phase": 2, "--since": 3, "--until": 4}
	last := -1
	for i := start; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return errors.New("selector value is missing")
		}
		position, ok := order[args[i]]
		if !ok || position <= last || strings.HasPrefix(args[i+1], "-") {
			return errors.New("selectors must be unique and in fixed order")
		}
		last = position
	}
	return nil
}

func canonicalDashboardRoot(root string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", errors.New("project root must be absolute and clean")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return "", errors.New("project root must be canonical")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("project root must be a non-symlink directory")
	}
	return root, nil
}

func requireDashboardRegular(path string) error {
	info, err := dashboardLstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("runtime sibling is not a non-symlink regular file")
	}
	return nil
}

func validDashboardMetadataIdentity(metadata dashboardRuntimeMetadata) bool {
	if !filepath.IsAbs(metadata.RepositoryID) || (metadata.ObjectFormat != "sha1" && metadata.ObjectFormat != "sha256") {
		return false
	}
	length := 40
	if metadata.ObjectFormat == "sha256" {
		length = 64
	}
	if len(metadata.Commit) != length {
		return false
	}
	for _, character := range metadata.Commit {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func verifyDashboardRepository(root string, metadata dashboardRuntimeMetadata) error {
	common, err := dashboardGitOutput(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	identity, err := dashboardEvalSymlinks(common)
	if err != nil || filepath.Clean(identity) != metadata.RepositoryID {
		return errors.New("repository identity mismatch")
	}
	objectFormat, err := dashboardGitOutput(root, "rev-parse", "--show-object-format")
	if err != nil || objectFormat != metadata.ObjectFormat {
		return errors.New("repository object format mismatch")
	}
	if _, err := dashboardGitOutput(root, "cat-file", "-e", metadata.Commit+"^{commit}"); err != nil {
		return errors.New("pinned commit is absent")
	}
	return nil
}

func verifyMetricsConfinement(root string) error {
	awf := filepath.Join(root, config.DirName)
	if info, err := dashboardLstat(awf); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New(".awf is not a confined directory")
	}
	metrics := filepath.Join(awf, "metrics")
	info, err := dashboardLstat(metrics)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New(".awf/metrics is not a confined directory")
	}
	canonical, err := dashboardEvalSymlinks(metrics)
	if err != nil || !dashboardPathWithin(root, canonical) {
		return errors.New(".awf/metrics escapes the project")
	}
	return nil
}

func compiledTelemetryMajor() (int, error) {
	var descriptor struct {
		Version telemetry.ProtocolVersion `json:"version"`
	}
	if err := json.Unmarshal(telemetry.DescriptorBytes(), &descriptor); err != nil { // coverage-ignore: DescriptorBytes is a compile-time validated JSON constant
		return 0, err
	}
	return int(descriptor.Version.Major), nil
}

func (policy dashboardWorkflowTelemetryPolicy) config() config.WorkflowTelemetryConfig {
	thresholds := policy.Diagnostics.Thresholds
	return config.WorkflowTelemetryConfig{
		Retention: config.TelemetryRetentionConfig{MaxCompletedEffortAgeDays: policy.Retention.MaxCompletedEffortAgeDays, MaxCompletedEffortCount: policy.Retention.MaxCompletedEffortCount},
		Widget:    config.TelemetryWidgetConfig{Enabled: policy.Widget.Enabled, ShowCost: policy.Widget.ShowCost},
		Diagnostics: config.TelemetryDiagnosticsConfig{
			HeuristicsEnabled: policy.Diagnostics.HeuristicsEnabled, MinimumBaselineSamples: policy.Diagnostics.MinimumBaselineSamples, BaselinePercentile: policy.Diagnostics.BaselinePercentile,
			Thresholds: config.TelemetryThresholdsConfig{
				PhaseReentryCount: thresholds.PhaseReentryCount, PhaseDurationSeconds: thresholds.PhaseDurationSeconds, PhaseTokens: thresholds.PhaseTokens,
				CompactionCount: thresholds.CompactionCount, HandoffCount: thresholds.HandoffCount, ToolFailureCount: thresholds.ToolFailureCount,
				GateFailureCount: thresholds.GateFailureCount, CacheReadPercentBelow: thresholds.CacheReadPercentBelow,
				SubagentQueueWaitSeconds: thresholds.SubagentQueueWaitSeconds, ImplementationReworkCount: thresholds.ImplementationReworkCount,
			},
		},
	}
}

func validDashboardPolicy(policy config.WorkflowTelemetryConfig) bool {
	d := policy.Diagnostics
	if policy.Retention.MaxCompletedEffortAgeDays < 0 || policy.Retention.MaxCompletedEffortCount < 0 || d.MinimumBaselineSamples <= 0 || d.BaselinePercentile < 1 || d.BaselinePercentile > 100 {
		return false
	}
	t := d.Thresholds
	positive := []int{t.PhaseReentryCount, t.PhaseDurationSeconds, t.PhaseTokens, t.CompactionCount, t.HandoffCount, t.ToolFailureCount, t.GateFailureCount, t.SubagentQueueWaitSeconds, t.ImplementationReworkCount}
	for _, value := range positive {
		if value <= 0 {
			return false
		}
	}
	return t.CacheReadPercentBelow >= 0 && t.CacheReadPercentBelow <= 100
}

func dashboardFileDigest(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func dashboardExecutableName(base string) string {
	return dashboardExecutableNameForOS(base, runtime.GOOS)
}

func dashboardExecutableNameForOS(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

func dashboardPathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func equalDashboardArgs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
