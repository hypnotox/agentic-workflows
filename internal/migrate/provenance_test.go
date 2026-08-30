package migrate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func TestRemoveRetiredClaimProvenancePreservesClaimBytes(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"LF metadata", "Intro.\n\n## Claims\n\n### `invariant: stable`\nProse.\nSummary: Short.\nOrigin: ADR-0001\nRevised-by: ADR-0002\nReferences: other/topic:claim\nBacking: unbacked\nVerify: inspection\n", "Intro.\n\n## Claims\n\n### `invariant: stable`\nProse.\nSummary: Short.\nReferences: other/topic:claim\nBacking: unbacked\nVerify: inspection\n"},
		{"CRLF no final newline", "Intro.\r\n\r\n## Claims\r\n### `rule: stable`\r\nProse.\r\nOrigin: ADR-0001\r\nRevised-by: ADR-0002", "Intro.\r\n\r\n## Claims\r\n### `rule: stable`\r\nProse."},
		{"optional revised and multiple claims", "Intro.\n## Claims\n### `rule: first`\nFirst.\nOrigin: ADR-0001\n\n### `invariant: second`\nSecond.\nOrigin: ADR-0002\nBacking: test\n", "Intro.\n## Claims\n### `rule: first`\nFirst.\n\n### `invariant: second`\nSecond.\nBacking: test\n"},
		{"prose fences and comments", "<!-- awf:comment Origin: ADR-0900 -->\nIntro.\n## Claims\n### `rule: stable`\nProse says Origin: ADR-0900.\n```md\nOrigin: ADR-0901\nRevised-by: ADR-0902\n```\n<!-- awf:comment author note -->\nOrigin: ADR-0001\n", "<!-- awf:comment Origin: ADR-0900 -->\nIntro.\n## Claims\n### `rule: stable`\nProse says Origin: ADR-0900.\n```md\nOrigin: ADR-0901\nRevised-by: ADR-0902\n```\n<!-- awf:comment author note -->\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := removeRetiredClaimProvenance([]byte(tc.source))
			if err != nil || !changed || string(got) != tc.want {
				t.Fatalf("changed=%t err=%v\ngot:  %q\nwant: %q", changed, err, got, tc.want)
			}
			again, changed, err := removeRetiredClaimProvenance(got)
			if err != nil || changed || !bytes.Equal(again, got) {
				t.Fatalf("idempotence changed=%t err=%v got=%q", changed, err, again)
			}
		})
	}
}

func TestRemoveRetiredClaimProvenanceRefusesMalformedOrAmbiguousSource(t *testing.T) {
	for _, source := range []string{
		"Intro.\n## Claims\n### `rule: stable`\nProse.\nOrigin: ADR-0001\nMore prose.\n",
		"Intro.\n## Claims\n### `rule: stable`\nProse.\nRevised-by: ADR-0002\n",
		"Intro.\n## Claims\n### `rule: stable`\nProse.\nOrigin: not-an-ADR\n",
		"Intro.\n## Claims\n### `rule: stable`\nProse.\nOrigin: ADR-0001\nOrigin: ADR-0002\n",
		"Intro.\n## Claims\n### `rule: stable`\nProse.\nOrigin: ADR-0001\n## Later\n",
		"Intro.\n## Claims\n### bad\nProse.\nOrigin: ADR-0001\n",
		"Intro.\n## Claims\n### `rule: stable`\n```\nOrigin: ADR-0001\n",
	} {
		got, changed, err := removeRetiredClaimProvenance([]byte(source))
		if err == nil || changed || !bytes.Equal(got, nil) {
			t.Fatalf("source %q: changed=%t got=%q err=%v, want refusal", source, changed, got, err)
		}
	}
}

func TestRetireClaimProvenanceMigrationConfinesPathsAndPreservesMode(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, currentStatePartsDir, "alpha", "one", "current-state.md")
	contents := "Intro.\n## Claims\n### `rule: stable`\nStable.\nOrigin: ADR-0001\n"
	testsupport.WriteFile(t, part, contents)
	if err := os.Chmod(part, 0o640); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, ".awf", "topics", "current-state.md")
	testsupport.WriteFile(t, outside, contents)
	other := filepath.Join(root, currentStatePartsDir, "alpha", "one", "notes.md")
	testsupport.WriteFile(t, other, contents)
	writeLock(t, root, 47)

	applied, changes, mutations, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{retireClaimProvenanceMetadataName, retireWorkflowConfigName}) || len(changes) != 1 || len(mutations) != 1 {
		t.Fatalf("applied=%v changes=%v mutations=%#v", applied, changes, mutations)
	}
	mutation := mutations[0]
	if mutation.Path != currentStatePartsDir+"/alpha/one/current-state.md" || mutation.Mode != 0o640 || string(mutation.Content) != "Intro.\n## Claims\n### `rule: stable`\nStable.\n" {
		t.Fatalf("mutation=%#v", mutation)
	}
	for _, p := range []string{part, outside, other} {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != contents {
			t.Fatalf("Build mutated %s: %q, %v", p, got, err)
		}
	}
}
