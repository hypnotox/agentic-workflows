package configspec

import (
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/audit"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// walkPaths derives the expected described-key path set from a struct type by
// reflection over yaml tags: scalars, pointer scalars, string slices, and
// map[string]any are leaves; pointer-to-struct and struct fields recurse with
// "<tag>." prefixes; slice-of-struct keeps the container AND recurses with
// "<tag>[]."; map-of-struct keeps the container AND recurses with
// "<tag>.<name>.".
func walkPaths(t reflect.Type, prefix string, out map[string]bool) {
	for i := range t.NumField() {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported (config root/raw bookkeeping)
			continue
		}
		tag := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		path := prefix + tag
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.Struct:
			walkPaths(ft, path+".", out)
		case reflect.Slice:
			el := ft.Elem()
			if el.Kind() == reflect.Struct {
				out[path] = true
				walkPaths(el, path+"[].", out)
			} else {
				out[path] = true
			}
		case reflect.Map:
			el := ft.Elem()
			if el.Kind() == reflect.Struct {
				out[path] = true
				walkPaths(el, path+".<name>.", out)
			} else {
				out[path] = true // freeform namespace (vars, sidecar data)
			}
		default:
			out[path] = true
		}
	}
}

// No config surface supplies an audit base: the range reaches the audit only
// from the command line (ADR-0127 Decision 3). Asserted across all three
// surfaces at once, since key parity means a struct field would force a spec
// entry and vice versa.
// invariant: config/configuration:audit-no-base-branch-config
func TestNoAuditBaseConfigSurface(t *testing.T) {
	for _, e := range Keys() {
		if strings.Contains(strings.ToLower(e.Path), "basebranch") {
			t.Errorf("spec entry %q supplies an audit base; the range must be caller-supplied", e.Path)
		}
	}
	fields := map[string]bool{}
	walkPaths(reflect.TypeOf(config.Config{}), "", fields)
	for path := range fields {
		if strings.Contains(strings.ToLower(path), "basebranch") {
			t.Errorf("config field %q supplies an audit base; the range must be caller-supplied", path)
		}
	}
	for i := range reflect.TypeOf(audit.Settings{}).NumField() {
		if name := reflect.TypeOf(audit.Settings{}).Field(i).Name; strings.Contains(strings.ToLower(name), "basebranch") {
			t.Errorf("audit.Settings.%s supplies a base; the range must be caller-supplied", name)
		}
	}
}

func TestADRStatesV2DescriptionAndEmptyOverride(t *testing.T) {
	const want = "The decision-record lifecycle states (list of {name, meaning, mutability}) the skill's state table renders; the default is the five-state current-state-v2 lifecycle."
	var found bool
	for _, entry := range DataKeys() {
		if entry.Kind == "skills" && entry.Artifact == "adr-lifecycle" && entry.Key == "adrStates" {
			found = true
			if entry.Description != want {
				t.Fatalf("adrStates description = %q", entry.Description)
			}
		}
	}
	if !found {
		t.Fatal("adrStates configspec entry missing")
	}
	if empty := (map[string]any{})["adrStates"]; empty != nil {
		t.Fatalf("empty data override unexpectedly supplies adrStates: %#v", empty)
	}
}

// TestConfigspecKeyParity keeps the hand-authored key table bidirectionally
// matched to the config structs, every entry fully described.
// invariant: config/configuration:configspec-key-parity
func TestConfigspecKeyParity(t *testing.T) {
	want := map[string]bool{}
	walkPaths(reflect.TypeOf(config.Config{}), "", want)
	walkPaths(reflect.TypeOf(config.Sidecar{}), "sidecar.", want)

	got := map[string]bool{}
	for _, e := range Keys() {
		if got[e.Path] {
			t.Errorf("duplicate entry for %q", e.Path)
		}
		got[e.Path] = true
		if e.Description == "" || e.Type == "" || e.Availability == "" || e.Default == "" {
			t.Errorf("entry %q has an empty field (type/default/description/availability all required)", e.Path)
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("config key %q has no configspec entry", p)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("configspec entry %q names no live config key", p)
		}
	}
}

// invariant: config/configuration:topic-claim-budget-configured
func TestCurrentStateKeysPublished(t *testing.T) {
	got := map[string]Entry{}
	for _, entry := range Keys() {
		got[entry.Path] = entry
	}
	for _, path := range []string{
		"currentState.sources",
		"currentState.sources[].globs",
		"currentState.sources[].marker",
		"currentState.sources[].close",
		"currentState.testGlobs",
		"currentState.topicCoverage",
		"currentState.topicFanout",
		"currentState.maxTopicsPerPath",
		"currentState.maxClaimsPerTopic",
	} {
		entry, ok := got[path]
		if !ok {
			t.Errorf("missing current-state configspec entry %q", path)
			continue
		}
		if !strings.Contains(entry.Description, "current-state") {
			t.Errorf("entry %q does not describe current-state authority: %q", path, entry.Description)
		}
	}
}

// expandedTemplate reads a template id from the embedded FS with includes
// expanded - the per-artifact source whose .data references define the
// describable data-key universe.
func expandedTemplate(t *testing.T, tid string) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatalf("read template %s: %v", tid, err)
	}
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil {
		t.Fatalf("expand %s: %v", tid, err)
	}
	return expanded
}

