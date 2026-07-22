//go:build !windows

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func isReparsePoint(string) (bool, error) { return false, nil }

func currentUID() (uint64, error) { return uint64(os.Getuid()), nil }

func currentOwnerIdentity() (string, error) {
	uid, _ := currentUID()
	return fmt.Sprintf("uid:%d", uid), nil
}

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func lockLeaseOperations(ctx context.Context, path string) (func() error, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil { // coverage-ignore: os.NewFile returns non-nil for a valid open file descriptor
		_ = syscall.Close(fd)
		return nil, errors.New("open lease operation guard")
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() error {
				unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				closeErr := file.Close()
				if unlockErr != nil { // coverage-ignore: valid held POSIX flock unlock cannot fail
					return unlockErr
				}
				return closeErr
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) { // coverage-ignore: valid local file flock returns only busy or success
			_ = file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(defaultLeasePoll):
		}
	}
}
