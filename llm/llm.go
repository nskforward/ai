package llm

import "context"

// Role роль сообщения
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message сообщение для LLM
type Message struct {
	// Role роль отправителя
	Role Role

	// Content текст сообщения
	Content string

	// ToolCallID ID вызова инструмента (для RoleTool)
	ToolCallID string

	// ToolCalls вызовы инструментов (для RoleAssistant)
	ToolCalls []ToolCall
}

// ToolCall вызов инструмента
type ToolCall struct {
	// ID уникальный идентификатор вызова
	ID string

	// Name имя инструмента
	Name string

	// Arguments аргументы
	Arguments map[string]interface{}
}

// ToolDefinition определение инструмента для LLM
type ToolDefinition struct {
	// Type тип (всегда "function")
	Type string

	// Function описание функции
	Function FunctionDefinition
}

// FunctionDefinition описание функции
type FunctionDefinition struct {
	// Name имя функции
	Name string

	// Description описание
	Description string

	// Parameters JSON Schema параметров
	Parameters map[string]interface{}
}

// GenerateRequest запрос на генерацию
type GenerateRequest struct {
	// Messages история сообщений
	Messages []Message

	// Tools доступные инструменты
	Tools []ToolDefinition

	// Model модель
	Model string

	// Temperature температура
	Temperature float64

	// MaxTokens максимальное количество токенов
	MaxTokens int
}

// GenerateResponse ответ генерации
type GenerateResponse struct {
	// Content текстовый контент
	Content string

	// ToolCalls вызовы инструментов
	ToolCalls []ToolCall

	// FinishReason причина завершения
	FinishReason string

	// Usage использование токенов
	Usage *TokenUsage

	// Model использованная модель
	Model string
}

// TokenUsage использование токенов
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// LLM определяет интерфейс LLM провайдера
type LLM interface {
	// Generate генерирует ответ
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)

	// GetName возвращает имя провайдера
	GetName() string
}
