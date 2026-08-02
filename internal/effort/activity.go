package effort

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const activitySchemaVersion = 1

// Activity mutations are serialized per resident in this process. Every
// owner-checked operation re-reads the resident while holding this guard, so a
// queued old owner cannot overwrite or remove a successor claim.
var activityLocks sync.Map // map[string]*sync.Mutex

func lockActivity(path string) func() {
	value, _ := activityLocks.LoadOrStore(path, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

type CheckoutRole string

const (
	CheckoutManaged   CheckoutRole = "managed"
	CheckoutReceiving CheckoutRole = "receiving"
)

// CheckoutFacts is the deliberately small result of resolving a checkout.
type CheckoutFacts struct {
	InvokingRoot string
	PrimaryRoot  string
}

type CheckoutResolutionKind string

const (
	CheckoutUnsafe             CheckoutResolutionKind = "unsafe"
	CheckoutRepositoryMismatch CheckoutResolutionKind = "repository-mismatch"
)

type CheckoutResolutionError struct {
	kind  CheckoutResolutionKind
	cause error
}

func NewCheckoutResolutionError(kind CheckoutResolutionKind, cause error) *CheckoutResolutionError {
	if kind != CheckoutUnsafe && kind != CheckoutRepositoryMismatch {
		panic("effort CheckoutResolutionError: invalid kind")
	}
	if cause == nil {
		panic("effort CheckoutResolutionError: missing cause")
	}
	return &CheckoutResolutionError{kind: kind, cause: cause}
}
func (e *CheckoutResolutionError) Kind() CheckoutResolutionKind { return e.kind }
func (e *CheckoutResolutionError) Error() string {
	return fmt.Sprintf("checkout resolution %s: %v", e.kind, e.cause)
}
func (e *CheckoutResolutionError) Unwrap() error { return e.cause }

// Activity is the optional protocol-1 advisory claim.
type Activity struct {
	SchemaVersion     int          `json:"schemaVersion"`
	Owner             string       `json:"owner"`
	AttachedAt        time.Time    `json:"attachedAt"`
	HeartbeatAt       time.Time    `json:"heartbeatAt"`
	CWD               string       `json:"cwd"`
	ReceivingCheckout string       `json:"receivingCheckout"`
	Role              CheckoutRole `json:"role"`
}
type ActivityCondition string

const (
	ActivityReady              ActivityCondition = "ready"
	ActivityAttached           ActivityCondition = "attached"
	ActivityTakenOver          ActivityCondition = "taken-over"
	ActivityHeartbeat          ActivityCondition = "heartbeat"
	ActivityCheckoutUpdated    ActivityCondition = "checkout-updated"
	ActivityDetached           ActivityCondition = "detached"
	ActivityNotOwner           ActivityCondition = "not-owner"
	ActivityMissing            ActivityCondition = "missing"
	ActivityInvalidMemory      ActivityCondition = "invalid-memory"
	ActivityUnsafeResident     ActivityCondition = "unsafe-resident"
	ActivityRepositoryMismatch ActivityCondition = "repository-mismatch"
)

type ActivityReply struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Condition     ActivityCondition    `json:"condition"`
	Effort        *ActivityEffort      `json:"effort,omitempty"`
	Memory        *MemoryMetadata      `json:"memory,omitempty"`
	Destination   *ActivityDestination `json:"destination,omitempty"`
	Activity      *Activity            `json:"activity,omitempty"`
	PriorClaim    *Activity            `json:"priorClaim,omitempty"`
	Outcome       *ActionableOutcome   `json:"outcome,omitempty"`
}
type ActivityEffort struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}
type ActivityDestination struct {
	CWD               string       `json:"cwd"`
	Role              CheckoutRole `json:"role"`
	ReceivingCheckout string       `json:"receivingCheckout"`
}
type ActionableOutcome struct {
	Category        string   `json:"category"`
	Condition       string   `json:"condition"`
	ChangedActivity bool     `json:"changedActivity"`
	ChangedMemory   bool     `json:"changedMemory"`
	ChangedCWD      bool     `json:"changedCwd"`
	NextActions     []string `json:"nextActions"`
	Cause           string   `json:"cause,omitempty"`
}

