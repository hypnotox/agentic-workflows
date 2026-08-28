package contextop

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/contextinput"
	"github.com/hypnotox/agentic-workflows/internal/contextq"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

type contextCaptureMetrics struct {
	files int
	bytes int
}

func capturedContextMetrics(input contextinput.Input) contextCaptureMetrics {
	metrics := contextCaptureMetrics{}
	for _, file := range input.Snapshot().Tree.List() {
		if file.Mode == snapshot.Regular || file.Mode == snapshot.Executable {
			metrics.files++
			metrics.bytes += len(file.Bytes)
		}
	}
	return metrics
}

// contextPerformanceFixture has marker and artifact inputs that context must
// read, plus regular payloads that only the retained complete baseline reads.
func contextPerformanceFixture(t testing.TB, size int) (*project.ProjectState, string, *awfgit.Repo) {
	t.Helper()
	root := contextPreparationFixture(t)
	payload := strings.Repeat("unrelated payload\n", 256)
	for i := range size {
		testsupport.WriteFile(t, filepath.Join(root, "payload", fmt.Sprintf("unrelated-%04d.bin", i)), payload)
	}
	for i := range 16 {
		testsupport.WriteFile(t, filepath.Join(root, "internal", "foo", fmt.Sprintf("marker-%04d.go", i)), "package foo\n// state: alpha/one:order\n")
	}
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 16 {
		path := fmt.Sprintf("generated/artifact-%04d.md", i)
		testsupport.WriteFile(t, filepath.Join(root, filepath.FromSlash(path)), strings.Repeat("artifact payload\n", 64))
		lock.Files[path] = manifest.Entry{TemplateID: "fixture"}
	}
	if err := lock.Save(filepath.Join(root, ".awf", "awf.lock")); err != nil {
		t.Fatal(err)
	}
	state, repo := contextPreparationProject(t, root)
	return state, root, repo
}

func renderOrdinaryContext(t testing.TB, state *project.ProjectState, repo *awfgit.Repo, paths []string, focusedRoute bool) (string, contextCaptureMetrics) {
	t.Helper()
	var (
		input contextinput.Input
		err   error
	)
	if focusedRoute {
		input, err = workingState(testsupport.Context(t), state, repo, paths)
	} else {
		input, err = workingCompleteState(testsupport.Context(t), state, repo)
	}
	if err != nil {
		t.Fatal(err)
	}
	text := contextq.RenderContextText(contextq.New(input).ContextForOptions(paths, contextq.ContextOptions{Selection: contextq.SelectionExplicit}), "live state for this project", nil)
	return text, capturedContextMetrics(input)
}

type ordinaryContextCase struct {
	name  string
	size  int
	paths []string
}

func ordinaryContextCases() []ordinaryContextCase {
	return []ordinaryContextCase{
		{name: "exact-files-small", size: 32, paths: []string{"internal/foo/x.go"}},
		{name: "exact-files-large", size: 256, paths: []string{"internal/foo/x.go"}},
		{name: "directory", size: 32, paths: []string{"internal/foo"}},
		{name: "marker-heavy", size: 32, paths: []string{"internal/foo/marker-0000.go"}},
		{name: "artifact-heavy", size: 32, paths: []string{"generated"}},
	}
}

func TestOrdinaryContextComparativeEvidence(t *testing.T) {
	for _, tc := range ordinaryContextCases() {
		t.Run(tc.name, func(t *testing.T) {
			state, _, repo := contextPerformanceFixture(t, tc.size)
			focusedText, focusedCapture := renderOrdinaryContext(t, state, repo, tc.paths, true)
			completeText, completeCapture := renderOrdinaryContext(t, state, repo, tc.paths, false)
			if focusedText != completeText {
				t.Fatal("focused ordinary output differs from complete-preparation baseline")
			}
			if focusedCapture.files >= completeCapture.files || focusedCapture.bytes >= completeCapture.bytes {
				t.Fatalf("focused capture did not skip unrelated regular payload: focused=%+v complete=%+v", focusedCapture, completeCapture)
			}
		})
	}
}

func BenchmarkOrdinaryContextComparativeEvidence(b *testing.B) {
	for _, tc := range ordinaryContextCases() {
		b.Run(tc.name, func(b *testing.B) {
			state, _, repo := contextPerformanceFixture(b, tc.size)
			for _, route := range []struct {
				name    string
				focused bool
			}{
				{name: "focused", focused: true},
				{name: "complete", focused: false},
			} {
				b.Run(route.name, func(b *testing.B) {
					_, capture := renderOrdinaryContext(b, state, repo, tc.paths, route.focused)
					b.ReportMetric(float64(capture.files), "files-read")
					b.ReportMetric(float64(capture.bytes), "bytes-read")
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						if _, err := renderOrdinaryContextForBenchmark(state, repo, tc.paths, route.focused); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func renderOrdinaryContextForBenchmark(state *project.ProjectState, repo *awfgit.Repo, paths []string, focusedRoute bool) (string, error) {
	var (
		input contextinput.Input
		err   error
	)
	if focusedRoute {
		input, err = workingState(context.Background(), state, repo, paths)
	} else {
		input, err = workingCompleteState(context.Background(), state, repo)
	}
	if err != nil {
		return "", err
	}
	return contextq.RenderContextText(contextq.New(input).ContextForOptions(paths, contextq.ContextOptions{Selection: contextq.SelectionExplicit}), "live state for this project", nil), nil
}
