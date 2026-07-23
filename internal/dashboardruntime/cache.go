package dashboardruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

var (
	cacheMkdir         = os.Mkdir
	cacheMkdirTemp     = os.MkdirTemp
	cacheChmod         = os.Chmod
	cacheRemoveAll     = os.RemoveAll
	cacheRename        = os.Rename
	cacheReadFile      = os.ReadFile
	cacheReadDir       = os.ReadDir
	cacheLstat         = os.Lstat
	cacheOpenFile      = os.OpenFile
	cacheOpen          = os.Open
	cacheEvalSymlinks  = filepath.EvalSymlinks
	cacheAcquireLock   = acquireAdvisoryLock
	cachePathExists    = pathExists
	cacheRemoveStaging = removeIncompleteStaging
	cacheRandomSuffix  = randomStagingSuffix
	cacheMaterialize   = materialize
	cacheReadPolicy    = readPolicySnapshot
	cacheBuildArtifact = buildArtifact
	cacheWriteSynced   = writeSyncedFile
	cacheDigestFile    = digestFile
	cacheSyncFile      = syncFile
	cacheSyncDirectory = syncDirectory
)

type runtimeMetadata struct {
	FormatVersion  int      `json:"formatVersion"`
	RepositoryID   string   `json:"repositoryID"`
	ObjectFormat   string   `json:"objectFormat"`
	Commit         string   `json:"commit"`
	GoVersion      string   `json:"goVersion"`
	GoExperiment   string   `json:"goExperiment"`
	GoToolchain    string   `json:"goToolchain"`
	GOOS           string   `json:"goos"`
	GOARCH         string   `json:"goarch"`
	GoFlags        []string `json:"goFlags"`
	PolicySchema   int      `json:"policySchema"`
	ProtocolMajor  int      `json:"protocolMajor"`
	BinarySHA256   string   `json:"binarySHA256"`
	LauncherSHA256 string   `json:"launcherSHA256"`
	PolicySHA256   string   `json:"policySHA256"`
}

