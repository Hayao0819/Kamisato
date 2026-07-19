package domain

// AllowedAdmin is an entry in the GitHub administrator allowlist.
type AllowedAdmin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
