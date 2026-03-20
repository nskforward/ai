package sandbox

import (
	"path/filepath"
	"strings"
)

type DefaultFSSandbox struct {
	allowedDirs []string
}

// NewFSSandbox creates a file system sandbox that only allows reading/writing
// inside the exact directory paths provided.
func NewFSSandbox(allowedDirs []string) *DefaultFSSandbox {
	var clean []string
	for _, d := range allowedDirs {
		clean = append(clean, filepath.Clean(d))
	}
	return &DefaultFSSandbox{allowedDirs: clean}
}

func (s *DefaultFSSandbox) isAllowed(p string) bool {
	cleanPath := filepath.Clean(p)
	for _, dir := range s.allowedDirs {
		if strings.HasPrefix(cleanPath, dir) {
			return true
		}
	}
	return false
}

func (s *DefaultFSSandbox) CheckRead(path string) error {
	if !s.isAllowed(path) {
		return ErrAccessDenied
	}
	return nil
}

func (s *DefaultFSSandbox) CheckWrite(path string) error {
	if !s.isAllowed(path) {
		return ErrAccessDenied
	}
	return nil
}