func publish(root, commit, objectFormat string, env BuildEnvironment) (Launcher, error) {
	repositoryID, err := repositoryIdentity(root)
	if err != nil {
		return Launcher{}, err
	}
	goVersion, goExperiment, goToolchain, err := inspectGo(env)
	if err != nil {
		return Launcher{}, err
	}
	metadata := runtimeMetadata{
		FormatVersion: formatVersion, RepositoryID: repositoryID, ObjectFormat: objectFormat, Commit: commit,
		GoVersion: goVersion, GoExperiment: goExperiment, GoToolchain: goToolchain,
		GOOS: env.GOOS, GOARCH: env.GOARCH, GoFlags: []string{"-buildvcs=false"},
		PolicySchema: policySchema, ProtocolMajor: protocolMajor,
	}
	keyMaterial, err := compactJSON(metadata)
	if err != nil { // coverage-ignore: runtimeMetadata contains only JSON-supported scalar and slice fields
		return Launcher{}, fmt.Errorf("%w: encode key metadata: %w", ErrSnapshot, err)
	}
	key := digestBytes(append(append(keyMaterial, '\n'), commit...))
	cacheRoot := filepath.Join(env.XDGCacheHome, "awf", "dashboard-runtime", "v1")
	if err := ensurePrivateCacheRoot(env.XDGCacheHome, cacheRoot); err != nil {
		return Launcher{}, err
	}
	lockPath := filepath.Join(cacheRoot, key+".lock")
	lock, err := cacheAcquireLock(lockPath)
	if err != nil {
		return Launcher{}, err
	}
	defer lock.release()

	entry := filepath.Join(cacheRoot, key)
	if exists, err := cachePathExists(entry); err != nil {
		return Launcher{}, fmt.Errorf("%w: inspect published entry: %w", ErrUnsafePath, err)
	} else if exists {
		if err := validatePublishedEntry(entry, metadata, env); err != nil {
			return Launcher{}, err
		}
		return Launcher{Path: filepath.Join(entry, artifactName("awf-dashboard", env.GOOS)), Commit: commit, CacheKey: key}, nil
	}
	if err := cacheRemoveStaging(cacheRoot, key); err != nil {
		return Launcher{}, err
	}
	suffix, err := cacheRandomSuffix()
	if err != nil {
		return Launcher{}, fmt.Errorf("%w: staging nonce: %w", ErrBuild, err)
	}
	staging := filepath.Join(cacheRoot, "."+key+".tmp-"+suffix)
	if err := cacheMkdir(staging, 0o700); err != nil {
		return Launcher{}, fmt.Errorf("%w: create staging: %w", ErrBuild, err)
	}
	published := false
	defer func() {
		if !published {
			_ = cacheRemoveAll(staging)
		}
	}()

	buildRoot, err := cacheMkdirTemp("", "awf-dashboard-runtime-build-")
	if err != nil {
		return Launcher{}, fmt.Errorf("%w: create isolated build directory: %w", ErrBuild, err)
	}
	if err := cacheChmod(buildRoot, 0o700); err != nil {
		_ = cacheRemoveAll(buildRoot)
		return Launcher{}, fmt.Errorf("%w: secure isolated build directory: %w", ErrUnsafePath, err)
	}
	defer func() { _ = cacheRemoveAll(buildRoot) }()
	if err := cacheMaterialize(root, commit, buildRoot); err != nil {
		return Launcher{}, err
	}
	policy, err := cacheReadPolicy(buildRoot)
	if err != nil {
		return Launcher{}, err
	}
	awfPath := filepath.Join(staging, artifactName("awf", env.GOOS))
	launcherPath := filepath.Join(staging, artifactName("awf-dashboard", env.GOOS))
	if err := cacheBuildArtifact(buildRoot, "./cmd/awf", awfPath, env); err != nil {
		return Launcher{}, err
	}
	if err := cacheBuildArtifact(buildRoot, "./cmd/awf-dashboard-launcher", launcherPath, env); err != nil {
		return Launcher{}, err
	}
	if err := cacheWriteSynced(filepath.Join(staging, "policy.json"), policy, 0o600); err != nil {
		return Launcher{}, fmt.Errorf("%w: write policy: %w", ErrSnapshot, err)
	}
	metadata.BinarySHA256, err = cacheDigestFile(awfPath)
	if err != nil {
		return Launcher{}, fmt.Errorf("%w: digest awf: %w", ErrSnapshot, err)
	}
	metadata.LauncherSHA256, err = cacheDigestFile(launcherPath)
	if err != nil {
		return Launcher{}, fmt.Errorf("%w: digest launcher: %w", ErrSnapshot, err)
	}
	metadata.PolicySHA256 = digestBytes(policy)
	metadataBytes, err := compactJSON(metadata)
	if err != nil { // coverage-ignore: runtimeMetadata contains only JSON-supported scalar and slice fields
		return Launcher{}, fmt.Errorf("%w: encode metadata: %w", ErrSnapshot, err)
	}
	if err := cacheWriteSynced(filepath.Join(staging, "metadata.json"), metadataBytes, 0o600); err != nil {
		return Launcher{}, fmt.Errorf("%w: write metadata: %w", ErrSnapshot, err)
	}
	if err := cacheSyncFile(awfPath); err != nil {
		return Launcher{}, fmt.Errorf("%w: flush awf: %w", ErrBuild, err)
	}
	if err := cacheSyncFile(launcherPath); err != nil {
		return Launcher{}, fmt.Errorf("%w: flush launcher: %w", ErrBuild, err)
	}
	if err := cacheSyncDirectory(staging); err != nil {
		return Launcher{}, fmt.Errorf("%w: flush staging: %w", ErrBuild, err)
	}
	if err := cacheRename(staging, entry); err != nil {
		if exists, statErr := cachePathExists(entry); statErr == nil && exists {
			expected := metadata
			expected.BinarySHA256, expected.LauncherSHA256, expected.PolicySHA256 = "", "", ""
			if validateErr := validatePublishedEntry(entry, expected, env); validateErr == nil {
				return Launcher{Path: filepath.Join(entry, artifactName("awf-dashboard", env.GOOS)), Commit: commit, CacheKey: key}, nil
			}
		}
		return Launcher{}, fmt.Errorf("%w: publish entry: %w", ErrCacheCollision, err)
	}
	published = true
	if err := cacheSyncDirectory(cacheRoot); err != nil {
		return Launcher{}, fmt.Errorf("%w: flush cache directory: %w", ErrBuild, err)
	}
	return Launcher{Path: filepath.Join(entry, artifactName("awf-dashboard", env.GOOS)), Commit: commit, CacheKey: key}, nil
}

func repositoryIdentity(root string) (string, error) {
	common, err := runGitOutput(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("%w: determine repository identity: %w", ErrInvalidRef, err)
	}
	canonical, err := cacheEvalSymlinks(common)
	if err != nil {
		return "", fmt.Errorf("%w: canonicalize repository identity: %w", ErrUnsafePath, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil { // coverage-ignore: git --path-format=absolute and EvalSymlinks produce an absolute path
		return "", fmt.Errorf("%w: absolute repository identity: %w", ErrUnsafePath, err)
	}
	return filepath.Clean(canonical), nil
}

func inspectGo(env BuildEnvironment) (string, string, string, error) {
	version, err := runGo(env, "version")
	if err != nil {
		return "", "", "", err
	}
	values, err := runGo(env, "env", "GOEXPERIMENT", "GOTOOLCHAIN")
	if err != nil {
		return "", "", "", err
	}
	parts := strings.Split(strings.TrimSuffix(values, "\n"), "\n")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("%w: unexpected go env output", ErrBuild)
	}
	return strings.TrimSpace(version), strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func runGo(env BuildEnvironment, args ...string) (string, error) {
	command := exec.Command(env.GoBinary, args...)
	command.Env = normalizedCommandEnvironment(env)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: go %s: %w: %s", ErrBuild, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func buildArtifact(root, packagePath, output string, env BuildEnvironment) error {
	command := exec.Command(env.GoBinary, "build", "-buildvcs=false", "-o", output, packagePath)
	command.Dir = root
	command.Env = normalizedCommandEnvironment(env)
	combined, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: build %s: %w: %s", ErrBuild, packagePath, err, strings.TrimSpace(string(combined)))
	}
	return nil
}

func normalizedCommandEnvironment(env BuildEnvironment) []string {
	blocked := map[string]bool{"GOWORK": true, "GOFLAGS": true, "CGO_ENABLED": true, "GOOS": true, "GOARCH": true}
	result := make([]string, 0, len(os.Environ())+5)
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		if !blocked[name] {
			result = append(result, item)
		}
	}
	return append(result, "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=0", "GOOS="+env.GOOS, "GOARCH="+env.GOARCH)
}

