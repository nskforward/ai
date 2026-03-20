package tool

import "context"

// Tool represents an executable function that an LLM can invoke.
type Tool interface {
	Name() string
	Description() string
	// Schema returns the JSON schema string for the function arguments.
	Schema() string
	// Execute runs the tool with JSON arguments and the UserID of the caller (for ACL).
	Execute(ctx context.Context, userID string, args string) (string, error)
	// RequiresAdmin returns true if this tool can only be called by white-listed admins.
	RequiresAdmin() bool
}
