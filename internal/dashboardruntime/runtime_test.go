package dashboardruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const fixturePolicy = `{"schemaVersion":1,"protocolMajor":2,"workflowTelemetry":{"retention":{"maxCompletedEffortAgeDays":17,"maxCompletedEffortCount":23},"widget":{"enabled":true,"showCost":false},"diagnostics":{"heuristicsEnabled":true,"minimumBaselineSamples":7,"baselinePercentile":91,"thresholds":{"phaseReentryCount":2,"phaseDurationSeconds":301,"phaseTokens":401,"compactionCount":3,"handoffCount":4,"toolFailureCount":5,"gateFailureCount":6,"cacheReadPercentBelow":12,"subagentQueueWaitSeconds":31,"implementationReworkCount":8}}}}`

type runtimeFixture struct {
	root  string
	cache string
	head  plumbing.Hash
	env   BuildEnvironment
}

func newRuntimeFixture(t *testing.T) runtimeFixture {
	t.Helper()
	repo, root := gitfixture.InitRepo(t)
	for _, dir := range []string{"cmd/awf", "cmd/awf-dashboard-launcher", ".awf"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	head := gitfixture.Commit(t, repo, root, "feat(awf): fixture runtime", map[string]string{
		"go.mod":                             "module example.com/dashboardfixture\n\ngo 1.26\n",
		"cmd/awf/main.go":                    "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"pinned-awf\")}\n",
		"cmd/awf-dashboard-launcher/main.go": "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"pinned-launcher\")}\n",
		".awf/config.yaml": `workflowTelemetry:
  retention:
    maxCompletedEffortAgeDays: 17
    maxCompletedEffortCount: 23
  widget:
    enabled: true
    showCost: false
  diagnostics:
    heuristicsEnabled: true
    minimumBaselineSamples: 7
    baselinePercentile: 91
    thresholds:
      phaseReentryCount: 2
      phaseDurationSeconds: 301
      phaseTokens: 401
      compactionCount: 3
      handoffCount: 4
      toolFailureCount: 5
      gateFailureCount: 6
      cacheReadPercentBelow: 12
      subagentQueueWaitSeconds: 31
      implementationReworkCount: 8
`,
	})
	cache := filepath.Join(t.TempDir(), "xdg")
	return runtimeFixture{
		root:  root,
		cache: cache,
		head:  head,
		env: BuildEnvironment{
			GOOS:         runtime.GOOS,
			GOARCH:       runtime.GOARCH,
			GoBinary:     goBinary(t),
			XDGCacheHome: cache,
			Stderr:       new(bytes.Buffer),
		},
	}
}

func goBinary(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// invariant: tooling/dashboard-runtime:pinned-development-runtime-cache
func TestResolveInitializesAbsentRefAtHEADAtomically(t *testing.T) {
	fixture := newRuntimeFixture(t)
	var stderr bytes.Buffer
	fixture.env.Stderr = &stderr

	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	if !launcher.Initialized {
		t.Fatal("first resolve did not report ref initialization")
	}
	if launcher.Commit != fixture.head.String() {
		t.Fatalf("commit = %s, want HEAD %s", launcher.Commit, fixture.head)
	}
	if !strings.Contains(stderr.String(), "refs/awf/dashboard-runtime") || !strings.Contains(stderr.String(), fixture.head.String()) {
		t.Fatalf("initialization diagnostic = %q", stderr.String())
	}
	if got := gitOutput(t, fixture.root, "rev-parse", "refs/awf/dashboard-runtime^{commit}"); got != fixture.head.String() {
		t.Fatalf("runtime ref = %s, want %s", got, fixture.head)
	}
}

func TestResolveConcurrentAbsentRefInitializationConverges(t *testing.T) {
	fixture := newRuntimeFixture(t)
	start := make(chan struct{})
	results := make(chan Launcher, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			launcher, err := Resolve(fixture.root, fixture.env)
			results <- launcher
			errs <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	err1, err2 := <-errs, <-errs
	if err1 != nil || err2 != nil {
		t.Fatalf("concurrent Resolve errors = %v, %v", err1, err2)
	}
	if first.Commit != fixture.head.String() || second.Commit != fixture.head.String() || first.Path != second.Path {
		t.Fatalf("concurrent results diverged: %+v %+v", first, second)
	}
	initialized := 0
	if first.Initialized {
		initialized++
	}
	if second.Initialized {
		initialized++
	}
	if initialized != 1 {
		t.Fatalf("Initialized count = %d, want 1", initialized)
	}
}

func TestAdvancePeelsExplicitRevisionToCommit(t *testing.T) {
	fixture := newRuntimeFixture(t)
	if _, err := Resolve(fixture.root, fixture.env); err != nil {
		t.Fatal(err)
	}
	second := commitRuntime(t, fixture, "second")
	gitRun(t, fixture.root, []string{"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@example.com"}, "tag", "-a", "candidate", "-m", "candidate", second.String())

	result, err := Advance(fixture.root, "candidate", fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	if result.OldCommit != fixture.head.String() || result.NewCommit != second.String() {
		t.Fatalf("advance = %+v, want %s -> %s", result, fixture.head, second)
	}
	if got := gitOutput(t, fixture.root, "rev-parse", "refs/awf/dashboard-runtime^{commit}"); got != second.String() {
		t.Fatalf("advanced ref = %s, want %s", got, second)
	}
}

func TestResolveBuildsPinnedCommitInIsolationFromDirtyCheckout(t *testing.T) {
	fixture := newRuntimeFixture(t)
	if err := os.WriteFile(filepath.Join(fixture.root, "cmd/awf/main.go"), []byte("package main\nthis is dirty and does not compile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "go.work"), []byte("invalid dirty workspace\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	awfPath := filepath.Join(filepath.Dir(launcher.Path), executableName("awf"))
	command := exec.Command(awfPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run pinned awf: %v: %s", err, output)
	}
	if string(output) != "pinned-awf" {
		t.Fatalf("pinned awf output = %q", output)
	}
}

func TestResolveUsesNormalizedGoEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell logging wrapper is Unix-only")
	}
	fixture := newRuntimeFixture(t)
	logPath := filepath.Join(t.TempDir(), "go-invocations.log")
	wrapper := filepath.Join(t.TempDir(), "go-wrapper")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'args=%%s|GOWORK=%%s|GOFLAGS=%%s|CGO_ENABLED=%%s|GOOS=%%s|GOARCH=%%s\\n' \"$*\" \"$GOWORK\" \"$GOFLAGS\" \"$CGO_ENABLED\" \"$GOOS\" \"$GOARCH\" >> %q\nexec %q \"$@\"\n", logPath, goBinary(t))
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	fixture.env.GoBinary = wrapper
	t.Setenv("GOWORK", "/dirty/workspace")
	t.Setenv("GOFLAGS", "-tags=dirty -race")
	t.Setenv("CGO_ENABLED", "1")

	if _, err := Resolve(fixture.root, fixture.env); err != nil {
		t.Fatal(err)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	builds := 0
	for _, line := range strings.Split(strings.TrimSpace(string(logBytes)), "\n") {
		if !strings.Contains(line, "args=build ") {
			continue
		}
		builds++
		for _, want := range []string{"-buildvcs=false", "|GOWORK=off|", "|GOFLAGS=|", "|CGO_ENABLED=0|", "|GOOS=" + runtime.GOOS + "|", "|GOARCH=" + runtime.GOARCH} {
			if !strings.Contains(line, want) {
				t.Errorf("build invocation %q does not contain %q", line, want)
			}
		}
		if strings.Contains(line, "-tags") {
			t.Errorf("build inherited tags: %q", line)
		}
	}
	if builds != 2 {
		t.Fatalf("build invocation count = %d, want 2; log:\n%s", builds, logBytes)
	}
}

func TestResolveReusesContentAddressedPublishedEntry(t *testing.T) {
	fixture := newRuntimeFixture(t)
	first, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	before := entryState(t, filepath.Dir(first.Path))
	time.Sleep(20 * time.Millisecond)
	second, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != second.Path || first.CacheKey != second.CacheKey || second.Initialized {
		t.Fatalf("cache was not reused: first=%+v second=%+v", first, second)
	}
	after := entryState(t, filepath.Dir(second.Path))
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("published entry was rewritten:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestResolveRecoversOnlyIncompleteSameKeyStaging(t *testing.T) {
	fixture := newRuntimeFixture(t)
	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Dir(launcher.Path)
	cacheRoot := filepath.Dir(entry)
	if err := os.RemoveAll(entry); err != nil {
		t.Fatal(err)
	}
	incomplete := filepath.Join(cacheRoot, "."+launcher.CacheKey+".tmp-0123456789abcdef0123456789abcdef")
	other := filepath.Join(cacheRoot, ".other.tmp-0123456789abcdef0123456789abcdef")
	for _, path := range []string{incomplete, other} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(incomplete, "partial"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Path != launcher.Path {
		t.Fatalf("recovered path = %s, want %s", resolved.Path, launcher.Path)
	}
	if _, err := os.Stat(incomplete); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("same-key incomplete staging remains: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated staging was removed: %v", err)
	}
}

func TestResolvePublishesExactEntryAtomically(t *testing.T) {
	fixture := newRuntimeFixture(t)
	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Dir(launcher.Path)
	entries, err := os.ReadDir(entry)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, item := range entries {
		if item.IsDir() {
			t.Fatalf("published entry contains directory %s", item.Name())
		}
		got = append(got, item.Name())
	}
	sort.Strings(got)
	want := []string{executableName("awf"), executableName("awf-dashboard"), "metadata.json", "policy.json"}
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("published files = %v, want %v", got, want)
	}
	cacheEntries, err := os.ReadDir(filepath.Dir(entry))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range cacheEntries {
		if strings.HasPrefix(item.Name(), "."+launcher.CacheKey+".tmp-") {
			t.Fatalf("staging visible after publication: %s", item.Name())
		}
	}
}

func TestResolveRejectsUnsafeCachePath(t *testing.T) {
	fixture := newRuntimeFixture(t)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture.cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(fixture.cache, "awf")); err != nil {
		t.Fatal(err)
	}

	_, err := Resolve(fixture.root, fixture.env)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Resolve error = %v, want ErrUnsafePath", err)
	}
}

func TestResolveRejectsPublishedMetadataCollision(t *testing.T) {
	fixture := newRuntimeFixture(t)
	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(filepath.Dir(launcher.Path), "metadata.json")
	var metadata map[string]any
	decodeJSONFile(t, metadataPath, &metadata)
	metadata["commit"] = strings.Repeat("0", len(launcher.Commit))
	writeCompactJSON(t, metadataPath, metadata)

	_, err = Resolve(fixture.root, fixture.env)
	if !errors.Is(err, ErrCacheCollision) {
		t.Fatalf("Resolve error = %v, want ErrCacheCollision", err)
	}
}

func TestResolvePublishesCompleteCanonicalPolicySnapshot(t *testing.T) {
	fixture := newRuntimeFixture(t)
	launcher, err := Resolve(fixture.root, fixture.env)
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(filepath.Dir(launcher.Path), "policy.json")
	got, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != fixturePolicy {
		t.Fatalf("policy.json =\n%s\nwant canonical:\n%s", got, fixturePolicy)
	}
	if !json.Valid(got) {
		t.Fatalf("policy is not JSON: %s", got)
	}
	if bytes.ContainsAny(got, "\n\r\t") || len(bytes.Fields(got)) != 1 {
		t.Fatalf("policy is not compact JSON: %s", got)
	}
}

func TestResolveDetectsArtifactAndPolicyDigestTampering(t *testing.T) {
	for _, name := range []string{executableName("awf"), "policy.json"} {
		t.Run(name, func(t *testing.T) {
			fixture := newRuntimeFixture(t)
			launcher, err := Resolve(fixture.root, fixture.env)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(filepath.Dir(launcher.Path), name)
			if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = Resolve(fixture.root, fixture.env)
			if !errors.Is(err, ErrSnapshot) {
				t.Fatalf("Resolve error = %v, want ErrSnapshot", err)
			}
		})
	}
}

func TestAdvanceUsesCompareAndSwapAgainstCapturedRef(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synchronized shell wrapper is Unix-only")
	}
	fixture := newRuntimeFixture(t)
	if _, err := Resolve(fixture.root, fixture.env); err != nil {
		t.Fatal(err)
	}
	left := commitRuntime(t, fixture, "left")
	right := commitRuntime(t, fixture, "right")
	wrapper, reached, release := synchronizedGoWrapper(t, 2)
	env := fixture.env
	env.GoBinary = wrapper
	results := make(chan AdvanceResult, 2)
	errs := make(chan error, 2)
	for _, revision := range []string{left.String(), right.String()} {
		go func(revision string) {
			result, err := Advance(fixture.root, revision, env)
			results <- result
			errs <- err
		}(revision)
	}
	waitForFileLines(t, reached, 2)
	if err := os.WriteFile(release, []byte("go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, second := <-results, <-results
	err1, err2 := <-errs, <-errs
	successes, conflicts := 0, 0
	for i, err := range []error{err1, err2} {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, ErrConcurrentAdvance) {
			conflicts++
			continue
		}
		t.Fatalf("advance %d unexpected error: %v (results %+v %+v)", i, err, first, second)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1; errors=%v,%v", successes, conflicts, err1, err2)
	}
	got := gitOutput(t, fixture.root, "rev-parse", "refs/awf/dashboard-runtime^{commit}")
	if got != left.String() && got != right.String() {
		t.Fatalf("runtime ref = %s, want one raced candidate", got)
	}
}

func TestPublishedEntryValidationRejectsEveryUnsafeShape(t *testing.T) {
	for _, scenario := range []string{"permissions", "extra", "member-directory", "metadata-malformed", "metadata-noncanonical", "policy-noncanonical"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := newRuntimeFixture(t)
			launcher, err := Resolve(fixture.root, fixture.env)
			if err != nil {
				t.Fatal(err)
			}
			entry := filepath.Dir(launcher.Path)
			var expected runtimeMetadata
			decodeJSONFile(t, filepath.Join(entry, "metadata.json"), &expected)
			expected.BinarySHA256, expected.LauncherSHA256, expected.PolicySHA256 = "", "", ""
			switch scenario {
			case "permissions":
				if err := os.Chmod(entry, 0o755); err != nil {
					t.Fatal(err)
				}
			case "extra":
				if err := os.WriteFile(filepath.Join(entry, "extra"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "member-directory":
				path := filepath.Join(entry, artifactName("awf", fixture.env.GOOS))
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "metadata-malformed":
				if err := os.WriteFile(filepath.Join(entry, "metadata.json"), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "metadata-noncanonical":
				var metadata any
				decodeJSONFile(t, filepath.Join(entry, "metadata.json"), &metadata)
				content, _ := json.MarshalIndent(metadata, "", "  ")
				if err := os.WriteFile(filepath.Join(entry, "metadata.json"), content, 0o600); err != nil {
					t.Fatal(err)
				}
			case "policy-noncanonical":
				policyPath := filepath.Join(entry, "policy.json")
				content, err := os.ReadFile(policyPath)
				if err != nil {
					t.Fatal(err)
				}
				content = append(content, '\n')
				if err := os.WriteFile(policyPath, content, 0o600); err != nil {
					t.Fatal(err)
				}
				var metadata runtimeMetadata
				decodeJSONFile(t, filepath.Join(entry, "metadata.json"), &metadata)
				metadata.PolicySHA256 = digestBytes(content)
				writeCompactJSON(t, filepath.Join(entry, "metadata.json"), metadata)
			}
			if err := validatePublishedEntry(entry, expected, fixture.env); err == nil {
				t.Fatalf("scenario %s accepted", scenario)
			}
		})
	}
}

func TestRuntimeValidationAndFailureBoundaries(t *testing.T) {
	if _, err := normalizeEnvironment(BuildEnvironment{XDGCacheHome: "relative"}); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("relative cache error = %v", err)
	}
	env, err := normalizeEnvironment(BuildEnvironment{XDGCacheHome: t.TempDir()})
	if err != nil || env.GOOS == "" || env.GOARCH == "" || env.GoBinary == "" || env.Stderr == nil {
		t.Fatalf("normalized environment = %#v, %v", env, err)
	}
	if _, err := Resolve(filepath.Join(t.TempDir(), "missing"), env); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("missing repository error = %v", err)
	}
	fixture := newRuntimeFixture(t)
	if _, err := Advance(fixture.root, "HEAD", fixture.env); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("absent ref advance error = %v", err)
	}
	if _, err := Resolve(fixture.root, fixture.env); err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(fixture.root, "-invalid", fixture.env); !errors.Is(err, ErrInvalidRef) {
		t.Fatalf("invalid revision error = %v", err)
	}
	badGo := fixture.env
	badGo.GoBinary = filepath.Join(t.TempDir(), "missing-go")
	if _, err := Resolve(fixture.root, badGo); !errors.Is(err, ErrBuild) {
		t.Fatalf("missing Go error = %v", err)
	}

	valid := config.WorkflowTelemetryConfig{
		Retention: config.TelemetryRetentionConfig{MaxCompletedEffortAgeDays: 1, MaxCompletedEffortCount: 1},
		Diagnostics: config.TelemetryDiagnosticsConfig{MinimumBaselineSamples: 1, BaselinePercentile: 50, Thresholds: config.TelemetryThresholdsConfig{
			PhaseReentryCount: 1, PhaseDurationSeconds: 1, PhaseTokens: 1, CompactionCount: 1, HandoffCount: 1,
			ToolFailureCount: 1, GateFailureCount: 1, CacheReadPercentBelow: 1, SubagentQueueWaitSeconds: 1, ImplementationReworkCount: 1,
		}},
	}
	if err := validateTelemetry(valid); err != nil {
		t.Fatal(err)
	}
	invalids := []config.WorkflowTelemetryConfig{}
	for i := range 12 {
		candidate := valid
		switch i {
		case 0:
			candidate.Retention.MaxCompletedEffortAgeDays = -1
		case 1:
			candidate.Retention.MaxCompletedEffortCount = -1
		case 2:
			candidate.Diagnostics.MinimumBaselineSamples = 0
		case 3:
			candidate.Diagnostics.Thresholds.PhaseReentryCount = 0
		case 4:
			candidate.Diagnostics.Thresholds.PhaseDurationSeconds = 0
		case 5:
			candidate.Diagnostics.Thresholds.PhaseTokens = 0
		case 6:
			candidate.Diagnostics.Thresholds.CompactionCount = 0
		case 7:
			candidate.Diagnostics.Thresholds.HandoffCount = 0
		case 8:
			candidate.Diagnostics.Thresholds.ToolFailureCount = 0
		case 9:
			candidate.Diagnostics.Thresholds.GateFailureCount = 0
		case 10:
			candidate.Diagnostics.Thresholds.SubagentQueueWaitSeconds = 0
		case 11:
			candidate.Diagnostics.Thresholds.ImplementationReworkCount = 0
		}
		invalids = append(invalids, candidate)
	}
	for _, candidate := range invalids {
		if err := validateTelemetry(candidate); err == nil {
			t.Fatal("invalid telemetry accepted")
		}
	}
	for _, percentile := range []int{0, 101} {
		candidate := valid
		candidate.Diagnostics.BaselinePercentile = percentile
		if validateTelemetry(candidate) == nil {
			t.Fatal("invalid percentile accepted")
		}
	}
	for _, percent := range []int{-1, 101} {
		candidate := valid
		candidate.Diagnostics.Thresholds.CacheReadPercentBelow = percent
		if validateTelemetry(candidate) == nil {
			t.Fatal("invalid cache percent accepted")
		}
	}
	if canonicalJSON([]byte("{")) || canonicalJSON([]byte("{}\n")) {
		t.Fatal("noncanonical JSON accepted")
	}

	temp := t.TempDir()
	if _, err := readPolicySnapshot(temp); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("missing policy error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(temp, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(temp, ".awf", "config.yaml"), []byte("bad: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPolicySnapshot(temp); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("invalid policy error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(temp, ".awf", "config.yaml"), []byte("workflowTelemetry:\n  diagnostics:\n    minimumBaselineSamples: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPolicySnapshot(temp); !errors.Is(err, ErrSnapshot) {
		t.Fatalf("invalid telemetry error = %v", err)
	}

	if artifactName("awf", "windows") != "awf.exe" || artifactName("awf", "linux") != "awf" {
		t.Fatal("artifact naming mismatch")
	}
	path := filepath.Join(t.TempDir(), "file")
	if exists, err := pathExists(path); err != nil || exists {
		t.Fatalf("missing path = %v, %v", exists, err)
	}
	if err := writeSyncedFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if exists, err := pathExists(path); err != nil || !exists {
		t.Fatalf("existing path = %v, %v", exists, err)
	}
	if err := writeSyncedFile(path, []byte("x"), 0o600); err == nil {
		t.Fatal("exclusive write overwrote file")
	}
	if err := syncFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("syncFile accepted missing path")
	}
	if err := syncDirectory(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("syncDirectory accepted missing path")
	}
	if _, err := digestFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("digestFile accepted missing path")
	}
	cacheRoot := t.TempDir()
	key := strings.Repeat("a", 64)
	unsafeStaging := filepath.Join(cacheRoot, "."+key+".tmp-"+strings.Repeat("b", 32))
	if err := os.Symlink(t.TempDir(), unsafeStaging); err != nil {
		t.Fatal(err)
	}
	if err := removeIncompleteStaging(cacheRoot, key); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("unsafe staging error = %v", err)
	}
}

