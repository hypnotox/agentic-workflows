package projector

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	awfdocs "github.com/hypnotox/agentic-workflows/internal/docs"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

func TestLoadTopicsPreservesAuthoredContent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "topics", "z-last.md"), []byte("---\npaths: [docs/**]\nextra: ignored\n---\nopaque\n"), 0o644)
	body := []byte("BODY {{VERSION}} ${name}\r\n")
	writeTestFile(t, filepath.Join(root, "docs", "topics", "nested", "a-first.md"), append([]byte("---\npaths: [./internal//**/*.go]\n---\n"), body...), 0o644)
	writeTestFile(t, filepath.Join(root, "docs", "ordinary.md"), []byte("not a topic input\n"), 0o644)

	topics, err := LoadTopics(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 {
		t.Fatalf("loaded %d topics, want only the two under docs/topics", len(topics))
	}
	if got := []string{topics[0].ID, topics[1].ID}; !slices.Equal(got, []string{"nested/a-first", "z-last"}) {
		t.Fatalf("topic IDs = %v", got)
	}
	if !bytes.Equal(topics[0].Body, body) {
		t.Fatalf("topic body = %q, want %q", topics[0].Body, body)
	}
}

func TestBuildSharesCanonicalWorkflowsAndFixedTemplates(t *testing.T) {
	outputs := Build()
	wantPaths := []string{
		".awf/.gitignore", ".awf/VERSION", ".awf/bootstrap.sh",
		".claude/skills/awf-changes/SKILL.md", ".claude/skills/awf-completion/SKILL.md",
		".claude/skills/awf-effort/SKILL.md", ".claude/skills/awf-topics/SKILL.md",
		".pi/skills/awf-changes/SKILL.md", ".pi/skills/awf-completion/SKILL.md",
		".pi/skills/awf-effort/SKILL.md", ".pi/skills/awf-topics/SKILL.md", "awf",
	}
	if got := outputPathsForTest(outputs); !slices.Equal(got, wantPaths) {
		t.Fatalf("output paths = %v, want %v", got, wantPaths)
	}
	for _, workflow := range []string{"topics", "effort", "changes", "completion"} {
		pi := outputForTest(t, outputs, ".pi/skills/awf-"+workflow+"/SKILL.md")
		claude := outputForTest(t, outputs, ".claude/skills/awf-"+workflow+"/SKILL.md")
		if !bytes.Equal(pi.Bytes, claude.Bytes) {
			t.Errorf("Pi and Claude %s skills differ", workflow)
		}
		var metadata struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}
		body, found, err := frontmatter.Parse(pi.Bytes, &metadata)
		if err != nil || !found || metadata.Name != "awf-"+workflow || metadata.Description == "" {
			t.Fatalf("%s skill metadata = %#v, %v", workflow, metadata, err)
		}
		canonical, ok := awfdocs.Page(workflow)
		if !ok || len(canonical) == 0 {
			t.Fatalf("missing canonical workflow %s", workflow)
		}
		if !bytes.HasPrefix(body, []byte(markdownMarker+"\n\n")) || !bytes.Equal(bytes.TrimPrefix(body, []byte(markdownMarker+"\n\n")), canonical) {
			t.Errorf("%s skill body does not share the complete canonical workflow", workflow)
		}
		if !hasOwnershipMarker(pi.Bytes) {
			t.Errorf("%s skill lacks a recognized ownership marker", workflow)
		}
	}
	for _, path := range []string{"awf", ".awf/bootstrap.sh"} {
		output := outputForTest(t, outputs, path)
		lines := strings.Split(string(output.Bytes), "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], "#!") || lines[1] != textMarker {
			t.Errorf("%s marker is not after shebang", path)
		}
		if output.Mode.Perm()&0o111 == 0 {
			t.Errorf("%s is not executable", path)
		}
	}
	launcher := PublicLauncher()
	if len(launcher) == 0 || bytes.Contains(launcher, []byte(textMarker)) {
		t.Fatal("public launcher is empty or carries adopter ownership")
	}
}

