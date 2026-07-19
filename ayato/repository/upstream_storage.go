package repository

import (
	"bytes"
	"io"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

type databaseArtifactSet struct {
	names    pacmanrepo.DatabaseArtifacts
	database []byte
	files    []byte
}

func scopedArtifacts(name, scope string) pacmanrepo.DatabaseArtifacts {
	return pacmanrepo.Artifacts(name).WithArchivePrefix(name + "." + scope)
}

func (r *binaryRepository) storeArtifactSet(
	repo, arch string,
	set databaseArtifactSet,
	optionalFiles bool,
	errorScope string,
) error {
	artifacts := []struct {
		name, kind string
		data       []byte
	}{
		{name: set.names.DatabaseArchive(), kind: "database", data: set.database},
		{name: set.names.FilesArchive(), kind: "files database", data: set.files},
	}
	for index, artifact := range artifacts {
		if index == 1 && optionalFiles && artifact.data == nil {
			continue
		}
		if err := r.storeBytes(repo, arch, artifact.name, artifact.data); err != nil {
			return errors.WrapErr(err, "store "+errorScope+" "+artifact.kind)
		}
	}
	return nil
}

func (r *binaryRepository) signArtifactSet(
	repo, arch string,
	set databaseArtifactSet,
) error {
	artifacts := []struct {
		name string
		data []byte
	}{
		{name: set.names.DatabaseArchive(), data: set.database},
		{name: set.names.FilesArchive(), data: set.files},
	}
	for _, artifact := range artifacts {
		var signature bytes.Buffer
		if err := pacmanrepo.SignDetached(
			r.dbSigner,
			bytes.NewReader(artifact.data),
			&signature,
		); err != nil {
			return errors.WrapErr(err, "sign merged database")
		}
		if err := r.storeBytes(
			repo,
			arch,
			artifact.name+".sig",
			signature.Bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

// fetchBytes reads a stored object fully; a missing object is (nil, nil) so a
// merge can treat it as empty.
func (r *binaryRepository) fetchBytes(repo, arch, file string) ([]byte, error) {
	value, err := r.Store.FetchFile(repo, arch, file)
	if errors.Is(err, blob.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer value.Close()
	return io.ReadAll(value)
}

func (r *binaryRepository) storeBytes(repo, arch, file string, data []byte) error {
	value := stream.NewFileStream(
		file,
		"application/gzip",
		byteSeeker{bytes.NewReader(data)},
	)
	return r.Store.StoreFile(repo, arch, value)
}

func bytesReaderOrNil(value []byte) io.Reader {
	if value == nil {
		return nil
	}
	return bytes.NewReader(value)
}

type byteSeeker struct{ *bytes.Reader }

func (byteSeeker) Close() error { return nil }
