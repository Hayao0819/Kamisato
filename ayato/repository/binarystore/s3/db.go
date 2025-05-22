package s3

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
)

func writeReadSeekerToFile(name string, stream io.Reader) error {
	// Create the file
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	// Write the stream to the file

	if seeker, ok := stream.(io.ReadSeeker); ok {
		if _, err = seeker.Seek(0, 0); err != nil {
			return err
		}
	}
	if _, err := io.Copy(file, stream); err != nil {
		return err
	}
	if seeker, ok := stream.(io.ReadSeeker); ok {
		seeker.Seek(0, 0)
	}
	return nil
}

func writeStreamToFile(dir string, stream domain.IFileStream) (string, error) {

	if stream == nil {
		return "", nil
	}
	fp := path.Join(dir, stream.FileName())
	if err := writeReadSeekerToFile(fp, stream); err != nil {
		return "", err
	}

	return fp, nil
}

func (s *S3) repoAdd(repo, arch string, pkgfile domain.IFileSeekStream, useSignedDB bool, gnupgDir *string) error {

	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	pkgPath, err := writeStreamToFile(t, pkgfile)
	if err != nil {
		return err
	}

	dbkey := key(repo, arch, repo+".db")
	dbobj, err := s.getObject(dbkey)
	if err != nil {
		return fmt.Errorf("failed to get object %s: %w", dbkey, err)
	}
	defer dbobj.Close()

	dbPath, err := writeStreamToFile(t, dbobj)
	if err != nil {
		return err
		// slog.Error("writeStreamToFile", "err", err)
	}

	return pacman.RepoAdd(dbPath, pkgPath, useSignedDB, gnupgDir)
}

func (s *S3) repoRemove(repo string, arch string, useSignedDB bool, gnupgDir *string) error {
	// return nil
	// _, err := s.prepareRepoExecuteEnv(repo, arch, name, pkgfile, signfile, dbfile, useSignedDB, gnupgDir)
	// return err
	return nil
}

func (s *S3) Init(repo string, arch string, useSignedDB bool, gnupgDir *string) error {
	slog.Debug("Init", "repo", repo, "arch", arch, "useSignedDB", useSignedDB, "gnupgDir", gnupgDir)
	if err := s.repoAdd(repo, arch, nil, useSignedDB, gnupgDir); err != nil {
		return err
	}

	return nil
}
