package filesystem

import "testing"

func TestNormalizePlatformPathTrustsOnlyDarwinPrivateVarAlias(t *testing.T) {
	for _, test := range []struct {
		name, goos, input, want string
	}{
		{name: "Darwin var root", goos: "darwin", input: "/var", want: "/private/var"},
		{name: "Darwin var descendant", goos: "darwin", input: "/var/folders/fixture", want: "/private/var/folders/fixture"},
		{name: "Darwin similar prefix", goos: "darwin", input: "/variable/fixture", want: "/variable/fixture"},
		{name: "Darwin embedded component", goos: "darwin", input: "/tmp/var/fixture", want: "/tmp/var/fixture"},
		{name: "Darwin canonical path", goos: "darwin", input: "/private/var/folders/fixture", want: "/private/var/folders/fixture"},
		{name: "Linux var descendant", goos: "linux", input: "/var/folders/fixture", want: "/var/folders/fixture"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizePlatformPath(test.goos, test.input); got != test.want {
				t.Fatalf("normalizePlatformPath(%q, %q) = %q, want %q", test.goos, test.input, got, test.want)
			}
		})
	}
}
