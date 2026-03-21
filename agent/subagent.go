package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/logger"
	"github.com/nskforward/ai/tool"
)

// SubAgent wraps LLM execution for a single, isolated step.
type SubAgent struct {
	provider llm.Provider
	tools    *tool.Registry
	maxSteps int
	log      logger.Logger
}

// Execute runs a fresh ReAct loop for a targeted sub-task.
func (sa *SubAgent) Execute(ctx context.Context, stepDescription string, priorContext string, transportName, userID string) (string, error) {
	prompt := fmt.Sprintf(`Твоя задача — выполнить один конкретный шаг из большого плана.
Шаг: %s

Контекст от предыдущих шагов:
%s

Возвращай только финальный результат своей работы.`, stepDescription, priorContext)

	history := []llm.Message{
		{Role: llm.RoleSystem, Content: prompt},
	}

	availableTools := sa.tools.GetTools()
	var finalResponse string

	for i := 0; i < sa.maxSteps; i++ {
		sa.log.Debug("subagent step", "iteration", i)
		resp, err := sa.provider.Generate(ctx, history, availableTools, nil)
		if err != nil {
			return "", fmt.Errorf("subagent LLM error: %w", err)
		}

		history = append(history, resp)

		if len(resp.ToolCalls) == 0 {
			finalResponse = resp.Content
			break
		}

		for _, tc := range resp.ToolCalls {
			start := time.Now()
			res, err := sa.tools.Execute(ctx, tc.Name, transportName, userID, tc.Args)
			elapsed := time.Since(start)

			if err != nil {
				sa.log.Error("subagent tool error", "tool", tc.Name, "error", err, "elapsed", elapsed)
				res = fmt.Sprintf("Error: %v", err)
			} else {
				sa.log.Debug("subagent tool executed", "tool", tc.Name, "elapsed", elapsed)
			}

			history = append(history, llm.Message{
				Role:       llm.RoleTool,
				Content:    res,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalResponse == "" {
		return "", fmt.Errorf("subagent failed to complete within %d steps", sa.maxSteps)
	}

	return finalResponse, nil
}
