package stream

import "io"

type commonFileStream interface {
	FileName() string
	ContentType() string
}

type IFileStream interface {
	io.ReadCloser    // ストリーミング返却
	commonFileStream // ファイル名、MIMEタイプを取得
}

type IFileSeekStream interface {
	io.ReadSeekCloser // ストリーミング返却
	commonFileStream  // ファイル名、MIMEタイプを取得
}
