package effort

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const partialSchemaVersion = 1

type PartialEvidence struct {
	SchemaVersion int         `json:"schemaVersion"`
	EffortID      string      `json:"effortId"`
	Action        string      `json:"action"`
	Base          string      `json:"base,omitempty"`
	Branch        string      `json:"branch"`
	Path          string      `json:"path,omitempty"`
	CommonDir     string      `json:"commonDir"`
	Tip           string      `json:"tip,omitempty"`
	TargetPath    string      `json:"targetPath,omitempty"`
	TargetBranch  string      `json:"targetBranch,omitempty"`
	Integration   Integration `json:"integration,omitempty"`
	DeleteForce   bool        `json:"deleteForce,omitempty"`
	BranchTip     string      `json:"branchTip,omitempty"`
}

func (p paths) partial(id, action string) string {
	return filepath.Join(p.efforts, "."+id+"."+action+".partial")
}
func validPartialAction(action string) bool {
	return action == "worktree" || action == "integration" || action == "removal"
}

var partialIdentity = requireIdentity

func (s store) putPartial(e PartialEvidence) error {
	if !uuidV4Pattern.MatchString(e.EffortID) || !validPartialAction(e.Action) || e.SchemaVersion != partialSchemaVersion || e.Branch != "awf/"+e.EffortID || filepath.Clean(e.CommonDir) != e.CommonDir {
		return errors.New("invalid partial-mutation evidence")
	}
	if e.Action == "worktree" && (!objectIDPattern.MatchString(e.Base) || e.Path == "" || !filepath.IsAbs(e.Path) || filepath.Clean(e.Path) != e.Path) {
		return errors.New("invalid worktree partial evidence")
	}
	if e.Action == "integration" && (!objectIDPattern.MatchString(e.Tip) || e.TargetPath == "" || e.TargetBranch == "" || (e.Integration != IntegrationFastForward && e.Integration != IntegrationMerge)) {
		return errors.New("invalid integration partial evidence")
	}
	if e.Action == "removal" && !objectIDPattern.MatchString(e.BranchTip) {
		return errors.New("invalid removal partial evidence")
	}
	if err := requireAbsent(s.paths.partial(e.EffortID, e.Action)); err != nil {
		return fmt.Errorf("require absent partial evidence: %w", err)
	}
	raw, err := json.Marshal(e)
	if err != nil { // coverage-ignore: PartialEvidence contains only JSON-marshalable scalar fields, so encoding/json cannot fail
		return fmt.Errorf("encode partial evidence: %w", err)
	}
	return atomicReplaceFS(s.filesystem(), s.paths.partial(e.EffortID, e.Action), raw, nil)
}

func (s store) getPartial(id, action string) (PartialEvidence, fileIdentity, error) {
	path := s.paths.partial(id, action)
	raw, identity, err := readRegularNoFollow(path)
	if err != nil {
		return PartialEvidence{}, fileIdentity{}, err
	}
	var e PartialEvidence
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return PartialEvidence{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	if err := requireJSONEOF(dec); err != nil {
		return PartialEvidence{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	if err := (store{paths: s.paths}).validatePartial(e, id, action); err != nil {
		return PartialEvidence{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	return e, identity, nil
}
func (s store) validatePartial(e PartialEvidence, id, action string) error {
	if e.EffortID != id || e.Action != action {
		return errors.New("partial evidence does not match stable path")
	}
	return s.putPartialValidation(e)
}
func (s store) putPartialValidation(e PartialEvidence) error {
	// Validate without publishing, keeping corrupt evidence byte-preserved.
	if !uuidV4Pattern.MatchString(e.EffortID) || !validPartialAction(e.Action) || e.SchemaVersion != partialSchemaVersion || e.Branch != "awf/"+e.EffortID || filepath.Clean(e.CommonDir) != e.CommonDir {
		return errors.New("invalid partial-mutation evidence")
	}
	if e.Action == "worktree" && (!objectIDPattern.MatchString(e.Base) || e.Path == "" || !filepath.IsAbs(e.Path) || filepath.Clean(e.Path) != e.Path) {
		return errors.New("invalid worktree partial evidence")
	}
	if e.Action == "integration" && (!objectIDPattern.MatchString(e.Tip) || e.TargetPath == "" || e.TargetBranch == "" || (e.Integration != IntegrationFastForward && e.Integration != IntegrationMerge)) {
		return errors.New("invalid integration partial evidence")
	}
	if e.Action == "removal" && !objectIDPattern.MatchString(e.BranchTip) {
		return errors.New("invalid removal partial evidence")
	}
	return nil
}
func (s store) clearPartial(id, action string) error {
	path := s.paths.partial(id, action)
	_, identity, err := s.getPartial(id, action)
	if err != nil {
		return err
	}
	if err := partialIdentity(path, identity); err != nil {
		return fmt.Errorf("verify partial evidence before removal: %w", err)
	}
	if err := s.filesystem().Remove(path); err != nil {
		return fmt.Errorf("remove partial evidence %s: %w", path, err)
	}
	d, err := s.filesystem().OpenDirectory(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open evidence directory for sync: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync evidence directory: %w", err)
	}
	return nil
}

func (s *Service) RecordPartial(e PartialEvidence) error {
	return s.store.withLock(func() error { return s.store.putPartial(e) })
}
func (s *Service) ClearPartial(id, action string) error {
	return s.store.withLock(func() error { return s.store.clearPartial(id, action) })
}

func gitPartial(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

var partialResolveGit = gitPartial

func resolvePartial(ctx context.Context, root, rev string) (string, error) {
	out, err := partialResolveGit(ctx, root, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	if !objectIDPattern.MatchString(v) {
		return "", errors.New("invalid object ID")
	}
	return v, nil
}
func ancestorPartial(ctx context.Context, root, older, newer string) (bool, error) {
	_, err := gitPartial(ctx, root, "merge-base", "--is-ancestor", older, newer)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
func branchExistsPartial(ctx context.Context, root, branch string) (bool, error) {
	_, err := gitPartial(ctx, root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func partialAbsent(path string) bool { _, err := os.Lstat(path); return errors.Is(err, os.ErrNotExist) }
