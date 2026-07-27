//go:build windows

package effort

import "errors"

func testCurrentEUID() int { return -1 }

func testChown(string, int) error { return errors.New("chown is unavailable on Windows") }

func testMkfifo(string, uint32) error { return errors.New("named pipes use a distinct Windows API") }
