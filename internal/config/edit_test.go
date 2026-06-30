package config

import (
	"strings"
	"testing"
)

func TestMarshalSkeleton(t *testing.T) {
	out, err := MarshalSkeleton(Skeleton{
		Prefix: "awf",
		Vars:   map[string]string{"b": "", "a": ""},
		Skills: []string{"tdd"},
		Agents: []string{},
		Docs:   []string{"workflow"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: awf\n" +
		"vars:\n  a: \"\"\n  b: \"\"\n" +
		"skills:\n  - tdd\n" +
		"agents: []\n" +
		"docs:\n  - workflow\n"
	if string(out) != want {
		t.Errorf("MarshalSkeleton:\n got: %q\nwant: %q", out, want)
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
			if string(got) != tc.want {
				t.Errorf("SetArrayMember:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestSetArray(t *testing.T) {
	cases := []struct {
		name, src, key string
		values         []string
		want           string
		wantErr        bool
	}{
		{"create absent key", "prefix: x\n", "targets", []string{"claude", "cursor"}, "prefix: x\ntargets:\n  - claude\n  - cursor\n", false},
		{"replace existing", "targets:\n  - claude\n", "targets", []string{"claude", "cursor"}, "targets:\n  - claude\n  - cursor\n", false},
		{"replace flow style", "targets: [cursor]\n", "targets", []string{"claude"}, "targets:\n  - claude\n", false},
		{"parse error", "targets: [a, b\n", "targets", []string{"x"}, "", true},
		{"non-mapping", "- a\n", "targets", []string{"x"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetArray([]byte(tc.src), tc.key, tc.values)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("SetArray:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestRemoveKey(t *testing.T) {
	cases := []struct {
		name, src, key, want string
		wantErr              bool
	}{
		{"present removes", "prefix: x\nhooks:\n  - a\nskills:\n  - b\n", "hooks", "prefix: x\nskills:\n  - b\n", false},
		{"absent no-op", "prefix: x\n", "hooks", "prefix: x\n", false},
		{"non-mapping", "- a\n- b\n", "hooks", "", true},
		{"parse error", "skills: [a, b\n", "hooks", "", true},
		{"empty doc", "", "hooks", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RemoveKey([]byte(tc.src), tc.key)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("RemoveKey:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestRemoveKeyPreservesComments(t *testing.T) {
	src := "# top comment\nprefix: x # inline\nhooks:\n  - a\n"
	got, err := RemoveKey([]byte(src), "hooks")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# top comment") || !strings.Contains(string(got), "# inline") {
		t.Errorf("comments lost:\n%s", got)
	}
	if strings.Contains(string(got), "hooks") {
		t.Errorf("hooks key not removed:\n%s", got)
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

func TestSetMappingScalar(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		value   bool
		want    string // substring that must be present
		wantErr bool
	}{
		{
			name:  "key absent appends mapping",
			src:   "# top comment\nprefix: x\n",
			value: true,
			want:  "bootstrap:\n  enabled: true",
		},
		{
			name:  "mapping without child appends child",
			src:   "prefix: x\nbootstrap:\n  other: y\n",
			value: true,
			want:  "enabled: true",
		},
		{
			name:  "mapping with child overwrites",
			src:   "prefix: x\nbootstrap:\n  enabled: false\n",
			value: true,
			want:  "enabled: true",
		},
		{
			name:  "key not a mapping is replaced",
			src:   "prefix: x\nbootstrap: 3\n",
			value: true,
			want:  "bootstrap:\n  enabled: true",
		},
		{
			name:  "value false renders false",
			src:   "prefix: x\n",
			value: false,
			want:  "enabled: false",
		},
		{
			name:    "non-mapping root errors",
			src:     "- a\n",
			value:   true,
			wantErr: true,
		},
		{
			name:    "unparseable YAML errors",
			src:     "a: [b\n",
			value:   true,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetMappingScalar([]byte(tc.src), "bootstrap", "enabled", tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, got)
			}
		})
	}
}

func TestSetMappingScalarPreservesComments(t *testing.T) {
	src := "# top comment\nprefix: x # inline\n"
	got, err := SetMappingScalar([]byte(src), "bootstrap", "enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# top comment") {
		t.Errorf("head comment lost:\n%s", got)
	}
	if !strings.Contains(string(got), "# inline") {
		t.Errorf("inline comment lost:\n%s", got)
	}
}
