package agent

import (
	"context"
	"testing"

	"github.com/nskforward/ai/llm"
	llmmock "github.com/nskforward/ai/llm/mock"
	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

func TestNewAgent(t *testing.T) {
	config := DefaultConfig()
	agent := NewAgent(config)

	if agent.GetName() != "Agent" {
		t.Errorf("expected name 'Agent', got '%s'", agent.GetName())
	}

	if agent.GetConfig() != config {
		t.Error("expected config to match")
	}
}

func TestNewAgentNilConfig(t *testing.T) {
	agent := NewAgent(nil)

	if agent.GetConfig() == nil {
		t.Error("expected default config to be set")
	}

	if agent.GetName() != "Agent" {
		t.Errorf("expected default name 'Agent', got '%s'", agent.GetName())
	}
}

func TestAgentSetLLM(t *testing.T) {
	agent := NewAgent(DefaultConfig())
	mockLLM := llmmock.NewMockLLM()

	agent.SetLLM(mockLLM)

	// Verify LLM is set by running a message
	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	resp, err := agent.Run(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Mock response" {
		t.Errorf("expected 'Mock response', got '%s'", resp.Text)
	}

	if mockLLM.CallCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", mockLLM.CallCount)
	}
}

func TestAgentRunWithoutLLM(t *testing.T) {
	agent := NewAgent(DefaultConfig())

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	_, err := agent.Run(ctx, agentCtx, msg)
	if err == nil {
		t.Error("expected error when LLM not set")
	}
}

func TestAgentRunWithCustomResponse(t *testing.T) {
	agent := NewAgent(DefaultConfig())
	mockLLM := llmmock.NewMockLLM().WithResponse("Custom response")
	agent.SetLLM(mockLLM)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	resp, err := agent.Run(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Custom response" {
		t.Errorf("expected 'Custom response', got '%s'", resp.Text)
	}
}

func TestAgentRunWithTool(t *testing.T) {
	config := DefaultConfig()
	config.SystemPrompt = "You are a helpful assistant."
	agent := NewAgent(config)

	// Register a mock tool
	mockTool := &mockTool{name: "echo"}
	agent.RegisterTool(mockTool)

	// First call returns tool call, second call returns final response
	callCount := 0
	mockLLM := llmmock.NewMockLLM()
	mockLLM.GenerateFunc = func(ctx context.Context, req *llm.GenerateRequest) (*llm.GenerateResponse, error) {
		callCount++
		if callCount == 1 {
			return &llm.GenerateResponse{
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call-1",
						Name:      "echo",
						Arguments: map[string]interface{}{"text": "hello"},
					},
				},
				FinishReason: "tool_calls",
				Model:        "mock-model",
			}, nil
		}
		return &llm.GenerateResponse{
			Content:      "Final response",
			FinishReason: "stop",
			Model:        "mock-model",
		}, nil
	}
	agent.SetLLM(mockLLM)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	agentCtx.IsAdmin = true
	msg := transport.NewMessage("Use echo tool")

	resp, err := agent.Run(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Final response" {
		t.Errorf("expected 'Final response', got '%s'", resp.Text)
	}

	if mockLLM.CallCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mockLLM.CallCount)
	}
}

func TestAgentHistory(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistorySize = 5
	config.EnableHistory = true
	agent := NewAgent(config)

	mockLLM := llmmock.NewMockLLM().WithResponse("Response")
	agent.SetLLM(mockLLM)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")

	// Send multiple messages
	for i := 0; i < 3; i++ {
		msg := transport.NewMessage("Message")
		_, err := agent.Run(ctx, agentCtx, msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify history is maintained
	history := agent.getHistory("session-456")
	// Each message adds: user message + assistant response = 2 messages
	// Plus system message at the start
	if len(history) < 3 {
		t.Errorf("expected at least 3 messages in history, got %d", len(history))
	}
}

// mockTool is a simple mock tool for testing
type mockTool struct {
	name string
}

func (t *mockTool) Name() string {
	return t.name
}

func (t *mockTool) Description() string {
	return "Mock tool for testing"
}

func (t *mockTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"text": {
			Type:        "string",
			Description: "Text to echo",
			Required:    true,
		},
	}
}

func (t *mockTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	text, _ := params["text"].(string)
	return &tools.ToolResult{
		Success: true,
		Output:  map[string]interface{}{"echoed": text},
	}, nil
}

func (t *mockTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.AutoApprove
}

func (t *mockTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return true
}
