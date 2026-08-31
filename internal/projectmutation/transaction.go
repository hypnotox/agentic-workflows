// Package projectmutation owns the shared mechanics of one focused project
// mutation. Operation packages retain validation, write order, rollback,
// recovery, outcome, and presentation policy.
package projectmutation

import (
	"context"
	"errors"
	"sync"

	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

// Scope is the physical-root coverage selected by a focused operation.
type Scope uint8

const (
	// TrackedScope covers only the selected checkout.
	TrackedScope Scope = iota + 1
	// ProjectScope covers the selected checkout and shared resident root.
	ProjectScope
)

// Phase identifies the mechanical boundary at which an operation stopped.
type Phase string

const (
	PhaseLease       Phase = "lease"
	PhaseAuthority   Phase = "authority"
	PhaseReload      Phase = "reload"
	PhasePublication Phase = "publication"
	PhaseCleanup     Phase = "cleanup"
	PhaseRelease     Phase = "release"
)

var (
	// ErrProjectLeaseCoverage identifies a missing tracked-and-resident lease.
	ErrProjectLeaseCoverage = errors.New("project mutation requires a covering project lease")
	// ErrTrackedLeaseCoverage identifies a missing selected-checkout lease.
	ErrTrackedLeaseCoverage = errors.New("project mutation requires a covering tracked lease")
	// ErrSynchronizationAttempted identifies a second publication attempt.
	ErrSynchronizationAttempted = errors.New("project mutation synchronization already attempted")
)

// Failure preserves a cause while making its retry boundary explicit.
type Failure struct {
	Phase Phase
	Cause error
}

func (e *Failure) Error() string { return e.Cause.Error() }
func (e *Failure) Unwrap() error { return e.Cause }

// FailurePhase returns the mechanical boundary retained by err.
func FailurePhase(err error) (Phase, bool) {
	var failure *Failure
	if !errors.As(err, &failure) {
		return "", false
	}
	return failure.Phase, true
}

// LeaseAcquirer is the narrow injection seam for acquisition and release
// faults. A nil acquirer selects the scope's production acquisition.
type LeaseAcquirer func(context.Context, string) (*filesystem.Lease, func() error, error)

// Transaction holds one operation-selected lease and its project loader.
// Callers explicitly decide when authority is loaded, mutated, reloaded, and
// synchronized.
type Transaction struct {
	ctx     context.Context
	root    string
	loader  *project.Loader
	lease   *filesystem.Lease
	release func() error
	scope   Scope

	mu           sync.Mutex
	synchronized bool
}

// AcquireProject acquires the complete tracked-and-resident transaction.
func AcquireProject(ctx context.Context, root string, loader *project.Loader, acquire LeaseAcquirer) (*Transaction, error) {
	return acquireTransaction(ctx, root, loader, ProjectScope, acquire)
}

// UseProject binds an already-held complete project lease after caller-owned
// setup such as compatibility gating and loader construction.
func UseProject(ctx context.Context, root string, loader *project.Loader, lease *filesystem.Lease) (*Transaction, error) {
	return useTransaction(ctx, root, loader, lease, ProjectScope, nil)
}

// UseTracked binds an already-held selected-checkout lease after caller-owned
// setup such as compatibility gating and loader construction.
func UseTracked(ctx context.Context, root string, loader *project.Loader, lease *filesystem.Lease) (*Transaction, error) {
	return useTransaction(ctx, root, loader, lease, TrackedScope, nil)
}

func acquireTransaction(ctx context.Context, root string, loader *project.Loader, scope Scope, acquire LeaseAcquirer) (*Transaction, error) {
	if loader == nil {
		return nil, &Failure{Phase: PhaseLease, Cause: errors.New("project mutation requires a project loader")}
	}
	if acquire == nil {
		switch scope {
		case ProjectScope:
			acquire = func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
				lease, err := loader.AcquireProjectLease(ctx, root)
				if err != nil {
					return nil, nil, err
				}
				return lease, lease.Release, nil
			}
		case TrackedScope:
			acquire = func(ctx context.Context, root string) (*filesystem.Lease, func() error, error) {
				lease, err := filesystem.AcquireTrackedLease(ctx, root)
				if err != nil {
					return nil, nil, err
				}
				return lease, lease.Release, nil
			}
		default:
			return nil, &Failure{Phase: PhaseLease, Cause: errors.New("project mutation requires a known lease scope")}
		}
	}
	lease, release, err := acquire(ctx, root)
	if err != nil {
		return nil, &Failure{Phase: PhaseLease, Cause: err}
	}
	if release == nil {
		if lease != nil {
			_ = lease.Release()
		}
		return nil, &Failure{Phase: PhaseLease, Cause: scopeContract(scope)}
	}
	return useTransaction(ctx, root, loader, lease, scope, release)
}

