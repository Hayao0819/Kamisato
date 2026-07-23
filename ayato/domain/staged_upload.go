package domain

// StagedFileRequest is one file a client intends to PUT directly to storage
// under a staged-upload intent.
type StagedFileRequest struct {
	Name string
	// Size is a client-declared hint; zero means unknown. The pipeline still
	// re-validates the real bytes at commit time.
	Size int64
}

// StagedUploadGrant is the presigned destination for a staged-upload intent:
// one URL per requested file name, valid for TTLSeconds.
type StagedUploadGrant struct {
	ID         string
	TTLSeconds int
	URLs       map[string]string
}

// StagedCommitEntry pairs a staged package with its optional staged signature,
// both looked up by name under the same intent.
type StagedCommitEntry struct {
	Package   string
	Signature string
}
