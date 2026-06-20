package stream

import "io"

// File はメタ情報付きの読み取り専用ファイルストリームです。
type File interface {
	io.ReadCloser
	FileName() string
	ContentType() string
}

// SeekFile はメタ情報付きのシーク可能なファイルストリームです。
type SeekFile interface {
	io.ReadSeekCloser
	FileName() string
	ContentType() string
}