func commitRuntime(t *testing.T, fixture runtimeFixture, label string) plumbing.Hash {
	t.Helper()
	repo, err := gitfixtureRepo(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	return gitfixture.Commit(t, repo, fixture.root, "feat(awf): "+label, map[string]string{
		"cmd/awf/main.go":                    fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Print(%q)}\n", label+"-awf"),
		"cmd/awf-dashboard-launcher/main.go": fmt.Sprintf("package main\nimport \"fmt\"\nfunc main(){fmt.Print(%q)}\n", label+"-launcher"),
	})
}

func gitfixtureRepo(root string) (*git.Repository, error) {
	return git.PlainOpen(root)
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

type fileState struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime int64
	Digest  string
}

func entryState(t *testing.T, dir string) []fileState {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	states := make([]fileState, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(content)
		states = append(states, fileState{entry.Name(), info.Size(), info.Mode(), info.ModTime().UnixNano(), hex.EncodeToString(digest[:])})
	}
	sort.Slice(states, func(i, j int) bool { return states[i].Name < states[j].Name })
	return states
}

func decodeJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		t.Fatal(err)
	}
}

func writeCompactJSON(t *testing.T, path string, value any) {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func gitRun(t *testing.T, root string, env []string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), env...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}

func synchronizedGoWrapper(t *testing.T, _ int) (wrapper, reached, release string) {
	t.Helper()
	dir := t.TempDir()
	reached = filepath.Join(dir, "reached")
	release = filepath.Join(dir, "release")
	wrapper = filepath.Join(dir, "go-wrapper")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = build ]; then
  printf 'build\n' >> %q
  while [ ! -f %q ]; do sleep 0.01; done
fi
exec %q "$@"
`, reached, release, goBinary(t))
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return wrapper, reached, release
}

func waitForFileLines(t *testing.T, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil && len(strings.Fields(string(content))) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	content, _ := os.ReadFile(path)
	t.Fatalf("timed out waiting for %d lines in %s; got %q", want, path, content)
}
