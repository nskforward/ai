package llm

import (
	"context"

	"github.com/nskforward/ai/tool"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a message exchanged with the LLM API.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string // Used when Role == RoleTool to identify which call is being answered.
}

// ToolCall represents a function call requested by the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args string // JSON arguments
}

// Provider represents an LLM backend (either Light or Heavy).
type Provider interface {
	// Generate takes the message history and available tools, and returns the LLM's next response message.
	Generate(ctx context.Context, history []Message, tools []tool.Tool) (Message, error)
}
