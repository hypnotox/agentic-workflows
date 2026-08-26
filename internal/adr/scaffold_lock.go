package adr

import (
	"context"
	"errors"
	"fmt"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
)

// acquireScaffoldLock configures the neutral lease owner with the established
// ADR cache namespace. The persistent protocol itself lives in filesystem so
// ADR scaffolding cannot diverge from project mutation serialization.
func acquireScaffoldLock(dir string) (func() error, error) {
	// Preserve the platform-specific ADR identity validation and diagnostics;
	// filesystem remains the sole lock allocation and acquisition owner.
	if _, err := canonicalDecisionsDirectory(dir); err != nil {
		return nil, fmt.Errorf("canonicalize decisions directory %s: %w", dir, err)
	}
	release, err := filesystem.Acquire(context.Background(), "adr-locks", dir)
	if err == nil {
		return release, nil
	}
	// Preserve the ADR-facing diagnostics through the shared owner's typed stage
	// identity rather than coupling control flow to its message text.
	var leaseErr *filesystem.LeaseError
	if !errors.As(err, &leaseErr) {
		return nil, err
	}
	switch leaseErr.Kind {
	case filesystem.LeaseCanonicalRoot:
		return nil, fmt.Errorf("canonicalize decisions directory %s: %w", dir, err)
	case filesystem.LeaseCacheLocation:
		return nil, fmt.Errorf("locate ADR lock cache: %w", err)
	case filesystem.LeaseCacheCreation:
		return nil, fmt.Errorf("create ADR lock cache: %w", err)
	case filesystem.LeaseCacheMode:
		return nil, fmt.Errorf("restrict ADR lock cache: %w", err)
	default:
		return nil, err
	}
}
