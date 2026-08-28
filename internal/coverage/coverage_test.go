package coverage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

var Check = func(profilePath, srcRoot, modPath string) (Report, error) {
	analysis, err := Analyze(profilePath, srcRoot, modPath)
	return analysis.Filtered, err
}

var CheckProfile = func(profilePath string) (Report, error) {
	analysis, err := AnalyzeProfile(profilePath)
	return analysis.Filtered, err
}

// writeProfile writes a coverprofile and returns its path.
func writeProfile(t *testing.T, dir, body string) string {
	t.Helper()
	return testsupport.WriteProfile(t, dir, body)
}

func writeNamedProfile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	testsupport.WriteFile(t, path, contents)
	return path
}

// module builds a temp module root: go.mod + one source file, and returns root + modpath.
func module(t *testing.T, src string) (root, modPath string) {
	t.Helper()
	root = t.TempDir()
	modPath = "example.com/m"
	testsupport.WriteGoModule(t, root, modPath, src)
	return root, modPath
}

func TestCheckCountsCoveredAndTotal(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root,
		modPath+"/f.go:2.10,2.12 2 1\n"+
			"\n"+ // blank line: exercises parseProfile's empty-line skip
			modPath+"/f.go:3.1,3.5 3 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 5 || rep.Covered != 2 {
		t.Fatalf("got %+v, want Covered 2 Total 5", rep)
	}
	if rep.OK() {
		t.Error("OK should be false below 100%")
	}
}

func TestCheckFailsBelow100(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK() {
		t.Fatal("expected OK false for an uncovered statement")
	}
	// A fully covered profile is OK and 100%.
	prof2 := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 1\n")
	rep2, err := Check(prof2, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if !rep2.OK() || rep2.Percent() != 100 {
		t.Fatalf("expected OK 100%%, got %+v (%.1f)", rep2, rep2.Percent())
	}
}

func TestCheckHonoursIgnoreMarker(t *testing.T) {
	// Line 2 carries the directive; its block is dropped from both counts,
	// leaving the line-3 covered block as the only statement -> 100%.
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.10 1 0\n"+
			modPath+"/f.go:3.1,3.10 1 1\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 || !rep.OK() {
		t.Fatalf("ignored block not dropped: %+v", rep)
	}
}

func TestCheckIgnoreMarkerLineAbove(t *testing.T) {
	// Directive on the line directly above the block also drops it.
	src := "package m\n//" + " coverage-ignore: panic guard\npanicline\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root, modPath+"/f.go:3.1,3.10 1 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 || !rep.OK() {
		t.Fatalf("line-above directive not honoured: %+v", rep)
	}
}

// invariant: tooling/quality-gates:coverage-ignore-reason (TestCheckRejectsReasonlessMarker)
func TestCheckRejectsReasonlessMarker(t *testing.T) {
	src := "package m\nvar x = 1 //" + " coverage-ignore\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.10 1 0\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected error for a reasonless coverage-ignore directive")
	}
}

