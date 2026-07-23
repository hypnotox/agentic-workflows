// Package dashboardruntime builds and publishes the immutable runtime used by
// repository-local Pi dashboard queries.
package dashboardruntime

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	runtimeRef    = "refs/awf/dashboard-runtime"
	formatVersion = 1
	policySchema  = 1
	protocolMajor = 2
)

var (
	runtimeGitOutput = runGitOutput
	runtimePublish   = publish
	runtimeLookPath  = exec.LookPath
	runtimeAbs       = filepath.Abs
	runtimeCacheDir  = os.UserCacheDir
	runtimeRandRead  = rand.Read

	ErrUnsafePath        = errors.New("unsafe dashboard runtime path")
	ErrInvalidRef        = errors.New("invalid dashboard runtime ref")
	ErrBuild             = errors.New("dashboard runtime build failed")
	ErrCacheCollision    = errors.New("dashboard runtime cache collision")
	ErrSnapshot          = errors.New("dashboard runtime snapshot invalid")
	ErrConcurrentAdvance = errors.New("dashboard runtime ref advanced concurrently")
)

// BuildEnvironment fixes every host input that is allowed to affect a build.
type BuildEnvironment struct {
	GOOS, GOARCH, GoBinary, XDGCacheHome string
	Stderr                               io.Writer
}

// Launcher identifies one immutable published launcher.
type Launcher struct {
	Path, Commit, CacheKey, Diagnostic string
	Initialized                        bool
}

// AdvanceResult reports a successful compare-and-swap pointer advance.
type AdvanceResult struct {
	OldCommit, NewCommit, LauncherPath string
}

// Resolve resolves (and, on first use, initializes) the repository runtime ref.
func Resolve(root string, env BuildEnvironment) (Launcher, error) {
	env, err := normalizeEnvironment(env)
	if err != nil {
		return Launcher{}, err
	}
	objectFormat, err := runtimeGitOutput(root, "rev-parse", "--show-object-format")
	if err != nil {
		return Launcher{}, fmt.Errorf("%w: determine object format: %w", ErrInvalidRef, err)
	}
	commit, present := resolveCommit(root, runtimeRef)
	initialized := false
	diagnostic := ""
	if !present {
		head, ok := resolveCommit(root, "HEAD")
		if !ok {
			return Launcher{}, fmt.Errorf("%w: HEAD is not a commit", ErrInvalidRef)
		}
		zero := strings.Repeat("0", len(head))
		if _, updateErr := runtimeGitOutput(root, "update-ref", runtimeRef, head, zero); updateErr != nil {
			winner, ok := resolveCommit(root, runtimeRef)
			if !ok || winner != head {
				return Launcher{}, fmt.Errorf("%w: initialize %s: %w", ErrInvalidRef, runtimeRef, updateErr)
			}
			commit = winner
		} else {
			commit = head
			initialized = true
			diagnostic = fmt.Sprintf("initialized %s at %s", runtimeRef, commit)
			fmt.Fprintln(env.Stderr, diagnostic)
		}
	}
	if commit == "" { // coverage-ignore: resolveCommit reports present only for a non-empty commit ID
		return Launcher{}, fmt.Errorf("%w: %s does not name a commit", ErrInvalidRef, runtimeRef)
	}
	launcher, err := runtimePublish(root, commit, objectFormat, env)
	if err != nil {
		return Launcher{}, err
	}
	launcher.Initialized = initialized
	launcher.Diagnostic = diagnostic
	return launcher, nil
}

