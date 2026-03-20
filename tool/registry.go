package tool

import (
	"context"
	"errors"
)

var ErrToolNotFound = errors.New("tool not found")
var ErrAdminRequired = errors.New("permission denied: this tool requires admin access")

// AdminUser defines a privileged user for a specific transport.
type AdminUser struct {
	Transport string
	UserID    string
}

// Registry manages available tools and validates execution rights (ACL).
type Registry struct {
	tools  map[string]Tool
	admins map[string]bool
}

// NewRegistry creates a new tool registry and registers the list of admin IDs.
func NewRegistry(admins []AdminUser) *Registry {
	adminMap := make(map[string]bool)
	for _, a := range admins {
		key := a.Transport + ":" + a.UserID
		adminMap[key] = true
	}
	return &Registry{
		tools:  make(map[string]Tool),
		admins: adminMap,
	}
}

// Register adds a new tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// GetTools returns all registered tools.
func (r *Registry) GetTools() []Tool {
	var list []Tool
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// Execute looks up a tool by name, checks ACL against transport+userID, and runs it.
func (r *Registry) Execute(ctx context.Context, name string, transportName string, userID string, args string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", ErrToolNotFound
	}

	if t.RequiresAdmin() {
		key := transportName + ":" + userID
		if !r.admins[key] {
			return "", ErrAdminRequired
		}
	}

	return t.Execute(ctx, transportName, userID, args)
}
