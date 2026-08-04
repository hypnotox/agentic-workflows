package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/effort"
	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
	"github.com/hypnotox/agentic-workflows/internal/presentation"
)

type RefusalError struct {
	Category        string
	Condition       string
	ChangedTopology bool
	NextAction      string
	Err             error
}

func (e *RefusalError) Error() string {
	message := fmt.Sprintf("worktree refusal (%s): %s; changed topology: %s; next action: %s", e.Category, e.Condition, yesNo(e.ChangedTopology), e.NextAction)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *RefusalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Diagnostic maps this topology-owned refusal without parsing its Error text.
func (e *RefusalError) Diagnostic() (presentation.Diagnostic, error) {
	changed, err := presentation.Prose(yesNo(e.ChangedTopology))
	if err != nil { // coverage-ignore: yesNo always supplies a nonempty topology fact
		return presentation.Diagnostic{}, err
	}
	axis, err := presentation.NewField("managed topology", changed)
	if err != nil { // coverage-ignore: fixed grammar-valid diagnostic label and validated value
		return presentation.Diagnostic{}, err
	}
	step, err := presentation.Prose(e.NextAction)
	if err != nil { // coverage-ignore: every refusal constructor supplies a nonempty next action
		return presentation.Diagnostic{}, err
	}
	diagnostic := presentation.Diagnostic{
		Condition: e.Condition,
		State:     e.Category,
		Changed:   []presentation.Field{axis},
		Steps:     []presentation.Value{step},
	}
	if e.Err != nil {
		diagnostic.Cause = e.Err.Error()
	}
	return diagnostic, nil
}

// CreationError describes a failed managed creation after its compensating
// finish attempt. It retains the legacy error text while exposing every changed
// axis and recovery sequence to the command presentation boundary.
type CreationError struct {
	Message         string
	Condition       string
	ChangedEffort   bool
	ChangedTopology bool
	Cause           error
	Steps           []string
}

func (e *CreationError) Error() string { return e.Message }
func (e *CreationError) Unwrap() error { return e.Cause }

func (e *CreationError) Diagnostic() (presentation.Diagnostic, error) {
	changed := make([]presentation.Field, 0, 2)
	for _, fact := range []struct {
		label string
		value bool
	}{{"effort resident", e.ChangedEffort}, {"managed topology", e.ChangedTopology}} {
		value, err := presentation.Prose(yesNo(fact.value))
		if err != nil { // coverage-ignore: yesNo always supplies a nonempty changed-axis value
			return presentation.Diagnostic{}, err
		}
		field, err := presentation.NewField(fact.label, value)
		if err != nil { // coverage-ignore: fixed grammar-valid diagnostic label and validated value
			return presentation.Diagnostic{}, err
		}
		changed = append(changed, field)
	}
	steps := make([]presentation.Value, 0, len(e.Steps))
	for _, text := range e.Steps {
		step, err := presentation.Prose(text)
		if err != nil { // coverage-ignore: manager-owned recovery sequences contain fixed nonempty steps
			return presentation.Diagnostic{}, err
		}
		steps = append(steps, step)
	}
	return presentation.Diagnostic{Condition: e.Condition, State: "creation", Changed: changed, Cause: e.Cause.Error(), Steps: steps}, nil
}

func refusal(category, condition string, changed bool, next string) error {
	return &RefusalError{Category: category, Condition: condition, ChangedTopology: changed, NextAction: next}
}

func refusalCause(category, condition string, changed bool, next string, err error) error {
	return &RefusalError{Category: category, Condition: condition, ChangedTopology: changed, NextAction: next, Err: err}
}

// exactRegistration proves that path is registered exactly once, on branch, and
// as an ordinary attached checkout. Registration parsing itself belongs to the
// Git seam, so this reads the seam's registrations rather than porcelain bytes.
func exactRegistration(ctx context.Context, checkout Runner, path, branch string) error {
	regs, err := checkout.WorktreeList(ctx)
	if err != nil {
		return err
	}
	var found []awfgit.WorktreeRegistration
	for _, r := range regs {
		if filepath.Clean(r.Path) == filepath.Clean(path) {
			found = append(found, r)
		}
		if r.Branch == branch && filepath.Clean(r.Path) != filepath.Clean(path) {
			return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed branch is registered elsewhere")}
		}
	}
	if len(found) != 1 {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed path is not uniquely registered")}
	}
	r := found[0]
	if r.Branch != branch || r.Detached || r.Bare {
		return &awfgit.HardSafetyError{Category: "repository-identity", Path: path, Err: errors.New("managed registration branch mismatch")}
	}
	return nil
}

var managedLstat = os.Lstat
var managedOwner = effort.ValidateCurrentOwner

func safeManagedPath(path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(strings.TrimPrefix(clean, volume), string(filepath.Separator))
	current := volume + string(filepath.Separator)
	var info os.FileInfo
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		componentInfo, componentErr := managedLstat(current)
		if componentErr != nil {
			return componentErr
		}
		info = componentInfo
		if componentInfo.Mode()&os.ModeSymlink != 0 {
			return &awfgit.HardSafetyError{Category: "symlink", Path: current}
		}
	}
	if info == nil {
		return errors.New("managed path has no components")
	}
	if !info.IsDir() {
		return &awfgit.HardSafetyError{Category: "file-type", Path: path}
	}
	// Ancestors outside the checkout are not manager-owned resident
	// components. ResolveControlRoots already rejects symlinked identity paths,
	// while ResidentRoot separately validates ownership from the primary
	// checkout through resident paths. Require current ownership only for the
	// live checkout or managed target passed here.
	if err := managedOwner(clean, info); err != nil {
		return err
	}
	return nil
}
