package middleware

import (
	"context"
	"testing"

	"github.com/nskforward/ai/transport"
)

func TestNewMiddlewareChain(t *testing.T) {
	chain := NewMiddlewareChain()
	if chain.Len() != 0 {
		t.Errorf("expected chain length 0, got %d", chain.Len())
	}
}

func TestMiddlewareChainWithMiddlewares(t *testing.T) {
	m1 := func(next Handler) Handler {
		return next
	}
	m2 := func(next Handler) Handler {
		return next
	}

	chain := NewMiddlewareChain(m1, m2)
	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}
}

func TestMiddlewareChainAppend(t *testing.T) {
	chain := NewMiddlewareChain()

	m := func(next Handler) Handler {
		return next
	}

	chain.Append(m)
	if chain.Len() != 1 {
		t.Errorf("expected chain length 1, got %d", chain.Len())
	}
}

func TestMiddlewareChainPrepend(t *testing.T) {
	chain := NewMiddlewareChain()

	m1 := func(next Handler) Handler {
		return next
	}
	m2 := func(next Handler) Handler {
		return next
	}

	chain.Append(m1)
	chain.Prepend(m2)

	if chain.Len() != 2 {
		t.Errorf("expected chain length 2, got %d", chain.Len())
	}
}

func TestMiddlewareChainThen(t *testing.T) {
	var callOrder []string

	m1 := func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
			callOrder = append(callOrder, "m1-before")
			resp, err := next(ctx, agentCtx, msg)
			callOrder = append(callOrder, "m1-after")
			return resp, err
		}
	}

	m2 := func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
			callOrder = append(callOrder, "m2-before")
			resp, err := next(ctx, agentCtx, msg)
			callOrder = append(callOrder, "m2-after")
			return resp, err
		}
	}

	finalHandler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		callOrder = append(callOrder, "handler")
		return transport.NewMessage("response"), nil
	}

	chain := NewMiddlewareChain(m1, m2)
	handler := chain.Then(finalHandler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user", "session", "test")
	msg := transport.NewMessage("test")

	resp, err := handler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}

	expectedOrder := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(callOrder) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d", len(expectedOrder), len(callOrder))
	}

	for i, expected := range expectedOrder {
		if callOrder[i] != expected {
			t.Errorf("expected call order[%d] = '%s', got '%s'", i, expected, callOrder[i])
		}
	}
}

func TestMiddlewareChainThenNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when handler is nil")
		}
	}()

	chain := NewMiddlewareChain()
	chain.Then(nil)
}
