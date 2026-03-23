package builtin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// FolderListTool выводит список файлов в папке
type FolderListTool struct{}

// NewFolderListTool создаёт новый Folder List инструмент
func NewFolderListTool() *FolderListTool {
	return &FolderListTool{}
}

func (t *FolderListTool) Name() string {
	return "folder_list"
}

func (t *FolderListTool) Description() string {
	return "Выводит список файлов и папок в указанной директории."
}

func (t *FolderListTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"path": {
			Type:        "string",
			Description: "Путь к директории",
			Required:    true,
		},
	}
}

func (t *FolderListTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	start := time.Now()

	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'path' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("failed to read directory: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		fileInfo := map[string]interface{}{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
		}
		if err == nil {
			fileInfo["size"] = info.Size()
			fileInfo["modified"] = info.ModTime().Format(time.RFC3339)
		}
		files = append(files, fileInfo)
	}

	return &tools.ToolResult{
		Success: true,
		Output: map[string]interface{}{
			"path":  path,
			"files": files,
			"count": len(files),
		},
		ExecutionTime: time.Since(start),
	}, nil
}

func (t *FolderListTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.AutoApprove
}

func (t *FolderListTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return true
}
