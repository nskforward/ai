package builtin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// FileWriteTool записывает содержимое в файл
type FileWriteTool struct{}

// NewFileWriteTool создаёт новый File Write инструмент
func NewFileWriteTool() *FileWriteTool {
	return &FileWriteTool{}
}

func (t *FileWriteTool) Name() string {
	return "file_write"
}

func (t *FileWriteTool) Description() string {
	return "Записывает содержимое в файл по указанному пути. Создаёт файл, если он не существует."
}

func (t *FileWriteTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"path": {
			Type:        "string",
			Description: "Путь к файлу",
			Required:    true,
		},
		"content": {
			Type:        "string",
			Description: "Содержимое для записи",
			Required:    true,
		},
	}
}

func (t *FileWriteTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	start := time.Now()

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'path' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	content, ok := params["content"].(string)
	if !ok {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'content' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("failed to write file: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}

	return &tools.ToolResult{
		Success: true,
		Output: map[string]interface{}{
			"path":    path,
			"written": len(content),
		},
		ExecutionTime: time.Since(start),
	}, nil
}

func (t *FileWriteTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.RequireAdminApproval
}

func (t *FileWriteTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return agentCtx.IsAdmin
}
