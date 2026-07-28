// Package contextspill recognizes context spill notices and records local,
// path-free observability events for the repository runner.
package contextspill

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const noticePrefix = "AWF_CONTEXT_SPILL_V1"

// Notice is the bounded data carried by a context spill descriptor.
type Notice struct {
	Bytes uint64
	Path  string
}

var (
	now       = time.Now
	lstatPath = os.Lstat
	mkdirPath = os.Mkdir
	openFile  = syscall.Open
	fstatFile = syscall.Fstat
	flockFile = syscall.Flock
	writeFile = syscall.Write
	fsyncFile = syscall.Fsync
	closeFile = syscall.Close
)

// ParseNotice recognizes the closed two-line context spill notice grammar.
func ParseNotice(data []byte) (Notice, bool, error) {
	if !strings.HasPrefix(string(data), noticePrefix) {
		return Notice{}, false, nil
	}
	invalid := func(reason string) (Notice, bool, error) {
		return Notice{}, true, errors.New("invalid context spill notice: " + reason)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) != 3 || lines[2] != "" {
		return invalid("expected exactly two newline-terminated lines")
	}
	fields := strings.Split(lines[0], " ")
	if len(fields) != 3 || fields[0] != noticePrefix || !strings.HasPrefix(fields[1], "bytes=") || fields[2] != "format=text" {
		return invalid("invalid descriptor line")
	}
	digits := strings.TrimPrefix(fields[1], "bytes=")
	if digits == "" || strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return invalid("bytes must be unsigned decimal")
	}
	count, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return invalid("bytes overflow")
	}
	path := lines[1]
	if path == "" || !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n") || filepath.Clean(path) != path {
		return invalid("spill path must be absolute, canonical, and newline-free")
	}
	return Notice{Bytes: count, Path: path}, true, nil
}

// ShellQuote renders argv as POSIX single-quoted shell words.
func ShellQuote(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}

// Log appends one path-free spill record beneath the repository-local private
// directory. It rejects symlink, ownership, type, and permission surprises.
func Log(root string, notice Notice, invocation []string) error {
	canonicalRoot, err := canonicalRoot(root)
	if err != nil {
		return err
	}
	awf := filepath.Join(canonicalRoot, ".awf")
	if err := inspectOwnedDirectory(awf, false); err != nil {
		return fmt.Errorf("inspect .awf: %w", err)
	}
	local := filepath.Join(awf, "local")
	if err := mkdirPath(local, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create .awf/local: %w", err)
	}
	if err := inspectOwnedDirectory(local, true); err != nil {
		return fmt.Errorf("inspect .awf/local: %w", err)
	}
	logPath := filepath.Join(local, "context-spills.log")
	fd, err := openFile(logPath, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open context spill log: %w", err)
	}
	acquired := false
	var first error
	var stat syscall.Stat_t
	statErr := fstatFile(fd, &stat)
	switch {
	case statErr != nil:
		first = fmt.Errorf("inspect context spill log: %w", statErr)
	case stat.Mode&syscall.S_IFMT != syscall.S_IFREG:
		first = errors.New("inspect context spill log: not a regular file")
	case stat.Mode&0o777 != 0o600:
		first = fmt.Errorf("inspect context spill log: mode is %04o, want 0600", stat.Mode&0o777)
	case uint64(stat.Uid) != uint64(os.Getuid()):
		first = errors.New("inspect context spill log: owned by another user")
	}
	if first == nil {
		if err := flockFile(fd, syscall.LOCK_EX); err != nil {
			first = fmt.Errorf("lock context spill log: %w", err)
		} else {
			acquired = true
		}
	}
	if first == nil {
		record := fmt.Sprintf("%s\tbytes=%d\tinvocation=%s\n", now().UTC().Format(time.RFC3339Nano), notice.Bytes, ShellQuote(invocation))
		first = writeAllFD(fd, []byte(record))
		if first != nil {
			first = fmt.Errorf("write context spill log: %w", first)
		}
	}
	if first == nil {
		if err := fsyncFile(fd); err != nil {
			first = fmt.Errorf("sync context spill log: %w", err)
		}
	}
	if acquired {
		if err := flockFile(fd, syscall.LOCK_UN); first == nil && err != nil {
			first = fmt.Errorf("unlock context spill log: %w", err)
		}
	}
	if err := closeFile(fd); first == nil && err != nil {
		first = fmt.Errorf("close context spill log: %w", err)
	}
	return first
}

func canonicalRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("canonicalize repository root: empty path")
	}
	absolute, err := filepath.Abs(root)
	if err != nil { // coverage-ignore: an explicit root does not depend on a missing working directory
		return "", fmt.Errorf("canonicalize repository root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("canonicalize repository root: %w", err)
	}
	return canonical, nil
}

func inspectOwnedDirectory(path string, private bool) error {
	info, err := lstatPath(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("unsafe non-directory or symlink path")
	}
	uid, ok := fileUID(info)
	if !ok {
		return errors.New("ownership unavailable")
	}
	if uid != uint64(os.Getuid()) {
		return errors.New("owned by another user")
	}
	if private && info.Mode().Perm() != 0o700 {
		return fmt.Errorf("mode is %04o, want 0700", info.Mode().Perm())
	}
	return nil
}

func fileUID(info os.FileInfo) (uint64, bool) {
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

func writeAllFD(fd int, data []byte) error {
	for len(data) > 0 {
		n, err := writeFile(fd, data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
