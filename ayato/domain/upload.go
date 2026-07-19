package domain

import "io"

// SeekFile is the upload capability required by the application. Keeping the
// port in domain lets callers provide any implementation without making the
// request model depend on Ayato's concrete stream package.
type SeekFile interface {
	io.ReadSeekCloser
	FileName() string
	ContentType() string
}

type UploadFiles struct {
	PkgFile SeekFile
	SigFile SeekFile
}