func activityReply(condition ActivityCondition) ActivityReply {
	return ActivityReply{SchemaVersion: activitySchemaVersion, Condition: condition}
}
func refusal(condition ActivityCondition, cause error) ActivityReply {
	r := activityReply(condition)
	category := "operation"
	if condition == ActivityUnsafeResident {
		category = "repository-identity"
	}
	if condition == ActivityRepositoryMismatch {
		category = "topology"
	}
	r.Outcome = &ActionableOutcome{Category: category, Condition: string(condition), NextActions: []string{"inspect the effort resident and retry"}}
	if cause != nil {
		r.Outcome.Cause = cause.Error()
	}
	return r
}

func validActivity(a Activity) error {
	if a.SchemaVersion != activitySchemaVersion || !uuidV4Pattern.MatchString(a.Owner) || !validActivityTime(a.AttachedAt) || !validActivityTime(a.HeartbeatAt) || !validActivityPath(a.CWD) || !validActivityPath(a.ReceivingCheckout) || (a.Role != CheckoutManaged && a.Role != CheckoutReceiving) {
		return errors.New("invalid protocol-1 activity")
	}
	return nil
}
func validActivityTime(v time.Time) bool {
	return !v.IsZero() && v.Location() == time.UTC && strings.HasSuffix(v.Format(time.RFC3339Nano), "Z")
}
func validActivityPath(v string) bool { return v != "" && filepath.IsAbs(v) && filepath.Clean(v) == v }

func (s store) readActivity(path string) (*Activity, error) {
	raw, err := readRegularNoFollowBounded(path, maxMemoryBytes)
	if err != nil {
		return nil, err
	}
	var a Activity
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(dec); err != nil {
		return nil, err
	}
	if err := validActivity(a); err != nil {
		return nil, err
	}
	return &a, nil
}
func (s store) replaceActivity(path string, a Activity) error {
	raw, err := json.Marshal(a)
	if err != nil { // coverage-ignore: Activity contains only JSON-native fields, so json.Marshal cannot fail
		return err
	}
	return s.replaceResident(path, raw, "activity")
}
func (s store) removeActivity(path string) error {
	if err := s.hit("activity.remove"); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := s.hit("activity.directory-fsync"); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func (s store) replaceResident(path string, raw []byte, label string) (returnErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+label+"-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, temp.Close())
		}
		if !published {
			if e := os.Remove(tempPath); e != nil && !errors.Is(e, os.ErrNotExist) { // coverage-ignore: the locally-created sibling can disappear, but a non-ENOENT removal failure requires a kernel or storage fault
				returnErr = errors.Join(returnErr, e)
			}
		}
	}()
	if err = s.hit(label + ".write"); err != nil {
		return err
	}
	if _, err = temp.Write(raw); err != nil { // coverage-ignore: fault stages cover the write boundary; a local temporary write failure requires a kernel or storage fault
		return err
	}
	if err = s.hit(label + ".fsync"); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil { // coverage-ignore: fault stages cover the fsync boundary; a local temporary sync failure requires a kernel or storage fault
		return err
	}
	if err = temp.Close(); err != nil { // coverage-ignore: a close failure after a successful local write requires a kernel or storage fault
		return err
	}
	closed = true
	if err = s.hit(label + ".rename"); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	published = true
	if err = s.hit(label + ".directory-fsync"); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func (s *Service) activityEffort(slug string) (*ActivityEffort, *MemoryMetadata, *ActivityReply) {
	r, err := s.store.loadDirectory(s.paths.effort(slug), slug, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			x := refusal(ActivityMissing, nil)
			return nil, nil, &x
		}
		x := refusal(ActivityUnsafeResident, err)
		return nil, nil, &x
	}
	raw, err := readRegularNoFollowBounded(s.paths.memoryFile(slug), maxMemoryBytes)
	if err != nil {
		x := refusal(ActivityInvalidMemory, err)
		x.Effort = &ActivityEffort{slug, r.Title}
		return nil, nil, &x
	}
	m, _, err := readMemoryMetadata(raw, slug)
	if err != nil {
		x := refusal(ActivityInvalidMemory, err)
		x.Effort = &ActivityEffort{slug, r.Title}
		return nil, nil, &x
	}
	return &ActivityEffort{slug, r.Title}, &m, nil
}

