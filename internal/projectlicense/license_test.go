package projectlicense

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

// invariant: tooling/project-license:project-license-agpl (TestProjectLicenseAGPL)
func TestProjectLicenseAGPL(t *testing.T) {
	if err := Verify(os.DirFS("../..")); err != nil {
		t.Fatalf("project license verification failed: %v", err)
	}
}

func TestVerifyExcludesDependencyMetadataAndThirdPartyNotices(t *testing.T) {
	projectRoot := os.DirFS("../..")
	root := fstest.MapFS{
		"go.mod": &fstest.MapFile{Data: []byte("dependency license: MIT License")},
		"NOTICE": &fstest.MapFile{Data: []byte("third-party notice: MIT License")},
	}
	for _, path := range []string{licensePath, readmePath, goreleaserPath} {
		data, err := fs.ReadFile(projectRoot, path)
		if err != nil {
			t.Fatalf("read fixture %s: %v", path, err)
		}
		root[path] = &fstest.MapFile{Data: data}
	}
	if err := Verify(root); err != nil {
		t.Fatalf("dependency metadata and third-party notices must not affect verification: %v", err)
	}
}

func TestVerifyReportsInvalidArtifacts(t *testing.T) {
	root := fstest.MapFS{
		licensePath:    &fstest.MapFile{Data: []byte("MIT License\n\n")},
		readmePath:     &fstest.MapFile{Data: []byte("MIT License")},
		goreleaserPath: &fstest.MapFile{Data: []byte("MIT License")},
	}
	err := Verify(root)
	if err == nil {
		t.Fatal("Verify succeeded for invalid project-license artifacts")
	}
	for _, want := range []string{
		"LICENSE has", "exactly one newline", "SHA-256", "README.md lacks the AGPL-3.0-only badge",
		"README.md lacks the AGPL-3.0-only footer", ".goreleaser.yaml archives must include LICENSE",
		"README.md contains an obsolete project MIT reference", ".goreleaser.yaml contains an obsolete project MIT reference",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Verify error lacks %q:\n%v", want, err)
		}
	}
}

func TestVerifyReportsUnreadableArtifacts(t *testing.T) {
	for _, missing := range []string{licensePath, readmePath, goreleaserPath} {
		t.Run(missing, func(t *testing.T) {
			root := fstest.MapFS{
				licensePath:    &fstest.MapFile{Data: []byte("bad")},
				readmePath:     &fstest.MapFile{Data: []byte("bad")},
				goreleaserPath: &fstest.MapFile{Data: []byte("bad")},
			}
			delete(root, missing)
			err := Verify(root)
			if err == nil || !strings.Contains(err.Error(), "read "+missing) {
				t.Fatalf("want read error for %s, got %v", missing, err)
			}
			if missing == licensePath && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("LICENSE read error must retain fs.ErrNotExist: %v", err)
			}
		})
	}
}
