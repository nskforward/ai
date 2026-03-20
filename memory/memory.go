package memory

import (
	"fmt"
	"strings"

	"github.com/nskforward/ai/storage"
)

// Manager handles the generation of the Table of Contents from skills.
type Manager struct {
	store storage.Storage
}

// NewManager creates a new memory manager using the provided storage.
func NewManager(store storage.Storage) *Manager {
	return &Manager{store: store}
}

// GenerateTOC scans the 'skills' directory in storage and returns a string
// representing the Table of Contents (TOC) to be injected into the System Prompt.
func (m *Manager) GenerateTOC() (string, error) {
	files, err := m.store.List("skills")
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "Нет накопленного опыта (папка skills/ пуста).", nil
	}

	var builder strings.Builder
	builder.WriteString("Доступный опыт и навыки (папка skills):\n")

	for _, f := range files {
		if strings.HasSuffix(f, ".md") {
			// For now we just list the filenames. 
			// Later we might read the first line (Title) of each .md file.
			builder.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return builder.String(), nil
}
