package initspec

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func descs() []catalog.VarDescriptor {
	return []catalog.VarDescriptor{
		{Key: "gateCmd", Kind: "string", Default: "./x gate", Options: []string{"./x gate", "make"}},
		{Key: "invariantsMarker", Kind: "enum", Target: "invariants-marker", Options: []string{"//", "#"}},
		{Key: "invariantsGlobs", Kind: "string", Target: "invariants-globs"},
	}
}

// errReader fails on the first Read, exercising prompt's non-EOF error branch.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestResolveSilentSeedsEmpty(t *testing.T) {
	vars, inv, err := Resolve(descs(), nil, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "" {
		t.Errorf("silent gateCmd = %q, want empty", vars["gateCmd"])
	}
	if inv != nil {
		t.Errorf("silent inv = %v, want nil", inv)
	}
}

func TestResolveExplicitAnswersWin(t *testing.T) {
	a := map[string]string{"gateCmd": "make test", "invariantsMarker": "//", "invariantsGlobs": "*.go,*.s"}
	vars, inv, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "make test" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if inv == nil || len(inv.Sources) != 1 || inv.Sources[0].Marker != "//" ||
		len(inv.Sources[0].Globs) != 2 || inv.Sources[0].Globs[0] != "*.go" {
		t.Errorf("inv = %+v", inv)
	}
}

func TestResolveInteractiveDefaultAndEnumIndex(t *testing.T) {
	// gateCmd: empty line → default; marker: "2" → second option; globs: literal.
	in := strings.NewReader("\n2\n*.go\n")
	vars, inv, err := Resolve(descs(), nil, in, &strings.Builder{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "./x gate" {
		t.Errorf("gateCmd = %q, want default", vars["gateCmd"])
	}
	if inv == nil || inv.Sources[0].Marker != "#" {
		t.Errorf("marker = %+v, want #", inv)
	}
}

func TestResolveInteractiveLiteralAndEnumNonNumeric(t *testing.T) {
	// gateCmd: literal; marker: non-numeric literal; globs: literal.
	in := strings.NewReader("custom\n//\n*.go\n")
	vars, inv, err := Resolve(descs(), nil, in, &strings.Builder{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "custom" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if inv.Sources[0].Marker != "//" {
		t.Errorf("marker = %q", inv.Sources[0].Marker)
	}
}

func TestResolvePromptReadError(t *testing.T) {
	if _, _, err := Resolve(descs(), nil, errReader{}, &strings.Builder{}, true); err == nil {
		t.Fatal("expected error from a failing reader")
	}
}

func TestResolveInvariantsHalfSetErrors(t *testing.T) {
	a := map[string]string{"invariantsMarker": "//"}
	if _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false); err == nil {
		t.Fatal("expected error for marker without globs")
	}
}

func TestResolveInvariantsWhitespaceGlobsIsHalfSet(t *testing.T) {
	// A marker plus an all-whitespace/comma globs value parses to zero globs, so it
	// is treated as half-set (error), not a marker-only source that scans nothing.
	a := map[string]string{"invariantsMarker": "//", "invariantsGlobs": ", ,"}
	if _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false); err == nil {
		t.Fatal("expected error for marker with whitespace-only globs")
	}
}

func TestDescribeNormalizesTargetAndIsValidJSON(t *testing.T) {
	b, err := Describe(descs())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"descriptors"`) || !strings.Contains(s, `"target": "var"`) {
		t.Errorf("describe JSON missing fields:\n%s", s)
	}
}

func TestParseAnswersFile(t *testing.T) {
	m, err := ParseAnswersFile([]byte("gateCmd: ./x gate\n"))
	if err != nil || m["gateCmd"] != "./x gate" {
		t.Fatalf("m=%v err=%v", m, err)
	}
	if _, err := ParseAnswersFile([]byte("- not a map\n")); err == nil {
		t.Fatal("expected error for non-map answers")
	}
}

func TestMergeSetFlags(t *testing.T) {
	base := map[string]string{}
	if err := MergeSetFlags(base, []string{"a=1", "b=2"}); err != nil {
		t.Fatal(err)
	}
	if base["a"] != "1" || base["b"] != "2" {
		t.Errorf("base=%v", base)
	}
	if err := MergeSetFlags(base, []string{"bad"}); err == nil {
		t.Fatal("expected error for missing =")
	}
}
