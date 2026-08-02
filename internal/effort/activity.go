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

// activityBeforePublish is a deterministic test seam. Correctness comes from
// identity-checked publication, not this process-local optimization.
var activityBeforePublish func()

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

type activityOperation string

const (
	activityResolve   activityOperation = "resolve"
	activityAttach    activityOperation = "attach"
	activityTakeover  activityOperation = "takeover"
	activityHeartbeat activityOperation = "heartbeat"
	activityCheckout  activityOperation = "checkout"
	activityDetach    activityOperation = "detach"
)

// refusalFor records only mutations that have already occurred at this protocol
// boundary. Attach and checkout are committed after Pi rebinds its live CWD;
// Phase 3 therefore consumes their ChangedCWD contract without asking Go to
// infer a runtime result.
func refusalFor(operation activityOperation, condition ActivityCondition, cause error, nextActions []string) ActivityReply {
	r := activityReply(condition)
	category := "operation"
	if condition == ActivityUnsafeResident {
		category = "repository-identity"
	}
	if condition == ActivityRepositoryMismatch {
		category = "topology"
	}
	if len(nextActions) == 0 {
		nextActions = []string{"inspect the effort resident and retry"}
	}
	r.Outcome = &ActionableOutcome{Category: category, Condition: string(condition), ChangedActivity: changedActivityForFailure(operation, cause), ChangedCWD: operation == activityAttach || operation == activityCheckout, NextActions: nextActions}
	if cause != nil {
		r.Outcome.Cause = cause.Error()
	}
	return r
}

func changedActivityForFailure(operation activityOperation, cause error) bool {
	if operation != activityAttach && operation != activityTakeover && operation != activityHeartbeat && operation != activityCheckout && operation != activityDetach {
		return false
	}
	var storage *ActivityStorageError
	return errors.As(cause, &storage) && storage.Stage == "directory-fsync"
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

type ActivityStorageError struct {
	Operation activityOperation
	Stage     string
	Err       error
}

func (e *ActivityStorageError) Error() string {
	return fmt.Sprintf("activity %s %s: %v", e.Operation, e.Stage, e.Err)
}
func (e *ActivityStorageError) Unwrap() error { return e.Err }

type ActivityPublicationRefusal struct {
	Operation activityOperation
	Err       error
}

func (e *ActivityPublicationRefusal) Error() string {
	return fmt.Sprintf("activity %s conditional-publication identity refusal: %v", e.Operation, e.Err)
}
func (e *ActivityPublicationRefusal) Unwrap() error { return e.Err }
func activityStorageFailure(operation activityOperation, stage string, err error) error {
	var identity *publicationIdentityError
	if errors.As(err, &identity) {
		return &ActivityPublicationRefusal{Operation: operation, Err: err}
	}
	return &ActivityStorageError{Operation: operation, Stage: stage, Err: err}
}
func (s store) replaceActivity(path string, a Activity, expected *fileIdentity, operation activityOperation) error {
	raw, err := json.Marshal(a)
	if err != nil { // coverage-ignore: Activity contains only JSON-native fields, so json.Marshal cannot fail
		return err
	}
	return s.replaceResidentExpected(path, raw, "activity", expected, operation)
}
func (s store) removeActivityExpected(path string, expected *fileIdentity) (returnErr error) {
	operation := activityDetach
	if err := s.hit("activity.remove"); err != nil {
		return activityStorageFailure(operation, "remove", err)
	}
	if expected == nil {
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return activityStorageFailure(operation, "remove", err)
		}
	} else {
		temp, err := os.CreateTemp(filepath.Dir(path), ".activity-remove-*.tmp")
		if err != nil { // coverage-ignore: an effort-owned directory accepts temp creation; failure requires a storage fault outside injectable activity stages
			return activityStorageFailure(operation, "remove", err)
		}
		tempPath := temp.Name()
		if err := temp.Close(); err != nil { // coverage-ignore: closing a newly created empty local temporary requires a storage fault
			return activityStorageFailure(operation, "remove", err)
		}
		defer func() { _ = os.Remove(tempPath) }()
		if err := removeAtomic(tempPath, path, expected); err != nil {
			return activityStorageFailure(operation, "remove", err)
		}
	}
	if err := s.hit("activity.directory-fsync"); err != nil {
		return activityStorageFailure(operation, "directory-fsync", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil { // coverage-ignore: the injected directory-fsync stage proves this boundary; a real sync failure requires a storage fault
		return activityStorageFailure(operation, "directory-fsync", err)
	}
	return nil
}
func (s store) replaceResidentExpected(path string, raw []byte, label string, expected *fileIdentity, operation activityOperation) (returnErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+label+"-*.tmp")
	if err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, activityStorageFailure(operation, "replace", temp.Close()))
		}
		if e := os.Remove(tempPath); e != nil && !errors.Is(e, os.ErrNotExist) { // coverage-ignore: the locally-created sibling can disappear, but a non-ENOENT removal failure requires a kernel or storage fault
			returnErr = errors.Join(returnErr, activityStorageFailure(operation, "replace", e))
		}
	}()
	if err = s.hit(label + ".write"); err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	if _, err = temp.Write(raw); err != nil { // coverage-ignore: fault stages cover the write boundary; a local temporary write failure requires a kernel or storage fault
		return activityStorageFailure(operation, "replace", err)
	}
	if err = s.hit(label + ".fsync"); err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	if err = temp.Sync(); err != nil { // coverage-ignore: fault stages cover the fsync boundary; a local temporary sync failure requires a kernel or storage fault
		return activityStorageFailure(operation, "replace", err)
	}
	if err = temp.Close(); err != nil { // coverage-ignore: a close failure after a successful local write requires a kernel or storage fault
		return activityStorageFailure(operation, "replace", err)
	}
	closed = true
	if err = s.hit(label + ".rename"); err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	if err = publishAtomic(tempPath, path, expected); err != nil {
		if expected == nil && errors.Is(err, os.ErrExist) {
			return activityStorageFailure(operation, "replace", publicationIdentityRefusal(err))
		}
		return activityStorageFailure(operation, "replace", err)
	}
	if err = s.hit(label + ".directory-fsync"); err != nil {
		return activityStorageFailure(operation, "directory-fsync", err)
	}
	if err := syncDirectory(dir); err != nil { // coverage-ignore: the injected directory-fsync stage proves this boundary; a real sync failure requires a storage fault
		return activityStorageFailure(operation, "directory-fsync", err)
	}
	return nil
}

