package migrate

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestApplyTopicClaimBudget(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
	}{
		{"creates currentState", "# keep\nprefix: example\n", "currentState:\n  maxClaimsPerTopic: 20"},
		{"adds to currentState", "prefix: example\ncurrentState:\n  topicCoverage: warn # keep\n", "  maxClaimsPerTopic: 20"},
		{"preserves explicit", "prefix: example\ncurrentState:\n  maxClaimsPerTopic: 7\n", "  maxClaimsPerTopic: 7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testsupport.WriteAwfConfig(t, root, tc.source)
			var out bytes.Buffer
			if err := applyTopicClaimBudget(root, &out); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(config.ConfigPath(root))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(body, []byte(tc.want)) {
				t.Fatalf("config missing %q:\n%s", tc.want, body)
			}
			if tc.name == "preserves explicit" {
				if out.Len() != 0 {
					t.Fatalf("explicit value output = %q", out.String())
				}
			} else if out.String() != "topic-claim-budget: set currentState.maxClaimsPerTopic to 20\n" {
				t.Fatalf("output = %q", out.String())
			}
			before := bytes.Clone(body)
			out.Reset()
			if err := applyTopicClaimBudget(root, &out); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(config.ConfigPath(root))
			if !bytes.Equal(before, after) || out.Len() != 0 {
				t.Fatalf("second run changed config or output: %q", out.String())
			}
		})
	}
}

func TestUpgradeSchemaFifteenToSixteenPreservesCutoff(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: example\ncurrentState:\n  topicCoverage: off\n")
	lock := &manifest.Lock{AWFVersion: "0.20.0", SchemaVersion: 15, Files: map[string]manifest.Entry{}, ADRFormatV1From: 2, ADRFormatV2From: 9, LegacyADRGaps: []int{1}}
	if err := lock.Save(config.LockPath(root)); err != nil {
		t.Fatal(err)
	}
	applied, err := Upgrade(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, []string{"topic-claim-budget"}) {
		t.Fatalf("applied = %#v", applied)
	}
	upgraded, err := manifest.Load(config.LockPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != 16 || upgraded.ADRFormatV1From != 2 || upgraded.ADRFormatV2From != 9 || !reflect.DeepEqual(upgraded.LegacyADRGaps, []int{1}) {
		t.Fatalf("upgraded lock = %#v", upgraded)
	}
	applied, err = Upgrade(root, io.Discard)
	if err != nil || len(applied) != 0 {
		t.Fatalf("second upgrade = %#v, %v", applied, err)
	}
}

func TestApplyTopicClaimBudgetAbsentAndMalformed(t *testing.T) {
	if err := applyTopicClaimBudget(t.TempDir(), io.Discard); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	testsupport.WriteAwfConfig(t, root, "prefix: [bad\n")
	if err := applyTopicClaimBudget(root, io.Discard); err == nil {
		t.Fatal("malformed config accepted")
	}
}
