package domain

// AllowedAdmin is a domain-layer copy of an admin allowlist entry (GitHub id +
// login) so handlers never import the repository package.
type AllowedAdmin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
