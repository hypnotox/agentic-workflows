package adr

import (
	"context"
	"fmt"
	"strings"

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
	// Preserve the ADR-facing diagnostics without retaining a second mechanism.
	message := err.Error()
	switch {
	case strings.Contains(message, "canonicalize lease root"):
		return nil, fmt.Errorf("canonicalize decisions directory %s: %w", dir, err)
	case strings.Contains(message, "locate lease cache"):
		return nil, fmt.Errorf("locate ADR lock cache: %w", err)
	case strings.Contains(message, "create lease cache"):
		return nil, fmt.Errorf("create ADR lock cache: %w", err)
	default:
		return nil, err
	}
}
