package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryResidentProtocol1Preflight(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	protocol1, err := residentProtocol1Efforts(root)
	if err != nil {
		t.Fatalf("inspect repository-resident telemetry: %v", err)
	}
	if len(protocol1) != 0 {
		t.Fatalf("protocol-1 resident efforts require explicit confirmed purge; automatic cleanup refused: %v", protocol1)
	}
}

func TestProtocol1PreflightNeverRemovesResidentEffort(t *testing.T) {
	root := newTestProject(t)
	stream := filepath.Join(root, ".awf", "metrics", "efforts", "legacy", "sessions", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(stream), 0o700); err != nil {
		t.Fatal(err)
	}
	const resident = `{"version":{"major":1,"minor":0},"eventId":"legacy"}` + "\n"
	if err := os.WriteFile(stream, []byte(resident), 0o600); err != nil {
		t.Fatal(err)
	}
	protocol1, err := residentProtocol1Efforts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(protocol1) != 1 || protocol1[0] != "legacy" {
		t.Fatalf("protocol-1 preflight = %v", protocol1)
	}
	if raw, err := os.ReadFile(stream); err != nil || string(raw) != resident {
		t.Fatalf("preflight changed resident evidence: %q, %v", raw, err)
	}
}

func residentProtocol1Efforts(root string) ([]string, error) {
	effortsRoot := filepath.Join(root, ".awf", "metrics", "efforts")
	protocol1 := map[string]bool{}
	err := filepath.WalkDir(effortsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		relative, err := filepath.Rel(effortsRoot, path)
		if err != nil {
			return err
		}
		parts := splitPath(relative)
		if len(parts) < 3 || parts[1] != "sessions" {
			return fmt.Errorf("unexpected resident stream path %s", relative)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var envelope struct {
				Version ProtocolVersion `json:"version"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
				_ = file.Close()
				return fmt.Errorf("decode %s: %w", relative, err)
			}
			if envelope.Version.Major == 1 {
				protocol1[parts[0]] = true
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(protocol1))
	for effortID := range protocol1 {
		result = append(result, effortID)
	}
	sortStrings(result)
	return result, nil
}

func splitPath(path string) []string {
	parts := []string{}
	for path != "." && path != "" {
		directory, base := filepath.Split(path)
		parts = append([]string{base}, parts...)
		path = filepath.Clean(directory)
	}
	return parts
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}
