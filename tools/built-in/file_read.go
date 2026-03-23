package builtin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// FileReadTool читает содержимое файла
type FileReadTool struct{}

// NewFileReadTool создаёт новый File Read инструмент
func NewFileReadTool() *FileReadTool {
	return &FileReadTool{}
}

func (t *FileReadTool) Name() string {
	return "file_read"
}

func (t *FileReadTool) Description() string {
	return "Читает содержимое файла по указанному пути."
}

func (t *FileReadTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"path": {
			Type:        "string",
			Description: "Путь к файлу",
			Required:    true,
		},
	}
}

func (t *FileReadTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	start := time.Now()

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'path' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("failed to read file: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}

	return &tools.ToolResult{
		Success: true,
		Output: map[string]interface{}{
			"path":    path,
			"content": string(content),
			"size":    len(content),
		},
		ExecutionTime: time.Since(start),
	}, nil
}

func (t *FileReadTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.RequireApproval
}

func (t *FileReadTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return true
}
