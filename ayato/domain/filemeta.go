package domain

import "time"

// FileMeta carries a served file's conditional-GET validators — ETag (empty
// without object versioning) and last-modified (zero when unknown) — as a domain
// copy so the transport layer never touches blob.
type FileMeta struct {
	ETag         string
	LastModified time.Time
}
