package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nskforward/ai/efficiency"
	"github.com/nskforward/ai/llm"
)

func main() {
	ctx := context.Background()

	// 1. Создание Prompt Cache
	cache := efficiency.NewPromptCache(100, 1*time.Hour)
	systemPrompt := "You are a helpful AI assistant. Be concise and efficient."
	systemPromptKey := cache.MarkCacheable(systemPrompt)

	fmt.Printf("System prompt cached with key: %s\n", systemPromptKey)

	// 2. Создание Model Router
	classifier := efficiency.NewRuleBasedClassifier()
	router := efficiency.NewSmartRouter(nil, nil)
	router.RegisterClassifier(classifier)

	// 3. Создание Budget Manager
	budgetConfig := &efficiency.BudgetConfig{
		DailyLimit:       50000,
		MonthlyLimit:     1000000,
		RequestLimit:     5000,
		Policy:           efficiency.BudgetPolicyAdaptive,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}

	budgetManager := efficiency.NewBudgetManager(budgetConfig)
	budgetMiddleware := efficiency.NewBudgetMiddleware(budgetManager)

	// 4. Создание Summarizer
	summarizer := efficiency.NewSimpleSummarizer(nil)

	// 5. Пример обработки сообщений
	messages := []string{
		"Hello, how are you?",
		"Can you explain what is machine learning?",
		"Write a Python function to sort a list",
		"Analyze the complexity of quicksort algorithm",
	}

	for i, msgText := range messages {
		fmt.Printf("\n=== Message %d ===\n", i+1)
		fmt.Printf("Input: %s\n", msgText)

		// Определение модели через роутер
		llmMessages := []llm.Message{
			{Role: llm.RoleUser, Content: msgText},
		}

		model, err := router.Route(ctx, llmMessages)
		if err != nil {
			log.Printf("Router error: %v", err)
			model = "openai/gpt-4o-mini"
		}
		fmt.Printf("Selected model: %s\n", model)

		// Оценка токенов (простая эвристика)
		estimatedTokens := len(msgText) * 2 // Примерная оценка

		// Проверка бюджета
		budgetResult, err := budgetManager.CheckBudget(ctx, estimatedTokens)
		if err != nil {
			log.Printf("Budget check error: %v", err)
			continue
		}

		if !budgetResult.Allowed {
			fmt.Printf("Budget exceeded: %s\n", budgetResult.Reason)
			continue
		}

		if budgetResult.Warning != "" {
			fmt.Printf("Budget warning: %s\n", budgetResult.Warning)
		}

		// Обработка с использованием middleware
		handler := func(adjustedTokens int) (*llm.GenerateResponse, error) {
			// Здесь был бы реальный вызов LLM
			// Для примера возвращаем mock ответ
			return &llm.GenerateResponse{
				Content: fmt.Sprintf("Response to: %s", msgText),
				Usage: &llm.TokenUsage{
					TotalTokens: adjustedTokens,
				},
				Model: model,
			}, nil
		}

		resp, err := budgetMiddleware.WrapRequest(ctx, estimatedTokens, handler)
		if err != nil {
			log.Printf("Request error: %v", err)
			continue
		}

		fmt.Printf("Response: %s\n", resp.Content)
		fmt.Printf("Tokens used: %d\n", resp.Usage.TotalTokens)

		// Проверка необходимости summarization
		history := []llm.Message{
			{Role: llm.RoleUser, Content: msgText},
			{Role: llm.RoleAssistant, Content: resp.Content},
		}

		if summarizer.ShouldSummarize(history, resp.Usage.TotalTokens) {
			fmt.Println("Summarization recommended")
			summary, err := summarizer.Summarize(ctx, history)
			if err == nil {
				fmt.Printf("Summary: %s\n", summary)
			}
		}
	}

	// 6. Статистика
	fmt.Println("\n=== Statistics ===")

	cacheStats := cache.GetStats()
	fmt.Printf("Cache: %d entries, %d hits\n", cacheStats.TotalEntries, cacheStats.TotalHits)

	budgetStats := budgetManager.GetStats()
	fmt.Printf("Budget: %d/%d daily tokens (%.1f%%)\n",
		budgetStats.DailyUsed,
		budgetStats.DailyUsed+budgetStats.DailyRemaining,
		budgetStats.DailyPercent)

	fmt.Printf("Budget: %d/%d monthly tokens (%.1f%%)\n",
		budgetStats.MonthlyUsed,
		budgetStats.MonthlyUsed+budgetStats.MonthlyRemaining,
		budgetStats.MonthlyPercent)
}
