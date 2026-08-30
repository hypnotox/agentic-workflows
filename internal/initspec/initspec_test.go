package initspec

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

func descs() []catalog.VarDescriptor {
	return []catalog.VarDescriptor{
		{Key: "gateCmd", Kind: "string", Default: "./x gate", Options: []string{"./x gate", "make"}},
		{Key: "flavor", Kind: "enum", Options: []string{"//", "#"}},
	}
}

// errReader fails on the first Read, exercising prompt's non-EOF error branch.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestResolveSilentSeedsEmpty(t *testing.T) {
	vars, _, err := Resolve(descs(), nil, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "" {
		t.Errorf("silent gateCmd = %q, want empty", vars["gateCmd"])
	}
}

func TestResolveExplicitAnswersWin(t *testing.T) {
	a := map[string]string{"gateCmd": "make test", "flavor": "//"}
	vars, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "make test" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if vars["flavor"] != "//" {
		t.Errorf("flavor = %q", vars["flavor"])
	}
}

type failingPromptWriter struct{}

func (failingPromptWriter) Write([]byte) (int, error) { return 0, errors.New("write") }

func TestWritePromptValidation(t *testing.T) {
	for _, test := range []struct {
		descriptor catalog.VarDescriptor
		options    []string
		tail       string
	}{
		{catalog.VarDescriptor{Key: "bad\n"}, nil, "tail"},
		{catalog.VarDescriptor{Key: "key", Description: " "}, nil, "tail"},
		{catalog.VarDescriptor{Key: "key"}, []string{" "}, "tail"},
		{catalog.VarDescriptor{Key: "key"}, nil, " "},
	} {
		if err := writePrompt(io.Discard, test.descriptor, test.options, test.tail); err == nil {
			t.Fatal("invalid prompt accepted")
		}
	}
	if err := writePrompt(failingPromptWriter{}, catalog.VarDescriptor{Key: "key"}, nil, "tail"); err == nil {
		t.Fatal("prompt write failure accepted")
	}
	if _, _, err := Resolve([]catalog.VarDescriptor{{Key: "choice", Kind: "enum", Options: []string{"one"}}}, nil, strings.NewReader(""), failingPromptWriter{}, true, nil); err == nil {
		t.Fatal("enum prompt write failure accepted")
	}
	if _, _, err := Resolve([]catalog.VarDescriptor{{Key: "key", Kind: "string"}}, nil, strings.NewReader(""), failingPromptWriter{}, true, nil); err == nil {
		t.Fatal("scalar prompt write failure accepted")
	}
}

func TestResolveInteractiveDefaultAndEnumIndex(t *testing.T) {
	// gateCmd: empty line → default; flavor: "2" → second enum option.
	in := strings.NewReader("\n2\n")
	vars, _, err := Resolve(descs(), nil, in, &strings.Builder{}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "./x gate" {
		t.Errorf("gateCmd = %q, want default", vars["gateCmd"])
	}
	if vars["flavor"] != "#" {
		t.Errorf("flavor = %q, want #", vars["flavor"])
	}
}

func TestResolveInteractiveLiteralAndEnumNonNumeric(t *testing.T) {
	// gateCmd: literal; flavor: non-numeric literal → literal value.
	in := strings.NewReader("custom\n//\n")
	vars, _, err := Resolve(descs(), nil, in, &strings.Builder{}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "custom" {
		t.Errorf("gateCmd = %q", vars["gateCmd"])
	}
	if vars["flavor"] != "//" {
		t.Errorf("flavor = %q", vars["flavor"])
	}
}

// An answer key matching no descriptor is a typo that would otherwise no-op
// silently, leaving the intended var empty (publication-degraded prose).
func TestResolveRejectsUnknownAnswerKey(t *testing.T) {
	a := map[string]string{"gatecmd": "make"} // typo'd case
	if _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil); err == nil {
		t.Fatal("expected error for unknown answer key")
	}
}

// An explicit enum answer outside the options must error like multiselect
// does, not land verbatim in vars.
func TestResolveRejectsInvalidEnumAnswer(t *testing.T) {
	a := map[string]string{"flavor": ";;"}
	if _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil); err == nil {
		t.Fatal("expected error for enum answer outside options")
	}
}

