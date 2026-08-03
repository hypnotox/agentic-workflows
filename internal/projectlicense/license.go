package projectlicense

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

const (
	licensePath    = "LICENSE"
	readmePath     = "README.md"
	goreleaserPath = ".goreleaser.yaml"
	licenseSHA256  = "d8a6cc31abc16b6748c7a21f21611f5a1ec33f67d22ca23d7da1c19b95496bee"
	licenseBytes   = 34020
	licenseBadge   = "[![License: AGPL-3.0-only]"
	licenseFooter  = "[GNU Affero General Public License v3.0 only](LICENSE)"
)

// Verify checks the root-owned AGPL-3.0-only license artifacts used for project releases.
func Verify(root fs.FS) error {
	license, err := fs.ReadFile(root, licensePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", licensePath, err)
	}

	var errs []error
	if len(license) != licenseBytes {
		errs = append(errs, fmt.Errorf("%s has %d bytes, want %d", licensePath, len(license), licenseBytes))
	}
	if !strings.HasSuffix(string(license), "\n") || strings.HasSuffix(string(license), "\n\n") {
		errs = append(errs, fmt.Errorf("%s must end with exactly one newline", licensePath))
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(license)); got != licenseSHA256 {
		errs = append(errs, fmt.Errorf("%s SHA-256 is %s, want %s", licensePath, got, licenseSHA256))
	}

	readme, err := fs.ReadFile(root, readmePath)
	if err != nil {
		errs = append(errs, fmt.Errorf("read %s: %w", readmePath, err))
	} else {
		if !strings.Contains(string(readme), licenseBadge) {
			errs = append(errs, fmt.Errorf("%s lacks the AGPL-3.0-only badge", readmePath))
		}
		if !strings.Contains(string(readme), licenseFooter) {
			errs = append(errs, fmt.Errorf("%s lacks the AGPL-3.0-only footer", readmePath))
		}
	}

	goreleaser, err := fs.ReadFile(root, goreleaserPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("read %s: %w", goreleaserPath, err))
	} else if !strings.Contains(string(goreleaser), "files:\n      - LICENSE") {
		errs = append(errs, fmt.Errorf("%s archives must include %s", goreleaserPath, licensePath))
	}

	// Only root-owned project license artifacts are policy inputs. Dependency metadata
	// and retained third-party notices remain outside this verification by design.
	for _, artifact := range []struct {
		path string
		data []byte
	}{
		{readmePath, readme},
		{goreleaserPath, goreleaser},
	} {
		if strings.Contains(string(artifact.data), "MIT") {
			errs = append(errs, fmt.Errorf("%s contains an obsolete project MIT reference", artifact.path))
		}
	}
	return errors.Join(errs...)
}