func (s *Service) activityEffort(slug string, operation activityOperation) (*ActivityEffort, *MemoryMetadata, *ActivityReply) {
	r, err := s.store.loadDirectory(s.paths.effort(slug), slug, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			x := refusalFor(operation, ActivityMissing, nil, nil)
			return nil, nil, &x
		}
		x := refusalFor(operation, ActivityUnsafeResident, err, nil)
		return nil, nil, &x
	}
	raw, err := readRegularNoFollowBounded(s.paths.memoryFile(slug), maxMemoryBytes)
	if err != nil {
		x := invalidMemoryRefusal(operation, slug, r.Title, nil, err, true)
		return nil, nil, &x
	}
	m, _, err := readMemoryMetadata(raw, slug)
	if err != nil {
		x := invalidMemoryRefusal(operation, slug, r.Title, raw, err, false)
		return nil, nil, &x
	}
	return &ActivityEffort{slug, r.Title}, &m, nil
}

func invalidMemoryRefusal(operation activityOperation, slug, title string, raw []byte, err error, readFailure bool) ActivityReply {
	action := "repair .awf/efforts/" + slug + "/memory.md manually: preserve its body and restore a matching effort identity with a recognized canonical or legacy metadata boundary"
	if !readFailure {
		doc := inspectMemory(raw, slug)
		if doc.boundary && doc.identity == slug && (doc.invalid["phase"] || doc.invalid["next"] || doc.invalid["updated"]) {
			action = memoryRepairCommand(slug, doc)
		}
	}
	var cause error
	if readFailure {
		cause = err
	}
	r := refusalFor(operation, ActivityInvalidMemory, cause, []string{action})
	r.Effort = &ActivityEffort{slug, title}
	return r
}