func useTransaction(ctx context.Context, root string, loader *project.Loader, lease *filesystem.Lease, scope Scope, release func() error) (*Transaction, error) {
	if loader == nil {
		if release != nil {
			_ = release()
		}
		return nil, &Failure{Phase: PhaseLease, Cause: errors.New("project mutation requires a project loader")}
	}
	tx := &Transaction{ctx: ctx, root: root, loader: loader, lease: lease, release: release, scope: scope}
	if !tx.covers() {
		if release != nil {
			_ = release()
		}
		return nil, &Failure{Phase: PhaseLease, Cause: scopeContract(scope)}
	}
	return tx, nil
}

func (t *Transaction) covers() bool {
	if t == nil || t.lease == nil || t.loader == nil {
		return false
	}
	if t.scope == TrackedScope {
		return t.lease.CoversTracked(t.root)
	}
	return t.scope == ProjectScope && t.loader.CoversProjectLease(t.ctx, t.root, t.lease)
}

func scopeContract(scope Scope) error {
	if scope == TrackedScope {
		return ErrTrackedLeaseCoverage
	}
	return ErrProjectLeaseCoverage
}

// Scope reports the physical-root coverage selected by the operation.
func (t *Transaction) Scope() Scope { return t.scope }

// Root reports the operation-selected project root.
func (t *Transaction) Root() string { return t.root }

// Open returns a root-confined filesystem handle for the selected project.
func (t *Transaction) Open() (*filesystem.Handle, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.covers() {
		return nil, &Failure{Phase: PhaseLease, Cause: scopeContract(t.scope)}
	}
	files, err := filesystem.Open(t.root)
	if err != nil {
		return nil, &Failure{Phase: PhaseAuthority, Cause: err}
	}
	return files, nil
}

// LoadAuthority opens the currently committed project authority while the
// operation-selected lease is held.
func (t *Transaction) LoadAuthority() (*project.Session, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.covers() {
		return nil, &Failure{Phase: PhaseLease, Cause: scopeContract(t.scope)}
	}
	session, err := t.loader.Load(t.ctx, t.root)
	if err != nil {
		return nil, &Failure{Phase: PhaseAuthority, Cause: err}
	}
	return session, nil
}

// LoadForMutation selects config authority through files and retains its exact
// replacement identity.
func (t *Transaction) LoadForMutation(files *filesystem.Handle) (*project.Session, *filesystem.ExpectedIdentity, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.covers() {
		return nil, nil, &Failure{Phase: PhaseLease, Cause: scopeContract(t.scope)}
	}
	session, identity, err := t.loader.LoadForMutation(t.ctx, t.root, files)
	if err != nil {
		return nil, nil, &Failure{Phase: PhaseAuthority, Cause: err}
	}
	return session, identity, nil
}

// Synchronize reloads committed authority and then performs one Publisher
// operation under the transaction's existing complete project lease.
func (t *Transaction) Synchronize() (publisher.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.scope != ProjectScope {
		return publisher.Result{}, &Failure{Phase: PhasePublication, Cause: errors.New("project mutation synchronization requires a project lease")}
	}
	if !t.covers() {
		return publisher.Result{}, &Failure{Phase: PhasePublication, Cause: ErrProjectLeaseCoverage}
	}
	if t.synchronized {
		return publisher.Result{}, &Failure{Phase: PhasePublication, Cause: ErrSynchronizationAttempted}
	}
	t.synchronized = true
	session, err := t.loader.Load(t.ctx, t.root)
	if err != nil {
		return publisher.Result{}, &Failure{Phase: PhaseReload, Cause: err}
	}
	result, err := publisher.New(session, project.Version).SyncLeased(t.ctx, t.lease)
	if err != nil {
		return result, &Failure{Phase: PhasePublication, Cause: err}
	}
	return result, nil
}

// Release ends the operation-selected lease and types a release fault.
func (t *Transaction) Release() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.release == nil {
		return nil
	}
	release := t.release
	t.release = nil
	if err := release(); err != nil {
		return &Failure{Phase: PhaseRelease, Cause: err}
	}
	return nil
}
