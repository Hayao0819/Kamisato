package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/alpm/pkg"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/gabriel-vasile/mimetype"
)

// FIXME: 消す
func openFileStreamWithTypeDetection(filePath string) (*domain.FileStream, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	mt, err := mimetype.DetectReader(file)
	if err != nil {
		return nil, err
	}
	file.Seek(0, io.SeekStart) // Reset the file pointer to the beginning

	return domain.NewFileStream(file.Name(), mt.String(), file), nil
}

func (s *Service) UploadPkgFile(rname string, name [2]string) error {
	pkgFilePath := name[0]
	// sigFile := name[1]
	slog.Info("upload pkg file", "file", pkgFilePath)

	// Verify repository directory
	if s.r.VerifyPkgRepo(rname) != nil {
		slog.Warn("repository directory not found", "repo", rname)
		if err := s.r.Init(rname, false, nil); err != nil {
			return fmt.Errorf("init repo err: %s", err.Error())
		}
	}

	// Get package file name
	p, err := pkg.GetPkgFromBin(pkgFilePath)
	if err != nil {
		return fmt.Errorf("get pkg from bin err: %s", err.Error())
	}

	// Get package information
	pi, err := p.PKGINFO()
	if err != nil {
		return fmt.Errorf("get pkginfo err: %s", err.Error())
	}
	slog.Info("get pkg from bin", "pkgname", pi.PkgName, "pkgver", pi.PkgVer)

	// Get IFileStream
	pkgFileStream, err := openFileStreamWithTypeDetection(pkgFilePath)
	if err != nil {
		return fmt.Errorf("open file stream err: %s", err.Error())
	}
	defer pkgFileStream.Close()

	// Store package file to the repository directory
	useSignedDB := false
	var gnupgDir *string // TODO: Check if the directory exists
	if err := s.r.StoreFile(rname, pi.Arch, pkgFileStream, useSignedDB, gnupgDir); err != nil {
		return fmt.Errorf("store file err: %s", err.Error())
	}

	// Store metadata to the kv store
	if err := s.r.StorePkgFileName(pi.PkgName, path.Base(pkgFilePath)); err != nil {
		return fmt.Errorf("store pkg file name err: %s", err.Error())
	}

	return nil
}
