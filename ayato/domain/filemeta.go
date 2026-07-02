package domain

import "time"

// FileMeta carries the conditional-GET validators for a served repository file:
// a strong ETag (empty when the backend has no object versioning) and the
// last-modified time (zero when unknown). The service maps the repository's blob
// metadata onto this domain type so the transport layer never touches blob.
type FileMeta struct {
	ETag         string
	LastModified time.Time
}
