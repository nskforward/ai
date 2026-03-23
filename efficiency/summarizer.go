package efficiency

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/nskforward/ai/llm"
)

// Summarizer сжимает историю сообщений
type Summarizer interface {
	// Summarize создаёт сводку из сообщений
	Summarize(ctx context.Context, messages []llm.Message) (string, error)

	// ShouldSummarize проверяет необходимость сводки
	ShouldSummarize(messages []llm.Message, tokenCount int) bool

	// Merge объединяет сводку с новыми сообщениями
	Merge(summary string, recentMessages []llm.Message) []llm.Message
}

// SummarizerConfig конфигурация summarizer
type SummarizerConfig struct {
	// Threshold порог токенов для активации summarization
	Threshold int

	// KeepRecent количество последних сообщений для сохранения
	KeepRecent int

	// Model модель для summarization
	Model string

	// Temperature температура для summarization
	Temperature float64
}

// DefaultSummarizerConfig возвращает конфигурацию по умолчанию
func DefaultSummarizerConfig() *SummarizerConfig {
	return &SummarizerConfig{
		Threshold:   4000,
		KeepRecent:  5,
		Model:       "openai/gpt-4o-mini",
		Temperature: 0.3,
	}
}

// LLMSummarizer реализация summarizer с использованием LLM
type LLMSummarizer struct {
	config *SummarizerConfig
	llm    llm.LLM
	mu     sync.RWMutex
}

// NewLLMSummarizer создаёт новый summarizer
func NewLLMSummarizer(provider llm.LLM, config *SummarizerConfig) *LLMSummarizer {
	if config == nil {
		config = DefaultSummarizerConfig()
	}

	return &LLMSummarizer{
		config: config,
		llm:    provider,
	}
}

// Summarize создаёт сводку из сообщений
func (s *LLMSummarizer) Summarize(ctx context.Context, messages []llm.Message) (string, error) {
	s.mu.RLock()
	llmProvider := s.llm
	config := s.config
	s.mu.RUnlock()

	if llmProvider == nil {
		return "", fmt.Errorf("LLM provider not set")
	}

	// Формируем промпт для summarization
	var conversationBuilder strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleSystem:
			conversationBuilder.WriteString(fmt.Sprintf("[System]: %s\n", msg.Content))
		case llm.RoleUser:
			conversationBuilder.WriteString(fmt.Sprintf("[User]: %s\n", msg.Content))
		case llm.RoleAssistant:
			conversationBuilder.WriteString(fmt.Sprintf("[Assistant]: %s\n", msg.Content))
		case llm.RoleTool:
			conversationBuilder.WriteString(fmt.Sprintf("[Tool Result]: %s\n", msg.Content))
		}
	}

	prompt := fmt.Sprintf(`Create a concise summary of the following conversation. Focus on:
1. Key topics discussed
2. Important decisions or conclusions
3. Action items or tasks mentioned
4. Context needed for future messages

Conversation:
%s

Summary:`, conversationBuilder.String())

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{
				Role:    llm.RoleUser,
				Content: prompt,
			},
		},
		Model:       config.Model,
		Temperature: config.Temperature,
		MaxTokens:   500,
	}

	resp, err := llmProvider.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}

	return resp.Content, nil
}

// ShouldSummarize проверяет необходимость сводки
func (s *LLMSummarizer) ShouldSummarize(messages []llm.Message, tokenCount int) bool {
	s.mu.RLock()
	threshold := s.config.Threshold
	s.mu.RUnlock()

	// Проверяем по количеству токенов
	if tokenCount >= threshold {
		return true
	}

	// Проверяем по количеству сообщений (приблизительно)
	// Считаем, что в среднем 1 сообщение = 100 токенов
	if len(messages)*100 >= threshold {
		return true
	}

	return false
}

// Merge объединяет сводку с новыми сообщениями
func (s *LLMSummarizer) Merge(summary string, recentMessages []llm.Message) []llm.Message {
	s.mu.RLock()
	keepRecent := s.config.KeepRecent
	s.mu.RUnlock()

	result := make([]llm.Message, 0, len(recentMessages)+1)

	// Добавляем системный промпт, если есть
	for _, msg := range recentMessages {
		if msg.Role == llm.RoleSystem {
			result = append(result, msg)
			break
		}
	}

	// Добавляем сводку как системное сообщение
	if summary != "" {
		summaryMsg := llm.Message{
			Role: llm.RoleSystem,
			Content: fmt.Sprintf(`Previous conversation summary:
%s

Continue the conversation based on this context.`, summary),
		}
		result = append(result, summaryMsg)
	}

	// Добавляем последние сообщения
	startIdx := 0
	nonSystemMessages := 0
	for i, msg := range recentMessages {
		if msg.Role != llm.RoleSystem {
			nonSystemMessages++
			if nonSystemMessages > len(recentMessages)-keepRecent {
				startIdx = i
				break
			}
		}
	}

	for i := startIdx; i < len(recentMessages); i++ {
		if recentMessages[i].Role != llm.RoleSystem {
			result = append(result, recentMessages[i])
		}
	}

	return result
}

// SimpleSummarizer простой summarizer без LLM (извлекает ключевые фразы)
type SimpleSummarizer struct {
	config *SummarizerConfig
}

// NewSimpleSummarizer создаёт простой summarizer
func NewSimpleSummarizer(config *SummarizerConfig) *SimpleSummarizer {
	if config == nil {
		config = DefaultSummarizerConfig()
	}

	return &SimpleSummarizer{
		config: config,
	}
}

// Summarize создаёт простую сводку (без LLM)
func (s *SimpleSummarizer) Summarize(ctx context.Context, messages []llm.Message) (string, error) {
	var topics []string
	var userMessages int
	var assistantMessages int

	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleUser:
			userMessages++
			// Извлекаем первые 50 символов как тему
			if len(msg.Content) > 50 {
				topics = append(topics, msg.Content[:50]+"...")
			} else {
				topics = append(topics, msg.Content)
			}
		case llm.RoleAssistant:
			assistantMessages++
		}
	}

	summary := fmt.Sprintf("Conversation with %d user messages and %d assistant responses. Key topics: %s",
		userMessages, assistantMessages, strings.Join(topics[:min(3, len(topics))], "; "))

	return summary, nil
}

// ShouldSummarize проверяет необходимость сводки
func (s *SimpleSummarizer) ShouldSummarize(messages []llm.Message, tokenCount int) bool {
	return tokenCount >= s.config.Threshold || len(messages)*100 >= s.config.Threshold
}

// Merge объединяет сводку с новыми сообщениями
func (s *SimpleSummarizer) Merge(summary string, recentMessages []llm.Message) []llm.Message {
	result := make([]llm.Message, 0, len(recentMessages)+1)

	// Добавляем системный промпт, если есть
	for _, msg := range recentMessages {
		if msg.Role == llm.RoleSystem {
			result = append(result, msg)
			break
		}
	}

	// Добавляем сводку
	if summary != "" {
		summaryMsg := llm.Message{
			Role:    llm.RoleSystem,
			Content: fmt.Sprintf("Previous conversation summary: %s", summary),
		}
		result = append(result, summaryMsg)
	}

	// Добавляем последние сообщения
	startIdx := max(0, len(recentMessages)-s.config.KeepRecent)
	for i := startIdx; i < len(recentMessages); i++ {
		if recentMessages[i].Role != llm.RoleSystem {
			result = append(result, recentMessages[i])
		}
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