// ResolveActivity validates the named destination but does not mutate an activity claim.
func (s *Service) ResolveActivity(ctx context.Context, slug string, role CheckoutRole, receiving string) ActivityReply {
	facts, err := s.resolveCheckout(ctx, s.paths.roots.InvokingRoot)
	if err != nil {
		return checkoutRefusal(err)
	}
	eff, m, bad := s.activityEffort(slug)
	if bad != nil {
		return *bad
	}
	if facts.PrimaryRoot != s.paths.roots.PrimaryRoot {
		return refusal(ActivityRepositoryMismatch, errors.New("invoking checkout belongs to another repository"))
	}
	prior, err := s.activityCurrent(slug)
	if err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	dest, err := s.destination(ctx, slug, role, receiving, prior, facts)
	if err != nil {
		return checkoutRefusal(err)
	}
	r := activityReply(ActivityReady)
	r.Effort = eff
	r.Memory = m
	r.Destination = &dest
	r.PriorClaim = prior
	return r
}
func (s *Service) destination(ctx context.Context, slug string, role CheckoutRole, receiving string, prior *Activity, facts CheckoutFacts) (ActivityDestination, error) {
	if role != CheckoutManaged && role != CheckoutReceiving {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("invalid checkout role"))
	}
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	receiving = filepath.Clean(receiving)
	if role == CheckoutManaged {
		regs, err := s.worktrees(ctx)
		if err != nil {
			return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, err)
		}
		found := false
		for _, r := range regs {
			if filepath.Clean(r.Path) == managed && r.Branch == "refs/heads/awf/"+slug {
				found = true
			}
		}
		if !found {
			return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("managed checkout is not registered"))
		}
		return ActivityDestination{CWD: managed, Role: role, ReceivingCheckout: chooseReceiving(prior, receiving)}, nil
	}
	if receiving == "." {
		receiving = ""
	}
	if receiving == "" && prior != nil {
		receiving = prior.ReceivingCheckout
	}
	if receiving == "" && filepath.Clean(facts.InvokingRoot) != managed {
		receiving = facts.InvokingRoot
	}
	if receiving == "" {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("supply an explicit receiving checkout"))
	}
	rf, err := s.resolveCheckout(ctx, receiving)
	if err != nil {
		return ActivityDestination{}, err
	}
	if rf.PrimaryRoot != s.paths.roots.PrimaryRoot {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("receiving checkout belongs to another repository"))
	}
	return ActivityDestination{CWD: rf.InvokingRoot, Role: role, ReceivingCheckout: rf.InvokingRoot}, nil
}
func chooseReceiving(prior *Activity, supplied string) string {
	if supplied != "" {
		return supplied
	}
	if prior != nil {
		return prior.ReceivingCheckout
	}
	return ""
}
func (s *Service) resolveCheckout(ctx context.Context, path string) (CheckoutFacts, error) {
	if s.checkoutResolver == nil {
		return CheckoutFacts{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("missing checkout resolution dependency"))
	}
	facts, err := s.checkoutResolver(ctx, path)
	if err != nil {
		return CheckoutFacts{}, err
	}
	if !validActivityPath(facts.InvokingRoot) || !validActivityPath(facts.PrimaryRoot) {
		return CheckoutFacts{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("checkout resolver returned non-canonical roots"))
	}
	return facts, nil
}
func checkoutRefusal(err error) ActivityReply {
	var ce *CheckoutResolutionError
	if errors.As(err, &ce) {
		if ce.Kind() == CheckoutUnsafe {
			return refusal(ActivityUnsafeResident, ce.Unwrap())
		}
		return refusal(ActivityRepositoryMismatch, ce.Unwrap())
	}
	return refusal(ActivityRepositoryMismatch, err)
}
func (s *Service) activityCurrent(slug string) (*Activity, error) {
	a, err := s.store.readActivity(s.paths.activityFile(slug))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // no activity resident is the documented non-error state
	}
	return a, err
}

