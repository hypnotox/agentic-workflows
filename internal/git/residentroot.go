package git

import (
	"context"
	"path/filepath"
)

// ProjectResidentRoot maps an invoking checkout to the checkout that owns the
// project's resident awf state. It is the one home for that resolution: the
// composition point in cmd/awf and the transitional project opener both call
// it, so the rule that resident state belongs to the primary checkout cannot
// drift between them.
//
// Any failure to resolve the topology returns invocationPath unchanged, which
// is the only answer that keeps a non-Git tree, a fixture tree, and an
// unresolvable checkout usable: resident state then lives where the command was
// invoked. The seam already owns both steps (ResolveControlRoots and
// ResidentRoot), so this adds no dependency in either direction.
func ProjectResidentRoot(ctx context.Context, invocationPath string) string {
	roots, err := ResolveControlRoots(ctx, invocationPath)
	if err != nil {
		return invocationPath
	}
	primary, err := roots.ResidentRoot(ResidentEfforts)
	if err != nil {
		return invocationPath
	}
	// Every resident name shares the primary checkout's .awf parent, so the
	// checkout is the parent of the resident root's own parent.
	return filepath.Dir(filepath.Dir(primary))
}
