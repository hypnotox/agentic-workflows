package git

import "testing"

// fixtureAllowlist is where a test file may reach Git directly. gitfixture is
// the single fixture home and owns both lanes; internal/git's own suites drive
// the mechanism the seam wraps, so they exercise what everyone else must not.
//
// internal/testsupport/deps_test.go deliberately does NOT need an entry. It
// carries a Git library path as fixture data, not as an import. This walker
// reads the import list, so that string constant is invisible to it and a
// carve-out would shield nothing.
//
// TestFixtureAllowlistEntriesAreAllLoadBearing proves that every retained
// entry remains necessary and prevents harmless-looking carve-outs from
// weakening the boundary.
var fixtureAllowlist = []string{
	"internal/testsupport/gitfixture/",
	"internal/git/",
}

// TestNoTestGitAccessOutsideTheFixtureHome fails when any test file builds Git
// state without gitfixture. Test fixtures were the larger half of the fork this
// effort removed: fourteen files imported go-git directly and nine more shelled
// out to git, each with its own idea of isolation, and two of them survived the
// conversion pass because the grep meant to find them could not match the
// argument shape they used. A structural walker cannot miss it that way.
// invariant: tooling/git-access:fixture-single-home (TestNoTestGitAccessOutsideTheFixtureHome)
func TestNoTestGitAccessOutsideTheFixtureHome(t *testing.T) {
	t.Parallel()
	findings, seen := walkGitAccess(t, true, fixtureAllowlist)
	if seen == 0 {
		t.Fatal("walked no test files, so the walk proves nothing")
	}
	for _, f := range findings {
		t.Errorf("%s", f)
	}
}

// TestFixtureAllowlistEntriesAreAllLoadBearing fails when an allowlist entry
// stops being needed. An allowlist that outlives its reason silently widens the
// hole it was carved for, so each entry must still shield a real finding.
func TestFixtureAllowlistEntriesAreAllLoadBearing(t *testing.T) {
	t.Parallel()
	for _, entry := range fixtureAllowlist {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			narrowed := []string{}
			for _, keep := range fixtureAllowlist {
				if keep != entry {
					narrowed = append(narrowed, keep)
				}
			}
			findings, _ := walkGitAccess(t, true, narrowed)
			if len(findings) == 0 {
				t.Errorf("allowlist entry %q shields nothing; remove it rather than leave the carve-out open", entry)
			}
		})
	}
}
