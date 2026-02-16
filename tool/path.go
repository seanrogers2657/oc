package tool

import "path/filepath"

// resolvePath resolves a potentially relative path against a working directory.
func resolvePath(workingDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(workingDir, path))
}
