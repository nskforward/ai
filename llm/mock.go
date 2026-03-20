package llm

import (
	"context"
	"fmt"

	"github.com/nskforward/ai/tool"
)

// MockProvider is a stub for testing the framework without API keys.
type MockProvider struct {
	Name string
}

func (m *MockProvider) Generate(ctx context.Context, history []Message, tools []tool.Tool) (Message, error) {
	lastMsg := history[len(history)-1]

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
