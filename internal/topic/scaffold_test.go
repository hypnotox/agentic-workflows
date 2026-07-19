package topic

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func scaffoldConfig() *config.Config { return &config.Config{Domains: []string{"rendering"}} }

func TestScaffoldFilesExactPairAndSlug(t *testing.T) {
	files, err := ScaffoldFiles(t.TempDir(), scaffoldConfig(), "rendering", "  Current State: Topics!  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v", files)
	}
	if files[0].Path != ".awf/topics/metadata/rendering/current-state-topics.yaml" || files[1].Path != ".awf/topics/parts/rendering/current-state-topics/current-state.md" {
		t.Fatalf("paths = %q, %q", files[0].Path, files[1].Path)
	}
	wantMetadata := "title: 'Current State: Topics!'\nsummary: Current project contracts for this topic.\npaths:\n    - replace/with/project/path/**\n# EDIT: replace the path placeholder above with anchored project paths.\n"
	if string(files[0].Content) != wantMetadata {
		t.Errorf("metadata:\n%s\nwant:\n%s", files[0].Content, wantMetadata)
	}
	wantPart := "<!-- awf:comment Replace the placeholder prose below, edit metadata paths, and add reviewed claims manually. -->\nCurrent project contracts for this topic are documented here.\n\n## Claims\n"
	if string(files[1].Content) != wantPart {
		t.Errorf("part:\n%s\nwant:\n%s", files[1].Content, wantPart)
	}
	for _, forbidden := range []string{"### `rule:", "### `invariant:", "Origin:", "Backing:", "Verify:"} {
		if strings.Contains(string(files[1].Content), forbidden) {
			t.Errorf("part invented %q", forbidden)
		}
	}
	root := t.TempDir()
	metadataRoot := filepath.Join(root, ".awf/topics/metadata")
	if _, _, err := ParseMetadata(metadataRoot, filepath.Join(metadataRoot, "rendering/current-state-topics.yaml"), files[0].Content); err != nil {
		t.Fatalf("scaffold metadata is invalid: %v", err)
	}
	if topic, err := ParsePart(TopicID{"rendering", "current-state-topics"}, files[1].Path, files[1].Content); err != nil {
		t.Fatalf("scaffold part is invalid: %v", err)
	} else if len(topic.Claims) != 0 {
		t.Fatalf("scaffold claims = %#v", topic.Claims)
	}
}

func TestScaffoldFilesValidationAndAllocation(t *testing.T) {
	for _, tc := range []struct {
		name, domain, title, contains string
	}{
		{"noncanonical domain", "Rendering", "Title", "lowercase kebab-case"},
		{"unknown domain", "tooling", "Title", "not configured"},
		{"empty slug", "rendering", "!!!", "no usable characters"},
		{"reserved index", "rendering", "Index!", "reserved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ScaffoldFiles(t.TempDir(), scaffoldConfig(), tc.domain, tc.title)
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error = %v, want %q", err, tc.contains)
			}
		})
	}

	root := t.TempDir()
	for _, path := range []string{
		".awf/topics/metadata/rendering/same.yaml",
		".awf/topics/parts/rendering/same/current-state.md",
	} {
		testsupport.WriteFile(t, filepath.Join(root, path), "existing\n")
	}
	files, err := ScaffoldFiles(root, scaffoldConfig(), "rendering", "Same")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(files[0].Path, "/same-2.yaml") || !strings.Contains(files[1].Path, "/same-2/") {
		t.Fatalf("collision paths = %#v", files)
	}
}

func TestScaffoldFilesRefusesEitherOrphanHalf(t *testing.T) {
	for _, orphan := range []string{
		".awf/topics/metadata/rendering/orphan.yaml",
		".awf/topics/parts/rendering/orphan/current-state.md",
	} {
		t.Run(filepath.Base(filepath.Dir(orphan))+filepath.Ext(orphan), func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteFile(t, filepath.Join(root, orphan), "orphan\n")
			if _, err := ScaffoldFiles(root, scaffoldConfig(), "rendering", "Orphan"); err == nil || !strings.Contains(err.Error(), "orphaned scaffold half") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestScaffoldFilesReportsMetadataAndPartStatErrors(t *testing.T) {
	for _, tc := range []struct {
		name, faultTree, wantPath string
	}{
		{name: "metadata tree", faultTree: "/topics/metadata/", wantPath: ".awf/topics/metadata/rendering/stat-failure.yaml"},
		{name: "part tree", faultTree: "/topics/parts/", wantPath: ".awf/topics/parts/rendering/stat-failure/current-state.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statErr := errors.New("staged stat failure")
			testsupport.SwapVar(t, &scaffoldStat, func(path string) (os.FileInfo, error) {
				if strings.Contains(filepath.ToSlash(path), tc.faultTree) {
					return nil, statErr
				}
				return nil, os.ErrNotExist
			})
			_, err := ScaffoldFiles(t.TempDir(), scaffoldConfig(), "rendering", "Stat Failure")
			if !errors.Is(err, statErr) || !strings.Contains(err.Error(), "inspect topic scaffold path") || !strings.Contains(err.Error(), tc.wantPath) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestScaffoldPathExists(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "present")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := scaffoldPathExists(path); err != nil || !ok {
		t.Fatalf("present = %v, %v", ok, err)
	}
	if ok, err := scaffoldPathExists(filepath.Join(root, "missing")); err != nil || ok {
		t.Fatalf("missing = %v, %v", ok, err)
	}
}