// ResolveActivity validates the named destination but does not mutate an activity claim.
func (s *Service) ResolveActivity(ctx context.Context, slug string, role CheckoutRole, receiving string) ActivityReply {
	facts, err := s.resolveCheckout(ctx, s.paths.roots.InvokingRoot)
	if err != nil {
		return checkoutRefusalFor(activityResolve, err)
	}
	eff, m, bad := s.activityEffort(slug, activityResolve)
	if bad != nil {
		return *bad
	}
	if facts.PrimaryRoot != s.paths.roots.PrimaryRoot {
		return refusalFor(activityResolve, ActivityRepositoryMismatch, errors.New("invoking checkout belongs to another repository"), nil)
	}
	prior, err := s.activityCurrent(slug)
	if err != nil {
		return refusalFor(activityResolve, ActivityUnsafeResident, err, nil)
	}
	dest, err := s.destination(ctx, slug, role, receiving, prior, facts)
	if err != nil {
		return checkoutRefusalFor(activityResolve, err)
	}
	r := activityReply(ActivityReady)
	r.Effort = eff
	r.Memory = m
	r.Destination = &dest
	r.PriorClaim = prior
	return r
}
func (s *Service) destination(ctx context.Context, slug string, role CheckoutRole, explicitReceiving string, prior *Activity, facts CheckoutFacts) (ActivityDestination, error) {
	if role != CheckoutManaged && role != CheckoutReceiving {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("invalid checkout role"))
	}
	managed := filepath.Clean(s.paths.managedWorktree(slug))
	if explicitReceiving != "" && !validActivityPath(explicitReceiving) {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("explicit receiving checkout must be an absolute clean path"))
	}
	var receiving string
	switch {
	case prior != nil && prior.ReceivingCheckout != "":
		receiving = prior.ReceivingCheckout
	case explicitReceiving != "":
		receiving = explicitReceiving
	case facts.InvokingRoot != managed:
		receiving = facts.InvokingRoot
	default:
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("supply an explicit receiving checkout"))
	}
	if !validActivityPath(receiving) || filepath.Clean(receiving) == managed {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("receiving checkout must be an absolute clean checkout other than the managed worktree"))
	}
	rf, err := s.resolveCheckout(ctx, receiving)
	if err != nil {
		return ActivityDestination{}, err
	}
	if rf.PrimaryRoot != s.paths.roots.PrimaryRoot {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("receiving checkout belongs to another repository"))
	}
	if role == CheckoutReceiving {
		return ActivityDestination{CWD: rf.InvokingRoot, Role: role, ReceivingCheckout: rf.InvokingRoot}, nil
	}
	regs, err := s.worktrees(ctx)
	if err != nil {
		return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, err)
	}
	for _, r := range regs {
		if filepath.Clean(r.Path) == managed && r.Branch == "refs/heads/awf/"+slug {
			return ActivityDestination{CWD: managed, Role: role, ReceivingCheckout: rf.InvokingRoot}, nil
		}
	}
	return ActivityDestination{}, NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("managed checkout is not registered"))
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
func checkoutRefusalFor(operation activityOperation, err error) ActivityReply {
	var ce *CheckoutResolutionError
	if errors.As(err, &ce) {
		if ce.Kind() == CheckoutUnsafe {
			return refusalFor(operation, ActivityUnsafeResident, ce.Unwrap(), nil)
		}
		return refusalFor(operation, ActivityRepositoryMismatch, ce.Unwrap(), nil)
	}
	return refusalFor(operation, ActivityRepositoryMismatch, err, nil)
}
func (s *Service) activityCurrent(slug string) (*Activity, error) {
	a, _, err := s.activityCurrentIdentity(slug)
	return a, err
}
func (s *Service) activityCurrentIdentity(slug string) (*Activity, *fileIdentity, error) {
	raw, identity, err := readRegularNoFollowBoundedIdentity(s.paths.activityFile(slug), maxMemoryBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	} //nolint:nilnil // absent activity is documented state
	if err != nil { // coverage-ignore: a non-ENOENT failure after a bounded no-follow read requires a resident race or storage fault
		return nil, nil, err
	}
	var a Activity
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return nil, nil, err
	}
	if err := requireJSONEOF(dec); err != nil {
		return nil, nil, err
	}
	if err := validActivity(a); err != nil {
		return nil, nil, err
	}
	return &a, &identity, nil
}
func (s *Service) publicationRefusal(slug string, operation activityOperation, eff *ActivityEffort, err error) ActivityReply {
	var refusal *ActivityPublicationRefusal
	if !errors.As(err, &refusal) {
		return refusalFor(operation, ActivityUnsafeResident, err, nil)
	}
	current, readErr := s.activityCurrent(slug)
	if readErr != nil || current == nil { // coverage-ignore: a second change after a conditional-publication refusal requires another concurrent mutation
		return refusalFor(operation, ActivityUnsafeResident, err, nil)
	}
	out := refusalFor(operation, ActivityNotOwner, refusal, nil)
	out.Effort, out.Activity = eff, current
	return out
}

