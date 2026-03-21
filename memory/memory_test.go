package memory_test

import (
	"strings"
	"testing"

	"github.com/nskforward/ai/memory"
)

// mockStorage is extremely simple
type mockStorage struct{}
func (m *mockStorage) Read(path string) ([]byte, error) { return nil, nil }
func (m *mockStorage) Write(path string, data []byte) error { return nil }
func (m *mockStorage) List(dir string) ([]string, error) {
	if dir == "skills" {
		return []string{"hello.md", "ignore.txt", "deploy.md"}, nil
	}
	return nil, nil
}
func (m *mockStorage) Delete(path string) error { return nil }

func TestMemoryTOC(t *testing.T) {
	mgr := memory.NewManager(&mockStorage{})

	toc, err := mgr.GenerateTOC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(toc, "hello.md") || !strings.Contains(toc, "deploy.md") {
		t.Errorf("TOC missing elements: %s", toc)
	}
	if strings.Contains(toc, "ignore.txt") {
		t.Errorf("TOC should only contain .md files")
	}
}
