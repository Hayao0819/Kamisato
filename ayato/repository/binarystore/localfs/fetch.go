package localfs

import (
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/provider"
	"github.com/Hayao0819/nahi/futils"
)

type streamFile struct {
	fileName    string
	contentType string
	file        *os.File
}

func (s *streamFile) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
func (s *streamFile) Read(p []byte) (n int, err error) {
	if s.file != nil {
		return s.file.Read(p)
	}
	return 0, nil
}

func (s *streamFile) FileName() string {
	return s.fileName
}
func (s *streamFile) ContentType() string {
	return s.contentType
}

func openStreamFile(filePath string) (*streamFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return &streamFile{
		fileName:    path.Base(filePath),
		contentType: "application/octet-stream",
		file:        file,
	}, nil
}

func (l *LocalPkgBinaryStore) FetchFile(repo string, arch string, file string) (provider.BinaryStream, error) {
	repoDir, err := l.getRepoDir(repo)
	if err != nil {
		return nil, err
	}

	pkgPath := path.Join(repoDir, arch, file)

	if !futils.Exists(pkgPath) {
		return nil, os.ErrNotExist
	}
	slog.Info("fetch pkg file", "file", pkgPath)
	streamFile, err := openStreamFile(pkgPath)
	if err != nil {
		return nil, err
	}
	return streamFile, nil
}
