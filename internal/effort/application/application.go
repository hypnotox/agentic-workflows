// Package application owns effort lifecycle and managed-worktree use cases.
package application

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// Kind identifies one parsed effort use case. Command grammar remains owned by
// the CLI adapter; this type carries only the selected application operation.
type Kind uint8

const (
	New Kind = iota + 1
	List
	Show
	Finish
	AddWorktree
	RemoveWorktree
	Integrate
)

// Request is the representation-neutral input to one effort use case.
type Request struct {
	Kind  Kind
	Slug  string
	Title string
	Base  string
}

// Result is a completely mapped application result. Release retains the
// mutation lease through the caller's final presentation write.
type Result struct {
	Document presentation.Document
	Release  func() error
}

// Checkout is the application boundary's Git contract. Worktree mechanics use
// its narrower Runner subset; effort creation additionally validates ref names.
type Checkout interface {
	worktree.Runner
	ValidateRefName(context.Context, string) (bool, error)
}

// Lease is the lifetime capability returned by mutation serialization.
type Lease interface{ Release() error }

// Dependencies are the production mechanisms selected at this application
// boundary. Tests compose the same boundary with controlled per-instance seams.
type Dependencies struct {
	ResolveRoots        func(context.Context, string) (awfgit.ControlRoots, error)
	OpenCheckout        func(string) (Checkout, error)
	OpenResident        func(string) (worktree.ResidentHandle, error)
	AcquireProjectLease func(context.Context, string, string) (Lease, error)
	Clock               func() time.Time
	UUID                func() (string, error)
	GateCommand         func(string) (string, error)
}

// ProductionDependencies selects the repository's ordinary mechanisms.
func ProductionDependencies() Dependencies {
	return Dependencies{
		ResolveRoots: awfgit.ResolveControlRoots,
		OpenCheckout: func(root string) (Checkout, error) { return awfgit.Open(root) },
		OpenResident: func(root string) (worktree.ResidentHandle, error) { return filesystem.Open(root) },
		AcquireProjectLease: func(ctx context.Context, root, residentRoot string) (Lease, error) {
			return filesystem.AcquireProjectLease(ctx, root, residentRoot)
		},
		Clock:       time.Now,
		UUID:        effort.RandomUUIDv4,
		GateCommand: integrationGateCommand,
	}
}

// Admission validates mutable project authority while a mutation lease is held.
type Admission func(context.Context, string) error

// Execute runs one request against production dependencies.
func Execute(ctx context.Context, root string, request Request, expectedArchiveMarker func() ([]byte, error), admit Admission) (Result, error) {
	return ExecuteWith(ctx, root, request, expectedArchiveMarker, admit, ProductionDependencies())
}

// ExecuteWith runs one request against explicitly selected dependencies.
func ExecuteWith(ctx context.Context, root string, request Request, expectedArchiveMarker func() ([]byte, error), admit Admission, deps Dependencies) (Result, error) {
	if err := validateDependencies(deps, expectedArchiveMarker); err != nil {
		return Result{}, err
	}
	var lease Lease
	if request.mutates() {
		if admit == nil {
			return Result{}, errors.New("effort application: missing mutation admission")
		}
		var err error
		lease, err = deps.AcquireProjectLease(ctx, root, awfgit.ProjectResidentRoot(ctx, root))
		if err != nil {
			return Result{}, err
		}
	}
	result := Result{}
	if lease != nil {
		result.Release = lease.Release
		if err := admit(ctx, root); err != nil {
			return result, err
		}
	}
	app, err := open(ctx, root, expectedArchiveMarker, deps)
	if err != nil {
		return result, err
	}
	document, err := app.execute(ctx, root, request)
	if err != nil {
		return result, presentError(err)
	}
	result.Document = document
	return result, nil
}