// Advance publishes revision and then advances the runtime ref with compare-and-swap.
func Advance(root, revision string, env BuildEnvironment) (AdvanceResult, error) {
	env, err := normalizeEnvironment(env)
	if err != nil {
		return AdvanceResult{}, err
	}
	objectFormat, err := runtimeGitOutput(root, "rev-parse", "--show-object-format")
	if err != nil {
		return AdvanceResult{}, fmt.Errorf("%w: determine object format: %w", ErrInvalidRef, err)
	}
	oldCommit, present := resolveCommit(root, runtimeRef)
	if !present {
		return AdvanceResult{}, fmt.Errorf("%w: %s is absent", ErrInvalidRef, runtimeRef)
	}
	newCommit, ok := resolveCommit(root, revision)
	if !ok {
		return AdvanceResult{}, fmt.Errorf("%w: revision %q does not peel to a commit", ErrInvalidRef, revision)
	}
	launcher, err := runtimePublish(root, newCommit, objectFormat, env)
	if err != nil {
		return AdvanceResult{}, err
	}
	if _, err := runtimeGitOutput(root, "update-ref", runtimeRef, newCommit, oldCommit); err != nil {
		return AdvanceResult{}, fmt.Errorf("%w: %w", ErrConcurrentAdvance, err)
	}
	return AdvanceResult{OldCommit: oldCommit, NewCommit: newCommit, LauncherPath: launcher.Path}, nil
}

func normalizeEnvironment(env BuildEnvironment) (BuildEnvironment, error) {
	if env.GOOS == "" {
		env.GOOS = runtime.GOOS
	}
	if env.GOARCH == "" {
		env.GOARCH = runtime.GOARCH
	}
	if env.GoBinary == "" {
		path, err := runtimeLookPath("go")
		if err != nil {
			return env, fmt.Errorf("%w: locate go: %w", ErrBuild, err)
		}
		env.GoBinary = path
	}
	absoluteGo, err := runtimeAbs(env.GoBinary)
	if err != nil {
		return env, fmt.Errorf("%w: Go binary path: %w", ErrUnsafePath, err)
	}
	env.GoBinary = absoluteGo
	if env.XDGCacheHome == "" {
		cache, err := runtimeCacheDir()
		if err != nil {
			return env, fmt.Errorf("%w: locate cache: %w", ErrUnsafePath, err)
		}
		env.XDGCacheHome = cache
	}
	if !filepath.IsAbs(env.XDGCacheHome) {
		return env, fmt.Errorf("%w: cache root must be absolute", ErrUnsafePath)
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	return env, nil
}

func resolveCommit(root, revision string) (string, bool) {
	if revision == "" || strings.HasPrefix(revision, "-") {
		return "", false
	}
	value, err := runtimeGitOutput(root, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if err != nil || value == "" {
		return "", false
	}
	return value, true
}

func runGitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func materialize(root, commit, destination string) error {
	command := exec.Command("git", "-C", root, "archive", "--format=tar", commit)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("%w: archive commit: %w", ErrBuild, err)
	}
	return extractArchive(bytes.NewReader(output), destination)
}

func extractArchive(archive io.Reader, destination string) error {
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: read archive: %w", ErrBuild, err)
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%w: archive member %q", ErrUnsafePath, header.Name)
		}
		path := filepath.Join(destination, clean)
		if !pathWithin(destination, path) { // coverage-ignore: a cleaned relative non-parent member joined to destination cannot escape it
			return fmt.Errorf("%w: archive member %q", ErrUnsafePath, header.Name)
		}
		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o700); err != nil {
				return fmt.Errorf("%w: create archive directory: %w", ErrBuild, err)
			}
		case tar.TypeReg, 0:
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("%w: create archive parent: %w", ErrBuild, err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o700|0o600)
			if err != nil {
				return fmt.Errorf("%w: create archive file: %w", ErrBuild, err)
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				return fmt.Errorf("%w: extract archive file: %w", ErrBuild, errors.Join(copyErr, closeErr))
			}
		default:
			return fmt.Errorf("%w: unsupported archive member %q", ErrUnsafePath, header.Name)
		}
	}
}

func randomStagingSuffix() (string, error) {
	value := make([]byte, 16)
	if _, err := runtimeRandRead(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func digestFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digestBytes(content), nil
}

func compactJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
