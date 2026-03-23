package tools

import (
	"context"
	"testing"

	"github.com/nskforward/ai/transport"
)

func TestNewToolManager(t *testing.T) {
	tm := NewToolManager()
	if tm == nil {
		t.Fatal("expected ToolManager to be created")
	}

	tools := tm.GetAll()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestToolManagerRegister(t *testing.T) {
	tm := NewToolManager()
	tool := &mockTool{name: "test"}

	err := tm.Register(tool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registered, err := tm.GetByName("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if registered.Name() != "test" {
		t.Errorf("expected tool name 'test', got '%s'", registered.Name())
	}
}

func TestToolManagerRegisterNil(t *testing.T) {
	tm := NewToolManager()

	err := tm.Register(nil)
	if err == nil {
		t.Error("expected error when registering nil tool")
	}
}

func TestToolManagerRegisterEmptyName(t *testing.T) {
	tm := NewToolManager()
	tool := &mockTool{name: ""}

	err := tm.Register(tool)
	if err == nil {
		t.Error("expected error when registering tool with empty name")
	}
}

func TestToolManagerRegisterDuplicate(t *testing.T) {
	tm := NewToolManager()
	tool1 := &mockTool{name: "test"}
	tool2 := &mockTool{name: "test"}

	err := tm.Register(tool1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = tm.Register(tool2)
	if err == nil {
		t.Error("expected error when registering duplicate tool")
	}
}

func TestToolManagerGetByNameNotFound(t *testing.T) {
	tm := NewToolManager()

	_, err := tm.GetByName("nonexistent")
	if err == nil {
		t.Error("expected error when getting nonexistent tool")
	}
}

func TestToolManagerGetAll(t *testing.T) {
	tm := NewToolManager()
	tool1 := &mockTool{name: "tool1"}
	tool2 := &mockTool{name: "tool2"}

	tm.Register(tool1)
	tm.Register(tool2)

	all := tm.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 tools, got %d", len(all))
	}
}

func TestToolManagerGetAvailable(t *testing.T) {
	tm := NewToolManager()

	availableTool := &mockTool{name: "available", available: true}
	unavailableTool := &mockTool{name: "unavailable", available: false}
	deniedTool := &mockTool{name: "denied", policy: Deny}

	tm.Register(availableTool)
	tm.Register(unavailableTool)
	tm.Register(deniedTool)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")

	defs := tm.GetAvailable(ctx, agentCtx)
	if len(defs) != 1 {
		t.Errorf("expected 1 available tool, got %d", len(defs))
	}

	if len(defs) > 0 && defs[0].Function.Name != "available" {
		t.Errorf("expected tool name 'available', got '%s'", defs[0].Function.Name)
	}
}

func TestToolManagerExecute(t *testing.T) {
	tm := NewToolManager()
	tool := &mockTool{name: "test", available: true}
	tm.Register(tool)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")
	params := map[string]interface{}{"key": "value"}

	result, err := tm.Execute(ctx, agentCtx, "test", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
}

func TestToolManagerExecuteNotFound(t *testing.T) {
	tm := NewToolManager()

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")
	params := map[string]interface{}{}

	_, err := tm.Execute(ctx, agentCtx, "nonexistent", params)
	if err == nil {
		t.Error("expected error when executing nonexistent tool")
	}
}

func TestToolManagerExecuteUnavailable(t *testing.T) {
	tm := NewToolManager()
	tool := &mockTool{name: "test", available: false}
	tm.Register(tool)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")
	params := map[string]interface{}{}

	result, err := tm.Execute(ctx, agentCtx, "test", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure for unavailable tool")
	}
}

func TestToolManagerExecuteDenied(t *testing.T) {
	tm := NewToolManager()
	tool := &mockTool{name: "test", available: true, policy: Deny}
	tm.Register(tool)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")
	params := map[string]interface{}{}

	result, err := tm.Execute(ctx, agentCtx, "test", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Error("expected failure for denied tool")
	}
}

func TestBuildParametersSchema(t *testing.T) {
	params := map[string]ParameterSchema{
		"required_param": {
			Type:        "string",
			Description: "A required parameter",
			Required:    true,
		},
		"optional_param": {
			Type:        "number",
			Description: "An optional parameter",
			Required:    false,
		},
	}

	schema := buildParametersSchema(params)

	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", schema["type"])
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties to be a map")
	}

	if len(properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(properties))
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected required to be a slice")
	}

	if len(required) != 1 {
		t.Errorf("expected 1 required parameter, got %d", len(required))
	}

	if required[0] != "required_param" {
		t.Errorf("expected required parameter 'required_param', got '%s'", required[0])
	}
}

// mockTool implements Tool interface for testing
type mockTool struct {
	name      string
	policy    ApprovalPolicy
	available bool
}

func (t *mockTool) Name() string {
	return t.name
}

func (t *mockTool) Description() string {
	return "Mock tool for testing"
}

func (t *mockTool) Parameters() map[string]ParameterSchema {
	return map[string]ParameterSchema{
		"key": {
			Type:        "string",
			Description: "Test parameter",
			Required:    true,
		},
	}
}

func (t *mockTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*ToolResult, error) {
	return &ToolResult{
		Success: true,
		Output:  map[string]interface{}{"result": "ok"},
	}, nil
}

func (t *mockTool) ApprovalPolicy() ApprovalPolicy {
	if t.policy != "" {
		return t.policy
	}
	return AutoApprove
}

func (t *mockTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return t.available
}
