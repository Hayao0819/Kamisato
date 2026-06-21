package s3

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/pacman"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/samber/lo"
)

// StoreFile stores the package object only. The database is updated by a
// separate RepoAdd call (the service drives both), matching the localfs store
// and letting an arch=any file be stored under "any/" without creating an "any"
// database.
func (s *S3) StoreFile(repo string, arch string, file stream.SeekFile) error {
	k := key(repo, arch, file.FileName())
	if err := s.putObject(k, file); err != nil {
		return fmt.Errorf("failed to store file %s: %w", k, err)
	}
	return nil
}

func (s *S3) StoreFileWithSignedURL(repo string, arch string, name string) (string, error) {
	k := key(repo, arch, name)

	input := awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	}

	presignClient := awss3.NewPresignClient(s.storage)
	presignResult, err := presignClient.PresignGetObject(s.ctx, &input, func(po *awss3.PresignOptions) {
		po.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL for %s: %w", k, err)
	}
	return presignResult.URL, nil
}

func fileAliasResolver(repo, _, filename string) string {
	if filename == repo+".db" {
		return repo + ".db.tar.gz"
	}
	if filename == repo+".files" {
		return repo + ".files.tar.gz"
	}
	return filename
}

func (s *S3) FetchFile(repo string, arch string, name string) (stream.File, error) {
	if name == repo+".db" {
		name = repo + ".db.tar.gz"
	}
	if name == repo+".files" {
		name = repo + ".files.tar.gz"
	}

	o, err := s.getObject(key(repo, arch, name))
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("file %s/%s/%s not found", repo, arch, name)
	}
	return o, nil
}

func (s *S3) DeleteFile(repo string, arch string, name string) error {
	k := key(repo, arch, name)
	if err := s.deleteObject(k); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", k, err)
	}
	return nil
}

func (s *S3) RepoNames() ([]string, error) {
	return s.listDirs("")
}

func (s *S3) Arches(repo string) ([]string, error) {
	slog.Debug("get arches", "repo", repo)
	dl, err := s.listDirs(repo + "/")
	if err != nil {
		return nil, err
	}
	return lo.Map(dl, func(name string, _ int) string {
		return path.Base(name)
	}), nil
}

func (s *S3) Files(repo string, arch string) ([]string, error) {
	l, err := s.listFiles(fmt.Sprintf("%s/%s/", repo, arch))
	if err != nil {
		return nil, err
	}

	l = lo.Map(l, func(name string, _ int) string {
		return path.Base(name)
	})

	for _, name := range l {
		r := fileAliasResolver(repo, arch, name)
		if r != name {
			l = append(l, r)
		}
	}
	ul := lo.Uniq(l)

	slog.Debug("get files", "repo", repo, "arch", arch, "files", ul)
	return ul, nil
}

func (s *S3) initRepo(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	slog.Debug("initRepo", "repo", repo, "arch", arch, "useSignedDB", useSignedDB, "gnupgDir", gnupgDir)
	t, err := os.MkdirTemp("", "ayato-s3-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	dbpath := path.Join(t, repo+".db.tar.gz")
	if err := pacman.RepoAdd(dbpath, "", useSignedDB, gnupgDir); err != nil {
		slog.Error("RepoAdd", "err", err)
		return err
	}

	dbkey := key(repo, arch, repo+".db.tar.gz")
	dbobj, err := stream.OpenFileWithType(dbpath)
	if err != nil {
		slog.Error("OpenFileWithType", "err", err)
		return err
	}
	defer dbobj.Close()

	if err := s.putObject(dbkey, dbobj); err != nil {
		slog.Error("putObject", "err", err)
		return utils.WrapErr(err, "failed to put object")
	}

	return nil
}

func (s *S3) RepoAdd(repo, arch string, pkgfile, sigfile stream.SeekFile, useSignedDB bool, gnupgDir *string) error {
	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	pkgPath, err := writeStreamToFile(t, pkgfile)
	if err != nil {
		return err
	}

	dbkey := key(repo, arch, repo+".db.tar.gz")
	dbobj, err := s.getObject(dbkey)
	if err != nil {
		return fmt.Errorf("failed to get object %s: %w", dbkey, err)
	}
	defer dbobj.Close()

	dbPath, err := writeStreamToFile(t, dbobj)
	if err != nil {
		return err
	}

	if err := pacman.RepoAdd(dbPath, pkgPath, useSignedDB, gnupgDir); err != nil {
		return err
	}

	return s.uploadDBArtifacts(repo, arch, t, pkgPath)
}

// uploadDBArtifacts mirrors every file in dir back to S3 (except skip), since
// repo-add/repo-remove rewrite the DB archives and signatures in place.
func (s *S3) uploadDBArtifacts(repo, arch, dir, skip string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fp := path.Join(dir, entry.Name())
		if skip != "" && fp == skip {
			continue
		}

		obj, err := stream.OpenFileWithType(fp)
		if err != nil {
			return err
		}
		if err := s.putObject(key(repo, arch, entry.Name()), obj); err != nil {
			obj.Close()
			return fmt.Errorf("failed to put object %s: %w", entry.Name(), err)
		}
		obj.Close()
	}

	return nil
}

func (s *S3) RepoRemove(repo string, arch string, pkg string, useSignedDB bool, gnupgDir *string) error {
	dbkey := key(repo, arch, repo+".db.tar.gz")
	dbobj, err := s.getObject(dbkey)
	if err != nil {
		return fmt.Errorf("failed to get object %s: %w", dbkey, err)
	}
	defer dbobj.Close()

	t, err := os.MkdirTemp("", "ayato-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	dbPath, err := writeStreamToFile(t, dbobj)
	if err != nil {
		return err
	}

	if err := pacman.RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
		slog.Error("RepoRemove", "err", err)
		return err
	}

	return s.uploadDBArtifacts(repo, arch, t, "")
}

func (s *S3) InitArch(repo string, arch string, useSignedDB bool, gnupgDir *string) error {
	slog.Debug("InitArch", "repo", repo, "arch", arch, "useSignedDB", useSignedDB, "gnupgDir", gnupgDir)
	return s.initRepo(repo, arch, useSignedDB, gnupgDir)
}
