package domain

// PackageSignature describes the detached PGP signature stored next to a
// package object, read without verification.
type PackageSignature struct {
	Present     bool   `json:"present"`
	Filename    string `json:"filename,omitempty"`
	KeyID       string `json:"key_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	CreatedAt   int64  `json:"created_at,omitempty"`
	Hash        string `json:"hash,omitempty"`
	PubKeyAlgo  string `json:"pubkey_algo,omitempty"`
}
