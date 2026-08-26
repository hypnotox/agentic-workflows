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

// CanonicalRoot returns the physical identity used to serialize a root. It
// resolves symbolic links; callers select the semantic scope separately.
func CanonicalRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	identity, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return filepath.Clean(identity), nil
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

// AcquireProjectLease returns the transaction identity for composition across
// immutable root discovery, mutable authority loading, planning, and outcome.
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
			return nil, fmt.Errorf("canonicalize lease root %s: %w", request.root, err)
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
				return nil, fmt.Errorf("acquire lease for %s: %w", identity.root, err)
			}
			if locked {
				break
			}
			select {
			case <-ctx.Done():
				_ = lock.Close()
				_ = lease.Release()
				return nil, fmt.Errorf("acquire lease for %s: %w", identity.root, ctx.Err())
			case <-time.After(20 * time.Millisecond):
			}
		}
		if err := os.Chmod(lock.Path(), 0o600); err != nil {
			_ = lock.Close()
			_ = lease.Release()
			return nil, fmt.Errorf("restrict lease file %s: %w", lock.Path(), err)
		}
		lease.locks = append(lease.locks, lock)
	}
	return lease, nil
}

func leaseCache(scope string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate lease cache: %w", err)
	}
	cache = filepath.Join(cache, "awf", scope)
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return "", fmt.Errorf("create lease cache %s: %w", cache, err)
	}
	if err := os.Chmod(cache, 0o700); err != nil {
		return "", fmt.Errorf("restrict lease cache %s: %w", cache, err)
	}
	return cache, nil
}
