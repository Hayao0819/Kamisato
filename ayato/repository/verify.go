package repository

import (
	"fmt"
	"path"

	"github.com/cockroachdb/errors"

	"github.com/samber/lo"
)

func (r *Repository) VerifyPkgRepo(name string) error {
	arches, err := r.Arches(name)
	if err != nil {
		return errors.Wrap(err, "failed to get arches")
	}

	for _, arch := range arches {
		files, err := r.Files(name, arch)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to get files for arch %s", arch))
		}

		requiredFiles := []string{
			name + ".db",
			name + ".db.tar.gz",
			name + ".files",
			name + ".files.tar.gz",
		}

		for _, file := range requiredFiles {
			if !lo.Contains(files, file) {
				return fmt.Errorf("%s not found in %s", file, path.Join(name, arch))
			}
		}
	}
	return nil
}
