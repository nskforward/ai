package tools

import (
	"context"
	"time"

	"github.com/nskforward/ai/transport"
)

// ApprovalPolicy политика подтверждения вызова инструмента
type ApprovalPolicy string

const (
	// AutoApprove автоматическое подтверждение без запроса
	AutoApprove ApprovalPolicy = "auto_approve"

	// RequireApproval требует подтверждение от пользователя
	RequireApproval ApprovalPolicy = "require_approval"

	// RequireAdminApproval требует подтверждение от администратора
	RequireAdminApproval ApprovalPolicy = "require_admin_approval"

	// Deny запрещено использование
	Deny ApprovalPolicy = "deny"
)

// ParameterSchema описывает параметр инструмента
type ParameterSchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ToolResult результат выполнения инструмента
type ToolResult struct {
	// Success успешность выполнения
	Success bool

	// Output результат
	Output interface{}

	// Error ошибка
	Error string

	// Metadata метаданные
	Metadata map[string]interface{}

	// ExecutionTime время выполнения
	ExecutionTime time.Duration
}

// Tool определяет интерфейс инструмента
type Tool interface {
	// Name возвращает имя инструмента
	Name() string

	// Description описание для LLM
	Description() string

	// Parameters возвращает описание параметров
	Parameters() map[string]ParameterSchema

	// Call выполняет инструмент
	Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*ToolResult, error)

	// ApprovalPolicy возвращает политику подтверждения
	ApprovalPolicy() ApprovalPolicy

	// IsAvailable проверяет доступность инструмента для пользователя
	IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool
}
