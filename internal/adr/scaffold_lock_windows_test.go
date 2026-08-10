//go:build windows

package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCanonicalDecisionsDirectoryCollapsesWindowsAliases(t *testing.T) {
	dir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalDecisionsDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{strings.ToUpper(dir), `\\?\` + dir} {
		got, err := canonicalDecisionsDirectory(alias)
		if err != nil {
			t.Fatalf("canonicalize alias %q: %v", alias, err)
		}
		if got != want {
			t.Fatalf("canonical identity for %q = %q, want %q", alias, got, want)
		}
	}
}

func TestCanonicalDecisionsDirectoryCollapsesVolumeAlias(t *testing.T) {
	dir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	device := ""
	for letter := 'Y'; letter >= 'D'; letter-- {
		candidate := string(letter) + ":"
		if _, err := os.Stat(candidate + `\`); os.IsNotExist(err) {
			device = candidate
			break
		}
	}
	if device == "" {
		t.Fatal("no unused drive letter for volume-alias proof")
	}
	volume := filepath.VolumeName(dir)
	if volume == "" {
		t.Fatalf("temporary directory %q has no Windows volume", dir)
	}
	targetBuffer := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.QueryDosDevice(windows.StringToUTF16Ptr(volume), &targetBuffer[0], uint32(len(targetBuffer)))
	if err != nil {
		t.Fatalf("resolve volume device %s: %v", volume, err)
	}
	target := windows.UTF16ToString(targetBuffer[:n])
	deviceName := windows.StringToUTF16Ptr(device)
	targetPath := windows.StringToUTF16Ptr(target)
	if err := windows.DefineDosDevice(windows.DDD_RAW_TARGET_PATH|windows.DDD_NO_BROADCAST_SYSTEM, deviceName, targetPath); err != nil {
		t.Fatalf("define volume alias %s -> %s: %v", device, target, err)
	}
	t.Cleanup(func() {
		flags := uint32(windows.DDD_REMOVE_DEFINITION | windows.DDD_EXACT_MATCH_ON_REMOVE | windows.DDD_RAW_TARGET_PATH | windows.DDD_NO_BROADCAST_SYSTEM)
		if err := windows.DefineDosDevice(flags, deviceName, targetPath); err != nil {
			t.Errorf("remove volume alias %s: %v", device, err)
		}
	})
	want, err := canonicalDecisionsDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalDecisionsDirectory(device + strings.TrimPrefix(dir, volume))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("volume alias identity = %q, want %q", got, want)
	}
}