func TestCheckMissingProfile(t *testing.T) {
	root, modPath := module(t, "package m\n")
	if _, err := Check(filepath.Join(root, "nope.out"), root, modPath); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestCheckMissingSourceFile(t *testing.T) {
	root, modPath := module(t, "package m\n")
	prof := writeProfile(t, root, modPath+"/ghost.go:2.1,2.5 1 0\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestCheckMalformedLines(t *testing.T) {
	root, modPath := module(t, "package m\n")
	for name, body := range map[string]string{
		"no-colon":       "garbage line\n",
		"few-fields":     modPath + "/f.go:2.1,2.5 1\n",
		"bad-span":       modPath + "/f.go:nope 1 0\n",
		"bad-stmt":       modPath + "/f.go:2.1,2.5 x 0\n",
		"bad-count":      modPath + "/f.go:2.1,2.5 1 x\n",
		"no-comma":       modPath + "/f.go:2.1 1 0\n",
		"no-dot":         modPath + "/f.go:2,3.4 1 0\n",   // span has a comma but no dot before it
		"bad-start":      modPath + "/f.go:x.1,2.3 1 0\n", // start line is non-numeric
		"bad-start-col":  modPath + "/f.go:1.x,2.3 1 0\n",
		"bad-end-dot":    modPath + "/f.go:1.2,3 1 0\n",
		"bad-end-line":   modPath + "/f.go:1.2,x.3 1 0\n",
		"bad-end-column": modPath + "/f.go:1.2,3.x 1 0\n",
		"zero-position":  modPath + "/f.go:0.1,2.3 1 0\n",
	} {
		t.Run(name, func(t *testing.T) {
			prof := writeProfile(t, root, body)
			if _, err := Check(prof, root, modPath); err == nil {
				t.Errorf("%s: expected parse error", name)
			}
		})
	}
}

func TestCheckScannerError(t *testing.T) {
	// A line longer than bufio.Scanner's default 64KiB token makes the scanner
	// error, exercising parseProfile's sc.Err() branch.
	root, modPath := module(t, "package m\n")
	prof := writeProfile(t, root, strings.Repeat("a", 70000)+"\n")
	if _, err := Check(prof, root, modPath); err == nil {
		t.Fatal("expected scanner error for an over-long profile line")
	}
}

func TestModulePathReadError(t *testing.T) {
	// modulePath's ReadFile error branch (the go.mod path is unreadable).
	if _, err := modulePath(filepath.Join(t.TempDir(), "absent.mod")); err == nil {
		t.Fatal("expected read error for a missing go.mod")
	}
}

func TestCheckProfileResolvesModule(t *testing.T) {
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.5 1 1\n")
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	rep, err := CheckProfile(prof)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("expected OK, got %+v", rep)
	}
}

func TestCheckProfileGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected getwd error to propagate")
	}
}

func TestCheckProfileNoModule(t *testing.T) {
	// No go.mod anywhere up the walk -> moduleRoot reaches the filesystem root and
	// errors. Stub hasGoMod (rather than rely on t.TempDir() having no go.mod
	// ancestor) so the root-reached branch is exercised hermetically - a stray
	// go.mod under /tmp must not short-circuit the walk.
	testsupport.SwapVar(t, &getwd, func() (string, error) { return t.TempDir(), nil })
	testsupport.SwapVar(t, &hasGoMod, func(string) bool { return false })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected go.mod-not-found error")
	}
}

func TestCheckProfileNoModuleLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// no module line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected no-module-line error")
	}
}

func TestCheckMergesDuplicateBlocks(t *testing.T) {
	// `go test ./... -coverpkg=./...` emits each block once per test binary. The
	// same block must be counted once, OR-ing the counts: here one statement,
	// covered in exactly one of three emissions -> 100%, Total 1 (not 3).
	root, modPath := module(t, "package m\nfunc F() {}\n")
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.5 1 0\n"+
			modPath+"/f.go:2.1,2.5 1 1\n"+
			modPath+"/f.go:2.1,2.5 1 0\n")
	rep, err := Check(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 || rep.Covered != 1 {
		t.Fatalf("duplicate blocks not merged: %+v, want Covered 1 Total 1", rep)
	}
}

func TestPercentEmptyIs100(t *testing.T) {
	if (Report{}).Percent() != 100 {
		t.Fatal("empty report should be 100%")
	}
}

