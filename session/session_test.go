package session_test

import (
	"testing"
	"time"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/session"
)

func TestMemoryStore(t *testing.T) {
	store := session.NewMemoryStore(1 * time.Hour)

	// Empty session
	history, err := store.Load("s1")
	if err != nil || history != nil {
		t.Fatalf("expected nil for new session, got %v, %v", history, err)
	}

	// Save and load
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "привет"},
		{Role: llm.RoleAssistant, Content: "здравствуйте"},
	}
	if err := store.Save("s1", msgs); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := store.Load("s1")
	if err != nil || len(loaded) != 2 {
		t.Fatalf("expected 2 messages, got %d, err: %v", len(loaded), err)
	}
	if loaded[0].Content != "привет" {
		t.Fatalf("wrong content: %s", loaded[0].Content)
	}

	// Verify isolation: modifying loaded slice doesn't affect stored
	loaded[0].Content = "changed"
	reloaded, _ := store.Load("s1")
	if reloaded[0].Content != "привет" {
		t.Fatal("session store did not return a copy")
	}
}
