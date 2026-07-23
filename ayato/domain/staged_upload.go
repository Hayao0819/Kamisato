package domain

// StagedFileRequest is one file a client intends to PUT directly to storage
// under a staged-upload intent.
type StagedFileRequest struct {
	Name string
	// Size is signed into the presigned PUT, so storage enforces it.
	Size int64
}

// StagedUploadGrant is the presigned destination for a staged-upload intent.
type StagedUploadGrant struct {
	ID         string
	TTLSeconds int
	URLs       map[string]string
}

// StagedCommitEntry pairs a staged package with its optional staged signature.
type StagedCommitEntry struct {
	Package   string
	Signature string
}
