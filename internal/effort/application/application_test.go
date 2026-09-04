package application

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

const archiveMarker = "*\n!.gitignore\n"

func testContext(t *testing.T) context.Context {
	t.Helper()
	return testsupport.Context(t)
}

func applicationRepo(t *testing.T) string {
	t.Helper()
	repo := gitfixture.InitNativeObjectFormat(t, t.TempDir(), "sha1")
	root := repo.Root()
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitfixture.NativeAdd(t, repo, "tracked.txt")
	for _, resident := range []string{"efforts", "worktrees", "effort-archive"} {
		directory := filepath.Join(root, ".awf", resident)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(directory, ".gitignore")
		if err := os.WriteFile(marker, []byte(archiveMarker), 0o600); err != nil {
			t.Fatal(err)
		}
		gitfixture.NativeAdd(t, repo, filepath.ToSlash(filepath.Join(".awf", resident, ".gitignore")))
	}
	gitfixture.NativeCommit(t, repo, "application base")
	return root
}

func markerRenderer() ([]byte, error)              { return []byte(archiveMarker), nil }
func allowAdmission(context.Context, string) error { return nil }

func executeAndRelease(t *testing.T, root string, request Request) Result {
	t.Helper()
	result, err := Execute(testContext(t), root, request, markerRenderer, allowAdmission)
	if err != nil {
		if result.Release != nil {
			_ = result.Release()
		}
		t.Fatal(err)
	}
	if request.mutates() != (result.Release != nil) {
		t.Fatalf("kind %d release presence = %t", request.Kind, result.Release != nil)
	}
	if result.Release != nil {
		if err := result.Release(); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func renderResult(t *testing.T, result Result) string {
	t.Helper()
	var output bytes.Buffer
	if err := presentation.Render(&output, result.Document); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestApplicationBoundaryOwnsTheCompleteEffortLifecycle(t *testing.T) {
	root := applicationRepo(t)
	created := executeAndRelease(t, root, Request{Kind: New, Slug: "application-life", Title: "Application life"})
	if output := renderResult(t, created); !strings.Contains(output, "managed worktree added for application-life") || !strings.Contains(output, "awf/application-life") {
		t.Fatalf("new output = %q", output)
	}
	for _, request := range []Request{{Kind: List}, {Kind: Show, Slug: "application-life"}, {Kind: Integrate, Slug: "application-life"}} {
		result := executeAndRelease(t, root, request)
		if output := renderResult(t, result); !strings.Contains(output, "application-life") && request.Kind != List {
			t.Fatalf("kind %d output = %q", request.Kind, output)
		}
	}

	removed := executeAndRelease(t, root, Request{Kind: RemoveWorktree, Slug: "application-life"})
	if output := renderResult(t, removed); !strings.Contains(output, "managed worktree topology is absent") {
		t.Fatalf("remove output = %q", output)
	}
	added := executeAndRelease(t, root, Request{Kind: AddWorktree, Slug: "application-life", Base: "HEAD"})
	if output := renderResult(t, added); !strings.Contains(output, "managed worktree added") {
		t.Fatalf("standalone add output = %q", output)
	}
	executeAndRelease(t, root, Request{Kind: RemoveWorktree, Slug: "application-life"})
	finished := executeAndRelease(t, root, Request{Kind: Finish, Slug: "application-life"})
	if output := renderResult(t, finished); !strings.Contains(output, "status: archived") || !strings.Contains(output, "archived resident") {
		t.Fatalf("finish output = %q", output)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "effort-archive")); err != nil {
		t.Fatal(err)
	}
}

func TestApplicationRetainsMutationLeaseForSuccessAndTypedFailure(t *testing.T) {
	root := applicationRepo(t)
	result, err := Execute(testContext(t), root, Request{Kind: New, Slug: strings.Repeat("s", 33), Title: "Invalid"}, markerRenderer, allowAdmission)
	if err == nil || result.Release == nil {
		t.Fatalf("typed refusal err=%v release-present=%t", err, result.Release != nil)
	}
	var typed interface {
		Diagnostic() (presentation.Diagnostic, error)
	}
	if !errors.As(err, &typed) {
		t.Fatalf("typed refusal lost application diagnostic: %T", err)
	}
	if releaseErr := result.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}

	failure := errors.New("lease failure")
	deps := ProductionDependencies()
	deps.AcquireProjectLease = func(context.Context, string, string) (Lease, error) { return nil, failure }
	result, err = ExecuteWith(testContext(t), root, Request{Kind: Finish, Slug: "missing"}, markerRenderer, allowAdmission, deps)
	if !errors.Is(err, failure) || result.Release != nil {
		t.Fatalf("lease failure result=%#v err=%v", result, err)
	}

	deps = ProductionDependencies()
	originalAcquire := deps.AcquireProjectLease
	acquired := false
	deps.AcquireProjectLease = func(ctx context.Context, tracked, resident string) (Lease, error) {
		lease, err := originalAcquire(ctx, tracked, resident)
		acquired = err == nil
		return lease, err
	}
	admissionFailure := errors.New("admission failure")
	result, err = ExecuteWith(testContext(t), root, Request{Kind: Finish, Slug: "missing"}, markerRenderer, func(context.Context, string) error {
		if !acquired {
			t.Fatal("mutation admission ran before lease acquisition")
		}
		return admissionFailure
	}, deps)
	if !errors.Is(err, admissionFailure) || result.Release == nil {
		t.Fatalf("admission failure result=%#v err=%v", result, err)
	}
	if releaseErr := result.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	if _, err := ExecuteWith(testContext(t), root, Request{Kind: Finish, Slug: "missing"}, markerRenderer, nil, ProductionDependencies()); err == nil || !strings.Contains(err.Error(), "mutation admission") {
		t.Fatalf("missing mutation admission error = %v", err)
	}
}

func TestApplicationDependencyAndCompositionFailures(t *testing.T) {
	valid := ProductionDependencies()
	setNil := []struct {
		name string
		edit func(*Dependencies)
	}{
		{"roots", func(d *Dependencies) { d.ResolveRoots = nil }},
		{"checkout", func(d *Dependencies) { d.OpenCheckout = nil }},
		{"resident", func(d *Dependencies) { d.OpenResident = nil }},
		{"lease", func(d *Dependencies) { d.AcquireProjectLease = nil }},
		{"clock", func(d *Dependencies) { d.Clock = nil }},
		{"uuid", func(d *Dependencies) { d.UUID = nil }},
		{"gate", func(d *Dependencies) { d.GateCommand = nil }},
	}
	for _, test := range setNil {
		t.Run(test.name, func(t *testing.T) {
			deps := valid
			test.edit(&deps)
			if _, err := ExecuteWith(testContext(t), "unused", Request{Kind: List}, markerRenderer, allowAdmission, deps); err == nil || !strings.Contains(err.Error(), "missing") {
				t.Fatalf("missing dependency error = %v", err)
			}
		})
	}
	if _, err := ExecuteWith(testContext(t), "unused", Request{Kind: List}, nil, allowAdmission, valid); err == nil || !strings.Contains(err.Error(), "archive marker") {
		t.Fatalf("missing marker error = %v", err)
	}

	root := applicationRepo(t)
	failure := errors.New("composition failure")
	deps := valid
	deps.ResolveRoots = func(context.Context, string) (awfgit.ControlRoots, error) { return awfgit.ControlRoots{}, failure }
	if _, err := ExecuteWith(testContext(t), root, Request{Kind: List}, markerRenderer, allowAdmission, deps); !errors.Is(err, failure) {
		t.Fatalf("root failure = %v", err)
	}
	deps = valid
	deps.OpenCheckout = func(string) (Checkout, error) { return nil, failure }
	if _, err := ExecuteWith(testContext(t), root, Request{Kind: List}, markerRenderer, allowAdmission, deps); !errors.Is(err, failure) {
		t.Fatalf("checkout failure = %v", err)
	}

	result, err := Execute(testContext(t), root, Request{Kind: Kind(255)}, markerRenderer, allowAdmission)
	if err == nil || !strings.Contains(err.Error(), "unknown request kind") || result.Release != nil {
		t.Fatalf("unknown request result=%#v err=%v", result, err)
	}
}

func TestIntegrationGateCommandUsesOnlyAConfiguredString(t *testing.T) {
	for _, test := range []struct {
		name       string
		configYAML string
		want       string
		wantErr    bool
	}{
		{name: "absent"},
		{name: "configured", configYAML: "prefix: example\nvars: {gateCmd: make gate}\n", want: "make gate"},
		{name: "blank", configYAML: "prefix: example\nvars: {gateCmd: \"  \"}\n"},
		{name: "non-string", configYAML: "prefix: example\nvars: {gateCmd: [make, gate]}\n"},
		{name: "malformed", configYAML: "unknown: value\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.configYAML != "" {
				if err := os.Mkdir(filepath.Join(root, ".awf"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte(test.configYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := integrationGateCommand(root)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("gate = %q, %v; want %q err=%t", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestIntegrateResolvesMalformedGateBeforeEffortLookup(t *testing.T) {
	root := applicationRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".awf", "config.yaml"), []byte("unknown: value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Execute(testContext(t), root, Request{Kind: Integrate, Slug: "missing-effort"}, markerRenderer, allowAdmission)
	if err == nil || !strings.Contains(err.Error(), "unknown") || strings.Contains(err.Error(), "missing-effort") || result.Release == nil {
		t.Fatalf("integration precedence result=%#v err=%v", result, err)
	}
	if releaseErr := result.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
}

func TestApplicationRequestMutationClassification(t *testing.T) {
	for _, kind := range []Kind{New, Finish, AddWorktree, RemoveWorktree, Integrate} {
		if !((Request{Kind: kind}).mutates()) {
			t.Fatalf("kind %d classified read-only", kind)
		}
	}
	for _, kind := range []Kind{List, Show, Kind(255)} {
		if (Request{Kind: kind}).mutates() {
			t.Fatalf("kind %d classified mutating", kind)
		}
	}
}

func TestApplicationOpenResidentFailureIsReportedByWorktreeUseCase(t *testing.T) {
	root := applicationRepo(t)
	executeAndRelease(t, root, Request{Kind: New, Slug: "resident-open", Title: "Resident open"})
	executeAndRelease(t, root, Request{Kind: RemoveWorktree, Slug: "resident-open"})
	failure := errors.New("resident open failure")
	deps := ProductionDependencies()
	deps.OpenResident = func(string) (worktree.ResidentHandle, error) { return nil, failure }
	result, err := ExecuteWith(testContext(t), root, Request{Kind: AddWorktree, Slug: "resident-open"}, markerRenderer, allowAdmission, deps)
	if !errors.Is(err, failure) || result.Release == nil {
		t.Fatalf("resident failure result=%#v err=%v", result, err)
	}
	if releaseErr := result.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
}
