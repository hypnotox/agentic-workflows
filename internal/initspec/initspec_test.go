package initspec

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
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
	vars, _, _, err := Resolve(descs(), nil, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vars["gateCmd"] != "" {
		t.Errorf("silent gateCmd = %q, want empty", vars["gateCmd"])
	}
}

func TestResolveExplicitAnswersWin(t *testing.T) {
	a := map[string]string{"gateCmd": "make test", "flavor": "//"}
	vars, _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil)
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

func TestResolveInteractiveDefaultAndEnumIndex(t *testing.T) {
	// gateCmd: empty line → default; flavor: "2" → second enum option.
	in := strings.NewReader("\n2\n")
	vars, _, _, err := Resolve(descs(), nil, in, &strings.Builder{}, true, nil)
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
	vars, _, _, err := Resolve(descs(), nil, in, &strings.Builder{}, true, nil)
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
	if _, _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil); err == nil {
		t.Fatal("expected error for unknown answer key")
	}
}

// An explicit enum answer outside the options must error like multiselect
// does, not land verbatim in vars.
func TestResolveRejectsInvalidEnumAnswer(t *testing.T) {
	a := map[string]string{"flavor": ";;"}
	if _, _, _, err := Resolve(descs(), a, strings.NewReader(""), &strings.Builder{}, false, nil); err == nil {
		t.Fatal("expected error for enum answer outside options")
	}
}

