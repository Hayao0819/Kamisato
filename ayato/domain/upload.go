package domain

import "github.com/Hayao0819/Kamisato/ayato/stream"

type UploadFiles struct {
	PkgFile stream.SeekFile
	SigFile stream.SeekFile
}
