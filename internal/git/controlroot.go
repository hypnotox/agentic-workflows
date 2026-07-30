package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ControlRoots identifies the invoking checkout, repository-wide Git common
// directory, and primary checkout that owns resident awf state.
type ControlRoots struct {
	InvokingRoot string
	CommonDir    string
	PrimaryRoot  string
}

// ResidentName is the closed set of repository-wide awf resident roots.
type ResidentName string

const (
	ResidentEfforts   ResidentName = "efforts"
	ResidentWorktrees ResidentName = "worktrees"
)

// HardSafetyError marks a safety refusal that cannot be overridden by force.
type HardSafetyError struct {
	Category string
	Path     string
	Err      error
}

func (e *HardSafetyError) Error() string {
	if e == nil {
		return "hard safety refusal"
	}
	message := "hard safety refusal"
	if e.Category != "" {
		message += " (" + e.Category + ")"
	}
	if e.Path != "" {
		message += " at " + e.Path
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *HardSafetyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Forceable reports whether the refusal may be overridden.
func (*HardSafetyError) Forceable() bool { return false }

// ResolveControlRoots derives the checkout-local and repository-wide control
// roots using native Git. Native Git is used only for topology discovery;
// OpenRepo remains the package's read-only go-git boundary.
func ResolveControlRoots(ctx context.Context, root string) (ControlRoots, error) {
	originalRoot, err := cleanAbsolute(root)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, fmt.Errorf("resolve invoking root: %w", err)
	}
	if err := lstatComponents(originalRoot); err != nil {
		return ControlRoots{}, err
	}
	native := newRunner(originalRoot)

	bare, err := runGitTextWith(ctx, native, "rev-parse", "--is-bare-repository")
	if err != nil {
		return ControlRoots{}, fmt.Errorf("inspect bare-repository state from %s: %w", originalRoot, err)
	}
	if bare != "true" && bare != "false" {
		return ControlRoots{}, &HardSafetyError{Category: "repository-identity", Path: originalRoot, Err: fmt.Errorf("unexpected bare-repository response %q", bare)}
	}
	if bare == "true" {
		return ControlRoots{}, &HardSafetyError{Category: "bare-repository", Path: originalRoot}
	}
	invokingRoot, err := runGitPathWith(ctx, native, "rev-parse", "--show-toplevel")
	if err != nil {
		return ControlRoots{}, fmt.Errorf("resolve invoking checkout from %s: %w", originalRoot, err)
	}
	commonDir, err := runGitPathWith(ctx, native, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return ControlRoots{}, fmt.Errorf("resolve common Git directory from %s: %w", originalRoot, err)
	}

	originalIdentity, err := stableIdentity(originalRoot)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, fmt.Errorf("resolve invoking-root identity: %w", err)
	}
	invokingIdentity, err := stableIdentity(invokingRoot)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, fmt.Errorf("resolve checkout identity: %w", err)
	}
	if !lexicallyContained(invokingIdentity, originalIdentity) { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, &HardSafetyError{Category: "repository-identity", Path: originalRoot, Err: errors.New("git resolved a checkout outside the invoking path")}
	}
	commonIdentity, err := stableIdentity(commonDir)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, &HardSafetyError{Category: "repository-identity", Path: commonDir, Err: err}
	}
	invokingGitDir, err := resolveValidated(invokingRoot, worktreeGitDir)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, identityError(invokingRoot, err)
	}
	invokingGitDirIdentity, err := stableIdentity(invokingGitDir)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return ControlRoots{}, identityError(invokingRoot, err)
	}

	porcelain, err := native.run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return ControlRoots{}, fmt.Errorf("list worktree topology from %s: %w", originalRoot, err)
	}
	records, err := parseWorktreePorcelain(porcelain)
	if err != nil {
		return ControlRoots{}, &HardSafetyError{Category: "unconfined", Path: commonDir, Err: err}
	}

	invokingListed := 0
	var primaries []string
	for _, record := range records {
		worktree, err := cleanAbsolute(record.path)
		if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
			return ControlRoots{}, &HardSafetyError{Category: "unconfined", Path: record.path, Err: err}
		}
		worktreeIdentity, err := stableIdentity(worktree)
		if err != nil {
			if record.prunable && os.IsNotExist(unwrappedError(err)) {
				continue
			}
			return ControlRoots{}, identityError(worktree, err)
		}
		if !record.bare && (sameCleanPath(worktreeIdentity, invokingIdentity) || sameCleanPath(worktreeIdentity, invokingGitDirIdentity)) {
			invokingListed++
		}
		if record.bare {
			continue
		}
		// Git reports a --separate-git-dir primary at CommonDir rather than at
		// its checkout. Only the invoking checkout's .git pointer provides an
		// authoritative reverse mapping. A linked checkout has no such mapping,
		// so it must fail missing-primary rather than guess from the filesystem.
		if sameCleanPath(worktreeIdentity, commonIdentity) {
			if sameCleanPath(invokingGitDirIdentity, commonIdentity) {
				primaries = append(primaries, invokingRoot)
			}
			continue
		}
		gitdir, err := resolveValidated(worktree, worktreeGitDir)
		if err != nil {
			if record.prunable && os.IsNotExist(unwrappedError(err)) {
				continue
			}
			return ControlRoots{}, identityError(worktree, err)
		}
		gitdirIdentity, err := stableIdentity(gitdir)
		if err != nil {
			return ControlRoots{}, identityError(worktree, err)
		}
		if sameCleanPath(gitdirIdentity, commonIdentity) {
			primaries = append(primaries, worktree)
		}
	}
	if invokingListed != 1 {
		return ControlRoots{}, &HardSafetyError{Category: "repository-identity", Path: invokingRoot, Err: fmt.Errorf("checkout appears %d times in Git worktree topology", invokingListed)}
	}
	if len(primaries) == 0 {
		return ControlRoots{}, &HardSafetyError{Category: "missing-primary", Path: commonDir}
	}
	if len(primaries) != 1 {
		return ControlRoots{}, &HardSafetyError{Category: "ambiguous-primary", Path: commonDir, Err: fmt.Errorf("found %d primary worktrees", len(primaries))}
	}
	return ControlRoots{InvokingRoot: invokingRoot, CommonDir: commonDir, PrimaryRoot: primaries[0]}, nil
}

