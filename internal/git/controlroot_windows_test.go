//go:build windows

package git

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsComponentAPIRejectsReparseAndForeignOwner(t *testing.T) {
	t.Run("reparse", func(t *testing.T) {
		api := windowsComponentAPI{
			attributes: func(windows.Handle) (uint32, error) { return windows.FILE_ATTRIBUTE_REPARSE_POINT, nil },
		}
		err := validateWindowsComponent(`C:\resident`, 1, true, api)
		var hard *HardSafetyError
		if !errors.As(err, &hard) || hard.Category != "symlink" || hard.Forceable() {
			t.Fatalf("reparse refusal = %v", err)
		}
	})

	t.Run("foreign owner", func(t *testing.T) {
		api := windowsComponentAPI{
			attributes: func(windows.Handle) (uint32, error) { return windows.FILE_ATTRIBUTE_DIRECTORY, nil },
			ownerSID:   func(windows.Handle) (string, error) { return "S-1-owner", nil },
			currentSID: func() (string, error) { return "S-1-current", nil },
		}
		err := validateWindowsComponent(`C:\resident`, 1, true, api)
		var hard *HardSafetyError
		if !errors.As(err, &hard) || hard.Category != "foreign-owner" || hard.Forceable() {
			t.Fatalf("foreign-owner refusal = %v", err)
		}
	})

	t.Run("ownership faults are not accepted", func(t *testing.T) {
		api := windowsComponentAPI{
			attributes: func(windows.Handle) (uint32, error) { return windows.FILE_ATTRIBUTE_DIRECTORY, nil },
			ownerSID:   func(windows.Handle) (string, error) { return "", errors.New("security descriptor fault") },
		}
		err := validateWindowsComponent(`C:\resident`, 1, true, api)
		if err == nil || !strings.Contains(err.Error(), "security descriptor fault") {
			t.Fatalf("owner fault = %v", err)
		}
	})

	t.Run("non-resident control ancestor checks reparse without owner lookup", func(t *testing.T) {
		ownerCalled := false
		api := windowsComponentAPI{
			attributes: func(windows.Handle) (uint32, error) { return windows.FILE_ATTRIBUTE_DIRECTORY, nil },
			ownerSID: func(windows.Handle) (string, error) {
				ownerCalled = true
				return "", nil
			},
		}
		if err := validateWindowsComponent(`C:\ancestor`, 1, false, api); err != nil {
			t.Fatal(err)
		}
		if ownerCalled {
			t.Fatal("non-resident ancestor unexpectedly required current ownership")
		}
	})
}
