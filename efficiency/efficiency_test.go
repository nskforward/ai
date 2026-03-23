package efficiency

import (
	"context"
	"testing"
	"time"

	"github.com/nskforward/ai/llm"
)

// TestPromptCache тестирует PromptCache
func TestPromptCache(t *testing.T) {
	cache := NewPromptCache(10, 1*time.Hour)

	// Test Set and Get
	cache.Set("key1", "value1", 0)
	val, ok := cache.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected value1, got %s", val)
	}

	// Test MarkCacheable
	key := cache.MarkCacheable("test prompt")
	val, ok = cache.Get(key)
	if !ok || val != "test prompt" {
		t.Errorf("Expected 'test prompt', got %s", val)
	}

	// Test Invalidate
	cache.Invalidate(key)
	_, ok = cache.Get(key)
	if ok {
		t.Error("Expected key to be invalidated")
	}

	// Test GetStats
	// key1 был проинвалидирован, поэтому должно быть 1 запись (от MarkCacheable)
	stats := cache.GetStats()
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 entry, got %d", stats.TotalEntries)
	}

	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)
	stats = cache.GetStats()
	if stats.TotalEntries != 3 {
		t.Errorf("Expected 3 entries, got %d", stats.TotalEntries)
	}

	// Test InvalidateAll
	cache.InvalidateAll()
	stats = cache.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after InvalidateAll, got %d", stats.TotalEntries)
	}
}

// TestPromptCacheExpiration тестирует истечение кэша
func TestPromptCacheExpiration(t *testing.T) {
	cache := NewPromptCache(10, 100*time.Millisecond)

	cache.Set("key1", "value1", 50*time.Millisecond)

	// Должен существовать
	_, ok := cache.Get("key1")
	if !ok {
		t.Error("Expected key to exist")
	}

	// Ждём истечения
	time.Sleep(100 * time.Millisecond)

	// Не должен существовать
	_, ok = cache.Get("key1")
	if ok {
		t.Error("Expected key to be expired")
	}
}

// TestSimpleSummarizer тестирует SimpleSummarizer
func TestSimpleSummarizer(t *testing.T) {
	summarizer := NewSimpleSummarizer(nil)

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello, how are you?"},
		{Role: llm.RoleAssistant, Content: "I'm doing well, thank you!"},
		{Role: llm.RoleUser, Content: "Can you help me with a task?"},
		{Role: llm.RoleAssistant, Content: "Of course! What do you need help with?"},
	}

	summary, err := summarizer.Summarize(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Test ShouldSummarize
	if !summarizer.ShouldSummarize(messages, 5000) {
		t.Error("Expected ShouldSummarize to return true for high token count")
	}

	if summarizer.ShouldSummarize(messages, 100) {
		t.Error("Expected ShouldSummarize to return false for low token count")
	}
}

// TestSimpleSummarizerMerge тестирует Merge
func TestSimpleSummarizerMerge(t *testing.T) {
	summarizer := NewSimpleSummarizer(nil)

	recentMessages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant"},
		{Role: llm.RoleUser, Content: "Message 1"},
		{Role: llm.RoleAssistant, Content: "Response 1"},
		{Role: llm.RoleUser, Content: "Message 2"},
		{Role: llm.RoleAssistant, Content: "Response 2"},
	}

	merged := summarizer.Merge("Previous conversation summary", recentMessages)

	// Должен содержать системный промпт, сводку и последние сообщения
	if len(merged) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(merged))
	}

	// Первый должен быть системным
	if merged[0].Role != llm.RoleSystem {
		t.Error("Expected first message to be system")
	}
}

