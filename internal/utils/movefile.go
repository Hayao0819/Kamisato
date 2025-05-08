package utils

import (
	"io"
	"os"
	"path"
	"path/filepath"
)

func MoveFile(org string, dst string) error {
	var orgabs, dstabs string
	var err error

	orgabs, err = filepath.Abs(org)
	if err != nil {
		return err
	}

	dstabs, err = filepath.Abs(dst)
	if err != nil {
		return err
	}

	// If the file is already in the destination, do nothing
	if path.Dir(orgabs) == path.Dir(dstabs) && path.Base(orgabs) == path.Base(dstabs) {
		return nil
	}
	// If the file directory is the same, just rename it
	if path.Dir(orgabs) == path.Dir(dstabs) {
		return os.Rename(orgabs, dstabs)
	}

	// If the file is not in the same directory, copy it and delete the original

	// Open the original file
	orgfile, err := os.Open(orgabs)
	if err != nil {
		return err
	}
	defer orgfile.Close()

	// Move the file to the directory
	if dststat, err := os.Stat(dstabs); err == nil && dststat.IsDir() || path.Base(orgabs) == path.Base(dstabs){
		dstabs = path.Join(dstabs, path.Base(orgabs))
	}

	// Create the parent directory
	if err := os.MkdirAll(path.Dir(dstabs), 0755); err != nil {
		return err
	}

	// Create the destination file
	dstfile, err := os.Create(dstabs)
	if err != nil {
		return err
	}
	defer dstfile.Close()


	// Copy the file
	_, err = io.Copy(dstfile, orgfile)
	orgfile.Close()
	if err != nil {
		return err
	}

	// Delete the original file
	err = os.Remove(orgabs)
	if err != nil {
		return err
	}

	return nil

}
