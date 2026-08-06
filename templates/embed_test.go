package templates

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func regularFiles(fsys fs.FS) ([]string, error) {
	var files []string
	if err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			files = append(files, name)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

func sourceTemplateFiles(fsys fs.FS) ([]string, error) {
	files, err := regularFiles(fsys)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(files, func(name string) bool {
		return path.Dir(name) == "." && strings.HasSuffix(name, ".go")
	}), nil
}

func fileSetDifference(left, right []string) []string {
	var difference []string
	for _, name := range left {
		if !slices.Contains(right, name) {
			difference = append(difference, name)
		}
	}
	return difference
}

func fileSetDiagnostics(source, embedded []string) string {
	missing := fileSetDifference(source, embedded)
	unexpected := fileSetDifference(embedded, source)
	var diagnostics []string
	if len(missing) > 0 {
		diagnostics = append(diagnostics, "missing from embed: "+strings.Join(missing, ", "))
	}
	if len(unexpected) > 0 {
		diagnostics = append(diagnostics, "unexpected in embed: "+strings.Join(unexpected, ", "))
	}
	return strings.Join(diagnostics, "\n")
}

func TestTemplateFileSetParity(t *testing.T) {
	tests := []struct {
		name       string
		source     fstest.MapFS
		embedded   fstest.MapFS
		diagnostic string
	}{
		{
			name: "exact parity with precise root Go exclusion",
			source: fstest.MapFS{
				"agents/a.md": {}, "embed.go": {}, "embed_test.go": {},
				"root.tmpl": {}, "skills/b.go": {},
			},
			embedded: fstest.MapFS{"agents/a.md": {}, "root.tmpl": {}, "skills/b.go": {}},
		},
		{
			name:       "source-only file",
			source:     fstest.MapFS{"agents/a.md": {}, "skills/b.md": {}},
			embedded:   fstest.MapFS{"agents/a.md": {}},
			diagnostic: "missing from embed: skills/b.md",
		},
		{
			name:       "embed-only files have sorted diagnostics",
			source:     fstest.MapFS{"agents/a.md": {}},
			embedded:   fstest.MapFS{"agents/a.md": {}, "skills/z.md": {}, "docs/a.md": {}},
			diagnostic: "unexpected in embed: docs/a.md, skills/z.md",
		},
		{
			name:       "source-only new top-level directory",
			source:     fstest.MapFS{"new-top-level/template.md": {}},
			embedded:   fstest.MapFS{},
			diagnostic: "missing from embed: new-top-level/template.md",
		},
		{
			name:       "dot and underscore files omitted without all semantics",
			source:     fstest.MapFS{"agents/.hidden.md": {}, "agents/_private.md": {}},
			embedded:   fstest.MapFS{},
			diagnostic: "missing from embed: agents/.hidden.md, agents/_private.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := sourceTemplateFiles(tt.source)
			if err != nil {
				t.Fatalf("source template files: %v", err)
			}
			embedded, err := regularFiles(tt.embedded)
			if err != nil {
				t.Fatalf("embedded template files: %v", err)
			}
			if got := fileSetDiagnostics(source, embedded); got != tt.diagnostic {
				t.Fatalf("diagnostic = %q, want %q", got, tt.diagnostic)
			}
		})
	}
}

// invariant: rendering/templates:source-embed-parity (TestSourceEmbedParity)
func TestSourceEmbedParity(t *testing.T) {
	source, err := sourceTemplateFiles(os.DirFS("."))
	if err != nil {
		t.Fatalf("source template files: %v", err)
	}
	embedded, err := regularFiles(FS)
	if err != nil {
		t.Fatalf("embedded template files: %v", err)
	}
	if diagnostic := fileSetDiagnostics(source, embedded); diagnostic != "" {
		t.Fatal(diagnostic)
	}
}