// TestRuleBasedClassifier тестирует RuleBasedClassifier
func TestRuleBasedClassifier(t *testing.T) {
	classifier := NewRuleBasedClassifier()

	// Test simple message
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello, how are you?"},
	}

	complexity, err := classifier.Classify(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if complexity != ComplexitySimple {
		t.Errorf("Expected simple, got %s", complexity)
	}

	// Test moderate message
	messages = []llm.Message{
		{Role: llm.RoleUser, Content: "Please explain how to write a function in Go"},
	}

	complexity, err = classifier.Classify(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if complexity != ComplexityModerate {
		t.Errorf("Expected moderate, got %s", complexity)
	}

	// Test complex message
	messages = []llm.Message{
		{Role: llm.RoleUser, Content: "Analyze and optimize this complex algorithm"},
	}

	complexity, err = classifier.Classify(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if complexity != ComplexityComplex {
		t.Errorf("Expected complex, got %s", complexity)
	}
}

// TestKeywordClassifier тестирует KeywordClassifier
func TestKeywordClassifier(t *testing.T) {
	classifier := NewKeywordClassifier()

	// Test simple
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "hi hello thanks"},
	}

	complexity, err := classifier.Classify(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if complexity != ComplexitySimple {
		t.Errorf("Expected simple, got %s", complexity)
	}

	// Test complex
	messages = []llm.Message{
		{Role: llm.RoleUser, Content: "analyze research design optimize"},
	}

	complexity, err = classifier.Classify(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if complexity != ComplexityComplex {
		t.Errorf("Expected complex, got %s", complexity)
	}
}

// TestSmartRouter тестирует SmartRouter
func TestSmartRouter(t *testing.T) {
	// Используем RuleBasedClassifier вместо LLM
	classifier := NewRuleBasedClassifier()
	config := DefaultRouterConfig()

	router := &SmartRouter{
		config:     config,
		classifier: classifier,
	}

	// Test simple routing
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello"},
	}

	model, err := router.Route(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if model != config.Mapping.Simple {
		t.Errorf("Expected %s, got %s", config.Mapping.Simple, model)
	}

	// Test complex routing
	messages = []llm.Message{
		{Role: llm.RoleUser, Content: "Analyze and optimize this complex system"},
	}

	model, err = router.Route(context.Background(), messages)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if model != config.Mapping.Complex {
		t.Errorf("Expected %s, got %s", config.Mapping.Complex, model)
	}
}

// TestBudgetManager тестирует BudgetManager
func TestBudgetManager(t *testing.T) {
	config := &BudgetConfig{
		DailyLimit:       1000,
		MonthlyLimit:     10000,
		RequestLimit:     500,
		Policy:           BudgetPolicyAdaptive,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}

	manager := NewBudgetManager(config)

	// Test CheckBudget
	result, err := manager.CheckBudget(context.Background(), 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("Expected budget to be allowed")
	}

	// Test RecordUsage
	manager.RecordUsage(100, "gpt-4o-mini", "session1", "user1")

	stats := manager.GetStats()
	if stats.DailyUsed != 100 {
		t.Errorf("Expected DailyUsed to be 100, got %d", stats.DailyUsed)
	}

	// Test budget exceeded
	result, err = manager.CheckBudget(context.Background(), 1000)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.AdjustedTokens == 0 {
		t.Error("Expected AdjustedTokens to be set")
	}
}

// TestBudgetManagerHardPolicy тестирует жёсткую политику
func TestBudgetManagerHardPolicy(t *testing.T) {
	config := &BudgetConfig{
		DailyLimit:       1000,
		MonthlyLimit:     10000,
		RequestLimit:     500,
		Policy:           BudgetPolicyHard,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}

	manager := NewBudgetManager(config)

	// Используем почти весь дневной лимит
	manager.RecordUsage(900, "gpt-4o-mini", "session1", "user1")

	// Проверяем, что запрос превышает лимит
	result, err := manager.CheckBudget(context.Background(), 200)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("Expected budget to be denied")
	}

	if result.Reason == "" {
		t.Error("Expected reason to be set")
	}
}

// TestBudgetManagerReset тестирует сброс лимитов
func TestBudgetManagerReset(t *testing.T) {
	config := DefaultBudgetConfig()
	manager := NewBudgetManager(config)

	// Используем токены
	manager.RecordUsage(1000, "gpt-4o-mini", "session1", "user1")

	stats := manager.GetStats()
	if stats.DailyUsed != 1000 {
		t.Errorf("Expected DailyUsed to be 1000, got %d", stats.DailyUsed)
	}

	// Сбрасываем дневной лимит
	manager.ResetDaily()

	stats = manager.GetStats()
	if stats.DailyUsed != 0 {
		t.Errorf("Expected DailyUsed to be 0 after reset, got %d", stats.DailyUsed)
	}
}

// TestSessionBudgetManager тестирует SessionBudgetManager
func TestSessionBudgetManager(t *testing.T) {
	config := &BudgetConfig{
		DailyLimit:       10000,
		MonthlyLimit:     100000,
		RequestLimit:     5000,
		Policy:           BudgetPolicyHard,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}
	globalManager := NewBudgetManager(config)
	sessionManager := NewSessionBudgetManager(globalManager)

	// Устанавливаем лимит сессии
	sessionManager.SetSessionLimit("session1", 500)

	// Проверяем бюджет сессии
	result, err := sessionManager.CheckSessionBudget(context.Background(), "session1", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !result.Allowed {
		t.Error("Expected budget to be allowed")
	}

	// Проверяем превышение лимита сессии
	result, err = sessionManager.CheckSessionBudget(context.Background(), "session1", 600)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.Allowed {
		t.Error("Expected budget to be denied")
	}
}

// TestBudgetMiddleware тестирует BudgetMiddleware
func TestBudgetMiddleware(t *testing.T) {
	config := &BudgetConfig{
		DailyLimit:       10000,
		MonthlyLimit:     100000,
		RequestLimit:     5000,
		Policy:           BudgetPolicyAdaptive,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}

	manager := NewBudgetManager(config)
	middleware := NewBudgetMiddleware(manager)

	// Test successful request
	handler := func(adjustedTokens int) (*llm.GenerateResponse, error) {
		return &llm.GenerateResponse{
			Content: "Test response",
			Usage: &llm.TokenUsage{
				TotalTokens: 100,
			},
			Model: "gpt-4o-mini",
		}, nil
	}

	resp, err := middleware.WrapRequest(context.Background(), 100, handler)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if resp.Content != "Test response" {
		t.Errorf("Expected 'Test response', got %s", resp.Content)
	}

	// Проверяем, что использование записано
	stats := manager.GetStats()
	if stats.DailyUsed != 100 {
		t.Errorf("Expected DailyUsed to be 100, got %d", stats.DailyUsed)
	}
}
