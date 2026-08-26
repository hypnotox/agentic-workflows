package filesystem

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gofrs/flock"
)

// LeaseErrorKind identifies the lease stage that failed so focused callers can
// preserve their own diagnostics without branching on filesystem error text.
type LeaseErrorKind string

const (
	LeaseCanonicalRoot LeaseErrorKind = "canonical-root"
	LeaseCacheLocation LeaseErrorKind = "cache-location"
	LeaseCacheCreation LeaseErrorKind = "cache-creation"
	LeaseCacheMode     LeaseErrorKind = "cache-mode"
	LeaseAcquisition   LeaseErrorKind = "acquisition"
	LeaseFileMode      LeaseErrorKind = "file-mode"
)

// LeaseError preserves the failed lease stage and underlying error identity.
type LeaseError struct {
	Kind  LeaseErrorKind
	Cause error
}

func (e *LeaseError) Error() string { return "lease " + string(e.Kind) + ": " + e.Cause.Error() }
func (e *LeaseError) Unwrap() error { return e.Cause }

func leaseError(kind LeaseErrorKind, cause error) error {
	return &LeaseError{Kind: kind, Cause: cause}
}

type leaseRequest struct {
	scope string
	root  string
}

type leaseIdentity struct {
	scope string
	root  string
}

// Lease is an acquired, ordered set of advisory identities. Keeping this value
// explicit lets an operation prove that authority loading and publication share
// the same transaction without making the mechanism own operation policy.
type Lease struct {
	identities []leaseIdentity
	locks      []*flock.Flock
	released   bool
}

// Release relinquishes every identity in reverse acquisition order.
func (l *Lease) Release() error {
	if l == nil || l.released {
		return nil
	}
	l.released = true
	var result error
	for index := len(l.locks) - 1; index >= 0; index-- {
		result = errors.Join(result, l.locks[index].Close())
	}
	return result
}

// CoversProject reports whether this live lease contains the complete tracked
// and resident identity set for roots.
func (l *Lease) CoversProject(tracked, resident string) bool {
	if l == nil || l.released {
		return false
	}
	want, err := canonicalIdentities([]leaseRequest{
		{scope: "project-tracked-locks", root: tracked},
		{scope: "project-resident-locks", root: resident},
	})
	return err == nil && slices.Equal(l.identities, want)
}

// Acquire obtains persistent advisory leases for roots in one semantic scope.
// It is process-exit safe because advisory locks belong to the open file
// descriptor, and callers should explicitly call the returned release.
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

// AcquireTrackedLease obtains the checkout-local mutation lease. Operations
// that do not reach primary-resident state must use this instead of a combined
// project lease so independent linked worktrees can proceed concurrently.
func AcquireTrackedLease(ctx context.Context, root string) (*Lease, error) {
	return acquire(ctx, []leaseRequest{{scope: "project-tracked-locks", root: root}})
}

// AcquireResidentLease obtains the primary-resident transaction capability.
// Resident-only operations use it so independent selected checkouts contend on
// their shared lifecycle state without unnecessarily serializing tracked work.
func AcquireResidentLease(ctx context.Context, root string) (*Lease, error) {
	return acquire(ctx, []leaseRequest{{scope: "project-resident-locks", root: root}})
}

// AcquireProjectLease returns the transaction identity for an operation that
// changes both checkout-local and primary-resident state. acquire canonicalizes
// and orders both identities before taking either lock.
func AcquireProjectLease(ctx context.Context, tracked, resident string) (*Lease, error) {
	return acquire(ctx, []leaseRequest{
		{scope: "project-tracked-locks", root: tracked},
		{scope: "project-resident-locks", root: resident},
	})
}

func canonicalIdentities(requests []leaseRequest) ([]leaseIdentity, error) {
	identities := make([]leaseIdentity, 0, len(requests))
	for _, request := range requests {
		identity, err := CanonicalRoot(request.root)
		if err != nil {
			return nil, leaseError(LeaseCanonicalRoot, fmt.Errorf("canonicalize lease root %s: %w", request.root, err))
		}
		identities = append(identities, leaseIdentity{scope: request.scope, root: identity})
	}
	slices.SortFunc(identities, func(a, b leaseIdentity) int {
		if a.root < b.root {
			return -1
		}
		if a.root > b.root {
			return 1
		}
		if a.scope < b.scope {
			return -1
		}
		if a.scope > b.scope {
			return 1
		}
		return 0
	})
	return slices.Compact(identities), nil
}

func acquire(ctx context.Context, requests []leaseRequest) (*Lease, error) {
	identities, err := canonicalIdentities(requests)
	if err != nil {
		return nil, err
	}
	lease := &Lease{identities: identities, locks: make([]*flock.Flock, 0, len(identities))}
	for _, identity := range identities {
		cache, err := leaseCache(identity.scope)
		if err != nil {
			_ = lease.Release()
			return nil, err
		}
		// Preserve the ADR lock key protocol: a scope picks its cache directory;
		// the physical-root key remains the SHA-256 of the canonical root.
		key := fmt.Sprintf("%x", sha256.Sum256([]byte(identity.root)))
		lock := flock.New(filepath.Join(cache, key+".lock"))
		for {
			locked, err := lock.TryLock()
			if err != nil {
				_ = lock.Close()
				_ = lease.Release()
				return nil, leaseError(LeaseAcquisition, fmt.Errorf("acquire lease for %s: %w", identity.root, err))
			}
			if locked {
				break
			}
			select {
			case <-ctx.Done():
				_ = lock.Close()
				_ = lease.Release()
				return nil, leaseError(LeaseAcquisition, fmt.Errorf("acquire lease for %s: %w", identity.root, ctx.Err()))
			case <-time.After(20 * time.Millisecond):
			}
		}
		if err := os.Chmod(lock.Path(), 0o600); err != nil {
			_ = lock.Close()
			_ = lease.Release()
			return nil, leaseError(LeaseFileMode, fmt.Errorf("restrict lease file %s: %w", lock.Path(), err))
		}
		lease.locks = append(lease.locks, lock)
	}
	return lease, nil
}

func leaseCache(scope string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", leaseError(LeaseCacheLocation, fmt.Errorf("locate lease cache: %w", err))
	}
	cache = filepath.Join(cache, "awf", scope)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return "", leaseError(LeaseCacheCreation, fmt.Errorf("create lease cache %s: %w", cache, err))
	}
	if err := os.Chmod(cache, 0o700); err != nil {
		return "", leaseError(LeaseCacheMode, fmt.Errorf("restrict lease cache %s: %w", cache, err))
	}
	return cache, nil
}
