package artifactfs

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/projector"
)

func TestTrackedArtifactsCreateWithoutEffortAndPreserveExistingContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		new  func(string) (string, error)
		want string
	}{
		{name: "intent", new: func(root string) (string, error) { return NewIntent(root, "ship-it") }, want: filepath.Join("docs", "changes", "ship-it", "intent.md")},
		{name: "spec", new: func(root string) (string, error) { return NewSpec(root, "ship-it") }, want: filepath.Join("docs", "changes", "ship-it", "spec.md")},
		{name: "plan", new: func(root string) (string, error) { return NewPlan(root, "ship-it") }, want: filepath.Join("docs", "plans", "ship-it.md")},
		{name: "adr", new: func(root string) (string, error) { return NewADR(root, "ship-it") }, want: filepath.Join("docs", "decisions", "ship-it.md")},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			relative, err := test.new(root)
			if err != nil {
				t.Fatal(err)
			}
			if relative != test.want {
				t.Fatalf("created path = %q, want %q", relative, test.want)
			}
			body, err := os.ReadFile(filepath.Join(root, relative))
			if err != nil || len(body) == 0 {
				t.Fatalf("created body = %q, %v", body, err)
			}
			if test.name == "adr" && !bytes.HasPrefix(body, []byte("---\nstatus: pending\n---\n")) {
				t.Fatalf("new ADR does not start pending: %q", body)
			}
			for _, unexpected := range []string{".git", ".awf", "AGENTS.md"} {
				if _, err := os.Stat(filepath.Join(root, unexpected)); !os.IsNotExist(err) {
					t.Fatalf("creation produced side effect %s: %v", unexpected, err)
				}
			}

			entries, err := os.ReadDir(filepath.Dir(filepath.Join(root, relative)))
			if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(relative) {
				t.Fatalf("creation added unrelated documents: %v, %v", entries, err)
			}

			edited := []byte("author-owned content\n")
			if err := os.WriteFile(filepath.Join(root, relative), edited, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := test.new(root); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("repeat creation error = %v", err)
			}
			preserved, err := os.ReadFile(filepath.Join(root, relative))
			if err != nil || !bytes.Equal(preserved, edited) {
				t.Fatalf("existing content = %q, %v", preserved, err)
			}
		})
	}
}

func TestChangeDefinitionDocumentsShareSlugWithoutRewritingSiblings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	created := make(map[string][]byte)
	for _, create := range []func(string, string) (string, error){NewSpec, NewIntent} {
		relative, err := create(root, "ship-it")
		if err != nil {
			t.Fatal(err)
		}
		body := []byte("authored " + filepath.Base(relative) + "\n")
		if err := os.WriteFile(filepath.Join(root, relative), body, 0o644); err != nil {
			t.Fatal(err)
		}
		created[relative] = body
		for path, want := range created {
			got, err := os.ReadFile(filepath.Join(root, path))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("sibling %s = %q, %v", path, got, err)
			}
		}
	}
	planRelative, err := NewPlan(root, "ship-it")
	if err != nil || planRelative != filepath.Join("docs", "plans", "ship-it.md") {
		t.Fatalf("plan destination = %q, %v", planRelative, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs", "changes", "ship-it"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("change definition documents after plan creation = %v, %v", entries, err)
	}
	for path, want := range created {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("change definition %s after plan creation = %q, %v", path, got, err)
		}
	}
}

func TestNewTopicRoundTripsAuthoredSelectorMeaning(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeProject(t, root)
	selectors := []string{"./**", `src\**\*.go`}
	relative, err := NewTopic(root, "code/go", selectors)
	if err != nil {
		t.Fatal(err)
	}
	wantRelative := filepath.Join("docs", "topics", "code", "go.md")
	if relative != wantRelative {
		t.Fatalf("topic path = %q, want %q", relative, wantRelative)
	}
	body, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`- "./**"`)) {
		t.Fatalf("topic silently rewrote authored ./** selector: %s", body)
	}
	for _, unexpected := range []string{".git", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, unexpected)); !os.IsNotExist(err) {
			t.Fatalf("topic creation produced side effect %s: %v", unexpected, err)
		}
	}

	sources, err := projector.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources.Topics) != 1 || sources.Topics[0].ID != "code/go" || sources.Topics[0].Global {
		t.Fatalf("loaded topic = %#v", sources.Topics)
	}
	if want := []string{"**", "src/**/*.go"}; !reflect.DeepEqual(sources.Topics[0].Paths, want) {
		t.Fatalf("normalized paths = %v, want %v", sources.Topics[0].Paths, want)
	}
	matches, err := projector.Resolve(root, nil)
	if err != nil || len(matches) != 0 {
		t.Fatalf("bare resolve treated ./** as global: %#v, %v", matches, err)
	}
	matches, err = projector.Resolve(root, []string{"docs/nonexistent.md"})
	if err != nil || len(matches) != 1 || matches[0].ID != "code/go" {
		t.Fatalf("path resolve = %#v, %v", matches, err)
	}

	edited := []byte("author-owned topic\n")
	if err := os.WriteFile(filepath.Join(root, relative), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTopic(root, "code/go", []string{"other/**"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("repeat topic creation error = %v", err)
	}
	preserved, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil || !bytes.Equal(preserved, edited) {
		t.Fatalf("existing topic = %q, %v", preserved, err)
	}
}

func TestNewTopicExplicitGlobalRemainsGlobal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeProject(t, root)
	if _, err := NewTopic(root, "global", []string{"**"}); err != nil {
		t.Fatal(err)
	}
	matches, err := projector.Resolve(root, nil)
	if err != nil || len(matches) != 1 || matches[0].ID != "global" {
		t.Fatalf("bare resolve = %#v, %v", matches, err)
	}
}