func validateDependencies(deps Dependencies, marker func() ([]byte, error)) error {
	switch {
	case deps.ResolveRoots == nil:
		return errors.New("effort application: missing control-root resolver")
	case deps.OpenCheckout == nil:
		return errors.New("effort application: missing checkout opener")
	case deps.OpenResident == nil:
		return errors.New("effort application: missing resident opener")
	case deps.AcquireProjectLease == nil:
		return errors.New("effort application: missing project lease acquirer")
	case deps.Clock == nil:
		return errors.New("effort application: missing clock")
	case deps.UUID == nil:
		return errors.New("effort application: missing UUID allocator")
	case deps.GateCommand == nil:
		return errors.New("effort application: missing integration gate resolver")
	case marker == nil:
		return errors.New("effort application: missing archive marker renderer")
	default:
		return nil
	}
}

func (r Request) mutates() bool {
	switch r.Kind {
	case New, Finish, AddWorktree, RemoveWorktree, Integrate:
		return true
	default:
		return false
	}
}

type app struct {
	service *effort.Service
	manager *worktree.Manager
	gate    func(string) (string, error)
}

func open(ctx context.Context, root string, expectedArchiveMarker func() ([]byte, error), deps Dependencies) (*app, error) {
	roots, err := deps.ResolveRoots(ctx, root)
	if err != nil {
		return nil, err
	}
	checkout, err := deps.OpenCheckout(roots.InvokingRoot)
	if err != nil {
		return nil, err
	}
	service, err := effort.Open(roots, effort.Dependencies{
		Clock:                 deps.Clock,
		UUID:                  deps.UUID,
		Worktrees:             checkout.WorktreeList,
		BranchExists:          checkout.BranchExists,
		ValidateRef:           checkout.ValidateRefName,
		ExpectedArchiveMarker: expectedArchiveMarker,
	})
	if err != nil {
		return nil, err
	}
	openCheckout := func(checkoutRoot string) (worktree.Runner, error) {
		return deps.OpenCheckout(checkoutRoot)
	}
	openResident := func(name awfgit.ResidentName) (worktree.ResidentHandle, error) {
		residentRoot, rootErr := roots.ResidentRoot(name)
		if rootErr != nil {
			return nil, rootErr
		}
		return deps.OpenResident(residentRoot)
	}
	manager, err := worktree.Open(roots, openCheckout, openResident)
	if err != nil {
		return nil, err
	}
	return &app{service: service, manager: manager, gate: deps.GateCommand}, nil
}

func (a *app) execute(ctx context.Context, root string, request Request) (presentation.Document, error) {
	switch request.Kind {
	case New:
		record, result, err := a.newEffort(ctx, effort.NewInput{Slug: request.Slug, Title: request.Title}, request.Base)
		if err != nil {
			return presentation.Document{}, err
		}
		return newDocument(record, result)
	case List:
		records, err := a.service.List()
		if err != nil {
			return presentation.Document{}, err
		}
		return listDocument(records)
	case Show:
		record, err := a.service.Show(request.Slug)
		if err != nil {
			return presentation.Document{}, err
		}
		return detailDocument(record)
	case Finish:
		result, err := a.service.Finish(ctx, request.Slug)
		if err != nil {
			return presentation.Document{}, err
		}
		return finishDocument(result, request.Slug)
	case AddWorktree:
		return worktreeDocument(a.addWorktree(ctx, request.Slug, request.Base))
	case RemoveWorktree:
		if _, err := a.service.Show(request.Slug); err != nil {
			return presentation.Document{}, err
		}
		return worktreeDocument(a.manager.Remove(ctx, request.Slug))
	case Integrate:
		gate, err := a.gate(root)
		if err != nil {
			return presentation.Document{}, err
		}
		if _, err := a.service.Show(request.Slug); err != nil {
			return presentation.Document{}, err
		}
		return worktreeDocument(a.manager.Integrate(ctx, request.Slug, gate))
	default:
		return presentation.Document{}, errors.New("effort application: unknown request kind")
	}
}

func (a *app) addWorktree(ctx context.Context, slug, base string) (worktree.Result, error) {
	if _, err := a.service.Show(slug); err != nil {
		return worktree.Result{}, err
	}
	return a.manager.Add(ctx, slug, base)
}

func integrationGateCommand(root string) (string, error) {
	cfg, err := config.Load(config.RootDir(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	command, ok := cfg.Vars["gateCmd"].(string)
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(command), nil
}
