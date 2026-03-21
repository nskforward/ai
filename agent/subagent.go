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

func (sa *SubAgent) Execute(ctx context.Context, stepDescription string, priorContext string, transportName, userID string) (string, error) {
	availableTools := sa.tools.GetTools()
	toolNames := make([]string, len(availableTools))
	for i, t := range availableTools {
		toolNames[i] = t.Name()
	}
	sa.log.Debug("subagent initialized", "tools", toolNames, "step", stepDescription)

	history := []llm.Message{
		{Role: llm.RoleSystem, Content: `Твоя задача — выполнить один конкретный шаг из большого плана.
КРИТИЧЕСКИЕ ПРАВИЛА:
1. ТЕБЕ ДОСТУПНЫ ИНСТРУМЕНТЫ: http_get, save_skill, read_file. ТЫ ОБЯЗАН ИХ ИСПОЛЬЗОВАТЬ для действий.
2. Если шаг требует СОХРАНЕНИЯ опыта, ТЫ ОБЯЗАН вызвать save_skill с полезным КОДОМ и ИНСТРУКЦИЯМИ внутри.
3. ТЫ ДОЛЖЕН СНАЧАЛА ПОПРОБОВАТЬ вызвать инструмент. Не предполагай заранее, что у тебя нет доступа. (Инструменты заданы технически, они ЕСТЬ).
4. Если инструмент вернул успех, ТВОЙ ответ должен быть максимально кратким подтверждением.
5. КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО возвращать длинные блоки кода в финальном ответе, если ты уже сохранил их.
Возвращай только финальный результат своей работы.`},
		{Role: llm.RoleUser, Content: fmt.Sprintf("Шаг: %s\n\nКонтекст от предыдущих шагов:\n%s", stepDescription, priorContext)},
	}
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
