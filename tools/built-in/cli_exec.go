package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// CLIExecTool выполняет CLI команды
type CLIExecTool struct{}

// NewCLIExecTool создаёт новый CLI Exec инструмент
func NewCLIExecTool() *CLIExecTool {
	return &CLIExecTool{}
}

func (t *CLIExecTool) Name() string {
	return "cli_exec"
}

func (t *CLIExecTool) Description() string {
	return "Выполняет команду в командной строке и возвращает результат."
}

func (t *CLIExecTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"command": {
			Type:        "string",
			Description: "Команда для выполнения",
			Required:    true,
		},
		"args": {
			Type:        "array",
			Description: "Аргументы команды",
			Required:    false,
		},
	}
}

func (t *CLIExecTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	start := time.Now()

	command, ok := params["command"].(string)
	if !ok || command == "" {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'command' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	var args []string
	if argsRaw, ok := params["args"].([]interface{}); ok {
		for _, a := range argsRaw {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}

	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()

	result := &tools.ToolResult{
		ExecutionTime: time.Since(start),
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("command failed: %v", err)
		result.Output = map[string]interface{}{
			"stdout": string(output),
			"stderr": err.Error(),
		}
	} else {
		result.Success = true
		result.Output = map[string]interface{}{
			"stdout": string(output),
		}
	}

	return result, nil
}

func (t *CLIExecTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.RequireAdminApproval
}

func (t *CLIExecTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return agentCtx.IsAdmin
}
