package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// applyDropAuditBase is the schema 10 -> 11 migration. Neither this repo's
// config nor examples/sundial sets audit.baseBranch, so no sync or check run
// exercises it (ADR-0127 Consequences): these fixtures are its only coverage.
func TestApplyDropAuditBase(t *testing.T) {
	cases := []struct {
		name, src, want string
		wantMsg         bool
	}{
		{
			name:    "removes the key, keeps siblings",
			src:     "prefix: ex\naudit:\n  baseBranch: develop\n  diffThreshold: 400\n",
			want:    "prefix: ex\naudit:\n  diffThreshold: 400\n",
			wantMsg: true,
		},
		{
			name:    "sole child drops the whole audit block",
			src:     "prefix: ex\naudit:\n  baseBranch: main\nskills: []\n",
			want:    "prefix: ex\nskills: []\n",
			wantMsg: true,
		},
		{
			name: "no audit block is a silent no-op",
			src:  "prefix: ex\nskills: []\n",
			want: "prefix: ex\nskills: []\n",
		},
		{
			name: "audit block without the key is a silent no-op",
			src:  "prefix: ex\naudit:\n  diffThreshold: 400\n",
			want: "prefix: ex\naudit:\n  diffThreshold: 400\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, ".awf", "config.yaml")
			testsupport.WriteFile(t, p, tc.src)
			var out bytes.Buffer
			if err := applyDropAuditBase(root, &out); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(p)
			if err != nil { // coverage-ignore: the migration just wrote this path
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("config:\ngot  %q\nwant %q", got, tc.want)
			}
			msg := strings.Contains(out.String(), "drop-audit-base: removed audit.baseBranch")
			if msg != tc.wantMsg {
				t.Errorf("announcement = %v, want %v (output %q)", msg, tc.wantMsg, out.String())
			}
			// A replay must neither change the file again nor re-announce.
			var second bytes.Buffer
			if err := applyDropAuditBase(root, &second); err != nil {
				t.Fatal(err)
			}
			again, rerr := os.ReadFile(p)
			if rerr != nil { // coverage-ignore: the path was read successfully a moment ago
				t.Fatal(rerr)
			}
			if string(again) != string(got) {
				t.Errorf("not idempotent: %q then %q", got, again)
			}
			if second.Len() != 0 {
				t.Errorf("replay re-announced: %q", second.String())
			}
		})
	}
}

func TestApplyDropAuditBaseNoConfig(t *testing.T) {
	var out bytes.Buffer
	if err := applyDropAuditBase(t.TempDir(), &out); err != nil {
		t.Fatalf("an absent config.yaml must be a no-op, got %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("nothing to remove must announce nothing, got %q", out.String())
	}
}

func TestApplyDropAuditBaseMalformedConfig(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "audit: [a, b\n")
	if err := applyDropAuditBase(root, &bytes.Buffer{}); err == nil {
		t.Fatal("a malformed config must surface the parse error, not be swallowed")
	}
}
