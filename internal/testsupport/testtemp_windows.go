//go:build windows

package testsupport

import (
	"errors"
	"io/fs"
)

var errTestTempUnsupportedPlatform = errors.New("test temp management is unsupported on windows")

func testTempRoot() (string, error)                  { return "", errTestTempUnsupportedPlatform }
func validateTestTempPath(string, fs.FileInfo) error { return errTestTempUnsupportedPlatform }
