// Package contextspill recognizes context spill notices and records local,
// path-free observability events for the repository runner.
package contextspill

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const noticePrefix = "AWF_CONTEXT_SPILL_V1"

// Notice is the bounded data carried by a context spill descriptor.
type Notice struct {
	Bytes uint64
	Path  string
}

var (
	now       = time.Now
	openAt    = unix.Openat
	mkdirAt   = unix.Mkdirat
	fstatFile = unix.Fstat
	flockFile = unix.Flock
	writeFile = unix.Write
	fsyncFile = unix.Fsync
	closeFile = unix.Close
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
// directory. Every descendant is opened relative to its verified parent
// descriptor, preventing path substitution between inspection and use.
func Log(root string, notice Notice, invocation []string) error {
	rootFD, err := openRepositoryRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = closeFile(rootFD) }()
	awfFD, err := openOwnedDirectoryAt(rootFD, ".awf", 0)
	if err != nil {
		return fmt.Errorf("open .awf: %w", err)
	}
	defer func() { _ = closeFile(awfFD) }()
	localFD, err := openLocalDirectory(awfFD)
	if err != nil {
		return err
	}
	defer func() { _ = closeFile(localFD) }()
	fd, err := openAt(localFD, "context-spills.log", unix.O_WRONLY|unix.O_APPEND|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open context spill log: %w", err)
	}
	first := inspectOwnedFD(fd, unix.S_IFREG, 0o600, "context spill log")
	acquired := false
	if first == nil {
		if err := flockFile(fd, unix.LOCK_EX); err != nil {
			first = fmt.Errorf("lock context spill log: %w", err)
		} else {
			acquired = true
		}
	}
	if first == nil {
		record := fmt.Sprintf("%s\tbytes=%d\tinvocation=%s\n", now().UTC().Format(time.RFC3339Nano), notice.Bytes, ShellQuote(invocation))
		if err := writeAllFD(fd, []byte(record)); err != nil {
			first = fmt.Errorf("write context spill log: %w", err)
		}
	}
	if first == nil {
		if err := fsyncFile(fd); err != nil {
			first = fmt.Errorf("sync context spill log: %w", err)
		}
	}
	if acquired {
		if err := flockFile(fd, unix.LOCK_UN); first == nil && err != nil {
			first = fmt.Errorf("unlock context spill log: %w", err)
		}
	}
	if err := closeFile(fd); first == nil && err != nil {
		first = fmt.Errorf("close context spill log: %w", err)
	}
	return first
}

// HasSafeLog reports whether a nonempty observability log exists after opening
// every component relative to a verified parent descriptor. An absent local
// directory or log is a safe empty state.
func HasSafeLog(root string) (bool, error) {
	rootFD, err := openRepositoryRoot(root)
	if err != nil {
		return false, err
	}
	defer func() { _ = closeFile(rootFD) }()
	awfFD, err := openOwnedDirectoryAt(rootFD, ".awf", 0)
	if err != nil {
		return false, fmt.Errorf("open .awf: %w", err)
	}
	defer func() { _ = closeFile(awfFD) }()
	localFD, err := openOwnedDirectoryAt(awfFD, "local", 0o700)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open .awf/local: %w", err)
	}
	defer func() { _ = closeFile(localFD) }()
	fd, err := openAt(localFD, "context-spills.log", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open context spill log: %w", err)
	}
	defer func() { _ = closeFile(fd) }()
	var stat unix.Stat_t
	if err := fstatFile(fd, &stat); err != nil {
		return false, fmt.Errorf("inspect context spill log: %w", err)
	}
	if err := validateOwnedStat(stat, unix.S_IFREG, 0o600); err != nil {
		return false, fmt.Errorf("inspect context spill log: %w", err)
	}
	return stat.Size > 0, nil
}

func openRepositoryRoot(root string) (int, error) {
	canonical, err := canonicalRoot(root)
	if err != nil {
		return -1, err
	}
	fd, err := openAt(unix.AT_FDCWD, canonical, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, fmt.Errorf("open repository root: %w", err)
	}
	return fd, nil
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

func openLocalDirectory(awfFD int) (int, error) {
	fd, err := openOwnedDirectoryAt(awfFD, "local", 0o700)
	if errors.Is(err, unix.ENOENT) {
		if mkdirErr := mkdirAt(awfFD, "local", 0o700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
			return -1, fmt.Errorf("create .awf/local: %w", mkdirErr)
		}
		fd, err = openOwnedDirectoryAt(awfFD, "local", 0o700)
	}
	if err != nil {
		return -1, fmt.Errorf("open .awf/local: %w", err)
	}
	return fd, nil
}

func openOwnedDirectoryAt(parent int, name string, mode uint32) (int, error) {
	fd, err := openAt(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, err
	}
	if err := inspectOwnedFD(fd, unix.S_IFDIR, mode, "directory"); err != nil {
		_ = closeFile(fd)
		return -1, err
	}
	return fd, nil
}

func inspectOwnedFD(fd int, kind uint32, mode uint32, label string) error {
	var stat unix.Stat_t
	if err := fstatFile(fd, &stat); err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if err := validateOwnedStat(stat, kind, mode); err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	return nil
}

func validateOwnedStat(stat unix.Stat_t, kind uint32, mode uint32) error {
	if stat.Mode&unix.S_IFMT != kind {
		if kind == unix.S_IFREG {
			return errors.New("not a regular file")
		}
		return errors.New("not a directory")
	}
	if mode != 0 && stat.Mode&0o777 != mode {
		return fmt.Errorf("mode is %04o, want %04o", stat.Mode&0o777, mode)
	}
	if uint64(stat.Uid) != uint64(os.Getuid()) {
		return errors.New("owned by another user")
	}
	return nil
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