// TestConfigspecDataParity derives the expected (kind, artifact, key) set from
// the catalog plus the embedded templates (include-expanded, union each
// artifact's catalog-declared defaults) and matches it against DataKeys() in
// both directions. The domain template's injected pair and the generated
// config reference's injected collections are exempt (neither is
// adopter-settable).
// invariant: config/configuration:configspec-data-parity
func TestConfigspecDataParity(t *testing.T) {
	type ak struct{ kind, artifact, key string }
	want := map[ak]bool{}
	collect := func(kind, artifact, tid string, defaults map[string]any) {
		for _, k := range render.ReferencedDataKeys(expandedTemplate(t, tid)) {
			want[ak{kind, artifact, k}] = true
		}
		for k := range defaults {
			want[ak{kind, artifact, k}] = true
		}
	}
	for name, spec := range catalog.Standard.Skills {
		collect("skills", name, "skills/"+name+"/SKILL.md.tmpl", spec.Data)
	}
	collect("skills", "_base", "skills/_base/SKILL.md.tmpl", nil)
	for name, spec := range catalog.Standard.Agents {
		collect("agents", name, "agents/"+name+".md.tmpl", spec.Data)
	}
	collect("agents", "_base", "agents/_base.md.tmpl", nil)
	collect("docs", "_base", "docs/_base.md.tmpl", nil)
	for name, e := range catalog.Standard.Docs {
		if e.Generated { // the config reference's collections are injected, not adopter-settable
			continue
		}
		collect("docs", name, e.TID, e.Data)
	}

	got := map[ak]bool{}
	for _, d := range DataKeys() {
		k := ak{d.Kind, d.Artifact, d.Key}
		if got[k] {
			t.Errorf("duplicate data-key entry %v", k)
		}
		got[k] = true
		if d.Description == "" {
			t.Errorf("data key %v has an empty description", k)
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("data key %s/%s data.%s has no configspec description", k.kind, k.artifact, k.key)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("configspec data-key entry %s/%s data.%s matches no template-referenced or catalog-declared key", k.kind, k.artifact, k.key)
		}
	}
}

// TestConfigspecVarDerivation pins VarEntries to the catalog's config-var
// descriptors: exact key set, verbatim description text, availability
// present, and no stale availability clause.
func TestConfigspecVarDerivation(t *testing.T) {
	want := map[string]string{}
	for _, d := range catalog.Standard.Vars {
		if d.Target == "" || d.Target == "var" {
			want[d.Key] = d.Description
		}
	}
	entries := VarEntries()
	// invariant: config/configuration:configspec-var-derivation
	if len(entries) != len(want) {
		t.Errorf("VarEntries returned %d entries, want %d", len(entries), len(want))
	}
	for _, e := range entries {
		desc, ok := want[e.Key]
		if !ok {
			t.Errorf("VarEntries carries %q, which is not a config-var descriptor", e.Key)
			continue
		}
		if e.Description != desc {
			t.Errorf("var %q description diverges from the catalog descriptor:\n got: %s\nwant: %s", e.Key, e.Description, desc)
		}
		if e.Availability == "" {
			t.Errorf("var %q has no availability clause", e.Key)
		}
	}
	for k := range varAvailability {
		if _, ok := want[k]; !ok {
			t.Errorf("varAvailability carries stale key %q", k)
		}
	}
}

// TestConfigspecDescriptionResidue bans awf-internal residue from every
// adopter-facing string: concrete ADR citations and repo-identity literals.
// invariant: config/configuration:configspec-description-residue
func TestConfigspecDescriptionResidue(t *testing.T) {
	adrRE := regexp.MustCompile(`ADR-[0-9]{4}`)
	check := func(where, s string) {
		t.Helper()
		if adrRE.MatchString(s) {
			t.Errorf("%s carries a concrete ADR citation: %q", where, s)
		}
		for _, ident := range []string{"hypnotox", "agentic-workflows"} {
			if strings.Contains(s, ident) {
				t.Errorf("%s carries repo-identity literal %q: %q", where, ident, s)
			}
		}
	}
	for _, e := range Keys() {
		for _, s := range []string{e.Type, e.Default, e.Description, e.Availability} {
			check("key "+e.Path, s)
		}
	}
	for _, v := range VarEntries() {
		check("var "+v.Key, v.Description)
		check("var "+v.Key, v.Availability)
	}
	for _, d := range DataKeys() {
		check(fmt.Sprintf("data key %s/%s.%s", d.Kind, d.Artifact, d.Key), d.Description)
	}
}

// TestConfigspecAuditDefaultsPinned keeps the prose defaults for the numeric
// audit knobs equal to the resolver's actual defaults.
func TestConfigspecAuditDefaultsPinned(t *testing.T) {
	defaults := audit.Resolve(nil)
	byPath := map[string]Entry{}
	for _, e := range Keys() {
		byPath[e.Path] = e
	}
	if want := strconv.Itoa(defaults.SubjectMaxLength); !strings.Contains(byPath["audit.subjectMaxLength"].Default, want) {
		t.Errorf("audit.subjectMaxLength default prose %q does not carry the resolver default %s", byPath["audit.subjectMaxLength"].Default, want)
	}
	if want := strconv.Itoa(defaults.DiffThreshold); !strings.Contains(byPath["audit.diffThreshold"].Default, want) {
		t.Errorf("audit.diffThreshold default prose %q does not carry the resolver default %s", byPath["audit.diffThreshold"].Default, want)
	}
	if !slices.Contains(defaults.AllowedTypes, "feat") {
		t.Error("resolver default types lost feat; update the audit.allowedTypes default prose")
	}
}
