package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseRangeTable pins the two accepted shapes and every rejection.
// invariant: git-range-rejects-malformed
func TestParseRangeTable(t *testing.T) {
	cases := []struct {
		arg          string
		bare         bool
		base, head   string
		wantErr      bool
		errSubstring string
	}{
		// Bare base: accepted only when the caller opts in.
		{arg: "HEAD", bare: true, base: "HEAD", head: "HEAD"},
		{arg: "v0.16.0", bare: true, base: "v0.16.0", head: "HEAD"},
		{arg: "HEAD", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		// Two-sided ranges parse the same either way.
		{arg: "a..b", bare: true, base: "a", head: "b"},
		{arg: "a..b", bare: false, base: "a", head: "b"},
		{arg: "v0.10.0..HEAD", bare: false, base: "v0.10.0", head: "HEAD"},
		// Empty input.
		{arg: "", bare: true, wantErr: true, errSubstring: "must not be empty"},
		{arg: "", bare: false, wantErr: true, errSubstring: "must not be empty"},
		// Three-dot and multi-"..": Cut would mangle both into a bogus rev.
		{arg: "a...b", bare: false, wantErr: true, errSubstring: "exactly two dots"},
		{arg: "a..b..c", bare: false, wantErr: true, errSubstring: "exactly two dots"},
		// Empty sides.
		{arg: "..b", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		{arg: "a..", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		// Dash-prefixed sides would reach git as option-like arguments.
		{arg: "-x", bare: true, wantErr: true, errSubstring: "must not start with a dash"},
		{arg: "-x", bare: false, wantErr: true, errSubstring: "must be <a>..<b>"},
		{arg: "-a..b", bare: false, wantErr: true, errSubstring: "must not start with a dash"},
		{arg: "a..-b", bare: false, wantErr: true, errSubstring: "must not start with a dash"},
	}
	for _, c := range cases {
		base, head, err := ParseRange(c.arg, c.bare)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRange(%q, %v) = %q, %q, nil; want an error", c.arg, c.bare, base, head)
				continue
			}
			if !strings.Contains(err.Error(), c.errSubstring) {
				t.Errorf("ParseRange(%q, %v) error = %v; want it to contain %q", c.arg, c.bare, err, c.errSubstring)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRange(%q, %v) unexpected error: %v", c.arg, c.bare, err)
			continue
		}
		if base != c.base || head != c.head {
			t.Errorf("ParseRange(%q, %v) = %q, %q; want %q, %q", c.arg, c.bare, base, head, c.base, c.head)
		}
	}
}

// TestParseRangeIsTheOnlyRangeParser fails if a second range parser reappears
// anywhere in the module. internal/git is skipped because ParseRange itself
// splits on ".."; examples/ is a separate module.
// invariant: git-range-parser-single-definition
func TestParseRangeIsTheOnlyRangeParser(t *testing.T) {
	root := moduleRoot(t)
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil { // coverage-ignore: path comes from a walk rooted at root, so it is always relative-able
			return rerr
		}
		if d.IsDir() {
			switch rel {
			case "internal/git", "examples", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil { // coverage-ignore: the walk just listed this regular file
			return rerr
		}
		for _, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, "strings.Cut(") && strings.Contains(line, `".."`) {
				offenders = append(offenders, rel)
				break
			}
		}
		return nil
	})
	if err != nil { // coverage-ignore: walking the checked-out module does not fail
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Errorf("range parsing must live only in internal/git.ParseRange; found a second parser in: %v", offenders)
	}
}

// moduleRoot walks up from the test's working directory to the go.mod owner, so
// the scan is not anchored to this package's depth.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil { // coverage-ignore: the test process always has a working directory
		t.Fatal(err)
	}
	for {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // coverage-ignore: the test tree always sits under a go.mod
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}