// ResidentRoot returns one closed resident root after proving that every
// existing component beneath PrimaryRoot is non-symlinked and current-owned.
func (r ControlRoots) ResidentRoot(name ResidentName) (string, error) {
	switch name {
	case ResidentEfforts, ResidentWorktrees:
	default:
		return "", &HardSafetyError{Category: "unknown-resident", Path: string(name)}
	}
	if !filepath.IsAbs(r.PrimaryRoot) {
		return "", &HardSafetyError{Category: "unconfined", Path: r.PrimaryRoot, Err: errors.New("primary root is not absolute")}
	}
	primary := filepath.Clean(r.PrimaryRoot)
	resident := filepath.Join(primary, ".awf", string(name))
	if !lexicallyContained(primary, resident) { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return "", &HardSafetyError{Category: "unconfined", Path: resident}
	}
	if err := lstatExistingComponents(primary); err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return "", err
	}
	if err := lstatResidentComponents(primary, resident); err != nil {
		return "", err
	}
	return resident, nil
}

type worktreeRecord struct {
	path     string
	head     string
	branch   string
	detached bool
	bare     bool
	prunable bool
}

// WorktreeRegistration is one native-Git worktree registration.
type WorktreeRegistration struct {
	Path     string
	HEAD     string
	Branch   string
	Detached bool
	Bare     bool
	Prunable bool
}

// ListWorktreeRegistrations returns the repository's native-Git registrations
// without consulting or scanning filesystem ancestors.
func ListWorktreeRegistrations(ctx context.Context, invokingRoot string) ([]WorktreeRegistration, error) {
	output, err := newRunner(invokingRoot).run(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, fmt.Errorf("list Git worktrees from %s: %w", invokingRoot, err)
	}
	records, err := parseWorktreePorcelain(output)
	if err != nil {
		return nil, &HardSafetyError{Category: "repository-identity", Path: invokingRoot, Err: err}
	}
	result := make([]WorktreeRegistration, 0, len(records))
	for _, record := range records {
		if !filepath.IsAbs(record.path) {
			return nil, &HardSafetyError{Category: "unconfined", Path: record.path, Err: errors.New("git registration path is not absolute")}
		}
		path := filepath.Clean(record.path)
		result = append(result, WorktreeRegistration{Path: path, HEAD: record.head, Branch: record.branch, Detached: record.detached, Bare: record.bare, Prunable: record.prunable})
	}
	return result, nil
}

func parseWorktreePorcelain(output []byte) ([]worktreeRecord, error) {
	if len(output) == 0 {
		return nil, errors.New("empty worktree topology")
	}
	if output[len(output)-1] != 0 {
		return nil, errors.New("worktree topology is not NUL terminated")
	}
	fields := strings.Split(string(output), "\x00")
	fields = fields[:len(fields)-1]
	var records []worktreeRecord
	var current *worktreeRecord
	seen := map[string]bool{}
	finish := func() error {
		if current == nil {
			return nil
		}
		if current.path == "" { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
			return errors.New("worktree record has no path")
		}
		if current.bare {
			if current.head != "" || current.branch != "" || current.detached {
				return errors.New("bare worktree record has checkout fields")
			}
			if current.prunable {
				return errors.New("bare worktree record is prunable")
			}
		} else {
			if current.head == "" {
				return errors.New("non-bare worktree record has no HEAD")
			}
			if (current.branch != "") == current.detached {
				return errors.New("non-bare worktree record must be exactly branched or detached")
			}
		}
		records = append(records, *current)
		current = nil
		seen = map[string]bool{}
		return nil
	}
	for _, field := range fields {
		if field == "" {
			if err := finish(); err != nil {
				return nil, err
			}
			continue
		}
		key, value, hasValue := strings.Cut(field, " ")
		if key == "worktree" {
			if current != nil {
				return nil, errors.New("worktree record is not NUL-record delimited")
			}
			if !hasValue || value == "" {
				return nil, errors.New("worktree field has no path")
			}
			current = &worktreeRecord{path: value}
			seen[key] = true
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("field %q precedes worktree", field)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate worktree field %q", key)
		}
		switch key {
		case "HEAD", "branch":
			if !hasValue || value == "" {
				return nil, fmt.Errorf("field %q requires a value", key)
			}
			if key == "HEAD" {
				current.head = value
			} else {
				if seen["detached"] { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
					return nil, errors.New("worktree is both branched and detached")
				}
				current.branch = value
			}
		case "detached":
			if hasValue || seen["branch"] {
				return nil, errors.New("invalid detached field")
			}
			current.detached = true
		case "bare":
			if hasValue {
				return nil, errors.New("invalid bare field")
			}
			current.bare = true
		case "prunable":
			if !hasValue || strings.TrimSpace(value) == "" {
				return nil, errors.New("prunable field requires a reason")
			}
			current.prunable = true
		default:
			return nil, fmt.Errorf("unknown worktree field %q", key)
		}
		seen[key] = true
	}
	if current != nil {
		return nil, errors.New("worktree record is not NUL-record delimited")
	}
	if len(records) == 0 {
		return nil, errors.New("empty worktree topology")
	}
	return records, nil
}

