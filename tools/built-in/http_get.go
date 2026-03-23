package builtin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nskforward/ai/tools"
	"github.com/nskforward/ai/transport"
)

// HTTPGetTool выполняет HTTP GET запросы
type HTTPGetTool struct {
	client *http.Client
}

// NewHTTPGetTool создаёт новый HTTP GET инструмент
func NewHTTPGetTool() *HTTPGetTool {
	return &HTTPGetTool{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *HTTPGetTool) Name() string {
	return "http_get"
}

func (t *HTTPGetTool) Description() string {
	return "Выполняет HTTP GET запрос к указанному URL и возвращает содержимое ответа."
}

func (t *HTTPGetTool) Parameters() map[string]tools.ParameterSchema {
	return map[string]tools.ParameterSchema{
		"url": {
			Type:        "string",
			Description: "URL для запроса",
			Required:    true,
		},
	}
}

func (t *HTTPGetTool) Call(ctx context.Context, agentCtx *transport.AgentContext, params map[string]interface{}) (*tools.ToolResult, error) {
	start := time.Now()

	url, ok := params["url"].(string)
	if !ok || url == "" {
		return &tools.ToolResult{
			Success:       false,
			Error:         "parameter 'url' is required and must be a string",
			ExecutionTime: time.Since(start),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("failed to create request: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("request failed: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &tools.ToolResult{
			Success:       false,
			Error:         fmt.Sprintf("failed to read response: %v", err),
			ExecutionTime: time.Since(start),
		}, nil
	}

	return &tools.ToolResult{
		Success: true,
		Output: map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(body),
			"headers":     resp.Header,
		},
		ExecutionTime: time.Since(start),
	}, nil
}

func (t *HTTPGetTool) ApprovalPolicy() tools.ApprovalPolicy {
	return tools.RequireApproval
}

func (t *HTTPGetTool) IsAvailable(ctx context.Context, agentCtx *transport.AgentContext) bool {
	return true
}
