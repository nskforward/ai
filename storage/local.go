package storage

import (
	"os"
	"path/filepath"
)

// LocalFS implements the Storage interface using the local filesystem.
type LocalFS struct {
	baseDir string
}

// NewLocalFS creates a new LocalFS storage rooted at baseDir.
func NewLocalFS(baseDir string) (*LocalFS, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalFS{baseDir: baseDir}, nil
}

// resolve returns the absolute path within the base directory.
// Security note: Sandbox layer will ensure paths don't contain "../",
// but this local layer assumes the path is a relative secure path like "skills/file.md".
func (l *LocalFS) resolve(path string) string {
	return filepath.Join(l.baseDir, path)
}

func (l *LocalFS) Read(path string) ([]byte, error) {
	data, err := os.ReadFile(l.resolve(path))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return data, err
}

func (l *LocalFS) Write(path string, data []byte) error {
	fullPath := l.resolve(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

func (l *LocalFS) List(dir string) ([]string, error) {
	fullDir := l.resolve(dir)
	entries, err := os.ReadDir(fullDir)
	if os.IsNotExist(err) {
		return nil, nil // If directory doesn't exist, we just return empty list
	}
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	return files, nil
}
