package filesystem

import (
	"path/filepath"
	"runtime"
	"strings"
)

const (
	darwinVarAlias   = "/var"
	darwinPrivateVar = "/private/var"
)

// NormalizePlatformPath returns the stable spelling for trusted operating-system
// path aliases without resolving caller-controlled symbolic links.
func NormalizePlatformPath(value string) string {
	return normalizePlatformPath(runtime.GOOS, value)
}

func normalizePlatformPath(goos, value string) string {
	clean := filepath.Clean(value)
	if goos != "darwin" {
		return clean
	}
	if clean == darwinVarAlias {
		return darwinPrivateVar
	}
	prefix := darwinVarAlias + string(filepath.Separator)
	if !strings.HasPrefix(clean, prefix) {
		return clean
	}
	return filepath.Join(darwinPrivateVar, strings.TrimPrefix(clean, prefix))
}