// invariant: tooling/quality-gates:covered-profile-honors-ignores (TestFilterDropsIgnoredKeepsRest)
func TestFilterDropsIgnoredKeepsRest(t *testing.T) {
	// Line 2 carries the directive; its block must be dropped from the emitted
	// profile, and the line-3 block must survive verbatim - the "covered" report
	// contains exactly the blocks the ADR-0012 gate holds accountable.
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.10 1 0\n"+
			modPath+"/f.go:3.1,3.10 1 1\n")
	got, err := Filter(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "mode: set\n" + modPath + "/f.go:3.1,3.10 1 1\n"
	if got != want {
		t.Fatalf("filtered profile mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestFilterMergesDuplicatesAndSortsDeterministically(t *testing.T) {
	// Duplicate emissions (one per test binary) collapse to one line with OR-ed
	// counts, and output is ordered by (file, startLine) regardless of input order.
	root, modPath := module(t, "package m\nfunc F() {}\nfunc G() {}\n")
	prof := writeProfile(t, root,
		modPath+"/f.go:3.1,3.5 1 0\n"+
			modPath+"/f.go:2.1,2.5 1 0\n"+
			modPath+"/f.go:2.1,2.5 1 1\n") // dup of line-2 block, covered
	got, err := Filter(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "mode: set\n" +
		modPath + "/f.go:2.1,2.5 1 1\n" +
		modPath + "/f.go:3.1,3.5 1 0\n"
	if got != want {
		t.Fatalf("filtered profile mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestFilterSortsAcrossFilesAndSpans(t *testing.T) {
	// Exercises both sort tiebreaks: ordering across distinct files, and - within
	// one file, at the same start line - ordering by span.
	root := t.TempDir()
	modPath := "example.com/m"
	testsupport.WriteGoModule(t, root, modPath, "package m\nfunc F() {}\n") // writes f.go
	if err := os.WriteFile(filepath.Join(root, "a.go"),
		[]byte("package m\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.5 1 1\n"+
			modPath+"/a.go:2.10,2.14 1 1\n"+ // same start line 2 as the next, larger span
			modPath+"/a.go:2.1,2.5 1 0\n")
	got, err := Filter(prof, root, modPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "mode: set\n" +
		modPath + "/a.go:2.1,2.5 1 0\n" +
		modPath + "/a.go:2.10,2.14 1 1\n" +
		modPath + "/f.go:2.1,2.5 1 1\n"
	if got != want {
		t.Fatalf("filtered profile mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestFilterParseError(t *testing.T) {
	root, modPath := module(t, "package m\n")
	if _, err := Filter(filepath.Join(root, "nope.out"), root, modPath); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestFilterIgnoredLinesError(t *testing.T) {
	// A reasonless directive surfaces from ignoredLines through Filter.
	src := "package m\nvar x = 1 //" + " coverage-ignore\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root, modPath+"/f.go:2.1,2.10 1 0\n")
	if _, err := Filter(prof, root, modPath); err == nil {
		t.Fatal("expected error for a reasonless directive")
	}
}

func TestFilterProfileResolvesModule(t *testing.T) {
	src := "package m\nvar x = 1 //" + " coverage-ignore: defensive\nvar y = 2\n"
	root, modPath := module(t, src)
	prof := writeProfile(t, root,
		modPath+"/f.go:2.1,2.10 1 0\n"+
			modPath+"/f.go:3.1,3.10 1 1\n")
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	got, err := FilterProfile(prof)
	if err != nil {
		t.Fatal(err)
	}
	want := "mode: set\n" + modPath + "/f.go:3.1,3.10 1 1\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFilterProfileGetwdError(t *testing.T) {
	testsupport.SwapVar(t, &getwd, func() (string, error) { return "", errors.New("boom") })
	if _, err := FilterProfile("x"); err == nil {
		t.Fatal("expected getwd error to propagate")
	}
}

func TestFilterProfileNoModuleLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// no module line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testsupport.SwapVar(t, &getwd, func() (string, error) { return root, nil })
	if _, err := FilterProfile("x"); err == nil {
		t.Fatal("expected no-module-line error")
	}
}

func TestMergeProfilesIsDeterministicAndORMergesExactBlocks(t *testing.T) {
	root := t.TempDir()
	first := writeNamedProfile(t, root, "first.out", "mode: set\n"+
		"example.com/m/a.go:2.1,2.5 1 0\n"+
		"example.com/m/a.go:2.1,2.5 1 1\n"+
		"example.com/m/c.go:4.1,4.5 1 0\n")
	second := writeNamedProfile(t, root, "second.out", "mode: set\n"+
		"example.com/m/a.go:2.1,2.5 1 0\n"+
		"example.com/m/b.go:3.1,3.5 1 1\n"+
		"example.com/m/c.go:4.1,4.5 1 0\n")

	want := "mode: set\n" +
		"example.com/m/a.go:2.1,2.5 1 1\n" +
		"example.com/m/b.go:3.1,3.5 1 1\n" +
		"example.com/m/c.go:4.1,4.5 1 0\n"
	for _, paths := range [][]string{{first, second}, {second, first}} {
		got, err := MergeProfiles(paths)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("merged profile mismatch:\ngot  %q\nwant %q", got, want)
		}
	}
}

func TestMergeProfilesRejectsInvalidShardInputs(t *testing.T) {
	const blockA = "example.com/m/a.go:2.1,2.5 1 0\n"
	for _, tc := range []struct {
		name       string
		first      string
		second     string
		wantSuffix string
	}{
		{name: "mixed modes", first: "mode: set\n" + blockA, second: "mode: count\n" + blockA, wantSuffix: `: coverage: mixed profile modes: "set" and "count"`},
		{name: "unsupported common mode", first: "mode: count\n" + blockA, second: "mode: count\n" + blockA, wantSuffix: `coverage: unsupported merge mode "count"; want "set"`},
		{name: "malformed first header", first: "not a header\n" + blockA, second: "mode: set\n" + blockA, wantSuffix: `:1: coverage: malformed profile header "not a header"`},
		{name: "malformed later header", first: "mode: set\n" + blockA, second: "mode: set\nmode: set\n" + blockA, wantSuffix: `:2: coverage: unexpected profile header "mode: set"`},
		{name: "conflicting statement counts", first: "mode: set\n" + blockA, second: "mode: set\nexample.com/m/a.go:2.1,2.5 2 0\n", wantSuffix: `:2: coverage: conflicting statement count for "example.com/m/a.go:2.1,2.5": 1 and 2`},
		{name: "conflicting duplicate statement counts", first: "mode: set\n" + blockA + "example.com/m/a.go:2.1,2.5 2 0\n", second: "mode: set\n" + blockA, wantSuffix: `:3: coverage: conflicting statement count for "example.com/m/a.go:2.1,2.5": 1 and 2`},
		{name: "noncanonical path", first: "mode: set\n" + blockA, second: "mode: set\nexample.com/m/../m/a.go:2.1,2.5 1 0\n", wantSuffix: `:2: coverage: noncanonical profile path "example.com/m/../m/a.go"`},
		{name: "reversed span", first: "mode: set\n" + blockA, second: "mode: set\nexample.com/m/a.go:3.1,2.5 1 0\n", wantSuffix: `:2: coverage: reversed profile span "3.1,2.5"`},
		{name: "negative statements", first: "mode: set\n" + blockA, second: "mode: set\nexample.com/m/a.go:2.1,2.5 -1 0\n", wantSuffix: `:2: coverage: negative statement count for "example.com/m/a.go:2.1,2.5"`},
		{name: "invalid set count", first: "mode: set\n" + blockA, second: "mode: set\nexample.com/m/a.go:2.1,2.5 1 2\n", wantSuffix: `:2: coverage: set-mode execution count for "example.com/m/a.go:2.1,2.5" must be 0 or 1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first := writeNamedProfile(t, root, "first.out", tc.first)
			second := writeNamedProfile(t, root, "second.out", tc.second)
			_, err := MergeProfiles([]string{first, second})
			if err == nil {
				t.Fatal("expected merge error")
			}
			if got := err.Error(); !strings.HasSuffix(got, tc.wantSuffix) {
				t.Fatalf("error = %q, want suffix %q", got, tc.wantSuffix)
			}
		})
	}
}

func TestMergeProfilesRequiresMultipleNonemptyProfiles(t *testing.T) {
	if _, err := MergeProfiles(nil); err == nil || err.Error() != "coverage: merge requires at least two profiles" {
		t.Fatalf("empty input error = %v", err)
	}
	root := t.TempDir()
	empty := writeNamedProfile(t, root, "empty.out", "mode: set\n")
	full := writeNamedProfile(t, root, "full.out", "mode: set\nexample.com/m/a.go:2.1,2.5 1 0\n")
	if _, err := MergeProfiles([]string{empty, full}); err == nil || !strings.HasSuffix(err.Error(), ": coverage: profile contains no blocks") {
		t.Fatalf("empty profile error = %v", err)
	}
	missingHeader := writeNamedProfile(t, root, "missing-header.out", "")
	if _, err := MergeProfiles([]string{missingHeader, full}); err == nil || !strings.HasSuffix(err.Error(), ":1: coverage: missing profile header") {
		t.Fatalf("missing header error = %v", err)
	}
}
