package commitpolicy

// Identity is one exact permitted Git name and email pair.
type Identity struct{ Name, Email string }

// Signer is one permitted SSH signing principal and public key pair.
type Signer struct{ Principal, Key string }

// Policy is the validated exact-commit policy supplied by project composition.
type Policy struct {
	AllowedIdentities []Identity
	RequireSigned     bool
	AllowedSigners    []Signer
}

// Person is the byte-for-byte author or committer identity recorded in a commit.
type Person struct{ Name, Email string }

// SignatureVerdict is the native SSH verification result for one commit.
type SignatureVerdict int

const (
	SignatureValid SignatureVerdict = iota
	SignatureMissing
	SignatureMalformed
	SignatureWrongKey
)

// Commit is the immutable fact set evaluated for one full object ID.
type Commit struct {
	ID                string
	Author, Committer Person
	Signature         SignatureVerdict
}