func TestResolvePromptReadError(t *testing.T) {
	if _, _, err := Resolve(descs(), nil, errReader{}, &strings.Builder{}, true, nil); err == nil {
		t.Fatal("expected error from a failing reader")
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

// An audit-scopes answer is comma-split, trimmed, empties dropped, and routed
// out of the vars map (ADR-0051).
func TestResolveAuditScopes(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "commitScopes", Kind: "string", Target: "audit-scopes"}}
	vars, scopes, err := Resolve(ds, map[string]string{"commitScopes": " adr, awf ,,plans "}, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := vars["commitScopes"]; ok {
		t.Error("audit-scopes answer must not land in the vars map")
	}
	if !slices.Equal(scopes, []string{"adr", "awf", "plans"}) {
		t.Errorf("scopes = %v, want [adr awf plans]", scopes)
	}
}

// An empty (or absent) audit-scopes answer resolves to nil - accept-any
// audit semantics, nothing written (ADR-0051, ADR-0017).
func TestResolveAuditScopesEmptyIsNil(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "commitScopes", Kind: "string", Target: "audit-scopes"}}
	_, scopes, err := Resolve(ds, nil, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if scopes != nil {
		t.Errorf("empty answer must resolve to nil scopes, got %v", scopes)
	}
}

// A prompt stream that hits EOF (e.g. /dev/null, which stats as a char device
// and so counts as interactive) switches every remaining descriptor to the
// silent path: the in-flight prompt keeps its default, no further prompt text
// is emitted, and later values resolve empty.
func TestResolveEOFFallsSilent(t *testing.T) {
	ds := []catalog.VarDescriptor{
		{Key: "first", Kind: "string", Default: "d1"},
		{Key: "second", Kind: "string", Default: "d2"},
	}
	var out strings.Builder
	vars, _, err := Resolve(ds, nil, strings.NewReader(""), &out, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "variable: first") {
		t.Errorf("the first prompt should have been emitted:\n%s", out.String())
	}
	if strings.Contains(out.String(), "variable: second") {
		t.Errorf("prompt text emitted after EOF:\n%s", out.String())
	}
	if vars["first"] != "d1" {
		t.Errorf(`vars["first"] = %q, want the prompted default "d1"`, vars["first"])
	}
	if vars["second"] != "" {
		t.Errorf(`vars["second"] = %q, want "" (silent path)`, vars["second"])
	}
}

// The needed filter (ADR-0086 Decision 6): vars outside the selection's
// referenced set are seeded empty without a prompt; explicit answers stay
// honored; a filter error propagates.
// invariant: tooling/init-and-enablement:init-prompts-enabled-vars (TestResolveSkipsUnneededVarPrompts)
func TestResolveSkipsUnneededVarPrompts(t *testing.T) {
	ds := []catalog.VarDescriptor{
		{Key: "a", Kind: "string"},
		{Key: "b", Kind: "string"},
	}
	needed := func() (map[string]bool, error) {
		return map[string]bool{"a": true}, nil
	}
	var out strings.Builder
	vars, _, err := Resolve(ds, nil, strings.NewReader("va\n"), &out, true, needed)
	if err != nil {
		t.Fatal(err)
	}
	if vars["a"] != "va" || vars["b"] != "" {
		t.Fatalf("want a prompted, b seeded empty; got %v", vars)
	}
	if !strings.Contains(out.String(), "a") || strings.Contains(out.String(), "b (") {
		t.Fatalf("transcript must prompt a and not b:\n%s", out.String())
	}
}

func TestResolveHonorsExplicitAnswerForUnneededVar(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "b", Kind: "string"}}
	needed := func() (map[string]bool, error) {
		return map[string]bool{}, nil
	}
	vars, _, err := Resolve(ds, map[string]string{"b": "x"}, strings.NewReader(""), &strings.Builder{}, true, needed)
	if err != nil {
		t.Fatal(err)
	}
	if vars["b"] != "x" {
		t.Fatalf("explicit answers are honored regardless of the filter, got %v", vars)
	}
}

func TestResolvePropagatesNeededError(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "a", Kind: "string"}}
	needed := func() (map[string]bool, error) {
		return nil, errors.New("boom")
	}
	if _, _, err := Resolve(ds, nil, strings.NewReader(""), &strings.Builder{}, false, needed); err == nil {
		t.Fatal("a needed-filter error must propagate")
	}
}
