package effort

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const activitySchemaVersion = 2

var activityLocks sync.Map // map[string]*sync.Mutex
var activityBeforePublish func()

func lockActivity(path string) func() {
	value, _ := activityLocks.LoadOrStore(path, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

// Activity is the optional protocol-v2 advisory claim.
type Activity struct {
	SchemaVersion int       `json:"schemaVersion"`
	Owner         string    `json:"owner"`
	AttachedAt    time.Time `json:"attachedAt"`
	HeartbeatAt   time.Time `json:"heartbeatAt"`
}
type ActivityCondition string

const (
	ActivityAttached       ActivityCondition = "attached"
	ActivityTakenOver      ActivityCondition = "taken-over"
	ActivityHeartbeat      ActivityCondition = "heartbeat"
	ActivityDetached       ActivityCondition = "detached"
	ActivityNotOwner       ActivityCondition = "not-owner"
	ActivityMissing        ActivityCondition = "missing"
	ActivityInvalidMemory  ActivityCondition = "invalid-memory"
	ActivityUnsafeResident ActivityCondition = "unsafe-resident"
)

type ActivityReply struct {
	SchemaVersion int                `json:"schemaVersion"`
	Condition     ActivityCondition  `json:"condition"`
	Effort        *ActivityEffort    `json:"effort,omitempty"`
	Memory        *MemoryMetadata    `json:"memory,omitempty"`
	Activity      *Activity          `json:"activity,omitempty"`
	Outcome       *ActionableOutcome `json:"outcome,omitempty"`
}
type ActivityEffort struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}
type ActionableOutcome struct {
	Category        string   `json:"category"`
	Condition       string   `json:"condition"`
	ChangedActivity bool     `json:"changedActivity"`
	NextActions     []string `json:"nextActions"`
	Cause           string   `json:"cause,omitempty"`
}

func activityReply(condition ActivityCondition) ActivityReply {
	return ActivityReply{SchemaVersion: activitySchemaVersion, Condition: condition}
}

type activityOperation string

const (
	activityAttach    activityOperation = "attach"
	activityTakeover  activityOperation = "takeover"
	activityHeartbeat activityOperation = "heartbeat"
	activityDetach    activityOperation = "detach"
)

func refusalFor(operation activityOperation, condition ActivityCondition, cause error, next []string) ActivityReply {
	if len(next) == 0 {
		next = []string{"inspect the effort resident and retry"}
	}
	r := activityReply(condition)
	r.Outcome = &ActionableOutcome{Category: "operation", Condition: string(condition), ChangedActivity: changedActivityForFailure(operation, cause), NextActions: next}
	if cause != nil {
		r.Outcome.Cause = cause.Error()
	}
	return r
}
func changedActivityForFailure(operation activityOperation, cause error) bool {
	var storage *activityStorageError
	return (operation == activityAttach || operation == activityTakeover || operation == activityHeartbeat || operation == activityDetach) && errors.As(cause, &storage) && storage.Stage == "directory-fsync"
}
func validActivity(a Activity) error {
	if a.SchemaVersion != activitySchemaVersion || !uuidV4Pattern.MatchString(a.Owner) || !validActivityTime(a.AttachedAt) || !validActivityTime(a.HeartbeatAt) {
		return errors.New("invalid protocol-2 activity")
	}
	return nil
}
func validActivityTime(v time.Time) bool {
	return !v.IsZero() && v.Location() == time.UTC && strings.HasSuffix(v.Format(time.RFC3339Nano), "Z")
}

type activityStorageError struct {
	Operation activityOperation
	Stage     string
	Err       error
}

func (e *activityStorageError) Error() string {
	return fmt.Sprintf("activity %s %s: %v", e.Operation, e.Stage, e.Err)
}
func (e *activityStorageError) Unwrap() error { return e.Err }

type activityPublicationRefusal struct {
	Operation activityOperation
	Err       error
}

func (e *activityPublicationRefusal) Error() string {
	return fmt.Sprintf("activity %s conditional-publication identity refusal: %v", e.Operation, e.Err)
}
func (e *activityPublicationRefusal) Unwrap() error { return e.Err }
func activityStorageFailure(operation activityOperation, stage string, err error) error {
	var identity *publicationIdentityError
	if errors.As(err, &identity) {
		return &activityPublicationRefusal{operation, err}
	}
	return &activityStorageError{operation, stage, err}
}
func (s store) replaceActivity(path string, a Activity, expected *fileIdentity, operation activityOperation) error {
	raw, err := json.Marshal(a)
	if err != nil { // coverage-ignore: Activity contains only JSON-native fields whose MarshalJSON methods cannot fail.
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
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return activityStorageFailure(operation, "remove", err)
		}
	} else {
		temp, err := os.CreateTemp(filepath.Dir(path), ".activity-remove-*.tmp")
		if err != nil { // coverage-ignore: the validated activity resident has a usable parent; failure requires a concurrent storage fault.
			return activityStorageFailure(operation, "remove", err)
		}
		tempPath := temp.Name()
		if err = temp.Close(); err != nil { // coverage-ignore: a newly created local temporary file cannot fail Close without a storage fault.
			return activityStorageFailure(operation, "remove", err)
		}
		defer os.Remove(tempPath)
		if err = removeAtomic(tempPath, path, expected); err != nil {
			return activityStorageFailure(operation, "remove", err)
		}
	}
	if err := s.hit("activity.directory-fsync"); err != nil {
		return activityStorageFailure(operation, "directory-fsync", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil { // coverage-ignore: injected directory-fsync faults cover this durability boundary; a real failure requires storage fault.
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
		if e := os.Remove(tempPath); e != nil && !errors.Is(e, os.ErrNotExist) { // coverage-ignore: cleanup of a local temporary can only fail through a storage fault.
			returnErr = errors.Join(returnErr, activityStorageFailure(operation, "replace", e))
		}
	}()
	if err = s.hit(label + ".write"); err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	if _, err = temp.Write(raw); err != nil { // coverage-ignore: injected write faults cover publication; a local temporary write failure requires storage fault.
		return activityStorageFailure(operation, "replace", err)
	}
	if err = s.hit(label + ".fsync"); err != nil {
		return activityStorageFailure(operation, "replace", err)
	}
	if err = temp.Sync(); err != nil { // coverage-ignore: injected fsync faults cover publication; a local temporary sync failure requires storage fault.
		return activityStorageFailure(operation, "replace", err)
	}
	if err = temp.Close(); err != nil { // coverage-ignore: a newly created local temporary file cannot fail Close without a storage fault.
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
	if err = syncDirectory(dir); err != nil { // coverage-ignore: injected directory-fsync faults cover this durability boundary; a real failure requires storage fault.
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
		x := invalidMemoryRefusal(operation, slug, err, true)
		return nil, nil, &x
	}
	m, _, err := readMemoryMetadata(raw, slug)
	if err != nil {
		x := invalidMemoryRefusal(operation, slug, err, false)
		return nil, nil, &x
	}
	return &ActivityEffort{slug, r.Title}, &m, nil
}
func invalidMemoryRefusal(operation activityOperation, slug string, err error, readFailure bool) ActivityReply {
	action := "repair .awf/efforts/" + slug + "/memory.md manually"
	var cause error
	if readFailure {
		cause = err
	}
	return refusalFor(operation, ActivityInvalidMemory, cause, []string{action})
}
func (s *Service) activityIdentity(slug string) (*fileIdentity, error) {
	_, identity, err := readRegularNoFollowBoundedIdentity(s.paths.activityFile(slug), maxMemoryBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil //nolint:nilnil // an absent activity is the documented attach state, not a decoding failure.
	}
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func (s *Service) activityCurrentIdentity(slug string) (*Activity, *fileIdentity, error) {
	raw, identity, err := readRegularNoFollowBoundedIdentity(s.paths.activityFile(slug), maxMemoryBytes)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
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
func (s *Service) publicationRefusal(op activityOperation, err error) ActivityReply {
	var refusal *activityPublicationRefusal
	if !errors.As(err, &refusal) {
		return refusalFor(op, ActivityUnsafeResident, err, nil)
	}
	return refusalFor(op, ActivityUnsafeResident, refusal, nil)
}
func (s *Service) AttachActivity(slug, owner string) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	eff, m, bad := s.activityEffort(slug, activityAttach)
	if bad != nil {
		return *bad
	}
	now := s.now().UTC()
	a := Activity{SchemaVersion: activitySchemaVersion, Owner: owner, AttachedAt: now, HeartbeatAt: now}
	if err := validActivity(a); err != nil {
		return refusalFor(activityAttach, ActivityUnsafeResident, err, nil)
	}
	identity, err := s.activityIdentity(slug)
	if err != nil {
		return refusalFor(activityAttach, ActivityUnsafeResident, err, nil)
	}
	op := activityAttach
	condition := ActivityAttached
	if identity != nil {
		op = activityTakeover
		condition = ActivityTakenOver
	}
	if activityBeforePublish != nil {
		activityBeforePublish()
	}
	if err = s.store.replaceActivity(s.paths.activityFile(slug), a, identity, op); err != nil {
		return s.publicationRefusal(op, err)
	}
	r := activityReply(condition)
	r.Effort = eff
	r.Memory = m
	r.Activity = &a
	return r
}
func (s *Service) HeartbeatActivity(slug, owner string) ActivityReply {
	return s.mutateActivity(slug, owner, activityHeartbeat, ActivityHeartbeat, func(a *Activity) { a.HeartbeatAt = s.now().UTC() })
}
func (s *Service) DetachActivity(slug, owner string) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	if _, err := s.store.loadDirectory(s.paths.effort(slug), slug, false); err != nil {
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
		return activityReply(ActivityDetached)
	}
	if a.Owner != owner {
		return refusalFor(activityDetach, ActivityNotOwner, nil, nil)
	}
	if activityBeforePublish != nil {
		activityBeforePublish()
	}
	if err = s.store.removeActivityExpected(s.paths.activityFile(slug), identity); err != nil {
		return s.publicationRefusal(activityDetach, err)
	}
	return activityReply(ActivityDetached)
}
func (s *Service) mutateActivity(slug, owner string, op activityOperation, condition ActivityCondition, change func(*Activity)) ActivityReply {
	unlock := lockActivity(s.paths.activityFile(slug))
	defer unlock()
	eff, m, bad := s.activityEffort(slug, op)
	if bad != nil {
		return *bad
	}
	a, identity, err := s.activityCurrentIdentity(slug)
	if err != nil {
		return refusalFor(op, ActivityUnsafeResident, err, nil)
	}
	if a == nil {
		return refusalFor(op, ActivityMissing, nil, nil)
	}
	if a.Owner != owner {
		return refusalFor(op, ActivityNotOwner, nil, nil)
	}
	change(a)
	if err = s.store.replaceActivity(s.paths.activityFile(slug), *a, identity, op); err != nil {
		return s.publicationRefusal(op, err)
	}
	r := activityReply(condition)
	r.Effort = eff
	r.Memory = m
	r.Activity = a
	return r
}
