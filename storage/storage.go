package storage

import "errors"

var ErrNotFound = errors.New("file not found")

// Storage defines the interface for the agent's memory persistence.
type Storage interface {
	// Read returns the contents of the file. Returns ErrNotFound if it doesn't exist.
	Read(path string) ([]byte, error)
	// Write saves data to the specified path, creating directories if needed.
	Write(path string, data []byte) error
	// List returns a list of file names in the given directory.
	List(dir string) ([]string, error)
}
