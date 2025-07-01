package utils

import "path/filepath"

func ResolvePath(baseDir, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return filepath.Clean(targetPath)
	}
	joined := filepath.Join(baseDir, targetPath)
	return filepath.Clean(joined)
}
