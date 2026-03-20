package sandbox_test

import (
	"testing"

	"github.com/nskforward/ai/sandbox"
)

func TestFSSandbox(t *testing.T) {
	s := sandbox.NewFSSandbox([]string{"skills", "data/docs"})

	tests := []struct {
		path    string
		allowed bool
	}{
		{"skills/deploy.md", true},
		{"skills/nested/plan.md", true},
		{"data/docs/info.txt", true},
		{"/etc/passwd", false},
		{"skills/../../etc/passwd", false},
		{"other_folder/file.md", false},
	}

	for _, tt := range tests {
		err := s.CheckRead(tt.path)
		if tt.allowed && err != nil {
			t.Errorf("expected allowed for %s, got err: %v", tt.path, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("expected denied for %s, got allowed", tt.path)
		}
	}
}
