package telemetry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const ownerOnlyMode fs.FileMode = 0o700

type ledgerPaths struct {
	project    string
	awf        string
	root       string
	efforts    string
	leases     string
	staging    string
	tombstones string
	trash      string
}

func newLedgerPaths(projectRoot string) (ledgerPaths, error) {
	if projectRoot == "" || !filepath.IsAbs(projectRoot) {
		return ledgerPaths{}, errors.New("project root must be an absolute path")
	}
	absolute, err := filepath.Abs(projectRoot)
	if err != nil { // coverage-ignore: filepath.Abs only cleans an already-absolute path after the check above
		return ledgerPaths{}, fmt.Errorf("resolve project root: %w", err)
	}
	awf := filepath.Join(absolute, ".awf")
	root := filepath.Join(awf, "metrics")
	return ledgerPaths{
		project:    absolute,
		awf:        awf,
		root:       root,
		efforts:    filepath.Join(root, "efforts"),
		leases:     filepath.Join(root, "leases"),
		staging:    filepath.Join(root, "staging"),
		tombstones: filepath.Join(root, "tombstones"),
		trash:      filepath.Join(root, "trash"),
	}, nil
}

func (p ledgerPaths) effort(id string) string { return filepath.Join(p.efforts, id) }
func (p ledgerPaths) creationLease(id string) string {
	return filepath.Join(p.leases, id+".create.json")
}
func (p ledgerPaths) appendLease(id string) string {
	return filepath.Join(p.leases, id+".append.json")
}
func (p ledgerPaths) leaseGuard() string         { return filepath.Join(p.leases, ".operations.lock") }
func (p ledgerPaths) tombstone(id string) string { return filepath.Join(p.tombstones, id+".json") }
func (p ledgerPaths) stream(effortID, sessionID string) string {
	return filepath.Join(p.effort(effortID), "sessions", sessionID+".jsonl")
}

func validatePathIdentifier(name, value string) error {
	return validateStringFormat(name, value, "identifier")
}

// inspectProjectAnchor validates the already-existing confinement anchor before
// any telemetry directory is created. Project directories need not be private,
// but they must be real directories owned by the caller where ownership is
// available.
func inspectProjectAnchor(project, awf string) error {
	for _, path := range []string{project, awf} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe project path %s", path)
		}
		reparse, err := isReparsePoint(path)
		if err != nil { // coverage-ignore: non-Windows reparse inspection is infallible; Windows is cross-compiled
			return fmt.Errorf("inspect reparse status for %s: %w", path, err)
		}
		if reparse { // coverage-ignore: reparse points are a Windows-only path capability
			return fmt.Errorf("unsafe reparse path %s", path)
		}
		if runtime.GOOS != "windows" {
			uid, ok := fileUID(info)
			if !ok { // coverage-ignore: supported POSIX stat always supplies a native UID field
				return fmt.Errorf("ownership is unavailable for %s", path)
			}
			current, err := currentUID()
			if err != nil { // coverage-ignore: POSIX owner identity uses the infallible numeric process UID
				return fmt.Errorf("determine current owner: %w", err)
			}
			if uid != current { // coverage-ignore: tests cannot create a foreign-owned path without privilege
				return fmt.Errorf("path is owned by another user: %s", path)
			}
		}
	}
	return nil
}

func inspectPath(path string, wantDir bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe symlink path %s", path)
	}
	reparse, err := isReparsePoint(path)
	if err != nil { // coverage-ignore: non-Windows reparse inspection is infallible; Windows is cross-compiled
		return fmt.Errorf("inspect reparse status for %s: %w", path, err)
	}
	if reparse { // coverage-ignore: reparse points are a Windows-only path capability
		return fmt.Errorf("unsafe reparse path %s", path)
	}
	if wantDir != info.IsDir() || (!wantDir && !info.Mode().IsRegular()) {
		return fmt.Errorf("unsafe non-regular path %s", path)
	}
	if runtime.GOOS == "windows" { // coverage-ignore: Windows path branch is verified by cross-compile
		return nil
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("path is not owner-only: %s", path)
	}
	uid, ok := fileUID(info)
	if !ok { // coverage-ignore: supported POSIX stat always supplies a native UID field
		return fmt.Errorf("ownership is unavailable for %s", path)
	}
	current, err := currentUID()
	if err != nil { // coverage-ignore: POSIX owner identity uses the infallible numeric process UID
		return fmt.Errorf("determine current owner: %w", err)
	}
	if uid != current { // coverage-ignore: tests cannot create a foreign-owned path without privilege
		return fmt.Errorf("path is owned by another user: %s", path)
	}
	return nil
}

func inspectConfined(root, target string, wantDir bool) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path escapes telemetry root: %s", target)
	}
	if err := inspectPath(root, true); err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := root
	components := strings.Split(relative, string(filepath.Separator))
	for index, component := range components {
		current = filepath.Join(current, component)
		isLast := index == len(components)-1
		if err := inspectPath(current, wantDir || !isLast); err != nil {
			return err
		}
	}
	return nil
}

func fileUID(info fs.FileInfo) (uint64, bool) {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return 0, false
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return 0, false
	}
	field := value.FieldByName("Uid")
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return field.Uint(), true
	default:
		return 0, false
	}
}
