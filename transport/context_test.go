package transport

import (
	"testing"
)

func TestNewAgentContext(t *testing.T) {
	ctx := NewAgentContext("user-123", "session-456", "telegram")

	if ctx.UserID != "user-123" {
		t.Errorf("expected UserID 'user-123', got '%s'", ctx.UserID)
	}

	if ctx.SessionID != "session-456" {
		t.Errorf("expected SessionID 'session-456', got '%s'", ctx.SessionID)
	}

	if ctx.TransportName != "telegram" {
		t.Errorf("expected TransportName 'telegram', got '%s'", ctx.TransportName)
	}

	if ctx.IsAdmin {
		t.Error("expected IsAdmin to be false by default")
	}

	if ctx.Metadata == nil {
		t.Error("expected Metadata to be initialized")
	}

	if ctx.Permissions == nil {
		t.Error("expected Permissions to be initialized")
	}

	if ctx.GetCreatedAt().IsZero() {
		t.Error("expected createdAt to be set")
	}
}

func TestAgentContextFields(t *testing.T) {
	ctx := NewAgentContext("user-123", "session-456", "console")
	ctx.UserName = "testuser"
	ctx.DisplayName = "Test User"
	ctx.Locale = "ru-RU"
	ctx.IsAdmin = true
	ctx.Metadata["key"] = "value"

	if ctx.UserName != "testuser" {
		t.Errorf("expected UserName 'testuser', got '%s'", ctx.UserName)
	}

	if ctx.DisplayName != "Test User" {
		t.Errorf("expected DisplayName 'Test User', got '%s'", ctx.DisplayName)
	}

	if ctx.Locale != "ru-RU" {
		t.Errorf("expected Locale 'ru-RU', got '%s'", ctx.Locale)
	}

	if !ctx.IsAdmin {
		t.Error("expected IsAdmin to be true")
	}

	if ctx.Metadata["key"] != "value" {
		t.Errorf("expected metadata key 'value', got '%v'", ctx.Metadata["key"])
	}
}

func TestHasPermission(t *testing.T) {
	ctx := NewAgentContext("user-123", "session-456", "console")
	ctx.Permissions = []string{"read", "write"}

	if !ctx.HasPermission("read") {
		t.Error("expected HasPermission('read') to be true")
	}

	if !ctx.HasPermission("write") {
		t.Error("expected HasPermission('write') to be true")
	}

	if ctx.HasPermission("admin") {
		t.Error("expected HasPermission('admin') to be false")
	}
}
