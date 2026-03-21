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

// GenerateOptions holds optional parameters for the Generate call.
type GenerateOptions struct {
	// ResponseFormat, if set, requests the LLM to return a structured JSON response
	// conforming to the provided JSON Schema string.
	ResponseFormat string
}

// Provider represents an LLM backend (either Light or Heavy).
type Provider interface {
	// Generate takes the message history, available tools, and options,
	// and returns the LLM's next response message.
	Generate(ctx context.Context, history []Message, tools []tool.Tool, opts *GenerateOptions) (Message, error)
}
