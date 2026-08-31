package fsfixture

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, faults ...Fault) (*Handle, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dir", "file"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := Open(root, faults...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	})
	return h, root
}

func TestOpenValidation(t *testing.T) {
	sentinel := errors.New("fault")
	for _, tc := range []struct {
		faults []Fault
		want   string
	}{
		{[]Fault{{Operation: "bad", Err: sentinel}}, `fsfixture: fault 0: unknown operation "bad"`},
		{[]Fault{{Operation: OperationRead}}, "fsfixture: fault 0: nil error"},
		{[]Fault{{Operation: OperationRead, Path: "..", Err: sentinel}}, `fsfixture: fault 0: invalid path ".."`},
		{[]Fault{{Operation: OperationRead, Path: "x", Err: sentinel}, {Operation: OperationRead, Path: "x", Err: sentinel}}, `fsfixture: fault 1: duplicate read fault for "x"`},
		// Each validation stage wins over every later applicable failure.
		{[]Fault{{Operation: "bad", Path: ".."}}, `fsfixture: fault 0: unknown operation "bad"`},
		{[]Fault{{Operation: OperationRead, Path: ".."}}, "fsfixture: fault 0: nil error"},
		{[]Fault{{Operation: OperationRead, Path: "..", Err: sentinel}, {Operation: OperationRead, Path: "..", Err: sentinel}}, `fsfixture: fault 0: invalid path ".."`},
		{[]Fault{{Operation: OperationRead, Path: "x", Err: sentinel}, {Operation: OperationRead, Path: "x", Err: sentinel}}, `fsfixture: fault 1: duplicate read fault for "x"`},
	} {
		if _, err := Open(t.TempDir(), tc.faults...); err == nil || err.Error() != tc.want {
			t.Fatalf("error = %v, want %q", err, tc.want)
		}
	}
	if _, err := Open(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing root succeeded")
	}
}

func TestFaultOperationsAndDelegation(t *testing.T) {
	for _, operation := range []Operation{OperationWalk, OperationWalkInfo, OperationRead, OperationInfo, OperationLinkInfo} {
		t.Run(string(operation), func(t *testing.T) {
			sentinel := errors.New("sentinel")
			path := "dir/file"
			if operation == OperationWalk || operation == OperationWalkInfo {
				path = "dir"
			}
			h, _ := fixture(t, Fault{Operation: operation, Path: path, Err: sentinel})
			var err error
			switch operation {
			case OperationWalk, OperationWalkInfo:
				err = h.Walk(".", func(string, fs.FileInfo) (bool, error) { return true, nil })
			case OperationRead:
				_, err = h.Read(path)
			case OperationInfo:
				_, err = h.Info(path)
			case OperationLinkInfo:
				_, err = h.LinkInfo(path)
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("identity: %v", err)
			}
		})
	}
	h, root := fixture(t, Fault{Operation: OperationRead, Path: "other", Err: errors.New("no")})
	if _, err := h.Read("dir/file"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dir/file", filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	link, err := h.LinkInfo("link")
	if err != nil || link.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("link metadata = %v, %v", link, err)
	}
	if info, err := h.Info("link"); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("followed metadata = %v, %v", info, err)
	}
	if err := h.Walk(".", func(string, fs.FileInfo) (bool, error) { return true, nil }); err != nil {
		t.Fatal(err)
	}
}

