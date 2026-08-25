package config

import (
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
