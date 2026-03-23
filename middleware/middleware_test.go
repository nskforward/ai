package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/nskforward/ai/transport"
)

func TestLoggingMiddleware(t *testing.T) {
	var logged []string

	logger := &testLogger{
		logFunc: func(format string, v ...interface{}) {
			logged = append(logged, format)
		},
	}

	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		return transport.NewMessage("response"), nil
	}

	middleware := LoggingMiddleware(logger)
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}

	if len(logged) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(logged))
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var logged []string

	logger := &testLogger{
		logFunc: func(format string, v ...interface{}) {
			logged = append(logged, format)
		},
	}

	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		panic("test panic")
	}

	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err == nil {
		t.Fatal("expected error after panic")
	}

	if resp != nil {
		t.Error("expected nil response after panic")
	}

	if len(logged) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logged))
	}
}

func TestRecoveryMiddlewareNoPanic(t *testing.T) {
	logger := &testLogger{
		logFunc: func(format string, v ...interface{}) {},
	}

	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		return transport.NewMessage("response"), nil
	}

	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}
}

func TestValidationMiddleware(t *testing.T) {
	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		return transport.NewMessage("response"), nil
	}

	middleware := ValidationMiddleware(NotEmpty())
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")

	// Test with empty message
	msg := transport.NewMessage("")
	_, err := wrappedHandler(ctx, agentCtx, msg)
	if err == nil {
		t.Error("expected error for empty message")
	}

	// Test with non-empty message
	msg = transport.NewMessage("Hello")
	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}
}

func TestValidationMiddlewareMaxLength(t *testing.T) {
	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		return transport.NewMessage("response"), nil
	}

	middleware := ValidationMiddleware(MaxLength(10))
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")

	// Test with long message
	msg := transport.NewMessage("This is a very long message that exceeds the limit")
	_, err := wrappedHandler(ctx, agentCtx, msg)
	if err == nil {
		t.Error("expected error for long message")
	}

	// Test with short message
	msg = transport.NewMessage("Short")
	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewTokenBucket(1, 1) // 1 token per second, capacity 1

	handler := func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
		return transport.NewMessage("response"), nil
	}

	middleware := RateLimitMiddleware(limiter)
	wrappedHandler := middleware(handler)

	ctx := context.Background()
	agentCtx := transport.NewAgentContext("user-123", "session-456", "test")
	msg := transport.NewMessage("Hello")

	// First request should succeed
	resp, err := wrappedHandler(ctx, agentCtx, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "response" {
		t.Errorf("expected response 'response', got '%s'", resp.Text)
	}

	// Second request should fail (rate limited)
	_, err = wrappedHandler(ctx, agentCtx, msg)
	if err == nil {
		t.Error("expected rate limit error")
	}
}

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(2, 5) // 2 tokens per second, capacity 5

	// Should allow 5 requests immediately
	for i := 0; i < 5; i++ {
		if !tb.Allow("user1") {
			t.Errorf("expected request %d to be allowed", i+1)
		}
	}

	// 6th request should be denied
	if tb.Allow("user1") {
		t.Error("expected 6th request to be denied")
	}

	// Different user should have separate bucket
	if !tb.Allow("user2") {
		t.Error("expected request for user2 to be allowed")
	}
}

// testLogger implements Logger interface for testing
type testLogger struct {
	logFunc func(format string, v ...interface{})
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(format, v...)
	}
}

// Ensure errors package is used
var _ = errors.New
