package commitpolicy

import "sort"

// Evaluate applies policy to unique commit facts and returns every mismatch in stable commit and field order.
func Evaluate(policy Policy, commits []Commit) Outcome {
	seen := map[string]bool{}
	violations := []Violation{}
	for _, commit := range commits {
		if seen[commit.ID] {
			continue
		}
		seen[commit.ID] = true
		if len(policy.AllowedIdentities) != 0 {
			if !allowed(policy.AllowedIdentities, commit.Author) {
				violations = append(violations, Violation{commit.ID, AuthorField, commit.Author.Name + " <" + commit.Author.Email + ">"})
			}
			if !allowed(policy.AllowedIdentities, commit.Committer) {
				violations = append(violations, Violation{commit.ID, CommitterField, commit.Committer.Name + " <" + commit.Committer.Email + ">"})
			}
		}
		if policy.RequireSigned && commit.Signature != SignatureValid {
			violations = append(violations, Violation{commit.ID, SignatureField, signatureName(commit.Signature)})
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Commit != violations[j].Commit {
			return violations[i].Commit < violations[j].Commit
		}
		return violations[i].Field < violations[j].Field
	})
	return Outcome{Violations: violations}
}
func allowed(identities []Identity, person Person) bool {
	for _, identity := range identities {
		if identity.Name == person.Name && identity.Email == person.Email {
			return true
		}
	}
	return false
}
func signatureName(v SignatureVerdict) string {
	switch v {
	case SignatureMissing:
		return "missing"
	case SignatureMalformed:
		return "malformed"
	case SignatureWrongKey:
		return "not signed by an allowed signer"
	default:
		return "invalid"
	}
}
