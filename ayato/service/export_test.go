package service

// ArchFromFilename exposes archFromFilename for the external test package.
func ArchFromFilename(filename string) (string, error) {
	return archFromFilename(filename)
}
