package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMarshalSkeleton(t *testing.T) {
	out, err := MarshalSkeleton(Skeleton{
		Prefix:       "awf",
		Vars:         map[string]string{"b": "", "a": ""},
		Skills:       []string{"tdd"},
		Agents:       []string{},
		Docs:         []string{"workflow"},
		CurrentState: &SkeletonCurrentState{MaxClaimsPerTopic: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: awf\n" +
		"vars:\n  a: \"\"\n  b: \"\"\n" +
		"skills:\n  - tdd\n" +
		"agents: []\n" +
		"docs:\n  - workflow\n" +
		"currentState:\n  maxClaimsPerTopic: 20\n" +
		"workflowTelemetry:\n" +
		"  retention:\n    maxCompletedEffortAgeDays: 90\n    maxCompletedEffortCount: 100\n" +
		"  widget:\n    enabled: true\n    showCost: true\n" +
		"  diagnostics:\n    heuristicsEnabled: true\n    minimumBaselineSamples: 10\n    baselinePercentile: 95\n" +
		"    thresholds:\n      phaseReentryCount: 2\n      phaseDurationSeconds: 14400\n      phaseTokens: 200000\n      compactionCount: 3\n      handoffCount: 3\n      toolFailureCount: 3\n      gateFailureCount: 2\n      cacheReadPercentBelow: 10\n      subagentQueueWaitSeconds: 60\n      implementationReworkCount: 2\n"
	// invariant: config/configuration:config-serialization-owned
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
		// invariant: config/configuration:remove-block-scoped
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
			// invariant: config/configuration:config-mutation-roundtrip
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

// invariant: config/configuration:topic-claim-budget-configured
func TestSetMappingInteger(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
		wantErr         bool
	}{
		{"creates mapping", "# top\nprefix: x\n", "# top\nprefix: x\ncurrentState:\n  maxClaimsPerTopic: 20\n", false},
		{"adds child preserving comment", "currentState:\n  topicCoverage: warn # keep\n", "currentState:\n  topicCoverage: warn # keep\n  maxClaimsPerTopic: 20\n", false},
		{"preserves existing integer", "currentState:\n  maxClaimsPerTopic: 7 # explicit\n", "currentState:\n  maxClaimsPerTopic: 7 # explicit\n", false},
		{"rejects non-mapping", "currentState: nope\n", "", true},
		{"rejects wrong existing kind", "currentState:\n  maxClaimsPerTopic: nope\n", "", true},
		{"rejects malformed", "currentState: [bad\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetMappingInteger([]byte(tc.src), "currentState", "maxClaimsPerTopic", 20)
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
	// invariant: config/configuration:glob-migration-anchored
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

func TestEnsureWorkflowTelemetryDefaults(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("# keep\nprefix: x\nworkflowTelemetry:\n  widget:\n    enabled: false # explicit\n"), &doc); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsureWorkflowTelemetryDefaults(&doc)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	widget, _ := mapValue(doc.Content[0], "workflowTelemetry")
	widget, _ = mapValue(widget, "widget")
	enabled, _ := mapValue(widget, "enabled")
	if enabled.Value != "false" || enabled.LineComment != "# explicit" {
		t.Fatalf("explicit leaf changed: %#v", enabled)
	}
	changed, err = EnsureWorkflowTelemetryDefaults(&doc)
	if err != nil || changed {
		t.Fatalf("second changed=%v err=%v", changed, err)
	}

	for _, node := range []*yaml.Node{
		{Kind: yaml.DocumentNode},
		{Kind: yaml.SequenceNode},
	} {
		if _, err := EnsureWorkflowTelemetryDefaults(node); err == nil {
			t.Errorf("accepted invalid node kind %d", node.Kind)
		}
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if changed, err := EnsureWorkflowTelemetryDefaults(root); err != nil || !changed {
		t.Fatalf("raw absent mapping changed=%v err=%v", changed, err)
	}
	for _, src := range []string{
		"workflowTelemetry: false\n",
		"workflowTelemetry:\n  widget: false\n",
		"workflowTelemetry:\n  diagnostics:\n    thresholds: false\n",
	} {
		var bad yaml.Node
		if err := yaml.Unmarshal([]byte(src), &bad); err != nil {
			t.Fatal(err)
		}
		if _, err := EnsureWorkflowTelemetryDefaults(&bad); err == nil {
			t.Errorf("accepted malformed telemetry mapping %q", src)
		}
	}
}
