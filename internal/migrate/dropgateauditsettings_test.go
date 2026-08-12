package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: config/migrations-and-locks:toggle-keys-dropped (TestDropGateAuditSettings)
func TestDropGateAuditSettings(t *testing.T) {
	root := t.TempDir()
	src := "# keep\nprefix: example\nhooks:\n  enabled: true\nrunner:\n  enabled: false\nproseGate:\n  enabled: true\n  exemptions: []\nmemoryCite:\n  enabled: false\n  exemptions: []\naudit:\n  allowedTypes: [feat]\n  subjectMaxLength: 9\n  diffThreshold: 1\n  dependencyManifests: [go.mod]\n  domainDocStaleness: false\n  domainCodeStaleness: false\n  undocumentedDomain: false\n  plainPunctuation: false\n  uncommittedChanges: false\n  allowedScopes: [awf]\ncurrentState:\n  maxTopicsPerPath: 3\n  testGlobs: ['**/*_test.go']\n"
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), src)
	var changes Changes
	if err := applyDropGateAuditSettings(root, &changes); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(bytes)
	for _, key := range []string{"hooks:", "runner:", "enabled:", "allowedTypes:", "subjectMaxLength:", "diffThreshold:", "dependencyManifests:", "domainDocStaleness:", "domainCodeStaleness:", "undocumentedDomain:", "plainPunctuation:", "uncommittedChanges:", "maxTopicsPerPath:"} {
		if strings.Contains(got, key) {
			t.Errorf("retired key %q remains:\n%s", key, got)
		}
	}
	want := "# keep\nprefix: example\nproseGate:\n  exemptions: []\nmemoryCite:\n  exemptions: []\naudit:\n  allowedScopes: [awf]\ncurrentState:\n  testGlobs: ['**/*_test.go']\n"
	if got != want {
		t.Errorf("migration changed surviving bytes:\n got %q\nwant %q", got, want)
	}
	wantChanges := "drop-gate-audit-settings: removed hooks\n" +
		"drop-gate-audit-settings: removed runner\n" +
		"drop-gate-audit-settings: removed proseGate.enabled\n" +
		"drop-gate-audit-settings: removed memoryCite.enabled\n" +
		"drop-gate-audit-settings: removed audit.allowedTypes\n" +
		"drop-gate-audit-settings: removed audit.subjectMaxLength\n" +
		"drop-gate-audit-settings: removed audit.diffThreshold\n" +
		"drop-gate-audit-settings: removed audit.dependencyManifests\n" +
		"drop-gate-audit-settings: removed audit.domainDocStaleness\n" +
		"drop-gate-audit-settings: removed audit.domainCodeStaleness\n" +
		"drop-gate-audit-settings: removed audit.undocumentedDomain\n" +
		"drop-gate-audit-settings: removed audit.plainPunctuation\n" +
		"drop-gate-audit-settings: removed audit.uncommittedChanges\n" +
		"drop-gate-audit-settings: removed currentState.maxTopicsPerPath\n"
	if got := changes.String(); got != wantChanges {
		t.Errorf("announcements:\n got %q\nwant %q", got, wantChanges)
	}
}

func TestDropGateAuditSettingsDropsEmptiedBlocks(t *testing.T) {
	root := t.TempDir()
	src := "prefix: example\nproseGate:\n  enabled: true\nmemoryCite:\n  enabled: false\naudit:\n  allowedTypes: [feat]\ncurrentState:\n  maxTopicsPerPath: 3\n"
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), src)
	var changes Changes
	if err := applyDropGateAuditSettings(root, &changes); err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(filepath.Join(root, ".awf", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(bytes), "prefix: example\n"; got != want {
		t.Errorf("emptied blocks remain:\n got %q\nwant %q", got, want)
	}
	wantChanges := "drop-gate-audit-settings: removed proseGate.enabled\n" +
		"drop-gate-audit-settings: removed memoryCite.enabled\n" +
		"drop-gate-audit-settings: removed audit.allowedTypes\n" +
		"drop-gate-audit-settings: removed currentState.maxTopicsPerPath\n"
	if got := changes.String(); got != wantChanges {
		t.Errorf("announcements:\n got %q\nwant %q", got, wantChanges)
	}
}

func TestDropGateAuditSettingsRejectsMalformedNestedMapping(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: example\naudit: [\n")
	var changes Changes
	if err := applyDropGateAuditSettings(root, &changes); err == nil {
		t.Fatal("malformed audit mapping accepted")
	}
	if _, err := loadForMigration(root); err == nil {
		t.Fatal("migration analysis accepted malformed audit mapping")
	}
}

func TestConfigForCurrentSchemaDropsAllGateAuditSettings(t *testing.T) {
	src := []byte("prefix: example\nhooks: {enabled: true}\nrunner: {enabled: true}\nproseGate: {enabled: true}\nmemoryCite: {enabled: true}\naudit: {allowedTypes: [feat], subjectMaxLength: 1, diffThreshold: 1, dependencyManifests: [go.mod], domainDocStaleness: false, domainCodeStaleness: false, undocumentedDomain: false, plainPunctuation: false, uncommittedChanges: false}\ncurrentState: {maxTopicsPerPath: 1}\n")
	got, err := ConfigForCurrentSchema(src, 37)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range gateAuditRetiredKeys {
		if strings.Contains(string(got), key.key+":") {
			t.Errorf("forward port retained %s.%s:\n%s", key.parent, key.key, got)
		}
	}
}
