package domain

import "github.com/Hayao0819/Kamisato/ayato/stream"

// UploadFiles represents the files to be uploaded.
type UploadFiles struct {
	PkgFile *stream.FileStream
	SigFile *stream.FileStream
}
