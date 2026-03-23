package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// BaseAgent базовая реализация агента
type BaseAgent struct {
	config *AgentConfig
	llm    llm.LLM
	tools  *tools.ToolManager
	mu     sync.RWMutex

	// history хранит историю сообщений по сессиям
	history map[string][]llm.Message
}

// NewAgent создаёт нового агента
func NewAgent(config *AgentConfig) *BaseAgent {
	if config == nil {
		config = DefaultConfig()
	}

	return &BaseAgent{
		config:  config,
		tools:   tools.NewToolManager(),
		history: make(map[string][]llm.Message),
	}
}

// GetName возвращает имя агента
func (a *BaseAgent) GetName() string {
	return a.config.Name
}

// GetConfig возвращает конфигурацию
func (a *BaseAgent) GetConfig() *AgentConfig {
	return a.config
}

// SetLLM устанавливает LLM провайдер
func (a *BaseAgent) SetLLM(provider llm.LLM) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.llm = provider
}

// RegisterTool регистрирует инструмент
func (a *BaseAgent) RegisterTool(tool tools.Tool) error {
	return a.tools.Register(tool)
}

// Run запускает обработку сообщения
func (a *BaseAgent) Run(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
	a.mu.RLock()
	llmProvider := a.llm
	config := a.config
	a.mu.RUnlock()

	if llmProvider == nil {
		return nil, fmt.Errorf("LLM provider not set")
	}

	// Добавляем сообщение пользователя в историю
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: msg.Text,
	}

	messages := a.getHistory(agentCtx.SessionID)
	messages = append(messages, userMsg)

	// Добавляем системный промпт в начало, если история пуста
	if len(messages) == 1 || !a.hasSystemMessage(messages) {
		systemMsg := llm.Message{
			Role:    llm.RoleSystem,
			Content: config.SystemPrompt,
		}
		messages = append([]llm.Message{systemMsg}, messages...)
	}

	// Получаем доступные инструменты для пользователя
	availableTools := a.tools.GetAvailable(ctx, agentCtx)

	// Цикл обработки (инструменты могут вызываться несколько раз)
	var response *llm.GenerateResponse
	for i := 0; i < config.MaxIterations; i++ {
		req := &llm.GenerateRequest{
			Messages:    messages,
			Model:       config.Model,
			Temperature: config.Temperature,
			MaxTokens:   config.MaxTokens,
		}

		// Добавляем инструменты, если есть
		if len(availableTools) > 0 {
			req.Tools = availableTools
		}

		resp, err := llmProvider.Generate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM generation failed: %w", err)
		}

		// Добавляем ответ ассистента в историю
		assistantMsg := llm.Message{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		}
		messages = append(messages, assistantMsg)

		// Если нет вызовов инструментов, возвращаем ответ
		if len(resp.ToolCalls) == 0 {
			response = resp
			break
		}

		// Обрабатываем вызовы инструментов
		for _, tc := range resp.ToolCalls {
			result, err := a.tools.Execute(ctx, agentCtx, tc.Name, tc.Arguments)
			if err != nil {
				// Добавляем ошибку как результат
				toolMsg := llm.Message{
					Role:       llm.RoleTool,
					Content:    fmt.Sprintf("Error: %v", err),
					ToolCallID: tc.ID,
				}
				messages = append(messages, toolMsg)
				continue
			}

			// Добавляем результат инструмента
			toolMsg := llm.Message{
				Role:       llm.RoleTool,
				Content:    fmt.Sprintf("%v", result.Output),
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
		}

		response = resp
	}

	if response == nil {
		return nil, fmt.Errorf("max iterations reached without response")
	}

	// Сохраняем историю
	a.saveHistory(agentCtx.SessionID, messages)

	// Создаём ответное сообщение
	reply := transport.NewMessage(response.Content)
	reply.UserID = agentCtx.UserID
	reply.SessionID = agentCtx.SessionID

	return reply, nil
}

func (a *BaseAgent) getHistory(sessionID string) []llm.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.history[sessionID]
}

func (a *BaseAgent) saveHistory(sessionID string, messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ограничиваем размер истории
	if a.config.EnableHistory && len(messages) > a.config.MaxHistorySize {
		// Сохраняем системный промпт и последние сообщения
		start := len(messages) - a.config.MaxHistorySize
		if messages[0].Role == llm.RoleSystem {
			messages = append([]llm.Message{messages[0]}, messages[start:]...)
		} else {
			messages = messages[start:]
		}
	}

	a.history[sessionID] = messages
}

func (a *BaseAgent) hasSystemMessage(messages []llm.Message) bool {
	for _, m := range messages {
		if m.Role == llm.RoleSystem {
			return true
		}
	}
	return false
}
