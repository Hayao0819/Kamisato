package localfs

import (
	"io"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/repository/stream"
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

func writeStreamToFile(dir string, stream stream.IFileStream) (string, error) {

	if stream == nil {
		return "", nil
	}
	fp := path.Join(dir, stream.FileName())
	if err := writeReadSeekerToFile(fp, stream); err != nil {
		return "", err
	}

	return fp, nil
}
