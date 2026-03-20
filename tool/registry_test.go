package tool_test

import (
	"context"
	"testing"

	"github.com/nskforward/ai/tool"
)

type DummyTool struct {
	adminOnly bool
}

func (d *DummyTool) Name() string { return "dummy" }
func (d *DummyTool) Description() string { return "" }
func (d *DummyTool) Schema() string { return "" }
func (d *DummyTool) RequiresAdmin() bool { return d.adminOnly }
func (d *DummyTool) Execute(ctx context.Context, tr, u, a string) (string, error) { return "ok", nil }

func TestRegistryACL(t *testing.T) {
	reg := tool.NewRegistry([]tool.AdminUser{{Transport: "telegram", UserID: "admin_user"}})

	reg.Register(&DummyTool{adminOnly: true})

	// Non-admin should fail
	_, err := reg.Execute(context.Background(), "dummy", "telegram", "guest", "{}")
	if err != tool.ErrAdminRequired {
		t.Fatalf("expected ErrAdminRequired, got: %v", err)
	}

	// Wrong transport should fail
	_, err = reg.Execute(context.Background(), "dummy", "console", "admin_user", "{}")
	if err != tool.ErrAdminRequired {
		t.Fatalf("expected ErrAdminRequired for wrong transport, got: %v", err)
	}

	// Admin should succeed
	res, err := reg.Execute(context.Background(), "dummy", "telegram", "admin_user", "{}")
	if err != nil || len(res) == 0 {
		t.Fatalf("expected success, got error: %v", err)
	}
}