func TestResolvePromptReadError(t *testing.T) {
	if _, _, _, err := Resolve(descs(), nil, errReader{}, &strings.Builder{}, true, nil); err == nil {
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

func trimDescs() []catalog.VarDescriptor {
	return []catalog.VarDescriptor{
		{Key: "skills", Kind: "multiselect", Target: "catalog-skills",
			Options: []string{"brainstorming", "bugfix", "tdd"}, Default: "brainstorming"},
		{Key: "docs", Kind: "multiselect", Target: "catalog-docs",
			Options: []string{"testing", "workflow"}, Default: "workflow"},
	}
}

func TestCatalogVarsComputesTrimOptions(t *testing.T) {
	cat := &catalog.Catalog{
		Skills: map[string]catalog.SkillSpec{"brainstorming": {Core: true}, "tdd": {}},
		Docs:   map[string]catalog.DocEntry{"workflow": {}, "testing": {}},
		Vars: []catalog.VarDescriptor{
			{Key: "gateCmd", Kind: "string"},
			{Key: "skills", Kind: "multiselect", Target: "catalog-skills"},
			{Key: "docs", Kind: "multiselect", Target: "catalog-docs"},
		},
	}
	got := CatalogVars(cat)
	if !slices.Equal(got[1].Options, []string{"brainstorming", "tdd"}) || got[1].Default != "brainstorming" {
		t.Errorf("skills descriptor = %+v", got[1])
	}
	// No doc carries Core any longer (ADR-0043): Options still lists every doc, but
	// Default is empty (no pre-selected core docs).
	if !slices.Equal(got[2].Options, []string{"testing", "workflow"}) || got[2].Default != "" {
		t.Errorf("docs descriptor = %+v", got[2])
	}
	if got[0].Options != nil { // non-trim descriptor untouched
		t.Errorf("gateCmd descriptor mutated: %+v", got[0])
	}
}

func TestResolveMultiselectSilentKeepsCore(t *testing.T) {
	_, trim, _, err := Resolve(trimDescs(), nil, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if trim != nil {
		t.Errorf("silent trim = %+v, want nil", trim)
	}
}

func TestResolveMultiselectExplicit(t *testing.T) {
	// Trailing comma on skills exercises splitNames' empty-segment skip; docs is
	// answered too so the catalog-docs trim dimension is populated.
	a := map[string]string{"skills": "tdd,brainstorming,", "docs": "testing"}
	_, trim, _, err := Resolve(trimDescs(), a, strings.NewReader(""), &strings.Builder{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if trim == nil || trim.Skills == nil || !slices.Equal(*trim.Skills, []string{"tdd", "brainstorming"}) {
		t.Errorf("trim.Skills = %+v", trim)
	}
	if trim.Docs == nil || !slices.Equal(*trim.Docs, []string{"testing"}) {
		t.Errorf("trim.Docs = %+v", trim)
	}
}

func TestResolveMultiselectExplicitUnknownName(t *testing.T) {
	a := map[string]string{"skills": "nope"}
	if _, _, _, err := Resolve(trimDescs(), a, strings.NewReader(""), &strings.Builder{}, false, nil); err == nil {
		t.Fatal("expected error for unknown option name")
	}
}

func TestResolveMultiselectInteractive(t *testing.T) {
	// skills: "1,3," -> brainstorming,tdd (trailing comma exercises the empty-token
	// skip); docs: empty -> keep core (nil dimension).
	in := strings.NewReader("1,3,\n\n")
	_, trim, _, err := Resolve(trimDescs(), nil, in, &strings.Builder{}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if trim == nil || trim.Skills == nil || !slices.Equal(*trim.Skills, []string{"brainstorming", "tdd"}) {
		t.Errorf("trim.Skills = %+v", trim)
	}
	if trim.Docs != nil {
		t.Errorf("empty docs prompt should keep core (nil), got %+v", trim.Docs)
	}
}

func TestResolveMultiselectInteractiveInvalidToken(t *testing.T) {
	for _, line := range []string{"9\n", "x\n"} { // out-of-range, non-numeric
		if _, _, _, err := Resolve(trimDescs(), nil, strings.NewReader(line), &strings.Builder{}, true, nil); err == nil {
			t.Errorf("expected error for input %q", line)
		}
	}
}

func TestResolveMultiselectPromptReadError(t *testing.T) {
	if _, _, _, err := Resolve(trimDescs(), nil, errReader{}, &strings.Builder{}, true, nil); err == nil {
		t.Fatal("expected read error from multiselect prompt")
	}
}

// An audit-scopes answer is comma-split, trimmed, empties dropped, and routed
// out of the vars map (ADR-0051).
func TestResolveAuditScopes(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "commitScopes", Kind: "string", Target: "audit-scopes"}}
	vars, _, scopes, err := Resolve(ds, map[string]string{"commitScopes": " adr, awf ,,plans "}, strings.NewReader(""), &strings.Builder{}, false, nil)
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
	_, _, scopes, err := Resolve(ds, nil, strings.NewReader(""), &strings.Builder{}, false, nil)
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
	vars, _, _, err := Resolve(ds, nil, strings.NewReader(""), &out, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "first:") {
		t.Errorf("the first prompt should have been emitted:\n%s", out.String())
	}
	if strings.Contains(out.String(), "second:") {
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
// invariant: init-prompts-enabled-vars
func TestResolveSkipsUnneededVarPrompts(t *testing.T) {
	ds := []catalog.VarDescriptor{
		{Key: "a", Kind: "string"},
		{Key: "b", Kind: "string"},
	}
	needed := func(*config.CatalogTrim) (map[string]bool, error) {
		return map[string]bool{"a": true}, nil
	}
	var out strings.Builder
	vars, _, _, err := Resolve(ds, nil, strings.NewReader("va\n"), &out, true, needed)
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
	needed := func(*config.CatalogTrim) (map[string]bool, error) {
		return map[string]bool{}, nil
	}
	vars, _, _, err := Resolve(ds, map[string]string{"b": "x"}, strings.NewReader(""), &strings.Builder{}, true, needed)
	if err != nil {
		t.Fatal(err)
	}
	if vars["b"] != "x" {
		t.Fatalf("explicit answers are honored regardless of the filter, got %v", vars)
	}
}

func TestResolvePropagatesNeededError(t *testing.T) {
	ds := []catalog.VarDescriptor{{Key: "a", Kind: "string"}}
	needed := func(*config.CatalogTrim) (map[string]bool, error) {
		return nil, errors.New("boom")
	}
	if _, _, _, err := Resolve(ds, nil, strings.NewReader(""), &strings.Builder{}, false, needed); err == nil {
		t.Fatal("a needed-filter error must propagate")
	}
}

// Multiselects prompt before vars regardless of descriptor order, so the
// needed filter always has the trim in hand.
func TestResolveMultiselectsPromptFirst(t *testing.T) {
	ds := []catalog.VarDescriptor{
		{Key: "gateCmd", Kind: "string"},
		{Key: "skills", Kind: "multiselect", Target: "catalog-skills",
			Options: []string{"brainstorming", "tdd"}, Default: "brainstorming"},
	}
	var out strings.Builder
	// First line answers the skills multiselect (empty keeps the default),
	// second the gateCmd prompt - the reverse of descriptor order.
	_, _, _, err := Resolve(ds, nil, strings.NewReader("\nmake gate\n"), &out, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	transcript := out.String()
	if !strings.Contains(transcript, "skills") || !strings.Contains(transcript, "gateCmd") {
		t.Fatalf("both prompts expected:\n%s", transcript)
	}
	if strings.Index(transcript, "skills") > strings.Index(transcript, "gateCmd") {
		t.Fatalf("the multiselect must prompt before the var:\n%s", transcript)
	}
}
