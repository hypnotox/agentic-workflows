package config

import (
	"slices"
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

func TestSetString(t *testing.T) {
	cases := []struct {
		name, src, key, value string
		want                  string
		wantErr               bool
	}{
		{"create absent key", "prefix: x\n", "integrationBranch", "main", "prefix: x\nintegrationBranch: main\n", false},
		{"replace existing", "integrationBranch: trunk\n", "integrationBranch", "main", "integrationBranch: main\n", false},
		{"replace non-scalar", "integrationBranch:\n  - trunk\n", "integrationBranch", "main", "integrationBranch: main\n", false},
		{"preserves comments and order", "# lead\nprefix: x # trailing\ndocsDir: docs\n", "integrationBranch", "main", "# lead\nprefix: x # trailing\ndocsDir: docs\nintegrationBranch: main\n", false},
		{"parse error", "prefix: [a, b\n", "integrationBranch", "main", "", true},
		{"non-mapping", "- a\n", "integrationBranch", "main", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetString([]byte(tc.src), tc.key, tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// invariant: config/configuration:config-serialization-owned (TestSetString)
			if string(got) != tc.want {
				t.Errorf("SetString:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// HasValue answers emptiness, not presence: only a key carrying a non-empty
// scalar counts, so a null or empty value reads the same as an absent key -
// which is what makes the port-forward seed exactly what the migration writes.
// A malformed document errors.
func TestHasValue(t *testing.T) {
	for _, tc := range []struct {
		name, src, key string
		want, wantErr  bool
	}{
		{name: "present with value", src: "integrationBranch: main\n", key: "integrationBranch", want: true},
		{name: "present but empty", src: "integrationBranch: \"\"\n", key: "integrationBranch"},
		{name: "present but null", src: "integrationBranch:\n", key: "integrationBranch"},
		{name: "absent", src: "prefix: x\n", key: "integrationBranch"},
		{name: "nested key does not count", src: "currentState:\n  integrationBranch: main\n", key: "integrationBranch"},
		{name: "mapping value is not a scalar value", src: "integrationBranch:\n  nested: main\n", key: "integrationBranch"},
		{name: "malformed", src: "prefix: [\n", key: "integrationBranch", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := HasValue([]byte(tc.src), tc.key)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a parse error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("HasValue = %v, want %v", got, tc.want)
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

func TestSetMappingInteger(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
		wantErr         bool
	}{
		{"creates mapping", "# top\nprefix: x\n", "# top\nprefix: x\ncurrentState:\n  maxTopicsPerPath: 20\n", false},
		{"adds child preserving comment", "currentState:\n  sources: [] # keep\n", "currentState:\n  sources: [] # keep\n  maxTopicsPerPath: 20\n", false},
		{"preserves existing integer", "currentState:\n  maxTopicsPerPath: 7 # explicit\n", "currentState:\n  maxTopicsPerPath: 7 # explicit\n", false},
		{"rejects non-mapping", "currentState: nope\n", "", true},
		{"rejects wrong existing kind", "currentState:\n  maxTopicsPerPath: nope\n", "", true},
		{"rejects malformed", "currentState: [bad\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetMappingInteger([]byte(tc.src), "currentState", "maxTopicsPerPath", 20)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The string editor the retired-command value migration consumes. Its fixtures
// deliberately spell no retired command name: the migration's own test owns those
// literals, and this file is inside the repo-wide sweep that forbids them.
func TestSetMappingString(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
		wantErr         bool
	}{
		{"replaces a present value", "vars:\n  gateCmd: ./x gate # keep\n", "vars:\n  gateCmd: ./x verify # keep\n", false},
		{"preserves the quoting style", "vars:\n  gateCmd: \"./x gate\"\n", "vars:\n  gateCmd: \"./x verify\"\n", false},
		{"absent key is a no-op", "prefix: x\n", "prefix: x\n", false},
		{"absent child is a no-op", "vars:\n  other: v\n", "vars:\n  other: v\n", false},
		{"non-mapping parent is a no-op", "vars: nope\n", "vars: nope\n", false},
		{"non-scalar child is a no-op", "vars:\n  gateCmd: [a]\n", "vars:\n  gateCmd: [a]\n", false},
		// An alias decodes to a string but is not a scalar node; rewriting it would
		// have to edit the anchor, so it is left alone rather than erroring. The
		// anchor lives under a foreign top-level key here only to keep the fixture
		// small; the shape awf's own strict parse accepts is anchored on a sibling
		// var, and internal/migrate covers that one end to end.
		{"aliased child is a no-op", "anchors:\n  a: &c ./x gate\nvars:\n  gateCmd: *c\n", "anchors:\n  a: &c ./x gate\nvars:\n  gateCmd: *c\n", false},
		{"rejects malformed", "vars: [bad\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetMappingString([]byte(tc.src), "vars", "gateCmd", "./x verify")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRemoveMappingKey(t *testing.T) {
	cases := []struct {
		name, src, key, child, want string
		wantErr                     bool
	}{
		{
			name: "removes child, keeps siblings and their comments",
			src:  "prefix: x\naudit:\n  baseBranch: develop\n  # keep me\n  diffThreshold: 400\n",
			key:  "audit", child: "baseBranch",
			want: "prefix: x\naudit:\n  # keep me\n  diffThreshold: 400\n",
		},
		{
			// A comment directly above the removed key is that key's head
			// comment, so it goes with it: the note described the setting
			// being retired and would be orphaned otherwise.
			name: "the removed key takes its own comment",
			src:  "audit:\n  # base to compare against\n  baseBranch: develop\n  diffThreshold: 400\n",
			key:  "audit", child: "baseBranch",
			want: "audit:\n  diffThreshold: 400\n",
		},
		{
			name: "sole child drops the parent mapping",
			src:  "prefix: x\naudit:\n  baseBranch: develop\nskills:\n  - a\n",
			key:  "audit", child: "baseBranch",
			want: "prefix: x\nskills:\n  - a\n",
		},
		{
			name: "absent child is a no-op",
			src:  "audit:\n  diffThreshold: 400\n",
			key:  "audit", child: "baseBranch",
			want: "audit:\n  diffThreshold: 400\n",
		},
		{
			name: "absent parent is a no-op",
			src:  "prefix: x\n",
			key:  "audit", child: "baseBranch",
			want: "prefix: x\n",
		},
		{
			name: "non-mapping parent is a no-op",
			src:  "audit: nope\n",
			key:  "audit", child: "baseBranch",
			want: "audit: nope\n",
		},
		{"parse error", "audit: [a, b\n", "audit", "baseBranch", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RemoveMappingKey([]byte(tc.src), tc.key, tc.child)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
			// Re-running must be a no-op, so a migration replay is safe.
			again, aerr := RemoveMappingKey(got, tc.key, tc.child)
			if aerr != nil {
				t.Fatal(aerr)
			}
			if string(again) != string(got) {
				t.Errorf("not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestMoveMappingKeyToBoolPreservesRemovedComments(t *testing.T) {
	src := "# before data\ndata: # data line\n  # before only\n  only: # key line\n    # null foot\n# before sections\nsections:\n  notes: true\n"
	got, err := MoveMappingKeyToBool([]byte(src), "data", "only", "dataDefaults", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# before data", "# data line", "# before only", "# key line", "# null foot", "# before sections", "sections:", "notes: true", "dataDefaults:", "only: false"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("combined edit lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "data:\n") {
		t.Errorf("empty source mapping remains:\n%s", got)
	}
}

func TestMoveMappingKeyToBoolShapes(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		value     bool
		wants     []string
		wantErr   bool
	}{
		{
			name:  "retains source siblings and merges existing target comments",
			src:   "data:\n  keep: value\n  # moved head\n  only: null # moved line\ndataDefaults:\n  # existing head\n  only: false # existing line\n",
			value: true,
			wants: []string{"data:", "keep: value", "dataDefaults:", "only: true", "# moved head", "# moved line", "# existing head", "# existing line"},
		},
		{name: "replaces non-mapping target", src: "data:\n  only: null\ndataDefaults: nope\n", wants: []string{"dataDefaults:", "only: false"}},
		{name: "absent source creates target", src: "prefix: x\n", wants: []string{"prefix: x", "dataDefaults:", "only: false"}},
		{name: "non-mapping source creates target", src: "data: nope\n", wants: []string{"data: nope", "dataDefaults:", "only: false"}},
		{name: "absent child creates target", src: "data:\n  keep: value\n", wants: []string{"keep: value", "dataDefaults:", "only: false"}},
		{name: "parse error", src: "data: [\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MoveMappingKeyToBool([]byte(tc.src), "data", "only", "dataDefaults", tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got:\n%s", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.wants {
				if !strings.Contains(string(got), want) {
					t.Errorf("combined edit missing %q:\n%s", want, got)
				}
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

// invariant backing lives at the cmd/awf call site (new-seeds-scaffold-vars);
// this pins the editor's presence/absence contract (ADR-0087).
func TestSeedVarKey(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		want      string // substring that must be present in the result
		unchanged bool   // result must be byte-identical to src
		wantErr   bool
	}{
		{name: "absent key seeded empty", src: "prefix: x\nvars:\n  other: set\n", want: "gateCmd: \"\""},
		{name: "present valued untouched", src: "prefix: x\nvars:\n  gateCmd: make gate\n", unchanged: true},
		{name: "present empty untouched", src: "prefix: x\nvars:\n  gateCmd: \"\"\n", unchanged: true},
		{name: "present null untouched", src: "prefix: x\nvars:\n  gateCmd:\n", unchanged: true},
		{name: "vars mapping absent created", src: "prefix: x\n", want: "vars:\n  gateCmd: \"\""},
		{name: "vars null replaced", src: "prefix: x\nvars:\n", want: "vars:\n  gateCmd: \"\""},
		{name: "malformed source errors", src: ":\n:", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SeedVarKey([]byte(tc.src), "gateCmd")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got:\n%s", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.unchanged && string(got) != tc.src {
				t.Errorf("present key must leave source byte-identical, got:\n%s", got)
			}
			if tc.want != "" && !strings.Contains(string(got), tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, got)
			}
		})
	}
}

func TestSeedVarKeyPreservesComments(t *testing.T) {
	src := "# top comment\nprefix: x # inline\nvars:\n  other: set\n"
	got, err := SeedVarKey([]byte(src), "gateCmd")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# top comment", "# inline", "other: set"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("%q lost:\n%s", want, got)
		}
	}
}

func TestAnchorNoSlashGlobs(t *testing.T) {
	src := []byte(`prefix: x
invariants:
  disabled: false
  sources:
    - globs:
        - '*.go'
        - cmd/**
      marker: //
audit:
  dependencyManifests:
    - go.mod
    - '**/package.json'
`)
	out, rewrites, err := AnchorNoSlashGlobs(src)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// invariant: config/validation:glob-migration-anchored (TestAnchorNoSlashGlobs)
	for _, want := range []string{"**/*.go", "cmd/**", "**/go.mod", "**/package.json"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "**/cmd/**") || strings.Contains(s, "**/**/package.json") {
		t.Errorf("slashed pattern was rewritten:\n%s", s)
	}
	wantRewrites := []GlobRewrite{
		{Key: "invariants.sources.globs", From: "*.go"},
		{Key: "audit.dependencyManifests", From: "go.mod"},
	}
	if !slices.Equal(rewrites, wantRewrites) {
		t.Errorf("rewrites = %v, want %v", rewrites, wantRewrites)
	}
	// Idempotent: a second pass changes and reports nothing.
	again, againRewrites, err := AnchorNoSlashGlobs(out)
	if err != nil || string(again) != s {
		t.Errorf("not idempotent (err %v):\n%s", err, again)
	}
	if len(againRewrites) != 0 {
		t.Errorf("idempotent pass must report no rewrites, got %v", againRewrites)
	}
}

func TestAnchorNoSlashGlobsAbsentKeysNoop(t *testing.T) {
	src := []byte("prefix: x\nskills:\n  - tdd\n")
	out, rewrites, err := AnchorNoSlashGlobs(src)
	if err != nil || strings.Contains(string(out), "**/") || len(rewrites) != 0 {
		t.Errorf("expected no-op, got (err %v, rewrites %v):\n%s", err, rewrites, out)
	}
}

func TestAnchorNoSlashGlobsParseError(t *testing.T) {
	if _, _, err := AnchorNoSlashGlobs([]byte("not: [valid")); err == nil {
		t.Error("expected parse error for malformed YAML")
	}
}

func TestAnchorNoSlashGlobsSkipsNonMappingSourceItem(t *testing.T) {
	src := []byte("invariants:\n  sources:\n    - just-a-scalar\n    - globs:\n        - '*.py'\n      marker: '#'\n")
	out, _, err := AnchorNoSlashGlobs(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "**/*.py") || !strings.Contains(string(out), "just-a-scalar") {
		t.Errorf("scalar source item must be skipped, mapping one rewritten:\n%s", out)
	}
}

func TestHasMapping(t *testing.T) {
	for _, tc := range []struct {
		name, src, key string
		want           bool
	}{
		{"mapping value", "prefix: x\ncurrentState:\n  maxTopicsPerPath: 8\n", "currentState", true},
		{"absent key", "prefix: x\n", "currentState", false},
		// A key present with a non-mapping value reports false, matching every
		// editor that declines a foreign shape rather than coercing it.
		{"scalar value", "prefix: x\ncurrentState: nope\n", "currentState", false},
		{"sequence value", "prefix: x\ncurrentState: [a]\n", "currentState", false},
		{"null value", "prefix: x\ncurrentState:\n", "currentState", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := HasMapping([]byte(tc.src), tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("HasMapping(%q, %q) = %v, want %v", tc.src, tc.key, got, tc.want)
			}
		})
	}
}

func TestHasMappingRefusesMalformedYAML(t *testing.T) {
	if _, err := HasMapping([]byte("prefix: [\n"), "currentState"); err == nil {
		t.Fatal("malformed YAML must surface the parse error, not report a bare false")
	}
}
