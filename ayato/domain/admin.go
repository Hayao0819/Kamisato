package domain

// AllowedAdmin is one entry of the admin allowlist exposed to the transport
// layer: the GitHub numeric id and its login. The service maps the repository's
// persisted entry onto this type so handlers never touch the repository package.
type AllowedAdmin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
