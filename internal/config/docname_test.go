package config

import "testing"

func TestValidateDocName(t *testing.T) {
	valid := []string{"ci", "release-process", "guides/foo", "guides/sub/bar", "a1"}
	for _, n := range valid {
		if err := ValidateDocName(n); err != nil {
			t.Errorf("ValidateDocName(%q) = %v, want nil", n, err)
		}
	}
	invalid := []string{"", "Foo", "guides/Foo", "a.md", "../x", "a..b", "/x", "x/", "a//b", "_base", "guides/_base", "under_score"}
	for _, n := range invalid {
		if err := ValidateDocName(n); err == nil {
			t.Errorf("ValidateDocName(%q) = nil, want error", n)
		}
	}
}
