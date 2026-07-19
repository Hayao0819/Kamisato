package stream

import "io"

type File interface {
	io.ReadCloser
	FileName() string
	ContentType() string
}

type SeekFile interface {
	File
	io.Seeker
}

// OnDiskFile is an optional SeekFile capability: the bytes already live in a real
// file at OnDiskPath, so a consumer that needs them on disk (e.g. to feed a
// file-based tool) can hardlink instead of copying.
type OnDiskFile interface {
	OnDiskPath() string
}