func worktreeGitDir(worktree string) (string, error) {
	dotGit := filepath.Join(worktree, ".git")
	info, err := platformLstatComponent(dotGit, false)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return filepath.Clean(dotGit), nil
	}
	raw, err := os.ReadFile(dotGit)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return "", err
	}
	pointer, ok := strings.CutPrefix(removeGitLineDelimiter(string(raw)), "gitdir: ")
	if !ok || pointer == "" || strings.ContainsAny(pointer, "\r\n\x00") {
		return "", fmt.Errorf("parse %s: expected one gitdir pointer", dotGit)
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(filepath.Dir(dotGit), pointer)
	}
	return filepath.Clean(pointer), nil
}

func runGitPathWith(ctx context.Context, native runner, args ...string) (string, error) {
	output, err := native.run(ctx, args...)
	if err != nil {
		return "", err
	}
	value := removeGitLineDelimiter(string(output))
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("git %s returned an invalid path response", strings.Join(args, " "))
	}
	return cleanAbsolute(value)
}

func runGitTextWith(ctx context.Context, native runner, args ...string) (string, error) {
	output, err := native.run(ctx, args...)
	if err != nil {
		return "", err
	}
	value := removeGitLineDelimiter(string(output))
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("git %s returned an invalid scalar response", strings.Join(args, " "))
	}
	return value, nil
}

func removeGitLineDelimiter(value string) string {
	if strings.HasSuffix(value, "\n") {
		value = strings.TrimSuffix(value, "\n")
		value = strings.TrimSuffix(value, "\r")
	}
	return value
}

// resolveValidated is the check-act-recheck identity ladder: it proves every
// component of path before resolve reads it and again afterwards, so a path
// whose shape changes around the resolution is refused rather than trusted.
func resolveValidated(path string, resolve func(string) (string, error)) (string, error) {
	if err := lstatComponents(path); err != nil {
		return "", err
	}
	resolved, err := resolve(path)
	if err != nil {
		return "", err
	}
	if err := lstatComponents(path); err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return "", err
	}
	return resolved, nil
}

// stableIdentity resolves path's symlink-free identity through the ladder, so
// the identity two paths are compared by is only accepted when the path held
// its shape across the resolution.
func stableIdentity(path string) (string, error) {
	return resolveValidated(path, filepath.EvalSymlinks)
}

// IsolatedGitEnvironment removes inherited repository selection, config, and
// credential controls before a native Git command is run against a validated root.
func IsolatedGitEnvironment(inherited []string) []string {
	filtered := make([]string, 0, len(inherited)+7)
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") || upper == "GCM_INTERACTIVE" || upper == "SSH_ASKPASS" {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"SSH_ASKPASS=true",
		"GCM_INTERACTIVE=Never",
	)
}

func cleanAbsolute(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func lexicallyContained(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func sameCleanPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func lstatComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
			continue
		}
		current = filepath.Join(current, component)
		if _, err := platformLstatComponent(current, false); err != nil {
			return err
		}
	}
	return nil
}

func lstatExistingComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
			continue
		}
		current = filepath.Join(current, component)
		if _, err := platformLstatComponent(current, false); os.IsNotExist(unwrappedError(err)) {
			return nil
		} else if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
			return err
		}
	}
	return nil
}

func lstatResidentComponents(primary, resident string) error {
	if _, err := platformLstatComponent(primary, true); os.IsNotExist(unwrappedError(err)) {
		return nil
	} else if err != nil { // coverage-ignore: requires an OS race or fault between adjacent validated identity operations
		return err
	}
	relative, _ := filepath.Rel(primary, resident)
	current := primary
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if _, err := platformLstatComponent(current, true); os.IsNotExist(unwrappedError(err)) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

func identityError(path string, err error) error {
	var hard *HardSafetyError
	if errors.As(err, &hard) {
		return err
	}
	return &HardSafetyError{Category: "repository-identity", Path: path, Err: err}
}

func unwrappedError(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}