func TestCreationRefusesSymlinkedDestinationDirectories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		relative string
		create   func(string) (string, error)
		outside  string
	}{
		{name: "intent", relative: "docs/changes/escaped", create: func(root string) (string, error) { return NewIntent(root, "escaped") }, outside: "intent.md"},
		{name: "spec", relative: "docs/changes/escaped", create: func(root string) (string, error) { return NewSpec(root, "escaped") }, outside: "spec.md"},
		{name: "plan", relative: "docs/plans", create: func(root string) (string, error) { return NewPlan(root, "escaped") }, outside: "escaped.md"},
		{name: "adr", relative: "docs/decisions", create: func(root string) (string, error) { return NewADR(root, "escaped") }, outside: "escaped.md"},
		{name: "topic", relative: "docs/topics", create: func(root string) (string, error) { return NewTopic(root, "escaped", []string{"src/**"}) }, outside: "escaped.md"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			outside := t.TempDir()
			link := filepath.Join(root, filepath.FromSlash(test.relative))
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			if _, err := test.create(root); err == nil || !strings.Contains(err.Error(), "not a directory") {
				t.Fatalf("creation through symlink error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(outside, test.outside)); !os.IsNotExist(err) {
				t.Fatalf("creation escaped checkout: %v", err)
			}
		})
	}
}

func TestInvalidArtifactInputsDoNotCreateMisleadingFiles(t *testing.T) {
	t.Parallel()

	for _, slug := range []string{"", ".", "..", "-leading", "bad slug", "nested/name", `nested\name`} {
		root := t.TempDir()
		if _, err := NewIntent(root, slug); err == nil {
			t.Errorf("NewIntent(%q) succeeded", slug)
		}
		if _, err := NewSpec(root, slug); err == nil {
			t.Errorf("NewSpec(%q) succeeded", slug)
		}
		if _, err := NewPlan(root, slug); err == nil {
			t.Errorf("NewPlan(%q) succeeded", slug)
		}
		if _, err := NewADR(root, slug); err == nil {
			t.Errorf("NewADR(%q) succeeded", slug)
		}
		assertNoArtifactRoots(t, root)
	}

	invalidTopics := []struct {
		id        string
		selectors []string
	}{
		{id: "", selectors: []string{"src/**"}},
		{id: "../escape", selectors: []string{"src/**"}},
		{id: "/absolute", selectors: []string{"src/**"}},
		{id: "topic.md", selectors: []string{"src/**"}},
		{id: `nested\topic`, selectors: []string{"src/**"}},
		{id: "valid", selectors: nil},
		{id: "valid", selectors: []string{"**", "src/**"}},
		{id: "valid", selectors: []string{"../**"}},
	}
	for _, test := range invalidTopics {
		root := t.TempDir()
		if _, err := NewTopic(root, test.id, test.selectors); err == nil {
			t.Errorf("NewTopic(%q, %v) succeeded", test.id, test.selectors)
		}
		if _, err := os.Stat(filepath.Join(root, "docs", "topics")); !os.IsNotExist(err) {
			t.Errorf("invalid topic created source directory: %v", err)
		}
	}
}

func writeProject(t *testing.T, root string) {
	t.Helper()
	filename := filepath.Join(root, ".awf", "project.md")
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("---\nformat: 2\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNoArtifactRoots(t *testing.T, root string) {
	t.Helper()
	for _, relative := range []string{"docs/changes", "docs/plans", "docs/decisions"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("invalid artifact created %s: %v", relative, err)
		}
	}
}
