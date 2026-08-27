package projectlicense

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func projectLicenseFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	projectRoot := os.DirFS("../..")
	root := fstest.MapFS{}
	for _, path := range []string{licensePath, readmePath, goreleaserPath} {
		data, err := fs.ReadFile(projectRoot, path)
		if err != nil {
			t.Fatalf("read fixture %s: %v", path, err)
		}
		root[path] = &fstest.MapFile{Data: data}
	}
	return root
}

func cloneFixture(root fstest.MapFS) fstest.MapFS {
	clone := fstest.MapFS{}
	for path, file := range root {
		copy := *file
		copy.Data = append([]byte(nil), file.Data...)
		clone[path] = &copy
	}
	return clone
}

// invariant: tooling/project-license:project-license-agpl (TestProjectLicenseAGPL)
func TestProjectLicenseAGPL(t *testing.T) {
	valid := projectLicenseFixture(t)
	if err := Verify(valid); err != nil {
		t.Fatalf("project license verification failed: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		data     func([]byte) []byte
		want     string
		wantAlso string
	}{
		{
			name: "license length",
			path: licensePath,
			data: func(data []byte) []byte {
				return append(append([]byte(nil), data[:len(data)-1]...), 'x', '\n')
			},
			want: "LICENSE has 34021 bytes, want 34020",
		},
		{
			name: "license terminal newline",
			path: licensePath,
			data: func(data []byte) []byte {
				copy := append([]byte(nil), data...)
				copy[len(copy)-1] = 'x'
				return copy
			},
			want: "LICENSE must end with exactly one newline",
		},
		{
			name: "license hash",
			path: licensePath,
			data: func(data []byte) []byte {
				copy := append([]byte(nil), data...)
				copy[0] ^= 1
				return copy
			},
			want: "LICENSE SHA-256",
		},
		{
			name: "complete README badge",
			path: readmePath,
			data: func(data []byte) []byte {
				return []byte(strings.Replace(string(data), licenseBadge, "[![License: AGPL-3.0-only](wrong.svg)](COPYING)", 1))
			},
			want: "README.md lacks the AGPL-3.0-only badge",
		},
		{
			name: "README footer",
			path: readmePath,
			data: func(data []byte) []byte {
				return []byte(strings.Replace(string(data), licenseFooter, "AGPL without canonical link", 1))
			},
			want: "README.md lacks the AGPL-3.0-only footer",
		},
		{
			name: "every archive includes license",
			path: goreleaserPath,
			data: func([]byte) []byte {
				return []byte("archives:\n  - files: [LICENSE, README.md]\n  - files: [README.md]\n# files:\n#   - LICENSE\n")
			},
			want: ".goreleaser.yaml archive 2 must include LICENSE",
		},
		{
			name: "structured archive source requires license",
			path: goreleaserPath,
			data: func([]byte) []byte {
				return []byte("archives:\n  - files:\n      - src: README.md\n")
			},
			want: ".goreleaser.yaml archive 1 must include LICENSE",
		},
		{
			name: "archive files must be a sequence",
			path: goreleaserPath,
			data: func([]byte) []byte {
				return []byte("archives:\n  - files: LICENSE\n")
			},
			want: "files must be a sequence",
		},
		{
			name: "archive file must be a string or mapping",
			path: goreleaserPath,
			data: func([]byte) []byte {
				return []byte("archives:\n  - files:\n      - [LICENSE]\n")
			},
			want: "archive file must be a string or mapping",
		},
		{
			name: "structured archive source must decode",
			path: goreleaserPath,
			data: func([]byte) []byte {
				return []byte("archives:\n  - files:\n      - src: [LICENSE]\n")
			},
			want:     "decode structured archive file:",
			wantAlso: "cannot unmarshal !!seq into string",
		},
		{
			name: "archive set exists",
			path: goreleaserPath,
			data: func([]byte) []byte { return []byte("version: 2\n") },
			want: ".goreleaser.yaml defines no archives",
		},
		{
			name: "archive config parses",
			path: goreleaserPath,
			data: func([]byte) []byte { return []byte("archives: [\n") },
			want: "parse .goreleaser.yaml archives",
		},
		{
			name: "README obsolete project MIT reference",
			path: readmePath,
			data: func(data []byte) []byte { return append(data, []byte("\nProject license: MIT\n")...) },
			want: "README.md contains an obsolete project MIT reference",
		},
		{
			name: "GoReleaser obsolete project MIT reference",
			path: goreleaserPath,
			data: func(data []byte) []byte { return append(data, []byte("\n# Project license: MIT\n")...) },
			want: ".goreleaser.yaml contains an obsolete project MIT reference",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := cloneFixture(valid)
			root[test.path].Data = test.data(root[test.path].Data)
			err := Verify(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Verify error lacks %q:\n%v", test.want, err)
			}
			if test.wantAlso != "" && !strings.Contains(err.Error(), test.wantAlso) {
				t.Fatalf("Verify error lacks %q:\n%v", test.wantAlso, err)
			}
		})
	}

	t.Run("dependency metadata and third-party notices are excluded", func(t *testing.T) {
		root := cloneFixture(valid)
		root["go.mod"] = &fstest.MapFile{Data: []byte("dependency license: MIT License")}
		root["NOTICE"] = &fstest.MapFile{Data: []byte("third-party notice: MIT License")}
		if err := Verify(root); err != nil {
			t.Fatalf("excluded license metadata affected project verification: %v", err)
		}
	})
}

func TestVerifyReportsUnreadableArtifacts(t *testing.T) {
	valid := projectLicenseFixture(t)
	for _, missing := range []string{licensePath, readmePath, goreleaserPath} {
		t.Run(missing, func(t *testing.T) {
			root := cloneFixture(valid)
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
