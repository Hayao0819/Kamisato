// Package secretbox seals and opens values persisted at rest. It is a narrow seam
// so the storage layer can encrypt sensitive records (the admin allowlist, stored
// credentials) without spreading crypto through call sites, and a future
// KMS/Secret-Manager backend can slot in behind the same interface.
package secretbox

// SecretBox seals a plaintext into an opaque ciphertext and opens it back.
// Implementations must be safe for concurrent use.
type SecretBox interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(ciphertext []byte) ([]byte, error)
}
