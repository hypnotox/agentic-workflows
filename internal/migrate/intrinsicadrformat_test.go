package migrate

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

func TestIntrinsicADRFormatMigrationDiscardsPermanentRoutingPayload(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".awf", "awf.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	before := []byte(`{"awfVersion":"0.30.0","schemaVersion":30,"files":{"x":{"templateId":"t","templateHash":"a","configHash":"b","outputHash":"c"}},"adrFormatV1From":2,"adrFormatV2From":3,"adrFormatV3From":4,"legacyAdrGaps":[1],"initializedWithVersion":"0.30.0"}`)
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := applyIntrinsicADRFormat(context.Background(), root, &out); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"adrFormatV1From", "adrFormatV2From", "adrFormatV3From", "legacyAdrGaps"} {
		if strings.Contains(string(after), key) {
			t.Fatalf("retired %s remained: %s", key, after)
		}
	}
	lock, err := manifest.Parse(after)
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 31 || lock.InitializedWithVersion != "0.30.0" || lock.Files["x"].OutputHash != "c" {
		t.Fatalf("migration did not preserve lock payload: %#v", lock)
	}
	if !strings.Contains(out.String(), "intrinsic-adr-format") {
		t.Fatalf("migration output = %q", out.String())
	}
}

func TestIntrinsicADRFormatMigrationSkipsAbsentAndCurrentLock(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := applyIntrinsicADRFormat(context.Background(), root, &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".awf", "awf.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"awfVersion":"0.31.0","schemaVersion":31,"files":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyIntrinsicADRFormat(context.Background(), root, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("skipped migration wrote %q", out.String())
	}
}

func TestIntrinsicADRFormatMigrationWriteFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".awf", "awf.lock")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applyIntrinsicADRFormat(context.Background(), root, io.Discard); err == nil {
		t.Fatal("accepted lock directory")
	}
}
