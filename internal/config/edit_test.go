package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAppendLocalDoc(t *testing.T) {
	src := "# lead\nprefix: example # keep\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/old\n    title: Old\n    description: Old document.\nvars:\n  x: y\n"
	got, err := AppendLocalDoc([]byte(src), LocalDoc{Name: "runbooks/api-v2", Title: "API v2", Description: "How to operate API v2"})
	if err != nil {
		t.Fatal(err)
	}
	want := "# lead\nprefix: example # keep\nprofile: full\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/old\n    title: Old\n    description: Old document.\n  - name: runbooks/api-v2\n    title: API v2\n    description: How to operate API v2\nvars:\n  x: y\n"
	if string(got) != want {
		t.Fatalf("AppendLocalDoc = %q, want %q", got, want)
	}
	parsed, err := Parse("", got)
	if err != nil {
		t.Fatal(err)
	}
	if err := parsed.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(parsed.LocalDocs) != 2 || parsed.LocalDocs[1] != (LocalDoc{Name: "runbooks/api-v2", Title: "API v2", Description: "How to operate API v2"}) {
		t.Fatalf("local docs = %#v", parsed.LocalDocs)
	}
	for _, bad := range []string{"localDocs: bad\n", "localDocs:\n  - name: runbooks/x\n    title: X\n"} {
		if _, err := AppendLocalDoc([]byte(bad), LocalDoc{Name: "runbooks/y", Title: "Y", Description: "Y"}); err == nil {
			t.Fatalf("AppendLocalDoc accepted malformed source %q", bad)
		}
	}
	if _, err := AppendLocalDoc(got, LocalDoc{Name: "runbooks/api-v2", Title: "Again", Description: "Again"}); err == nil {
		t.Fatal("AppendLocalDoc accepted duplicate")
	}
	created, err := AppendLocalDoc([]byte("prefix: example\nprofile: full\nintegrationBranch: main\n"), LocalDoc{Name: "runbooks/new", Title: "New", Description: "New document"})
	if err != nil || !strings.Contains(string(created), "localDocs:\n  - name: runbooks/new") {
		t.Fatalf("AppendLocalDoc absent list = %q, %v", created, err)
	}
}

