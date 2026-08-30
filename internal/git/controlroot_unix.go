//go:build linux || darwin

package git

import (
	"fmt"
	"os"
	"os/user"
	"reflect"
	"strconv"
)

func ownedByCurrentUser(info os.FileInfo) bool {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return true
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return true
	}
	uid := value.FieldByName("Uid")
	if !uid.IsValid() || !uid.CanUint() {
		return true
	}
	current, err := user.Current()
	if err != nil {
		return false
	}
	currentUID, err := strconv.ParseUint(current.Uid, 10, 64)
	if err != nil {
		return false
	}
	return uid.Uint() == currentUID
}

func platformLstatComponent(path string, requireOwner bool) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, &HardSafetyError{Category: "symlink", Path: path}
	}
	if requireOwner && !ownedByCurrentUser(info) {
		return nil, &HardSafetyError{Category: "foreign-owner", Path: path}
	}
	return info, nil
}