func validatePublishedEntry(entry string, expected runtimeMetadata, env BuildEnvironment) error {
	info, err := cacheLstat(entry)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: published entry is not a private directory", ErrUnsafePath)
	}
	entries, err := cacheReadDir(entry)
	if err != nil {
		return fmt.Errorf("%w: read published entry: %w", ErrUnsafePath, err)
	}
	gotNames := make([]string, 0, len(entries))
	for _, item := range entries {
		gotNames = append(gotNames, item.Name())
		itemInfo, statErr := cacheLstat(filepath.Join(entry, item.Name()))
		if statErr != nil || !itemInfo.Mode().IsRegular() || itemInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: published member %q is not regular", ErrUnsafePath, item.Name())
		}
	}
	sort.Strings(gotNames)
	wantNames := []string{artifactName("awf", env.GOOS), artifactName("awf-dashboard", env.GOOS), "metadata.json", "policy.json"}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		return fmt.Errorf("%w: published member set differs", ErrCacheCollision)
	}
	metadataBytes, err := cacheReadFile(filepath.Join(entry, "metadata.json"))
	if err != nil {
		return fmt.Errorf("%w: read metadata: %w", ErrSnapshot, err)
	}
	var actual runtimeMetadata
	if err := json.Unmarshal(metadataBytes, &actual); err != nil {
		return fmt.Errorf("%w: decode metadata: %w", ErrCacheCollision, err)
	}
	canonical, _ := compactJSON(actual)
	if !bytes.Equal(metadataBytes, canonical) {
		return fmt.Errorf("%w: metadata is not canonical", ErrCacheCollision)
	}
	actualBase := actual
	actualBase.BinarySHA256, actualBase.LauncherSHA256, actualBase.PolicySHA256 = "", "", ""
	if !reflect.DeepEqual(actualBase, expected) {
		return fmt.Errorf("%w: metadata does not match cache key", ErrCacheCollision)
	}
	checks := []struct{ name, digest string }{
		{artifactName("awf", env.GOOS), actual.BinarySHA256},
		{artifactName("awf-dashboard", env.GOOS), actual.LauncherSHA256},
		{"policy.json", actual.PolicySHA256},
	}
	for _, check := range checks {
		digest, err := digestFile(filepath.Join(entry, check.name))
		if err != nil || digest != check.digest {
			return fmt.Errorf("%w: digest mismatch for %s", ErrSnapshot, check.name)
		}
	}
	policy, err := cacheReadFile(filepath.Join(entry, "policy.json"))
	if err != nil || !canonicalJSON(policy) {
		return fmt.Errorf("%w: policy is not canonical JSON", ErrSnapshot)
	}
	return nil
}

func removeIncompleteStaging(cacheRoot, key string) error {
	entries, err := cacheReadDir(cacheRoot)
	if err != nil {
		return fmt.Errorf("%w: read cache staging: %w", ErrUnsafePath, err)
	}
	prefix := "." + key + ".tmp-"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || len(entry.Name()) != len(prefix)+32 {
			continue
		}
		suffix := strings.TrimPrefix(entry.Name(), prefix)
		if strings.Trim(suffix, "0123456789abcdef") != "" {
			continue
		}
		path := filepath.Join(cacheRoot, entry.Name())
		info, err := cacheLstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: unsafe staging entry", ErrUnsafePath)
		}
		if err := cacheRemoveAll(path); err != nil {
			return fmt.Errorf("%w: remove incomplete staging: %w", ErrBuild, err)
		}
	}
	return nil
}

func artifactName(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

func pathExists(path string) (bool, error) {
	_, err := cacheLstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func writeSyncedFile(path string, content []byte, mode os.FileMode) error {
	file, err := cacheOpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = file.Write(content); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func syncFile(path string) error {
	file, err := cacheOpen(path)
	if err != nil {
		return err
	}
	err = file.Sync()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func syncDirectory(path string) error {
	directory, err := cacheOpen(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	if closeErr := directory.Close(); err == nil {
		err = closeErr
	}
	return err
}
