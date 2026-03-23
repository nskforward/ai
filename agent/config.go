package agent

import "time"

// AgentConfig конфигурация агента
type AgentConfig struct {
	// Name имя агента
	Name string

	// SystemPrompt системный промпт
	SystemPrompt string

	// Model модель по умолчанию
	Model string

	// Temperature температура
	Temperature float64

	// MaxTokens максимум токенов ответа
	MaxTokens int

	// MaxHistorySize максимум сообщений в истории
	MaxHistorySize int

	// EnableHistory включить историю
	EnableHistory bool

	// MaxIterations максимум итераций (инструменты + ответы)
	MaxIterations int

	// Timeout таймаут обработки
	Timeout time.Duration
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		Name:           "Agent",
		SystemPrompt:   "You are a helpful AI assistant.",
		Model:          "openai/gpt-4o-mini",
		Temperature:    0.7,
		MaxTokens:      2000,
		MaxHistorySize: 50,
		EnableHistory:  true,
		MaxIterations:  10,
		Timeout:        60 * time.Second,
	}
}
