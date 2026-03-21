package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/nskforward/ai/tool"
)

// MockProvider is a stub for testing the framework without API keys.
type MockProvider struct {
	Name string
}

func (m *MockProvider) Generate(ctx context.Context, history []Message, tools []tool.Tool, opts *GenerateOptions) (Message, error) {
	lastMsg := history[len(history)-1]
	
	// If the last message is from the system asking for a plan, look at the previous user message
	userText := lastMsg.Content
	if lastMsg.Role == RoleSystem && len(history) >= 2 {
		userText = history[len(history)-2].Content
	}

	// Structured output JSON mocking
	if opts != nil && opts.ResponseFormat != "" {
		if strings.Contains(userText, "complex") {
			return Message{
				Role: RoleAssistant,
				Content: `{"is_complex": true, "reasoning": "Test complex task", "steps": [{"id": 1, "description": "do step 1"}, {"id": 2, "description": "do step 2"}]}`,
			}, nil
		}
		return Message{
			Role: RoleAssistant,
			Content: `{"is_complex": false, "reasoning": "Simple task", "steps": []}`,
		}, nil
	}

	// Simulate tool usage
	if lastMsg.Role == RoleUser && lastMsg.Content == "test read" {
		return Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Name: "read_file",
					Args: `{"path": "skills/how_to_hello.md"}`,
				},
			},
		}, nil
	}

	if lastMsg.Role == RoleUser && lastMsg.Content == "test save" {
		return Message{
			Role: RoleAssistant,
			ToolCalls: []ToolCall{
				{
					ID:   "call_2",
					Name: "save_skill",
					Args: `{"filename": "test.md", "content": "Это тестовый опыт."}`,
				},
			},
		}, nil
	}

	// If it was a tool response
	if lastMsg.Role == RoleTool {
		return Message{
			Role:    RoleAssistant,
			Content: fmt.Sprintf("[%s] Я успешно применил инструмент. Результат: %s", m.Name, lastMsg.Content),
		}, nil
	}

	// Generic chat response
	return Message{
		Role:    RoleAssistant,
		Content: fmt.Sprintf("[%s] Я получил ваше сообщение: '%s'", m.Name, lastMsg.Content),
	}, nil
}
