package stream

import (
	"io"
)

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

type FileStream struct {
	fileName    string
	contentType string
	stream      io.ReadSeekCloser
}

func NewFileStream(fileName string, contentType string, stream io.ReadSeekCloser) *FileStream {
	return &FileStream{
		fileName:    fileName,
		contentType: contentType,
		stream:      stream,
	}
}

func (f *FileStream) Read(p []byte) (n int, err error) {
	if f.stream != nil {
		return f.stream.Read(p)
	}
	return 0, nil
}
func (f *FileStream) Close() error {
	if f.stream != nil {
		return f.stream.Close()
	}
	return nil
}
func (f *FileStream) FileName() string {
	return f.fileName
}
func (f *FileStream) ContentType() string {
	if f.contentType != "" {
		return f.contentType
	}
	return "application/octet-stream"
}

func (f *FileStream) Seek(offset int64, whence int) (int64, error) {
	return f.stream.Seek(offset, whence)
}