func (s *Service) AttachActivity(ctx context.Context, slug string, a Activity) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	eff, m, bad := s.activityEffort(slug, activityAttach)
	if bad != nil {
		return *bad
	}
	// The service owns persistence time; callers cannot backdate a new claim.
	// Populate both fields before validation so the command boundary supplies
	// only caller-owned identity and checkout facts.
	now := s.now().UTC()
	a.AttachedAt, a.HeartbeatAt = now, now
	if err := validActivity(a); err != nil {
		return refusalFor(activityAttach, ActivityRepositoryMismatch, err, nil)
	}
	if err := s.verifyActivityDestination(ctx, slug, a); err != nil {
		return checkoutRefusalFor(activityAttach, err)
	}
	prior, identity, err := s.activityCurrentIdentity(slug)
	if err != nil {
		return refusalFor(activityAttach, ActivityUnsafeResident, err, nil)
	}
	operation := activityAttach
	if prior != nil {
		operation = activityTakeover
	}
	if activityBeforePublish != nil {
		activityBeforePublish()
	}
	if err := s.store.replaceActivity(s.paths.activityFile(slug), a, identity, operation); err != nil {
		return s.publicationRefusal(slug, operation, eff, err)
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
	return s.mutateActivity(slug, owner, activityHeartbeat, ActivityHeartbeat, func(a *Activity) { a.HeartbeatAt = s.now().UTC() })
}
func (s *Service) CheckoutActivity(ctx context.Context, slug, owner, cwd string, role CheckoutRole) ActivityReply {
	// Revalidate the current claim before deciding the only legal destination.
	prior, err := s.activityCurrent(slug)
	if err != nil {
		return refusalFor(activityCheckout, ActivityUnsafeResident, err, nil)
	}
	if prior == nil {
		return refusalFor(activityCheckout, ActivityMissing, nil, nil)
	}
	candidate := *prior
	candidate.CWD, candidate.Role = cwd, role
	if err := s.verifyActivityDestination(ctx, slug, candidate); err != nil {
		return checkoutRefusalFor(activityCheckout, err)
	}
	return s.mutateActivity(slug, owner, activityCheckout, ActivityCheckoutUpdated, func(a *Activity) { a.CWD = cwd; a.Role = role; a.HeartbeatAt = s.now().UTC() })
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
	requestedCWD := filepath.Clean(a.CWD)
	cwd, err := s.resolveCheckout(ctx, requestedCWD)
	if err != nil {
		return err
	}
	if cwd.InvokingRoot != requestedCWD {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("resolved checkout does not match requested cwd"))
	}
	requestedReceiving := filepath.Clean(a.ReceivingCheckout)
	receiving, err := s.resolveCheckout(ctx, requestedReceiving)
	if err != nil {
		return err
	}
	if receiving.InvokingRoot != requestedReceiving {
		return NewCheckoutResolutionError(CheckoutRepositoryMismatch, errors.New("resolved checkout does not match requested receiving checkout"))
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
			return refusalFor(activityDetach, ActivityMissing, nil, nil)
		}
		return refusalFor(activityDetach, ActivityUnsafeResident, err, nil)
	}
	a, identity, err := s.activityCurrentIdentity(slug)
	if err != nil {
		return refusalFor(activityDetach, ActivityUnsafeResident, err, nil)
	}
	if a == nil {
		out := activityReply(ActivityDetached)
		out.Effort = &ActivityEffort{slug, r.Title}
		return out
	}
	if a.Owner != owner {
		out := refusalFor(activityDetach, ActivityNotOwner, nil, nil)
		out.Effort = &ActivityEffort{slug, r.Title}
		out.Activity = a
		return out
	}
	if activityBeforePublish != nil {
		activityBeforePublish()
	}
	if err := s.store.removeActivityExpected(s.paths.activityFile(slug), identity); err != nil {
		return s.publicationRefusal(slug, activityDetach, &ActivityEffort{slug, r.Title}, err)
	}
	out := activityReply(ActivityDetached)
	out.Effort = &ActivityEffort{slug, r.Title}
	return out
}
func (s *Service) mutateActivity(slug, owner string, operation activityOperation, condition ActivityCondition, change func(*Activity)) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	r := activityReply(condition)
	eff, m, bad := s.activityEffort(slug, operation)
	if bad != nil {
		return *bad
	}
	a, identity, err := s.activityCurrentIdentity(slug)
	if err != nil {
		return refusalFor(operation, ActivityUnsafeResident, err, nil)
	}
	if a == nil {
		return refusalFor(operation, ActivityMissing, nil, nil)
	}
	if a.Owner != owner {
		out := refusalFor(operation, ActivityNotOwner, nil, nil)
		out.Effort = eff
		out.Activity = a
		return out
	}
	change(a)
	if err := validActivity(*a); err != nil {
		return refusalFor(operation, ActivityRepositoryMismatch, err, nil)
	}
	if activityBeforePublish != nil {
		activityBeforePublish()
	}
	if err := s.store.replaceActivity(s.paths.activityFile(slug), *a, identity, operation); err != nil {
		return s.publicationRefusal(slug, operation, eff, err)
	}
	r.Effort = eff
	r.Memory = m
	r.Activity = a
	return r
}
