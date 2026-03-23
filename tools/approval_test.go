package tools

import (
	"context"
	"testing"

	"github.com/nskforward/ai/transport"
)

// MockTool мок инструмента для тестов
type MockTool struct {
	name        string
	policy      ApprovalPolicy
	isAvailable bool
	callResult  *ToolResult
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return "Mock tool for testing"
}

func (m *MockTool) Parameters() map[string]ParameterSchema {
	return map[string]ParameterSchema{
		"input": {
			Type:        "string",
			Description: "Input parameter",
			Required:    true,
		},
	}
}

func (m *MockTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*ToolResult, error) {
	return m.callResult, nil
}

func (m *MockTool) ApprovalPolicy() ApprovalPolicy {
	return m.policy
}

func (m *MockTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return m.isAvailable
}

func TestApprovalManager_AutoApprove(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	tool := &MockTool{
		name:        "test_tool",
		policy:      AutoApprove,
		isAvailable: true,
		callResult:  &ToolResult{Success: true, Output: "result"},
	}

	approved, err := am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval for AutoApprove policy")
	}
}

func TestApprovalManager_Deny(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	tool := &MockTool{
		name:        "denied_tool",
		policy:      Deny,
		isAvailable: true,
	}

	approved, err := am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	if err == nil {
		t.Fatal("expected error for Deny policy")
	}
	if approved {
		t.Fatal("expected denial for Deny policy")
	}
}

func TestApprovalManager_RequireApproval_Approved(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	// Регистрируем автоматический подтверждатель
	am.RegisterApprover(RequireApproval, NewAutoApprover())

	tool := &MockTool{
		name:        "approval_tool",
		policy:      RequireApproval,
		isAvailable: true,
	}

	approved, err := am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval from AutoApprover")
	}
}

func TestApprovalManager_RequireApproval_Rejected(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	// Регистрируем отклоняющий подтверждатель
	am.RegisterApprover(RequireApproval, NewConsoleApprover(func(req *ApprovalRequest) bool {
		return false // Always reject
	}))

	tool := &MockTool{
		name:        "approval_tool",
		policy:      RequireApproval,
		isAvailable: true,
	}

	approved, err := am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	if err == nil {
		t.Fatal("expected error for rejected approval")
	}
	if approved {
		t.Fatal("expected rejection")
	}
}

func TestApprovalManager_RequireAdminApproval(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()

	// Тест для администратора
	adminCtx := &transport.AgentContext{
		UserID:    "admin1",
		SessionID: "session1",
		IsAdmin:   true,
	}

	am.RegisterApprover(RequireAdminApproval, NewAdminApprover(
		func(agentCtx *transport.AgentContext) bool {
			return agentCtx.IsAdmin
		},
		func(req *ApprovalRequest) bool {
			return true // Admin approves
		},
	))

	tool := &MockTool{
		name:        "admin_tool",
		policy:      RequireAdminApproval,
		isAvailable: true,
	}

	approved, err := am.RequestApproval(ctx, adminCtx, tool, map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval for admin")
	}

	// Тест для не-администратора
	userCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
		IsAdmin:   false,
	}

	approved, err = am.RequestApproval(ctx, userCtx, tool, map[string]interface{}{"input": "test"})
	if err == nil {
		t.Fatal("expected error for non-admin")
	}
	if approved {
		t.Fatal("expected rejection for non-admin")
	}
}

func TestApprovalManager_NoApprover(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	tool := &MockTool{
		name:        "no_approver_tool",
		policy:      RequireApproval,
		isAvailable: true,
	}

	// Не регистрируем подтверждателя
	approved, err := am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	if err == nil {
		t.Fatal("expected error when no approver registered")
	}
	if approved {
		t.Fatal("expected rejection when no approver")
	}
}

func TestApprovalManager_GetPendingRequests(t *testing.T) {
	am := NewApprovalManager()

	// Изначально нет ожидающих запросов
	pending := am.GetPendingRequests()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending requests, got %d", len(pending))
	}
}

func TestApprovalManager_GetHistory(t *testing.T) {
	am := NewApprovalManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	am.RegisterApprover(RequireApproval, NewAutoApprover())

	tool := &MockTool{
		name:        "history_tool",
		policy:      RequireApproval,
		isAvailable: true,
	}

	// Выполняем несколько запросов
	for i := 0; i < 3; i++ {
		_, _ = am.RequestApproval(ctx, agentCtx, tool, map[string]interface{}{"input": "test"})
	}

	history := am.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
}

func TestToolManager_WithApproval(t *testing.T) {
	tm := NewToolManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	// Регистрируем подтверждателя
	tm.GetApprovalManager().RegisterApprover(RequireApproval, NewAutoApprover())

	// Регистрируем инструмент с RequireApproval
	tool := &MockTool{
		name:        "managed_tool",
		policy:      RequireApproval,
		isAvailable: true,
		callResult:  &ToolResult{Success: true, Output: "success"},
	}

	err := tm.Register(tool)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// Выполняем инструмент
	result, err := tm.Execute(ctx, agentCtx, "managed_tool", map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful execution")
	}
}

func TestToolManager_DenyPolicy(t *testing.T) {
	tm := NewToolManager()
	ctx := context.Background()
	agentCtx := &transport.AgentContext{
		UserID:    "user1",
		SessionID: "session1",
	}

	// Регистрируем инструмент с Deny
	tool := &MockTool{
		name:        "denied_tool",
		policy:      Deny,
		isAvailable: true,
	}

	err := tm.Register(tool)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// Выполняем инструмент
	result, err := tm.Execute(ctx, agentCtx, "denied_tool", map[string]interface{}{"input": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected denied execution")
	}
}
