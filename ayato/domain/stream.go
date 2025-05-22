package domain

import (
	"io"
)

type IFileStream interface {
	io.ReadCloser        // ストリーミング返却
	FileName() string    // ダウンロード時のファイル名
	ContentType() string // MIMEタイプ（例: application/zip）
}

type FileStream struct {
	fileName    string
	contentType string
	stream      io.ReadCloser
}

func NewFileStream(fileName string, contentType string, stream io.ReadCloser) *FileStream {
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