func TestLoadTopicsRejectsInvalidSources(t *testing.T) {
	for name, topic := range map[string]string{
		"frontmatter":            "plain\n",
		"empty paths":            "---\npaths: []\n---\nbody\n",
		"escape":                 "---\npaths: [../**]\n---\nbody\n",
		"unsupported glob":       "---\npaths: ['docs/[ab]']\n---\nbody\n",
		"global mixed with path": "---\npaths: ['**', 'src/**']\n---\nbody\n",
		"global duplicated":      "---\npaths: ['**', '**']\n---\nbody\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "docs", "topics", "bad.md"), []byte(topic), 0o644)
			if _, err := LoadTopics(root); err == nil {
				t.Fatal("LoadTopics accepted invalid source")
			}
			if _, err := Check(root); err == nil {
				t.Fatal("Check accepted invalid topic")
			}
			if _, err := Render(root); err == nil {
				t.Fatal("Render accepted invalid topic")
			}
			if _, err := os.Stat(filepath.Join(root, "awf")); !os.IsNotExist(err) {
				t.Fatalf("invalid topic allowed generated writes: %v", err)
			}
		})
	}
}

func TestResolveGlobalsPathsAndEmptyResults(t *testing.T) {
	root := t.TempDir()
	writeTopicForTest(t, root, "z-global.md", []string{"**"})
	writeTopicForTest(t, root, "a-go.md", []string{"src/**/*.go"})
	writeTopicForTest(t, root, "m-root.md", []string{"*", "docs/**"})
	writeTopicForTest(t, root, "n-normalized-double-star.md", []string{"./**"})

	matches, err := Resolve(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []TopicMatch{{ID: "z-global", SourcePath: "docs/topics/z-global.md"}}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("empty Resolve = %#v, want %#v", matches, want)
	}

	matches, err = Resolve(root, []string{`src\new\future.go`, "unknown/file.txt", "src/new/future.go"})
	if err != nil {
		t.Fatal(err)
	}
	want = []TopicMatch{
		{ID: "a-go", SourcePath: "docs/topics/a-go.md"},
		{ID: "n-normalized-double-star", SourcePath: "docs/topics/n-normalized-double-star.md"},
		{ID: "z-global", SourcePath: "docs/topics/z-global.md"},
	}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("path Resolve = %#v, want %#v", matches, want)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "new", "future.go")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("resolve target unexpectedly exists: %v", err)
	}

	matches, err = Resolve(root, []string{"README.md"})
	if err != nil {
		t.Fatal(err)
	}
	want = []TopicMatch{
		{ID: "m-root", SourcePath: "docs/topics/m-root.md"},
		{ID: "n-normalized-double-star", SourcePath: "docs/topics/n-normalized-double-star.md"},
		{ID: "z-global", SourcePath: "docs/topics/z-global.md"},
	}
	if !reflect.DeepEqual(matches, want) {
		t.Fatalf("root Resolve = %#v, want %#v", matches, want)
	}

	writeTestFile(t, filepath.Join(root, "docs", "topics", "z-global.md"), []byte("---\npaths: [elsewhere/**]\n---\nbody\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "docs", "topics", "n-normalized-double-star.md"), []byte("---\npaths: [elsewhere/**]\n---\nbody\n"), 0o644)
	matches, err = Resolve(root, []string{"nested/no-match"})
	if err != nil || len(matches) != 0 {
		t.Fatalf("no-match Resolve = %#v, %v", matches, err)
	}
}

func TestResolveValidatesPathsWhenGlobalsExist(t *testing.T) {
	root := t.TempDir()
	writeTopicForTest(t, root, "global.md", []string{"**"})
	for _, bad := range []string{"", "/absolute", "../escape", `C:\absolute`} {
		if _, err := Resolve(root, []string{bad}); err == nil {
			t.Errorf("Resolve accepted %q", bad)
		}
	}
}

func TestResolveCoverageReportsGlobalsGapsOverlapAndNormalizedInputs(t *testing.T) {
	root := t.TempDir()
	writeTopicForTest(t, root, "z-global.md", []string{"**"})
	writeTopicForTest(t, root, "a-go.md", []string{"src/**/*.go"})
	writeTopicForTest(t, root, "m-src.md", []string{"src/**"})
	before := snapshotTree(t, root)

	coverage, err := ResolveCoverage(root, []string{`src\future\new.go`, "missing/file.txt", "src/else/../future/new.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := Coverage{
		Globals: []TopicMatch{{ID: "z-global", SourcePath: "docs/topics/z-global.md"}},
		Paths: []PathCoverage{
			{Path: "missing/file.txt"},
			{Path: "src/future/new.go", Matches: []TopicMatch{
				{ID: "a-go", SourcePath: "docs/topics/a-go.md"},
				{ID: "m-src", SourcePath: "docs/topics/m-src.md"},
			}},
		},
	}
	if !reflect.DeepEqual(coverage, want) {
		t.Fatalf("ResolveCoverage = %#v, want %#v", coverage, want)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "future", "new.go")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("coverage target unexpectedly exists: %v", err)
	}
	after := snapshotTree(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("coverage changed repository tree: before %#v, after %#v", before, after)
	}
}

func TestResolveCoverageRequiresAndValidatesPaths(t *testing.T) {
	root := t.TempDir()
	writeTopicForTest(t, root, "global.md", []string{"**"})
	if _, err := ResolveCoverage(root, nil); err == nil {
		t.Fatal("ResolveCoverage accepted no paths")
	}
	for _, bad := range []string{"", "/absolute", "../escape", `C:\absolute`} {
		if _, err := ResolveCoverage(root, []string{bad}); err == nil {
			t.Errorf("ResolveCoverage accepted %q", bad)
		}
	}
	writeTestFile(t, filepath.Join(root, "docs", "topics", "global.md"), []byte("malformed\n"), 0o644)
	if _, err := ResolveCoverage(root, []string{"future/path"}); err == nil {
		t.Fatal("ResolveCoverage accepted malformed topic source")
	}
}

func TestRenderCheckRepairAndUnmanagedMarkers(t *testing.T) {
	root := t.TempDir()
	topicBody := []byte("---\npaths: [src/**]\n---\n" + markdownMarker + "\nopaque\n")
	topicPath := filepath.Join(root, "docs", "topics", "marker.md")
	writeTestFile(t, topicPath, topicBody, 0o644)
	ordinaryBody := []byte("ordinary documentation\x00\xff\n")
	ordinaryPath := filepath.Join(root, "docs", "ordinary.md")
	writeTestFile(t, ordinaryPath, ordinaryBody, 0o644)

	first, err := Render(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Changed) == 0 || !slices.IsSorted(first.Changed) {
		t.Fatalf("first changed = %v", first.Changed)
	}
	findings, err := Check(root)
	if err != nil || len(findings) != 0 {
		t.Fatalf("clean Check = %#v, %v", findings, err)
	}

	writeTestFile(t, filepath.Join(root, ".pi", "skills", "awf-topics", "SKILL.md"), []byte(markdownMarker+"\nmodified\n"), 0o644)
	if err := os.Chmod(filepath.Join(root, "awf"), 0o611); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "old.md"), []byte("<!-- GENERATED by awf: do not edit; change .awf/ and run `awf render` -->\nold\n"), 0o644)
	writeTestFile(t, filepath.Join(root, ".awf", "efforts", "ignored", "memory.md"), []byte(markdownMarker+"\nignored\n"), 0o644)
	adrBody := []byte(markdownMarker + "\nauthor-owned decision content\n")
	adrPath := filepath.Join(root, "docs", "decisions", "opaque.md")
	writeTestFile(t, adrPath, adrBody, 0o644)
	planBody := []byte(markdownMarker + "\nauthor-owned plan content\n")
	planPath := filepath.Join(root, "docs", "plans", "opaque.md")
	writeTestFile(t, planPath, planBody, 0o644)

	findings, err = Check(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{
		{Path: ".pi/skills/awf-topics/SKILL.md", Message: "generated content differs"},
		{Path: "awf", Message: "generated executable state differs"},
		{Path: "old.md", Message: "unmanaged file still carries an AWF ownership marker"},
	}
	if !reflect.DeepEqual(findings, want) {
		t.Fatalf("drift findings = %#v, want %#v", findings, want)
	}

	repaired, err := Render(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(repaired.Changed, []string{".pi/skills/awf-topics/SKILL.md", "awf"}) {
		t.Fatalf("repaired = %v", repaired.Changed)
	}
	if !slices.Equal(repaired.Unmanaged, []string{"old.md"}) {
		t.Fatalf("render unmanaged = %v", repaired.Unmanaged)
	}
	if _, err := os.Stat(filepath.Join(root, "old.md")); err != nil {
		t.Fatalf("render removed unmanaged file: %v", err)
	}
	preservedADR, err := os.ReadFile(adrPath)
	if err != nil || !bytes.Equal(preservedADR, adrBody) {
		t.Fatalf("author-owned ADR after render/check = %q, %v", preservedADR, err)
	}
	preservedPlan, err := os.ReadFile(planPath)
	if err != nil || !bytes.Equal(preservedPlan, planBody) {
		t.Fatalf("author-owned plan after render/check = %q, %v", preservedPlan, err)
	}
	preservedTopic, err := os.ReadFile(topicPath)
	if err != nil || !bytes.Equal(preservedTopic, topicBody) {
		t.Fatalf("topic after render/check = %q, %v", preservedTopic, err)
	}
	preservedOrdinary, err := os.ReadFile(ordinaryPath)
	if err != nil || !bytes.Equal(preservedOrdinary, ordinaryBody) {
		t.Fatalf("ordinary documentation after render/check = %q, %v", preservedOrdinary, err)
	}
}

func TestCheckPrunesUnmanagedMarkersInNestedRepositories(t *testing.T) {
	root := t.TempDir()
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}

	marker := []byte(markdownMarker + "\nretired\n")
	writeTestFile(t, filepath.Join(root, "ordinary", "retired.md"), marker, 0o644)
	writeTestFile(t, filepath.Join(root, "nested-with-git-dir", ".git", "config"), []byte("[core]\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "nested-with-git-dir", "retired.md"), marker, 0o644)
	writeTestFile(t, filepath.Join(root, "nested-with-git-file", ".git"), []byte("gitdir: ../worktrees/nested\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "nested-with-git-file", "retired.md"), marker, 0o644)

	findings, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{{Path: "ordinary/retired.md", Message: "unmanaged file still carries an AWF ownership marker"}}
	if !reflect.DeepEqual(findings, want) {
		t.Fatalf("Check = %#v, want %#v", findings, want)
	}
}

func TestRenderPreflightsUnmanagedCollision(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "awf"), []byte("<!-- GENERATED by awf: not an ownership marker -->\nrepository-owned\n"), 0o644)

	if _, err := Render(root); err == nil || !strings.Contains(err.Error(), "not marked") {
		t.Fatalf("Render error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "VERSION")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("preflight allowed a partial write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "awf"))
	if err != nil || string(got) != "<!-- GENERATED by awf: not an ownership marker -->\nrepository-owned\n" {
		t.Fatalf("collision file = %q, %v", got, err)
	}
}

func TestRetiredResidentMarkerIsReportedWithoutScanningMemory(t *testing.T) {
	root := t.TempDir()
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	legacy := "# GENERATED by awf: do not edit; change .awf/ and run `awf render`\n"
	writeTestFile(t, filepath.Join(root, ".awf", "efforts", ".gitignore"), []byte(legacy), 0o644)
	writeTestFile(t, filepath.Join(root, ".awf", "efforts", "active", "memory.md"), []byte(markdownMarker+"\nopaque\n"), 0o644)

	result, err := Render(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Unmanaged, []string{".awf/efforts/.gitignore"}) {
		t.Fatalf("unmanaged = %v", result.Unmanaged)
	}
	findings, err := Check(root)
	if err != nil || len(findings) != 1 || findings[0].Path != ".awf/efforts/.gitignore" {
		t.Fatalf("Check = %#v, %v", findings, err)
	}
}

func TestInitProjectsWithoutDescriptorOrGitAndPreservesAgentFiles(t *testing.T) {
	for _, authored := range []bool{false, true} {
		t.Run(fmt.Sprint(authored), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", t.TempDir())
			files := map[string][]byte{
				"AGENTS.md": []byte("# Local\r\nKeep {{VERSION}} literal.\r\n"),
				"CLAUDE.md": []byte("@AGENTS.md\nClaude-specific guidance.\n"),
			}
			if authored {
				for path, body := range files {
					writeTestFile(t, filepath.Join(root, path), body, 0o644)
				}
			}
			if _, err := Init(root); err != nil {
				t.Fatal(err)
			}
			if findings, err := Check(root); err != nil || len(findings) != 0 {
				t.Fatalf("Check after Init = %#v, %v", findings, err)
			}
			if result, err := Init(root); err != nil || len(result.Changed) != 0 {
				t.Fatalf("repeat Init = %#v, %v", result, err)
			}
			for path, want := range files {
				got, err := os.ReadFile(filepath.Join(root, path))
				if authored {
					if err != nil || !bytes.Equal(got, want) {
						t.Errorf("authored %s changed: %q, %v", path, got, err)
					}
				} else if !os.IsNotExist(err) {
					t.Errorf("Init created %s: %v", path, err)
				}
			}
			for _, path := range []string{".git", ".awf/project.md"} {
				if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
					t.Errorf("Init created %s: %v", path, err)
				}
			}
		})
	}
}

func TestVersionRecordTracksRendererWithoutControllingResolution(t *testing.T) {
	root := t.TempDir()
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "1.2.3"
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	versionPath := filepath.Join(root, filepath.FromSlash(VersionPath))
	got, err := os.ReadFile(versionPath)
	if err != nil || string(got) != "1.2.3\n" {
		t.Fatalf("version record = %q, %v", got, err)
	}
	Version = "2.0.0"
	findings, err := Check(root)
	if err != nil || !slices.Contains(findings, Finding{Path: VersionPath, Message: "generated content differs"}) {
		t.Fatalf("new renderer Check = %#v, %v", findings, err)
	}
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(versionPath)
	if err != nil || string(got) != "2.0.0\n" {
		t.Fatalf("updated version record = %q, %v", got, err)
	}
	if err := os.Remove(versionPath); err != nil {
		t.Fatal(err)
	}
	findings, err = Check(root)
	want := []Finding{{Path: VersionPath, Message: "missing generated file"}}
	if err != nil || !reflect.DeepEqual(findings, want) {
		t.Fatalf("missing record Check = %#v, %v", findings, err)
	}
	writeTopicForTest(t, root, "global.md", []string{"**"})
	for _, record := range []string{"", "not configuration\n", "0.0.0\n"} {
		if record != "" {
			writeTestFile(t, versionPath, []byte(record), 0o644)
		}
		if matches, err := Resolve(root, nil); err != nil || len(matches) != 1 {
			t.Fatalf("Resolve with record %q = %#v, %v", record, matches, err)
		}
		if coverage, err := ResolveCoverage(root, []string{"future/file"}); err != nil || len(coverage.Globals) != 1 {
			t.Fatalf("coverage with record %q = %#v, %v", record, coverage, err)
		}
	}
	if _, err := Render(root); err != nil {
		t.Fatal(err)
	}
	if findings, err := Check(root); err != nil || len(findings) != 0 {
		t.Fatalf("repaired version Check = %#v, %v", findings, err)
	}
}

func TestVersionDestinationMustBeRegular(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint(symlink), func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, filepath.FromSlash(VersionPath))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			if symlink {
				if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), destination); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(destination, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := Render(root); err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("Render = %v", err)
			}
			findings, err := Check(root)
			if err != nil || !slices.Contains(findings, Finding{Path: VersionPath, Message: "generated path is not a regular file"}) {
				t.Fatalf("Check = %#v, %v", findings, err)
			}
		})
	}
}

func writeTopicForTest(t *testing.T, root, name string, patterns []string) {
	t.Helper()
	var source strings.Builder
	source.WriteString("---\npaths:\n")
	for _, pattern := range patterns {
		source.WriteString("  - '")
		source.WriteString(pattern)
		source.WriteString("'\n")
	}
	source.WriteString("---\nbody\n")
	writeTestFile(t, filepath.Join(root, "docs", "topics", filepath.FromSlash(name)), []byte(source.String()), 0o644)
}

func writeTestFile(t *testing.T, path string, content []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	if err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[filepath.ToSlash(relative)] = "directory"
			return nil
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = "file:" + string(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func outputPathsForTest(outputs []Output) []string {
	paths := make([]string, len(outputs))
	for i, output := range outputs {
		paths[i] = output.Path
	}
	return paths
}

func outputForTest(t *testing.T, outputs []Output, path string) Output {
	t.Helper()
	for _, output := range outputs {
		if output.Path == path {
			return output
		}
	}
	t.Fatalf("output %s not found", path)
	return Output{}
}
