package contextspill

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// invariant: tooling/context-and-topic:context-spill-observability
func TestSpillObservabilityStorageContract(t *testing.T) {
	t.Run("exact notice grammar", testParseNoticeContract)
	t.Run("exact record", testLogExactRecord)
	t.Run("exact record quoting", testShellQuote)
	t.Run("secure durable concurrent append", testLogSecureAppendAndConcurrency)
	t.Run("no-follow path rejection", testLogRejectsUnsafePaths)
	t.Run("operational error preservation", testLogOperationalFailures)
	t.Run("ownership and descriptor anchoring", testLogDirectoryValidationAndDescriptorAnchoring)
	t.Run("safe log states", testHasSafeLogRejectsForeignOwnerAndMissingPaths)
	t.Run("safe log operational errors", testHasSafeLogOperationalErrors)
	t.Run("partial append writes", testWriteAllFDHandlesPartialWrites)
}

func testParseNoticeContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spill.txt")
	if err := os.WriteFile(path, []byte("spill"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := []byte("AWF_CONTEXT_SPILL_V1 bytes=8193 format=text\n" + path + "\n")
	notice, recognized, err := ParseNotice(valid)
	if err != nil || !recognized {
		t.Fatalf("ParseNotice(valid) = %#v, %v, %v", notice, recognized, err)
	}
	if notice.Bytes != 8193 || notice.Path != path {
		t.Fatalf("notice = %#v", notice)
	}
	for name, data := range map[string][]byte{
		"ordinary output":       []byte("context: live state\n"),
		"near miss prefix":      []byte("AWF_CONTEXT_SPILL bytes=1 format=text\n/tmp/x\n"),
		"missing second line":   []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\n"),
		"extra line":            []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\n/tmp/x\nextra\n"),
		"missing final newline": []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\n/tmp/x"),
		"bad bytes":             []byte("AWF_CONTEXT_SPILL_V1 bytes=+1 format=text\n/tmp/x\n"),
		"overflow bytes":        []byte("AWF_CONTEXT_SPILL_V1 bytes=18446744073709551616 format=text\n/tmp/x\n"),
		"bad format":            []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=json\n/tmp/x\n"),
		"relative path":         []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\nrelative\n"),
		"unclean path":          []byte("AWF_CONTEXT_SPILL_V1 bytes=1 format=text\n/tmp/a/../x\n"),
	} {
		t.Run(name, func(t *testing.T) {
			_, gotRecognized, gotErr := ParseNotice(data)
			if name == "ordinary output" || name == "near miss prefix" {
				if gotRecognized || gotErr != nil {
					t.Fatalf("ParseNotice() = recognized %v, err %v", gotRecognized, gotErr)
				}
				return
			}
			if !gotRecognized || gotErr == nil {
				t.Fatalf("ParseNotice() = recognized %v, err %v", gotRecognized, gotErr)
			}
		})
	}
}

func testLogExactRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldNow := now
	now = func() time.Time { return time.Date(2026, 7, 27, 12, 34, 56, 123, time.FixedZone("offset", 3600)) }
	t.Cleanup(func() { now = oldNow })
	if err := Log(root, Notice{Bytes: 9000, Path: "/tmp/secret"}, []string{"./x", "context", "a b"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".awf", "local", "context-spills.log"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "2026-07-27T11:34:56.000000123Z\tbytes=9000\tinvocation='./x' 'context' 'a b'\n"
	if string(data) != want {
		t.Fatalf("record=%q, want %q", data, want)
	}
}

func testShellQuote(t *testing.T) {
	got := ShellQuote([]string{"./x", "context", "", "a b", "it's"})
	want := "'./x' 'context' '' 'a b' 'it'\\''s'"
	if got != want {
		t.Fatalf("ShellQuote() = %q, want %q", got, want)
	}
}

func testLogSecureAppendAndConcurrency(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldNow := now
	now = func() time.Time { return time.Date(2026, 7, 27, 12, 34, 56, 123, time.FixedZone("offset", 3600)) }
	t.Cleanup(func() { now = oldNow })

	const writers = 16
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Log(root, Notice{Bytes: uint64(9000 + i), Path: "/tmp/secret"}, []string{"./x", "context", string(rune('a' + i))})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	local := filepath.Join(root, ".awf", "local")
	logPath := filepath.Join(local, "context-spills.log")
	assertMode(t, local, 0o700)
	assertMode(t, logPath, 0o600)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/tmp/secret") {
		t.Fatal("log serialized ephemeral spill path")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != writers {
		t.Fatalf("records = %d, want %d\n%s", len(lines), writers, data)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "2026-07-27T11:34:56.000000123Z\tbytes=") || !strings.Contains(line, "\tinvocation='./x' 'context' '") {
			t.Errorf("malformed record %q", line)
		}
	}
}

func testLogRejectsUnsafePaths(t *testing.T) {
	t.Run("awf symlink", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Symlink(t.TempDir(), filepath.Join(root, ".awf")); err != nil {
			t.Fatal(err)
		}
		if err := Log(root, Notice{Bytes: 1}, nil); err == nil {
			t.Fatal("expected symlink rejection")
		}
	})
	t.Run("local mode", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".awf", "local"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Log(root, Notice{Bytes: 1}, nil); err == nil {
			t.Fatal("expected local mode rejection")
		}
	})
	t.Run("log symlink", func(t *testing.T) {
		root := t.TempDir()
		local := filepath.Join(root, ".awf", "local")
		if err := os.MkdirAll(local, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "target"), filepath.Join(local, "context-spills.log")); err != nil {
			t.Fatal(err)
		}
		if err := Log(root, Notice{Bytes: 1}, nil); err == nil {
			t.Fatal("expected log symlink rejection")
		}
	})
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}
