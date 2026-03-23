# Efficiency Package

Пакет `efficiency` предоставляет компоненты для оптимизации использования токенов в AI агентах.

## Компоненты

### 1. Prompt Caching (`cache.go`)

Кеширование системных промптов для уменьшения количества токенов.

```go
// Создание кэша
cache := efficiency.NewPromptCache(100, 1*time.Hour)

// Кеширование промпта
key := cache.MarkCacheable("You are a helpful assistant")

// Получение из кэша
prompt, ok := cache.Get(key)

// Статистика
stats := cache.GetStats()
```

**Возможности:**
- Автоматическое истечение записей
- Ограничение размера кэша
- Сбор статистики (hit count)
- Периодическая очистка

### 2. Rolling Summarization (`summarizer.go`)

Сжатие истории сообщений для экономии токенов.

```go
// Создание summarizer
summarizer := efficiency.NewSimpleSummarizer(nil)

// Проверка необходимости summarization
if summarizer.ShouldSummarize(messages, tokenCount) {
    // Создание сводки
    summary, err := summarizer.Summarize(ctx, messages)
    
    // Объединение сводки с новыми сообщениями
    merged := summarizer.Merge(summary, recentMessages)
}
```

**Реализации:**
- `SimpleSummarizer` - простой summarizer без LLM
- `LLMSummarizer` - summarizer с использованием LLM

### 3. Model Routing (`router.go`)

Маршризация запросов по сложности для использования разных моделей.

```go
// Создание роутера
classifier := efficiency.NewRuleBasedClassifier()
router := &efficiency.SmartRouter{
    config:     efficiency.DefaultRouterConfig(),
    classifier: classifier,
}

// Определение модели
model, err := router.Route(ctx, messages)
```

**Классификаторы:**
- `RuleBasedClassifier` - классификация на основе правил
- `KeywordClassifier` - классификация по ключевым словам
- `LLMClassifier` - классификация с использованием LLM

**Уровни сложности:**
- `simple` - простые вопросы, приветствия
- `moderate` - многошаговые задачи, генерация кода
- `complex` - сложный анализ, исследования

### 4. Budget Management (`budget.go`)

Управление бюджетом токенов с поддержкой различных политик.

```go
// Создание менеджера бюджета
config := &efficiency.BudgetConfig{
    DailyLimit:       100000,
    MonthlyLimit:     2000000,
    RequestLimit:     10000,
    Policy:           efficiency.BudgetPolicyAdaptive,
    WarningThreshold: 0.8,
}

manager := efficiency.NewBudgetManager(config)

// Проверка бюджета
result, err := manager.CheckBudget(ctx, estimatedTokens)

// Запись использования
manager.RecordUsage(tokens, model, sessionID, userID)

// Статистика
stats := manager.GetStats()
```

**Политики:**
- `BudgetPolicyHard` - жёсткий лимит, прерывает при превышении
- `BudgetPolicySoft` - мягкий лимит, предупреждает
- `BudgetPolicyAdaptive` - адаптивный лимит, уменьшает MaxTokens

**Middleware:**
```go
// Создание middleware
middleware := efficiency.NewBudgetMiddleware(manager)

// Обёртка запроса
resp, err := middleware.WrapRequest(ctx, estimatedTokens, handler)
```

## Интеграция с Agent

```go
// Создание агента с efficiency компонентами
agent := agent.NewAgent(&agent.AgentConfig{
    Name:         "EfficientAgent",
    SystemPrompt: "You are a helpful assistant",
    Model:        "openai/gpt-4o-mini",
})

// Использование кэша
cache := efficiency.NewPromptCache(100, 1*time.Hour)
systemPromptKey := cache.MarkCacheable(agent.GetConfig().SystemPrompt)

// Использование роутера
classifier := efficiency.NewRuleBasedClassifier()
router := &efficiency.SmartRouter{
    config:     efficiency.DefaultRouterConfig(),
    classifier: classifier,
}

// Использование бюджета
budgetManager := efficiency.NewBudgetManager(efficiency.DefaultBudgetConfig())
budgetMiddleware := efficiency.NewBudgetMiddleware(budgetManager)

// Обработка сообщения с оптимизацией
func handleMessage(ctx context.Context, msg *transport.Message) (*transport.Message, error) {
    // Определение модели
    model, _ := router.Route(ctx, []llm.Message{{Role: llm.RoleUser, Content: msg.Text}})
    
    // Проверка бюджета
    estimatedTokens := estimateTokens(msg.Text)
    result, _ := budgetManager.CheckBudget(ctx, estimatedTokens)
    
    if !result.Allowed {
        return nil, fmt.Errorf("budget exceeded: %s", result.Reason)
    }
    
    // Генерация с использованием определённой модели
    resp, err := agent.RunWithModel(ctx, msg, model)
    
    // Запись использования
    if resp.Usage != nil {
        budgetManager.RecordUsage(resp.Usage.TotalTokens, model, msg.SessionID, msg.UserID)
    }
    
    return resp, err
}
```

## Тестирование

```bash
go test ./efficiency/... -v
```

## Статистика

Все компоненты предоставляют методы для сбора статистики:

- `PromptCache.GetStats()` - статистика кэша
- `BudgetManager.GetStats()` - статистика использования бюджета

## Конфигурация

### Prompt Caching
- `maxSize` - максимальное количество записей
- `ttl` - время жизни записей

### Summarization
- `Threshold` - порог токенов для активации
- `KeepRecent` - количество последних сообщений для сохранения
- `Model` - модель для summarization

### Model Routing
- `Mapping` - маппинг сложности на модель
- `DefaultModel` - модель по умолчанию
- `ClassifierModel` - модель для классификации

### Budget Management
- `DailyLimit` - дневной лимит токенов
- `MonthlyLimit` - месячный лимит токенов
- `RequestLimit` - лимит на один запрос
- `Policy` - политика управления
- `WarningThreshold` - порог предупреждения