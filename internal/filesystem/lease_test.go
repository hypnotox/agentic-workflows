package filesystem

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func Acquire(ctx context.Context, scope string, roots ...string) (func() error, error) {
	requests := make([]leaseRequest, 0, len(roots))
	for _, root := range roots {
		requests = append(requests, leaseRequest{scope: scope, root: root})
	}
	lease, err := acquire(ctx, requests)
	if err != nil {
		return nil, err
	}
	return lease.Release, nil
}

func TestLeaseErrorCarriesTypedStageIdentity(t *testing.T) {
	_, err := Acquire(context.Background(), "lease-test", filepath.Join(t.TempDir(), "missing"))
	var leaseErr *LeaseError
	if !errors.As(err, &leaseErr) || leaseErr.Kind != LeaseCanonicalRoot || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("canonical lease error = %v, identity = %#v", err, leaseErr)
	}
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestRootScopedProjectMutationLeases)
func TestRootScopedProjectMutationLeases(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(filepath.Dir(root), "alias")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(alias)
		canonical, err := CanonicalRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		aliased, err := CanonicalRoot(alias)
		if err != nil {
			t.Fatal(err)
		}
		if canonical != aliased {
			t.Fatalf("alias identity = %q, want %q", aliased, canonical)
		}
	}
	release, err := Acquire(context.Background(), "lease-test", root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = Acquire(ctx, "lease-test", root)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended acquire = %v, want context deadline", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(context.Background(), "lease-test", root)
	if err != nil {
		t.Fatal(err)
	}
	if err := second(); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestAcquireProjectRetainsDistinctScopesAtSameRoot)
func TestAcquireProjectRetainsDistinctScopesAtSameRoot(t *testing.T) {
	root := t.TempDir()
	lease, err := AcquireProjectLease(context.Background(), root, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, "project-tracked-locks", root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("tracked scope acquired through project lease = %v, want deadline", err)
	}
	if _, err := Acquire(ctx, "project-resident-locks", root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("resident scope acquired through project lease = %v, want deadline", err)
	}
}

func TestLeaseHelperProcess(t *testing.T) {
	if os.Getenv("AWF_LEASE_HELPER") != "1" {
		return
	}
	var release func() error
	var err error
	if os.Getenv("AWF_PROJECT_LEASE") == "1" {
		var lease *Lease
		lease, err = AcquireProjectLease(context.Background(), os.Getenv("AWF_TRACKED_ROOT"), os.Getenv("AWF_RESIDENT_ROOT"))
		if lease != nil {
			release = lease.Release
		}
	} else {
		roots := strings.Split(os.Getenv("AWF_LEASE_ROOTS"), string(os.PathListSeparator))
		release, err = Acquire(context.Background(), os.Getenv("AWF_LEASE_SCOPE"), roots...)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("AWF_LEASE_READY"), []byte("ready"), 0o600); err != nil {
		os.Exit(3)
	}
	defer func() { _ = release() }()
	time.Sleep(time.Hour)
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestLeaseCrossProcessContentionAndProcessDeathRelease)
func TestLeaseCrossProcessContentionAndProcessDeathRelease(t *testing.T) {
	root := t.TempDir()
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLeaseHelperProcess$")
	cmd.Env = append(os.Environ(), "AWF_LEASE_HELPER=1", "AWF_LEASE_SCOPE=cross-process-test", "AWF_LEASE_ROOTS="+root, "AWF_LEASE_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not acquire lease")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, "cross-process-test", root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cross-process contention = %v, want deadline", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed helper exited successfully")
	}
	release, err := Acquire(context.Background(), "cross-process-test", root)
	if err != nil {
		t.Fatalf("lease remained after process death: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestLeaseOrderingAndRestrictivePersistentModes)
func TestLeaseOrderingAndRestrictivePersistentModes(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	one, err := Acquire(context.Background(), "ordered-test", first, second)
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan error, 1)
	go func() {
		two, err := Acquire(context.Background(), "ordered-test", second, first)
		if err == nil {
			err = two()
		}
		acquired <- err
	}()
	select {
	case err := <-acquired:
		t.Fatalf("reverse acquisition passed before release: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	if err := one(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reverse ordered acquisition deadlocked")
	}
	cache, err := leaseCache("ordered-test")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(cache); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("cache mode = %v, %v", info, err)
	}
	identity, _ := CanonicalRoot(first)
	lockPath := filepath.Join(cache, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(identity))))
	if info, err := os.Stat(lockPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("lock mode = %v, %v", info, err)
	}
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestAcquireProjectSerializesSharedResidentAcrossTrackedRoots)
func TestAcquireProjectSerializesSharedResidentAcrossTrackedRoots(t *testing.T) {
	firstTracked, secondTracked, resident := t.TempDir(), t.TempDir(), t.TempDir()
	ready := filepath.Join(t.TempDir(), "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLeaseHelperProcess$")
	cmd.Env = append(os.Environ(),
		"AWF_LEASE_HELPER=1", "AWF_PROJECT_LEASE=1",
		"AWF_TRACKED_ROOT="+firstTracked, "AWF_RESIDENT_ROOT="+resident,
		"AWF_LEASE_READY="+ready,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("project-lease helper did not acquire")
		}
		time.Sleep(10 * time.Millisecond)
	}

	trackedOnly, err := Acquire(context.Background(), "project-tracked-locks", secondTracked)
	if err != nil {
		t.Fatalf("independent tracked root contended: %v", err)
	}
	if err := trackedOnly(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := AcquireProjectLease(ctx, secondTracked, resident); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shared resident project lease = %v, want deadline", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed project-lease helper exited successfully")
	}
	lease, err := AcquireProjectLease(context.Background(), secondTracked, resident)
	if err != nil {
		t.Fatalf("shared resident remained leased after process death: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

// invariant: tooling/filesystem-access:root-scoped-project-mutation-leases (TestAcquireProjectAllowsIndependentTrackedRoots)
func TestAcquireProjectAllowsIndependentTrackedRoots(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	one, err := Acquire(context.Background(), "project-tracked-locks", first)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = one() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	two, err := Acquire(ctx, "project-tracked-locks", second)
	if err != nil {
		t.Fatalf("independent tracked lease: %v", err)
	}
	if err := two(); err != nil {
		t.Fatal(err)
	}
}
