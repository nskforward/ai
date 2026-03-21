package session

import "github.com/nskforward/ai/llm"

// Store manages conversation histories for sessions.
type Store interface {
	// Load returns the conversation history for the given session.
	Load(sessionID string) ([]llm.Message, error)
	// Save persists the updated conversation history.
	Save(sessionID string, history []llm.Message) error
}
