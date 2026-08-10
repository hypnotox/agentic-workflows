package adr

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// acquireScaffoldLock serializes one decisions directory's identity transaction.
// The persistent lock file is deliberately retained: deleting it could split
// concurrent users across distinct inodes.
func acquireScaffoldLock(dir string) (func() error, error) {
	identity, err := canonicalDecisionsDirectory(dir)
	if err != nil { // coverage-ignore: canonical directory faults require an unavailable or malformed filesystem path
		return nil, fmt.Errorf("canonicalize decisions directory %s: %w", dir, err)
	}
	cache, err := os.UserCacheDir()
	if err != nil { // coverage-ignore: UserCacheDir only fails when the host user environment is unavailable
		return nil, fmt.Errorf("locate ADR lock cache: %w", err)
	}
	cache = filepath.Join(cache, "awf", "adr-locks")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return nil, fmt.Errorf("create ADR lock cache %s: %w", cache, err)
	}
	if err := os.Chmod(cache, 0o700); err != nil { // coverage-ignore: chmod of this user-owned cache needs a host filesystem fault
		return nil, fmt.Errorf("restrict ADR lock cache %s: %w", cache, err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	lock := flock.New(filepath.Join(cache, key+".lock"))
	if err := lock.Lock(); err != nil { // coverage-ignore: flock acquisition faults require host filesystem failure; contention blocks instead
		return nil, fmt.Errorf("lock ADR decisions directory %s: %w", identity, err)
	}
	if err := os.Chmod(lock.Path(), 0o600); err != nil { // coverage-ignore: chmod of the just-created lock needs a host filesystem fault
		return nil, fmt.Errorf("restrict ADR lock file %s: %w", lock.Path(), errors.Join(err, lock.Close()))
	}
	return lock.Close, nil
}