func TestSetArrayMember(t *testing.T) {
	cases := []struct {
		name, src, key, item string
		add                  bool
		want                 string
		wantErr              bool
	}{
		{"add appends", "skills:\n  - a\n", "skills", "b", true, "skills:\n  - a\n  - b\n", false},
		{"add idempotent", "skills:\n  - a\n", "skills", "a", true, "skills:\n  - a\n", false},
		{"add to empty flow", "agents: []\n", "agents", "x", true, "agents:\n  - x\n", false},
		{"add to bare key", "docs:\n", "docs", "d", true, "docs:\n  - d\n", false},
		{"add absent key", "prefix: x\n", "domains", "p", true, "prefix: x\ndomains:\n  - p\n", false},
		{"add to flow with items", "skills: [a, b]\n", "skills", "c", true, "skills:\n  - a\n  - b\n  - c\n", false},
		{"remove from items", "skills:\n  - a\n  - b\n", "skills", "a", false, "skills:\n  - b\n", false},
		{"remove last empties", "docs:\n  - d\n", "docs", "d", false, "docs: []\n", false},
		// invariant: config/configuration:remove-block-scoped (remove block-scoped)
		{"remove block-scoped", "skills:\n  - debugging\ndocs:\n  - debugging\n", "docs", "debugging", false, "skills:\n  - debugging\ndocs: []\n", false},
		{"remove not found", "skills:\n  - a\n", "skills", "z", false, "", true},
		{"remove from empty flow", "skills: []\n", "skills", "a", false, "", true},
		{"remove bare key", "skills:\n", "skills", "a", false, "", true},
		{"remove absent key", "prefix: x\n", "skills", "a", false, "", true},
		{"parse error", "skills: [a, b\n", "skills", "c", true, "", true},
		{"non-mapping", "- a\n- b\n", "skills", "c", true, "", true},
		{"empty doc", "", "skills", "c", true, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetArrayMember([]byte(tc.src), tc.key, tc.item, tc.add)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// invariant: config/configuration:config-mutation-roundtrip (TestSetArrayMember)
			if string(got) != tc.want {
				t.Errorf("SetArrayMember:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}
func TestSetArrayMemberPreservesComments(t *testing.T) {
	src := "# top comment\nprefix: x\nskills:\n  - a # inline\n"
	got, err := SetArrayMember([]byte(src), "skills", "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# top comment") {
		t.Errorf("head comment lost:\n%s", got)
	}
	if !strings.Contains(string(got), "# inline") {
		t.Errorf("inline comment lost:\n%s", got)
	}
	if !strings.Contains(string(got), "- b") {
		t.Errorf("member not added:\n%s", got)
	}
}

// invariant: config/configuration:sidecar-authoring-roundtrip (TestSidecarLeafRoundTrip)
func TestSidecarLeafRoundTrip(t *testing.T) {
	src := []byte("# retained\ndata:\n  old: keep # comment\nitems: [one, two]\n")
	out, present, changed, err := EditSidecar(src, SidecarEdit{Field: "data.new", Mode: "value", Value: "text"})
	if err != nil || !present || !changed {
		t.Fatalf("edit=%q present=%v changed=%v err=%v", out, present, changed, err)
	}
	if !strings.Contains(string(out), "old: keep # comment") || !strings.Contains(string(out), "new: text") {
		t.Fatalf("unrelated YAML was not retained: %s", out)
	}
	out, present, changed, err = EditSidecar(out, SidecarEdit{Field: "items", Mode: "add", Value: "two"})
	if err != nil || !present || changed {
		t.Fatalf("duplicate add changed=%v err=%v", changed, err)
	}
	out, present, changed, err = EditSidecar(out, SidecarEdit{Field: "items", Mode: "remove", Value: "one"})
	if err != nil || !changed || !strings.Contains(string(out), "- two") {
		t.Fatalf("remove=%q present=%v changed=%v err=%v", out, present, changed, err)
	}
	out, present, changed, err = EditSidecar([]byte("data:\n  item: x\n"), SidecarEdit{Field: "data.item", Mode: "reset"})
	if err != nil || present || !changed || out != nil {
		t.Fatalf("final reset=%q present=%v changed=%v err=%v", out, present, changed, err)
	}
}

// invariant: config/configuration:sidecar-authoring-roundtrip (TestSidecarLeafStructuredListsAndPruning)
func TestSidecarLeafStructuredListsAndPruning(t *testing.T) {
	src := []byte("# retained\ndata:\n  members: [1, {name: api, enabled: true}] # list\n  sibling: keep\nother: value # untouched\n")
	number, err := DecodeJSONValue(`1`)
	if err != nil {
		t.Fatal(err)
	}
	out, present, changed, err := EditSidecar(src, SidecarEdit{Field: "data.members", Mode: "add", Value: number})
	if err != nil || !present || changed || string(out) != string(src) {
		t.Fatalf("structurally duplicate number changed source: %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	mapping, err := DecodeJSONValue(`{"enabled":true,"name":"api"}`)
	if err != nil {
		t.Fatal(err)
	}
	out, present, changed, err = EditSidecar(src, SidecarEdit{Field: "data.members", Mode: "remove", Value: mapping})
	if err != nil || !present || !changed || strings.Contains(string(out), "enabled: true") || !strings.Contains(string(out), "sibling: keep") || !strings.Contains(string(out), "other: value # untouched") {
		t.Fatalf("structured removal = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	out, present, changed, err = EditSidecar([]byte("data:\n  nested:\n    leaf: value\n  sibling: keep\n"), SidecarEdit{Field: "data.nested.leaf", Mode: "reset"})
	if err != nil || !present || !changed || strings.Contains(string(out), "nested:") || !strings.Contains(string(out), "sibling: keep") {
		t.Fatalf("ancestor pruning = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
}

// invariant: config/configuration:sidecar-authoring-roundtrip (TestSidecarLeafModesAndRefusals)
func TestSidecarLeafModesAndRefusals(t *testing.T) {
	out, present, changed, err := EditSidecar(nil, SidecarEdit{Field: "data.value", Mode: "value", Value: ""})
	if err != nil || !present || !changed || string(out) != "data:\n  value: \"\"\n" {
		t.Fatalf("empty scalar = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	if out, present, changed, err = EditSidecar([]byte("data:\n  value: text\n"), SidecarEdit{Field: "data.absent", Mode: "remove", Value: "x"}); err != nil || !present || changed || string(out) != "data:\n  value: text\n" {
		t.Fatalf("absent remove = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	if _, _, _, err := EditSidecar([]byte("data: scalar\n"), SidecarEdit{Field: "data.key", Mode: "value", Value: "x"}); err == nil || !strings.Contains(err.Error(), "intermediate mapping conflict") {
		t.Fatalf("intermediate conflict = %v", err)
	}
	if _, _, _, err := EditSidecar(nil, SidecarEdit{Field: "data.value", Mode: "unknown", Value: "x"}); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestSidecarEdgeCases(t *testing.T) {
	for _, src := range [][]byte{[]byte("data: [unterminated\n"), []byte("- item\n")} {
		if _, _, _, err := EditSidecar(src, SidecarEdit{Field: "data.value", Mode: "value", Value: "x"}); err == nil {
			t.Fatalf("accepted invalid sidecar %q", src)
		}
	}

	src := []byte("data:\n  value: old\n")
	out, present, changed, err := EditSidecar(src, SidecarEdit{Field: "data.absent", Mode: "reset"})
	if err != nil || !present || changed || string(out) != string(src) {
		t.Fatalf("absent reset = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	out, present, changed, err = EditSidecar(src, SidecarEdit{Field: "data.value", Mode: "value", Value: "new"})
	if err != nil || !present || !changed || string(out) != "data:\n  value: new\n" {
		t.Fatalf("replace value = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	for _, mode := range []string{"add", "remove"} {
		if _, _, _, err := EditSidecar([]byte("items: scalar\n"), SidecarEdit{Field: "items", Mode: mode, Value: "x"}); err == nil || !strings.Contains(err.Error(), "not a list") {
			t.Fatalf("%s scalar list = %v", mode, err)
		}
	}
}

func TestSidecarJSONStructuredValues(t *testing.T) {
	null, err := DecodeJSONValue("null")
	if err != nil || null != nil {
		t.Fatalf("JSON null = %#v, %v", null, err)
	}
	out, present, changed, err := EditSidecar(nil, SidecarEdit{Field: "data.value", Mode: "value", Value: null})
	if err != nil || !present || !changed || string(out) != "data:\n  value: null\n" {
		t.Fatalf("null value = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	for _, input := range []string{"1.5", "1e3"} {
		value, err := DecodeJSONValue(input)
		if err != nil {
			t.Fatal(err)
		}
		node, err := valueNode(value)
		if err != nil || node.Tag != "!!float" || node.Value != input {
			t.Fatalf("JSON number %q = %#v, %v", input, node, err)
		}
	}
	array, err := DecodeJSONValue(`[1,null,{"enabled":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := array.([]any); !ok {
		t.Fatalf("JSON array type = %T", array)
	}
	out, present, changed, err = EditSidecar(nil, SidecarEdit{Field: "data.value", Mode: "value", Value: array})
	if err != nil || !present || !changed || string(out) != "data:\n  value:\n    - 1\n    - null\n    - enabled: true\n" {
		t.Fatalf("array value = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	out, present, changed, err = EditSidecar([]byte("items:\n  - [1, null]\n"), SidecarEdit{Field: "items", Mode: "add", Value: []any{json.Number("1"), nil}})
	if err != nil || !present || changed || string(out) != "items:\n  - [1, null]\n" {
		t.Fatalf("array identity = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
}

func TestSidecarStructuralAliasesAndSequences(t *testing.T) {
	value, err := DecodeJSONValue(`{"name":"api"}`)
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("items:\n  - &api {name: api}\n  - *api\n")
	out, present, changed, err := EditSidecar(src, SidecarEdit{Field: "items", Mode: "add", Value: value})
	if err != nil || !present || changed || string(out) != string(src) {
		t.Fatalf("alias identity = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	_, root, err := parseMapping([]byte("flow: [one, two]\nblock:\n  - one\n  - two\n"))
	if err != nil {
		t.Fatal(err)
	}
	flow, _ := mapValue(root, "flow")
	block, _ := mapValue(root, "block")
	if !reflect.DeepEqual(nodeValue(flow), nodeValue(block)) {
		t.Fatalf("sequence values differ: %#v != %#v", nodeValue(flow), nodeValue(block))
	}
	_, aliases, err := parseMapping(src)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := mapValue(aliases, "items")
	if got, want := nodeValue(items.Content[1]), nodeValue(items.Content[0]); !reflect.DeepEqual(got, want) {
		t.Fatalf("alias value = %#v, want %#v", got, want)
	}
}

func TestSidecarListRemovalRemovesEveryStructuralDuplicate(t *testing.T) {
	src := []byte("items:\n  - one\n  - two\n  - one\n  - three\n  - one\n")
	out, present, changed, err := EditSidecar(src, SidecarEdit{Field: "items", Mode: "remove", Value: "one"})
	if err != nil || !present || !changed || string(out) != "items:\n  - two\n  - three\n" {
		t.Fatalf("duplicate removal = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	retry, present, changed, err := EditSidecar(out, SidecarEdit{Field: "items", Mode: "remove", Value: "one"})
	if err != nil || !present || changed || string(retry) != string(out) {
		t.Fatalf("duplicate removal retry = %q present=%v changed=%v err=%v", retry, present, changed, err)
	}
}

// invariant: config/configuration:sidecar-authoring-roundtrip (TestSidecarJSONNumbersRetainExactValueAndStructuralIdentity)
func TestSidecarJSONNumbersRetainExactValueAndStructuralIdentity(t *testing.T) {
	const exact = `9007199254740993`
	value, err := DecodeJSONValue(exact)
	if err != nil {
		t.Fatal(err)
	}
	out, present, changed, err := EditSidecar(nil, SidecarEdit{Field: "data.number", Mode: "value", Value: value})
	if err != nil || !present || !changed || string(out) != "data:\n  number: 9007199254740993\n" {
		t.Fatalf("exact JSON number = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
	list := []byte("items:\n  - 9007199254740993\n")
	out, present, changed, err = EditSidecar(list, SidecarEdit{Field: "items", Mode: "add", Value: value})
	if err != nil || !present || changed || string(out) != string(list) {
		t.Fatalf("exact JSON number identity = %q present=%v changed=%v err=%v", out, present, changed, err)
	}
}

func TestDecodeJSONValueExactlyOne(t *testing.T) {
	for _, input := range []string{`{"x":[1]}`, `true`, `null`, `"text"`, `1`, `9007199254740993`} {
		if _, err := DecodeJSONValue(input); err != nil {
			t.Fatalf("valid JSON %q: %v", input, err)
		}
	}
	for _, input := range []string{`1 2`, `1 }`, ``, `{`, `true false`} {
		if _, err := DecodeJSONValue(input); err == nil {
			t.Fatalf("accepted invalid or trailing JSON %q", input)
		}
	}
}
