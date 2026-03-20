package transport

// Message represents an incoming or outgoing communication.
type Message struct {
	SessionID string
	UserID    string
	Text      string
}

// Transport defines how the agent receives inputs and sends outputs.
type Transport interface {
	// Read blocks until a new message is received.
	Read() (Message, error)
	// Write sends a response back to the user via the given session ID.
	Write(sessionID string, text string) error
}
