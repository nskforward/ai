package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/nskforward/ai/transport"
)

// ApprovalStatus статус подтверждения
type ApprovalStatus string

const (
	// ApprovalPending ожидает подтверждения
	ApprovalPending ApprovalStatus = "pending"
	// ApprovalApproved одобрено
	ApprovalApproved ApprovalStatus = "approved"
	// ApprovalRejected отклонено
	ApprovalRejected ApprovalStatus = "rejected"
	// ApprovalTimeout таймаут
	ApprovalTimeout ApprovalStatus = "timeout"
)

// ApprovalRequest запрос на подтверждение
type ApprovalRequest struct {
	// ID уникальный идентификатор запроса
	ID string

	// ToolName имя инструмента
	ToolName string

	// ToolDescription описание инструмента
	ToolDescription string

	// Params параметры вызова
	Params map[string]interface{}

	// AgentContext контекст агента
	AgentContext *transport.AgentContext

	// Policy политика подтверждения
	Policy ApprovalPolicy

	// Status текущий статус
	Status ApprovalStatus

	// CreatedAt время создания
	CreatedAt int64
}

// ApprovalResponse ответ на запрос подтверждения
type ApprovalResponse struct {
	// RequestID ID запроса
	RequestID string

	// Approved одобрено ли
	Approved bool

	// Reason причина отказа (опционально)
	Reason string

	// RespondedBy кто ответил
	RespondedBy string
}

// Approver определяет интерфейс обработчика подтверждений
type Approver interface {
	// Approve обрабатывает запрос подтверждения
	Approve(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error)
}

// ApprovalManager управляет подтверждениями
type ApprovalManager struct {
	mu        sync.RWMutex
	requests  map[string]*ApprovalRequest
	approvers map[ApprovalPolicy]Approver
	history   []*ApprovalRequest
}

// NewApprovalManager создаёт новый менеджер подтверждений
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		requests:  make(map[string]*ApprovalRequest),
		approvers: make(map[ApprovalPolicy]Approver),
		history:   make([]*ApprovalRequest, 0),
	}
}

// RegisterApprover регистрирует обработчик подтверждений для политики
func (am *ApprovalManager) RegisterApprover(policy ApprovalPolicy, approver Approver) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.approvers[policy] = approver
}

// RequestApproval запрашивает подтверждение на вызов инструмента
func (am *ApprovalManager) RequestApproval(
	ctx context.Context,
	agentCtx *transport.AgentContext,
	tool Tool,
	params map[string]interface{},
) (bool, error) {
	policy := tool.ApprovalPolicy()

	// AutoApprove не требует подтверждения
	if policy == AutoApprove {
		return true, nil
	}

	// Deny всегда запрещает
	if policy == Deny {
		return false, fmt.Errorf("tool %q is denied by policy", tool.Name())
	}

	// Проверяем наличие обработчика
	am.mu.RLock()
	approver, exists := am.approvers[policy]
	am.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("no approver registered for policy %q", policy)
	}

	// Создаём запрос
	req := &ApprovalRequest{
		ID:              generateRequestID(),
		ToolName:        tool.Name(),
		ToolDescription: tool.Description(),
		Params:          params,
		AgentContext:    agentCtx,
		Policy:          policy,
		Status:          ApprovalPending,
	}

	// Сохраняем запрос
	am.mu.Lock()
	am.requests[req.ID] = req
	am.mu.Unlock()

	// Запрашиваем подтверждение
	resp, err := approver.Approve(ctx, req)
	if err != nil {
		am.mu.Lock()
		req.Status = ApprovalRejected
		am.mu.Unlock()
		return false, fmt.Errorf("approval failed: %w", err)
	}

	// Обновляем статус
	am.mu.Lock()
	if resp.Approved {
		req.Status = ApprovalApproved
	} else {
		req.Status = ApprovalRejected
	}
	// Переносим в историю
	delete(am.requests, req.ID)
	am.history = append(am.history, req)
	am.mu.Unlock()

	if !resp.Approved {
		reason := resp.Reason
		if reason == "" {
			reason = "no reason provided"
		}
		return false, fmt.Errorf("tool %q approval rejected: %s", tool.Name(), reason)
	}

	return true, nil
}

// GetPendingRequests возвращает ожидающие подтверждения запросы
func (am *ApprovalManager) GetPendingRequests() []*ApprovalRequest {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*ApprovalRequest, 0, len(am.requests))
	for _, req := range am.requests {
		if req.Status == ApprovalPending {
			result = append(result, req)
		}
	}
	return result
}

// GetHistory возвращает историю подтверждений
func (am *ApprovalManager) GetHistory() []*ApprovalRequest {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]*ApprovalRequest, len(am.history))
	copy(result, am.history)
	return result
}

// ConsoleApprover запрашивает подтверждение через консоль
type ConsoleApprover struct {
	promptFunc func(req *ApprovalRequest) bool
}

// NewConsoleApprover создаёт консольный обработчик подтверждений
func NewConsoleApprover(promptFunc func(req *ApprovalRequest) bool) *ConsoleApprover {
	return &ConsoleApprover{
		promptFunc: promptFunc,
	}
}

// Approve запрашивает подтверждение через консоль
func (ca *ConsoleApprover) Approve(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error) {
	approved := ca.promptFunc(req)

	resp := &ApprovalResponse{
		RequestID: req.ID,
		Approved:  approved,
	}

	if !approved {
		resp.Reason = "user rejected"
	}

	return resp, nil
}

// AutoApprover автоматически подтверждает все запросы (для тестов)
type AutoApprover struct{}

// NewAutoApprover создаёт автоматический обработчик
func NewAutoApprover() *AutoApprover {
	return &AutoApprover{}
}

// Approve автоматически подтверждает
func (aa *AutoApprover) Approve(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error) {
	return &ApprovalResponse{
		RequestID:   req.ID,
		Approved:    true,
		RespondedBy: "auto",
	}, nil
}

// AdminApprover требует подтверждения от администратора
type AdminApprover struct {
	checkAdmin func(agentCtx *transport.AgentContext) bool
	promptFunc func(req *ApprovalRequest) bool
}

// NewAdminApprover создаёт обработчик для администраторов
func NewAdminApprover(
	checkAdmin func(agentCtx *transport.AgentContext) bool,
	promptFunc func(req *ApprovalRequest) bool,
) *AdminApprover {
	return &AdminApprover{
		checkAdmin: checkAdmin,
		promptFunc: promptFunc,
	}
}

// Approve проверяет права администратора и запрашивает подтверждение
func (aa *AdminApprover) Approve(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error) {
	// Проверяем, является ли пользователь администратором
	if !aa.checkAdmin(req.AgentContext) {
		return &ApprovalResponse{
			RequestID: req.ID,
			Approved:  false,
			Reason:    "requires admin privileges",
		}, nil
	}

	// Запрашиваем подтверждение
	approved := aa.promptFunc(req)

	return &ApprovalResponse{
		RequestID:   req.ID,
		Approved:    approved,
		RespondedBy: req.AgentContext.UserID,
	}, nil
}

// generateRequestID генерирует уникальный ID запроса
var requestCounter uint64
var requestMutex sync.Mutex

func generateRequestID() string {
	requestMutex.Lock()
	defer requestMutex.Unlock()
	requestCounter++
	return fmt.Sprintf("approval-%d", requestCounter)
}
