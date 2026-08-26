// Package effortop coordinates resolved effort use cases over effort residents and managed worktrees.
package effortop

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/effort"
	"github.com/hypnotox/agentic-workflows/internal/filesystem"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
	"github.com/hypnotox/agentic-workflows/internal/worktree"
)

// AcquireMutationLease is the sole effort application-composition policy for
// resident and dual-root mutation transactions. Command code supplies only the
// parsed operation shape and holds the returned capability through rendering.
func AcquireMutationLease(ctx context.Context, root, subcommand string, noWorktree bool) (*filesystem.Lease, error) {
	if !mutates(subcommand) {
		return nil, nil
	}
	residentRoot := awfgit.ProjectResidentRoot(ctx, root)
	if needsProjectLease(subcommand, noWorktree) {
		return filesystem.AcquireProjectLease(ctx, root, residentRoot)
	}
	return filesystem.AcquireResidentLease(ctx, residentRoot)
}

func mutates(subcommand string) bool {
	switch subcommand {
	case "new", "finish", "worktree", "integrate", "memory edit", "memory update", "activity attach", "activity heartbeat", "activity detach":
		return true
	default:
		return false
	}
}

func needsProjectLease(subcommand string, noWorktree bool) bool {
	switch subcommand {
	case "finish", "worktree", "integrate":
		return true
	case "new":
		return !noWorktree
	default:
		return false
	}
}

// New creates an effort resident and, unless excluded, its managed worktree.
func New(ctx context.Context, service *effort.Service, manager *worktree.Manager, input effort.NewInput, base string, noWorktree bool) (presentation.Document, error) {
	if noWorktree {
		record, err := service.New(ctx, input)
		if err != nil {
			return presentation.Document{}, err
		}
		absent := worktree.Result{Condition: "no managed worktree", NextAction: "continue the effort in " + service.InvokingRoot()}
		return newDocument(record, absent)
	}
	record, result, err := manager.NewEffort(ctx, input, base)
	if err != nil {
		return presentation.Document{}, err
	}
	return newDocument(record, result)
}

// List returns the readable active-effort listing.
func List(service *effort.Service) (presentation.Document, error) {
	records, err := service.List()
	if err != nil {
		return presentation.Document{}, err
	}
	return effort.ListDocument(records)
}

// Show returns one effort's readable detail.
func Show(service *effort.Service, slug string) (presentation.Document, error) {
	record, err := service.Show(slug)
	if err != nil {
		return presentation.Document{}, err
	}
	detail, err := record.Detail()
	if err != nil { // coverage-ignore: production composition resolves control roots first, rejecting line breaks before absolute resident paths can reach presentation
		return presentation.Document{}, err
	}
	return detail.Document()
}

// Finish finishes an effort resident and returns its readable mutation result.
func Finish(ctx context.Context, service *effort.Service, slug string) (presentation.Document, error) {
	result, err := service.Finish(ctx, slug)
	if err != nil {
		return presentation.Document{}, err
	}
	mutation, err := result.FinishMutation(slug)
	if err != nil { // coverage-ignore: production composition resolves control roots first, rejecting line breaks before absolute archive paths can reach presentation
		return presentation.Document{}, err
	}
	return mutation.Document()
}

// AddWorktree adds the managed worktree for an effort.
func AddWorktree(ctx context.Context, manager *worktree.Manager, slug, base string) (presentation.Document, error) {
	return worktreeDocument(manager.Add(ctx, slug, base))
}

// RemoveWorktree removes the managed worktree for an effort.
func RemoveWorktree(ctx context.Context, manager *worktree.Manager, slug string) (presentation.Document, error) {
	return worktreeDocument(manager.Remove(ctx, slug))
}

// Integrate verifies and fast-forwards an effort branch into its base branch.
func Integrate(ctx context.Context, root string, manager *worktree.Manager, slug string) (presentation.Document, error) {
	gate, err := integrationGateCommand(root)
	if err != nil {
		return presentation.Document{}, err
	}
	return worktreeDocument(manager.Integrate(ctx, slug, gate))
}

// ReadMemory performs one bounded memory read.
func ReadMemory(service *effort.Service, input effort.MemoryReadInput) (effort.MemoryOperationResult, error) {
	return service.Memory(input)
}

// EditMemory performs one exact memory edit or preview.
func EditMemory(service *effort.Service, input effort.MemoryEditInput) (effort.MemoryOperationResult, error) {
	return service.Memory(input)
}

// UpdateMemory updates effort memory metadata or previews the update.
func UpdateMemory(service *effort.Service, input effort.MemoryUpdateInput) (effort.MemoryOperationResult, error) {
	return service.UpdateMemory(input)
}

// AttachActivity attaches one bounded activity owner.
func AttachActivity(service *effort.Service, slug, owner string) effort.ActivityReply {
	return service.AttachActivity(slug, owner)
}

// HeartbeatActivity refreshes one bounded activity owner.
func HeartbeatActivity(service *effort.Service, slug, owner string) effort.ActivityReply {
	return service.HeartbeatActivity(slug, owner)
}

// DetachActivity detaches one bounded activity owner.
func DetachActivity(service *effort.Service, slug, owner string) effort.ActivityReply {
	return service.DetachActivity(slug, owner)
}

func newDocument(record effort.Record, result worktree.Result) (presentation.Document, error) {
	mutation, err := result.Mutation()
	if err != nil {
		return presentation.Document{}, err
	}
	mutation, err = record.NewEffortMutation(mutation)
	if err != nil {
		return presentation.Document{}, err
	}
	return mutation.Document()
}

func worktreeDocument(result worktree.Result, err error) (presentation.Document, error) {
	if err != nil {
		return presentation.Document{}, err
	}
	mutation, err := result.Mutation()
	if err != nil {
		return presentation.Document{}, err
	}
	return mutation.Document()
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
