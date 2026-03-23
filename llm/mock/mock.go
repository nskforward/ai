package mock

import (
	"context"

	"github.com/nskforward/ai/llm"
)

// MockLLM мок LLM провайдера для тестирования
type MockLLM struct {
	// GenerateFunc функция для обработки Generate вызовов
	GenerateFunc func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error)

	// CallCount счётчик вызовов
	CallCount int

	// LastRequest последний запрос
	LastRequest *llm.GenerateRequest
}

// NewMockLLM создаёт новый мок LLM
func NewMockLLM() *MockLLM {
	return &MockLLM{
		GenerateFunc: func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
			return &llm.GenerateResponse{
				Content:      "Mock response",
				FinishReason: "stop",
				Model:        "mock-model",
				Usage: &llm.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}, nil
		},
	}
}

// WithResponse настраивает мок на возврат конкретного ответа
func (m *MockLLM) WithResponse(content string) *MockLLM {
	m.GenerateFunc = func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
		return &llm.GenerateResponse{
			Content:      content,
			FinishReason: "stop",
			Model:        "mock-model",
			Usage: &llm.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}, nil
	}
	return m
}

// WithToolCalls настраивает мок на возврат вызовов инструментов
func (m *MockLLM) WithToolCalls(toolCalls []llm.ToolCall) *MockLLM {
	m.GenerateFunc = func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
		return &llm.GenerateResponse{
			Content:      "",
			ToolCalls:    toolCalls,
			FinishReason: "tool_calls",
			Model:        "mock-model",
			Usage: &llm.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}, nil
	}
	return m
}

// WithError настраивает мок на возврат ошибки
func (m *MockLLM) WithError(err error) *MockLLM {
	m.GenerateFunc = func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
		return nil, err
	}
	return m
}

// Generate реализует интерфейс LLM
func (m *MockLLM) Generate(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
	m.CallCount++
	m.LastRequest = req
	return m.GenerateFunc(ctx, req)
}

// GetName возвращает имя провайдера
func (m *MockLLM) GetName() string {
	return "mock"
}
