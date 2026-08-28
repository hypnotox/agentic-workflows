package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

type blockingDiagnosticWriter struct {
	started chan struct{}
	unblock chan struct{}
}

func (w *blockingDiagnosticWriter) Write(value []byte) (int, error) {
	select {
	case w.started <- struct{}{}:
	default:
	}
	<-w.unblock
	return len(value), nil
}

type failingDiagnosticWriter struct{}

func (failingDiagnosticWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func assertDiagnosticLeaseLifetime(t *testing.T, held handlerResult, acquire func(context.Context) (*filesystem.Lease, error)) {
	t.Helper()
	writer := &blockingDiagnosticWriter{started: make(chan struct{}, 1), unblock: make(chan struct{})}
	done := make(chan int, 1)
	go func() { done <- completeHandlerResult(io.Discard, writer, held) }()
	select {
	case <-writer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("diagnostic writer did not begin")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if lease, err := acquire(ctx); err == nil {
		_ = lease.Release()
		t.Fatal("competing lease acquired while diagnostic write was blocked")
	}
	close(writer.unblock)
	if code := <-done; code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	lease, err := acquire(context.Background())
	if err != nil {
		t.Fatalf("competing lease remained held after diagnostic completion: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestEffortRefusalLeaseCoversDiagnosticWrite(t *testing.T) {
	root := commandRepo(t)
	t.Chdir(root)
	var stdout bytes.Buffer
	result := newRunner(os.Getwd, os.Stdin, func() bool { return false }).handlers["effort"](&cmdCtx{ctx: testContext(t), root: root, sub: "new", inv: invocation{positionals: []string{"too long"}, bools: map[string]bool{"--no-worktree": true}, values: map[string]string{"--slug": strings.Repeat("s", 33)}}, stdout: &stdout})
	if result.err == nil || result.release == nil {
		t.Fatalf("result = %#v, want held typed refusal", result)
	}
	resident := awfgit.ProjectResidentRoot(testContext(t), root)
	assertDiagnosticLeaseLifetime(t, result, func(ctx context.Context) (*filesystem.Lease, error) {
		return filesystem.AcquireResidentLease(ctx, resident)
	})
}

func TestUpgradePartialAuthorityLeaseCoversDiagnosticWrite(t *testing.T) {
	root := scaffoldProject(t)
	if err := os.Remove(filepath.Join(root, ".awf", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	var release func() error
	err := runUpgradeFlags(testContext(t), root, false, leaseRetainingWriter{Writer: io.Discard, retain: func(value func() error) { release = value }})
	if err == nil || release == nil {
		t.Fatalf("runUpgradeFlags error=%v hasRelease=%t, want held partial-authority failure", err, release != nil)
	}
	assertDiagnosticLeaseLifetime(t, handlerFailureHeld(err, release), func(ctx context.Context) (*filesystem.Lease, error) {
		return filesystem.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
	})
}

func TestDiagnosticWriterFailureReleasesHeldLease(t *testing.T) {
	root := commandRepo(t)
	resident := awfgit.ProjectResidentRoot(testContext(t), root)
	lease, err := filesystem.AcquireResidentLease(testContext(t), resident)
	if err != nil {
		t.Fatal(err)
	}
	if code := completeHandlerResult(io.Discard, failingDiagnosticWriter{}, handlerFailureHeld(errors.New("typed diagnostic write failure"), lease.Release)); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	second, err := filesystem.AcquireResidentLease(context.Background(), resident)
	if err != nil {
		t.Fatalf("lease leaked after diagnostic writer failure: %v", err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}