func TestFixtureWalkParity(t *testing.T) {
	h, root := fixture(t)
	if err := os.Symlink("dir", filepath.Join(root, "inside")); err != nil {
		t.Skip(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skip(err)
	}
	seen := map[string]bool{}
	modes := map[string]fs.FileMode{}
	if err := h.Walk(".", func(path string, info fs.FileInfo) (bool, error) {
		seen[path] = true
		modes[path] = info.Mode()
		return path != "dir", nil
	}); err != nil {
		t.Fatal(err)
	}
	if seen["dir/file"] || seen["inside/file"] || seen["escape/file"] {
		t.Fatalf("walk traversed skipped directory or symlink: %v", seen)
	}
	for _, path := range []string{"inside", "escape"} {
		if modes[path]&fs.ModeSymlink == 0 {
			t.Fatalf("walk metadata for %s = %v, want symlink", path, modes[path])
		}
	}
	for _, subtree := range []string{"inside", "escape"} {
		calls := 0
		if err := h.Walk(subtree, func(path string, info fs.FileInfo) (bool, error) {
			calls++
			if path != subtree || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("Walk(%q) = %q %v", subtree, path, info.Mode())
			}
			return true, nil
		}); err != nil {
			t.Fatalf("Walk(%q): %v", subtree, err)
		}
		if calls != 1 {
			t.Fatalf("Walk(%q) callbacks = %d, want 1", subtree, calls)
		}
		sentinel := errors.New("callback")
		if err := h.Walk(subtree, func(string, fs.FileInfo) (bool, error) { return true, sentinel }); !errors.Is(err, sentinel) {
			t.Fatalf("Walk(%q) callback = %v", subtree, err)
		}
	}
	fileSeen := false
	if err := h.Walk("dir/file", func(string, fs.FileInfo) (bool, error) { fileSeen = true; return false, nil }); err != nil || !fileSeen {
		t.Fatalf("nondirectory false = %v, seen=%t", err, fileSeen)
	}
	sentinel := errors.New("callback")
	if err := h.Walk(".", func(string, fs.FileInfo) (bool, error) { return false, sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestFixtureWalkRootFaultOrdering(t *testing.T) {
	for _, operation := range []Operation{OperationWalk, OperationWalkInfo} {
		t.Run(string(operation), func(t *testing.T) {
			sentinel := errors.New("root fault")
			h, _ := fixture(t, Fault{Operation: operation, Path: "dir", Err: sentinel})
			if err := h.Walk("dir", func(string, fs.FileInfo) (bool, error) { return true, nil }); !errors.Is(err, sentinel) {
				t.Fatalf("%s root fault = %v", operation, err)
			}
		})
	}
	for _, link := range []struct {
		name, target string
	}{
		{"internal symlink", "dir"},
		{"escaping symlink", t.TempDir()},
	} {
		for _, operation := range []Operation{OperationWalk, OperationWalkInfo} {
			t.Run(link.name+"/"+string(operation), func(t *testing.T) {
				sentinel := errors.New("symlink root fault")
				h, root := fixture(t, Fault{Operation: operation, Path: "link", Err: sentinel})
				if err := os.Symlink(link.target, filepath.Join(root, "link")); err != nil {
					t.Skip(err)
				}
				callbacks := 0
				err := h.Walk("link", func(string, fs.FileInfo) (bool, error) {
					callbacks++
					return true, nil
				})
				if !errors.Is(err, sentinel) {
					t.Fatalf("%s %s root fault = %v", link.name, operation, err)
				}
				if callbacks != 0 {
					t.Fatalf("%s %s callbacks = %d, want 0", link.name, operation, callbacks)
				}
			})
		}
	}
}

func TestFixtureErrors(t *testing.T) {
	h, _ := fixture(t)
	for _, operation := range []Operation{OperationRead, OperationInfo, OperationLinkInfo} {
		var err error
		switch operation {
		case OperationRead:
			_, err = h.Read("..")
		case OperationInfo:
			_, err = h.Info("..")
		default:
			_, err = h.LinkInfo("..")
		}
		if err == nil {
			t.Fatal(operation)
		}
	}
	if err := h.Walk("", func(string, fs.FileInfo) (bool, error) { return true, nil }); err == nil {
		t.Fatal("invalid walk")
	}
	for _, operation := range []func(string) error{func(path string) error {
		return h.Walk(path, func(string, fs.FileInfo) (bool, error) { return true, nil })
	}, func(path string) error { _, err := h.Read(path); return err }, func(path string) error { _, err := h.Info(path); return err }, func(path string) error { _, err := h.LinkInfo(path); return err }} {
		if err := operation("missing"); err == nil {
			t.Fatal("missing operation succeeded")
		}
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFaultsPreserveIdentityAndDelegate(t *testing.T) {
	for _, op := range []Operation{OperationWalk, OperationWalkInfo, OperationRead, OperationInfo, OperationLinkInfo} {
		sentinel := errors.New("selected " + string(op))
		h, _ := fixture(t, Fault{Operation: op, Path: faultPath(op), Err: sentinel})
		if err := invokeFixtureOperation(h, op, faultPath(op)); !errors.Is(err, sentinel) {
			t.Fatalf("%s selected fault identity: %v", op, err)
		}
		h, _ = fixture(t, Fault{Operation: op, Path: "other", Err: errors.New("fault")})
		if err := invokeFixtureOperation(h, op, "dir/file"); err != nil {
			t.Fatalf("%s did not delegate: %v", op, err)
		}
	}
}

func faultPath(op Operation) string {
	if op == OperationWalk || op == OperationWalkInfo {
		return "dir"
	}
	return "dir/file"
}

func invokeFixtureOperation(h *Handle, op Operation, path string) error {
	switch op {
	case OperationWalk, OperationWalkInfo:
		return h.Walk(".", func(string, fs.FileInfo) (bool, error) { return true, nil })
	case OperationRead:
		_, err := h.Read(path)
		return err
	case OperationInfo:
		_, err := h.Info(path)
		return err
	case OperationLinkInfo:
		_, err := h.LinkInfo(path)
		return err
	}
	return nil
}
