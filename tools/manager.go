package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/transport"
)

// ToolManager управляет инструментами
type ToolManager struct {
	mu              sync.RWMutex
	tools           map[string]Tool
	approvalManager *ApprovalManager
}

// NewToolManager создаёт новый менеджер инструментов
func NewToolManager() *ToolManager {
	return &ToolManager{
		tools:           make(map[string]Tool),
		approvalManager: NewApprovalManager(),
	}
}

// SetApprovalManager устанавливает менеджер подтверждений
func (tm *ToolManager) SetApprovalManager(am *ApprovalManager) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.approvalManager = am
}

// GetApprovalManager возвращает менеджер подтверждений
func (tm *ToolManager) GetApprovalManager() *ApprovalManager {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.approvalManager
}

// Register регистрирует инструмент
func (tm *ToolManager) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	tm.tools[name] = tool
	return nil
}

// GetByName получает инструмент по имени
func (tm *ToolManager) GetByName(name string) (Tool, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tool, exists := tm.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	return tool, nil
}

// GetAll возвращает все инструменты
func (tm *ToolManager) GetAll() []Tool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]Tool, 0, len(tm.tools))
	for _, tool := range tm.tools {
		result = append(result, tool)
	}
	return result
}

// GetAvailable возвращает доступные инструменты для пользователя в формате LLM
func (tm *ToolManager) GetAvailable(ctx context.Context, agentCtx *transport.AgentContext) []llm.ToolDefinition {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []llm.ToolDefinition
	for _, tool := range tm.tools {
		if tool.IsAvailable(ctx, agentCtx) && tool.ApprovalPolicy() != Deny {
			def := llm.ToolDefinition{
				Type: "function",
				Function: llm.FunctionDefinition{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  buildParametersSchema(tool.Parameters()),
				},
			}
			result = append(result, def)
		}
	}
	return result
}

// Execute выполняет инструмент
func (tm *ToolManager) Execute(ctx context.Context, agentCtx *transport.AgentContext, name string, params map[string]interface{}) (*ToolResult, error) {
	tool, err := tm.GetByName(name)
	if err != nil {
		return nil, err
	}

	// Проверяем доступность
	if !tool.IsAvailable(ctx, agentCtx) {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q is not available for this user", name),
		}, nil
	}

	// Проверяем политику подтверждения
	policy := tool.ApprovalPolicy()
	if policy == Deny {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool %q is denied", name),
		}, nil
	}

	// Запрашиваем подтверждение через ApprovalManager
	if tm.approvalManager != nil {
		approved, err := tm.approvalManager.RequestApproval(ctx, agentCtx, tool, params)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("approval failed: %v", err),
			}, nil
		}
		if !approved {
			return &ToolResult{
				Success: false,
				Error:   fmt.Sprintf("tool %q execution was not approved", name),
			}, nil
		}
	}

	// Выполняем инструмент
	return tool.Call(ctx, agentCtx, params)
}

// buildParametersSchema строит JSON Schema для параметров
func buildParametersSchema(params map[string]ParameterSchema) map[string]interface{} {
	properties := make(map[string]interface{})
	var required []string

	for name, schema := range params {
		prop := map[string]interface{}{
			"type":        schema.Type,
			"description": schema.Description,
		}
		properties[name] = prop
		if schema.Required {
			required = append(required, name)
		}
	}

	result := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		result["required"] = required
	}

	return result
}
