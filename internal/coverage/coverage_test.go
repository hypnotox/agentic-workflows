package coverage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile writes a coverprofile and returns its path.
func writeProfile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "cover.out")
	if err := os.WriteFile(p, []byte("mode: set\n"+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// module builds a temp module root: go.mod + one source file, and returns root + modpath.
func module(t *testing.T, src string) (root, modPath string) {
	t.Helper()
	root = t.TempDir()
	modPath = "example.com/m"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modPath+"\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
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

// invariant: coverage-gate-100
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

// invariant: coverage-ignore-reason
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
		"no-colon":   "garbage line\n",
		"few-fields": modPath + "/f.go:2.1,2.5 1\n",
		"bad-span":   modPath + "/f.go:nope 1 0\n",
		"bad-stmt":   modPath + "/f.go:2.1,2.5 x 0\n",
		"bad-count":  modPath + "/f.go:2.1,2.5 1 x\n",
		"no-comma":   modPath + "/f.go:2.1 1 0\n",
		"no-dot":     modPath + "/f.go:2,3.4 1 0\n",   // span has a comma but no dot before it
		"bad-start":  modPath + "/f.go:x.1,2.3 1 0\n", // start line is non-numeric
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
	swapGetwd(t, func() (string, error) { return root, nil })
	rep, err := CheckProfile(prof)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("expected OK, got %+v", rep)
	}
}

func TestCheckProfileGetwdError(t *testing.T) {
	swapGetwd(t, func() (string, error) { return "", errors.New("boom") })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected getwd error to propagate")
	}
}

func TestCheckProfileNoModule(t *testing.T) {
	// A working directory with no go.mod ancestor -> moduleRoot fails.
	dir := t.TempDir()
	swapGetwd(t, func() (string, error) { return dir, nil })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected go.mod-not-found error")
	}
}

func TestCheckProfileNoModuleLine(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("// no module line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	swapGetwd(t, func() (string, error) { return root, nil })
	if _, err := CheckProfile("x"); err == nil {
		t.Fatal("expected no-module-line error")
	}
}

func TestPercentEmptyIs100(t *testing.T) {
	if (Report{}).Percent() != 100 {
		t.Fatal("empty report should be 100%")
	}
}

// swapGetwd overrides the package getwd seam for the duration of a test.
func swapGetwd(t *testing.T, fn func() (string, error)) {
	t.Helper()
	orig := getwd
	getwd = fn
	t.Cleanup(func() { getwd = orig })
}