func (s *Service) AttachActivity(ctx context.Context, slug string, a Activity) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	eff, m, bad := s.activityEffort(slug)
	if bad != nil {
		return *bad
	}
	// The service owns persistence time; callers cannot backdate a new claim.
	// Populate both fields before validation so the command boundary supplies
	// only caller-owned identity and checkout facts.
	now := s.now().UTC()
	a.AttachedAt, a.HeartbeatAt = now, now
	if err := validActivity(a); err != nil {
		return refusal(ActivityRepositoryMismatch, err)
	}
	if err := s.verifyActivityDestination(ctx, slug, a); err != nil {
		return checkoutRefusal(err)
	}
	prior, err := s.activityCurrent(slug)
	if err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	if err := s.store.replaceActivity(s.paths.activityFile(slug), a); err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	r := activityReply(ActivityAttached)
	if prior != nil {
		r.Condition = ActivityTakenOver
		r.PriorClaim = prior
	}
	r.Effort = eff
	r.Memory = m
	r.Activity = &a
	return r
}
func (s *Service) HeartbeatActivity(slug, owner string) ActivityReply {
	return s.mutateActivity(slug, owner, ActivityHeartbeat, func(a *Activity) { a.HeartbeatAt = s.now().UTC() })
}
func (s *Service) CheckoutActivity(ctx context.Context, slug, owner, cwd string, role CheckoutRole) ActivityReply {
	// Revalidate the current claim before deciding the only legal destination.
	prior, err := s.activityCurrent(slug)
	if err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	if prior == nil {
		return refusal(ActivityMissing, nil)
	}
	candidate := *prior
	candidate.CWD, candidate.Role = cwd, role
	if err := s.verifyActivityDestination(ctx, slug, candidate); err != nil {
		return checkoutRefusal(err)
	}
	return s.mutateActivity(slug, owner, ActivityCheckoutUpdated, func(a *Activity) { a.CWD = cwd; a.Role = role; a.HeartbeatAt = s.now().UTC() })
}

// verifyActivityDestination is the last policy check before an activity write.
// It proves both paths resolve to this service's repository and that a managed
// role names exactly the native registered worktree, never an inferred path.
func (s *Service) verifyActivityDestination(ctx context.Context, slug string, a Activity) error {
	if a.Role != CheckoutManaged && a.Role != CheckoutReceiving {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("invalid checkout role"))
	}
	invoking, err := s.resolveCheckout(ctx, s.paths.roots.InvokingRoot)
	if err != nil {
		return err
	}
	cwd, err := s.resolveCheckout(ctx, a.CWD)
	if err != nil {
		return err
	}
	receiving, err := s.resolveCheckout(ctx, a.ReceivingCheckout)
	if err != nil {
		return err
	}
	if invoking.PrimaryRoot != s.paths.roots.PrimaryRoot || cwd.PrimaryRoot != s.paths.roots.PrimaryRoot || receiving.PrimaryRoot != s.paths.roots.PrimaryRoot {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("checkout belongs to another repository"))
	}
	if a.Role == CheckoutReceiving {
		if a.CWD != a.ReceivingCheckout {
			return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("receiving activity cwd does not match receiving checkout"))
		}
		return nil
	}
	if a.CWD != filepath.Clean(s.paths.managedWorktree(slug)) {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("managed activity cwd is not the effort worktree"))
	}
	registrations, err := s.worktrees(ctx)
	if err != nil {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, err)
	}
	for _, registration := range registrations {
		if filepath.Clean(registration.Path) == a.CWD && registration.Branch == "refs/heads/awf/"+slug {
			return nil
		}
	}
	return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("managed checkout is not registered"))
}
func (s *Service) DetachActivity(slug, owner string) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	r, err := s.store.loadDirectory(s.paths.effort(slug), slug, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return refusal(ActivityMissing, nil)
		}
		return refusal(ActivityUnsafeResident, err)
	}
	a, err := s.activityCurrent(slug)
	if err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	if a == nil {
		out := activityReply(ActivityDetached)
		out.Effort = &ActivityEffort{slug, r.Title}
		return out
	}
	if a.Owner != owner {
		out := refusal(ActivityNotOwner, nil)
		out.Effort = &ActivityEffort{slug, r.Title}
		out.Activity = a
		return out
	}
	if err := s.store.removeActivity(s.paths.activityFile(slug)); err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	out := activityReply(ActivityDetached)
	out.Effort = &ActivityEffort{slug, r.Title}
	return out
}
func (s *Service) mutateActivity(slug, owner string, condition ActivityCondition, change func(*Activity)) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	r := activityReply(condition)
	eff, m, bad := s.activityEffort(slug)
	if bad != nil {
		return *bad
	}
	a, err := s.activityCurrent(slug)
	if err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	if a == nil {
		return refusal(ActivityMissing, nil)
	}
	if a.Owner != owner {
		out := refusal(ActivityNotOwner, nil)
		out.Effort = eff
		out.Activity = a
		return out
	}
	change(a)
	if err := validActivity(*a); err != nil {
		return refusal(ActivityRepositoryMismatch, err)
	}
	if err := s.store.replaceActivity(s.paths.activityFile(slug), *a); err != nil {
		return refusal(ActivityUnsafeResident, err)
	}
	r.Effort = eff
	r.Memory = m
	r.Activity = a
	return r
}
