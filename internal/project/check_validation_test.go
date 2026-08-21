package project

import "testing"

func TestValidateArtifactRejectsMalformedAndIncompleteFrontmatter(t *testing.T) {
	for name, content := range map[string]string{
		"malformed":  "---\nname: [\n---\n",
		"missing":    "plain text\n",
		"empty-name": "---\nname: ' '\ndescription: present\n---\n",
		"empty-desc": "---\nname: present\ndescription: ' '\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateArtifact([]byte(content), MarkdownAgentDialect); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		})
	}
	if err := validateArtifact([]byte("---\nname: present\ndescription: present\n---\n"), MarkdownAgentDialect); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
}
