# AI Agent Library - Architecture Document

**Версия:** 1.1
**Дата:** 23 марта 2026  
**Статус:** Черновик (ожидает утверждения)

---

## 1. Введение

### 1.1 Цель проекта

Создание универсальной библиотеки на языке Golang для построения AI агентов любой сложности - от простых ботов до сложных самообучающихся систем. Основной упор на безопасность, расширяемость и эффективное использование токенов.

### 1.2 Ключевые требования

- **Безопасность**: Встроенные механизмы авторизации, аудита, rate limiting
- **Расширяемость**: Поддержка различных Transport, LLM провайдеров, инструментов
- **Эффективность**: Оптимизация токенов (caching, summarization, smart routing)
- **Модульность**: Возможность использовать только нужные компоненты

---

## 2. Высокоуровневая архитектура

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Application Layer                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │
│  │  Console    │  │  Telegram   │  │   Discord   │  │    HTTP     │  │
│  │  Transport  │  │  Transport  │  │  Transport  │  │  Transport  │  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │
│         │                │                │                │          │
│         └────────────────┴────────┬───────┴────────────────┘          │
│                                    │                                    │
│                          ┌─────────▼─────────┐                        │
│                          │   Transport       │                        │
│                          │   Interface        │                        │
│                          └─────────┬─────────┘                        │
│                                    │                                    │
│                          ┌─────────▼─────────┐                        │
│                          │   Agent Context    │◄── UserID, SessionID, │
│                          │   (Authorization)  │    TransportName,     │
│                          └─────────┬─────────┘    IsAdmin             │
│                                    │                                    │
│         ┌──────────────────────────┼──────────────────────────┐       │
│         │                          │                          │       │
│  ┌──────▼──────┐          ┌───────▼───────┐          ┌──────▼──────┐ │
│  │  Middleware  │          │    Agent      │          │   Storage   │ │
│  │    Chain     │          │    Core       │          │  (Memory,   │ │
│  │              │◄─────────►│               │◄─────────►│  Redis,     │ │
│  │ - Logging    │          │               │          │  Postgres)  │ │
│  │ - Auth       │          └───────┬───────┘          └─────────────┘ │
│  │ - RateLimit  │                  │                                    │
│  │ - Validation │          ┌───────▼───────┐                          │
│  └──────────────┘          │    Tools       │                          │
│                            │    Manager      │                          │
│                            └───────┬───────┘                          │
│                                    │                                    │
│                          ┌─────────▼─────────┐                        │
│                          │       LLM          │                        │
│                          │    Provider        │                        │
│                          │   (OpenRouter)     │                        │
│                          └────────────────────┘                        │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Компоненты системы

### 3.1 Transport Layer

Абстракция для взаимодействия с внешней средой. Каждый Transport создаёт Agent Context при инициализации сессии.

#### Интерфейс Transport

```go
// Transport определяет интерфейс для источников сообщений
type Transport interface {
    // Name возвращает имя транспорта
    Name() string
    
    // Start запускает приём сообщений
    Start(ctx context.Context) error
    
    // Stop останавливает приём сообщений
    Stop() error
    
    // Send отправляет сообщение
    Send(ctx context.Context, msg *Message) error
    
    // Handle регистрирует обработчик сообщений
    Handle(handler MessageHandler)
    
    // createContext создаёт AgentContext для новой сессии
    createContext(msg *Message) *AgentContext
}

// MessageHandler определяет функцию обработки сообщений
type MessageHandler func(ctx context.Context, agentCtx *AgentContext, msg *Message) error
```

#### Структура Message

```go
// Message представляет универсальное сообщение
type Message struct {
    // ID уникальный идентификатор сообщения
    ID string
    
    // Text текст сообщения
    Text string
    
    // RawText оригинальный текст (до обработки)
    RawText string
    
    // UserID идентификатор пользователя (из Transport)
    UserID string
    
    // SessionID идентификатор сессии
    SessionID string
    
    // IsGroup указывает, что сообщение из группы
    IsGroup bool
    
    // Metadata дополнительные метаданные
    Metadata map[string]interface{}
    
    // Attachments вложенные файлы
    Attachments []Attachment
    
    // Timestamp временная метка
    Timestamp time.Time
    
    // RawData сырые данные от транспорта
    RawData interface{}
}

// AttachmentType тип вложения
type AttachmentType string

const (
    AttachmentTypeImage   AttachmentType = "image"
    AttachmentTypeAudio   AttachmentType = "audio"
    AttachmentTypeVideo   AttachmentType = "video"
    AttachmentTypeFile    AttachmentType = "file"
    AttachmentTypeText    AttachmentType = "text"
)

// Attachment представляет вложение
type Attachment struct {
    Type    AttachmentType
    URL     string
    Content []byte
    Name    string
    Size    int64
}
```

#### Agent Context (Контекст авторизации)

```go
// AgentContext содержит информацию о пользователе и сессии
// Примечание: UserID и SessionID дублируются в Message для удобства,
// но AgentContext используется для авторизации на уровне сессии,
// rate limiting и хранения состояния между сообщениями
type AgentContext struct {
    // UserID уникальный идентификатор пользователя
    UserID string
    
    // SessionID идентификатор сессии/диалога
    SessionID string
    
    // TransportName имя транспорта, от которого получено сообщение
    TransportName string
    
    // IsAdmin флаг администратора
    IsAdmin bool
    
    // UserName имя пользователя (опционально)
    UserName string
    
    // DisplayName отображаемое имя
    DisplayName string
    
    // Locale языковой стандарт пользователя
    Locale string
    
    // Metadata дополнительные данные авторизации
    Metadata map[string]interface{}
    
    // Permissions права доступа
    Permissions []string
    
    // RateLimitCtx контекст rate limiting
    RateLimitCtx *RateLimitContext
    
    // createdAt время создания контекста
    createdAt time.Time
}
```

### 3.2 Middleware System

Система промежуточного ПО по принципу Chain of Responsibility, аналогичная http.Middleware в Go.

#### Интерфейс Middleware

```go
// Middleware определяет интерфейс промежуточного ПО
type Middleware func(next Handler) Handler

// Handler определяет функцию обработки с контекстом агента
type Handler func(ctx context.Context, agentCtx *AgentContext, msg *Message) (*Message, error)

// MiddlewareChain управляет цепочкой middleware
type MiddlewareChain struct {
    middlewares []Middleware
}

// NewMiddlewareChain создаёт новую цепочку
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain

// Then создаёт финальный обработчик из цепочки
func (mc *MiddlewareChain) Then(handler Handler) Handler

// Append добавляет middleware в конец цепочки
func (mc *MiddlewareChain) Append(m ...Middleware)

// Prepend добавляет middleware в начало
func (mc *MiddlewareChain) Prepend(m ...Middleware)
```

#### Встроенные Middleware

1. **LoggingMiddleware** - логирование входящих и исходящих сообщений
2. **AuthMiddleware** - проверка прав доступа
3. **RateLimitMiddleware** - ограничение частоты запросов (Token Bucket / Sliding Window)
4. **ValidationMiddleware** - валидация входящих сообщений
5. **MetricsMiddleware** - сбор метрик (токены, время ответа, ошибки)
6. **RecoveryMiddleware** - восстановление после паник

#### Примеры использования Middleware

```go
// Создание цепочки middleware
chain := middleware.NewMiddlewareChain(
    middleware.Recovery(),
    middleware.Logging(logger),
    middleware.RateLimit(rateLimiter),
    middleware.Auth(authenticator),
)

// Применение цепочки
handler := chain.Then(agentHandler)
```

### 3.3 Tools System

Расширенная система управления инструментами с поддержкой современных паттернов.

#### Интерфейс Tool

```go
// ApprovalPolicy политика подтверждения вызова инструмента
type ApprovalPolicy string

const (
    // AutoApprove автоматическое подтверждение без запроса
    AutoApprove ApprovalPolicy = "auto_approve"
    
    // RequireApproval требует подтверждения от пользователя
    RequireApproval ApprovalPolicy = "require_approval"
    
    // RequireAdminApproval требует подтверждения от администратора
    RequireAdminApproval ApprovalPolicy = "require_admin_approval"
    
    // Deny запрещено использование
    Deny ApprovalPolicy = "deny"
)

// Tool определяет интерфейс инструмента
type Tool interface {
    // Name возвращает имя инструмента
    Name() string
    
    // Description описание для LLM
    Description() string
    
    // Parameters JSON Schema параметров
    Parameters() *jsonschema.Schema
    
    // ResultSchema JSON Schema результата (опционально)
    // Некоторые LLM (например, GigaChat) требуют схему результата
    ResultSchema() *jsonschema.Schema
    
    // Call выполняет инструмент (Tool Calling)
    Call(ctx context.Context, agentCtx *AgentContext, params map[string]interface{}) (*ToolResult, error)
    
    // ApprovalPolicy возвращает политику подтверждения
    ApprovalPolicy() ApprovalPolicy
    
    // IsAvailable проверяет доступность инструмента для пользователя
    IsAvailable(ctx context.Context, agentCtx *AgentContext) bool
}

// ToolResult результат выполнения инструмента
type ToolResult struct {
    // Success успешность выполнения
    Success bool
    
    // Output результат (строка или JSON)
    Output interface{}
    
    // Error ошибка
    Error string
    
    // Metadata метаданные
    Metadata map[string]interface{}
    
    // TokensUsed количество использованных токенов
    TokensUsed int
    
    // ExecutionTime время выполнения
    ExecutionTime time.Duration
}
```

#### Встроенные инструменты

Библиотека включает набор встроенных инструментов с различными политиками подтверждения:

`http_get` - HTTP GET запрос к URL (RequireApproval)
`file_read` - Чтение файла (RequireApproval)
`file_write` - Запись в файл (RequireAdminApproval)
`folder_list` - Список файлов в папке (AutoApprove)
`cli_exec` - Выполнение CLI команд (RequireAdminApproval)

### Пример создания инструмента

```go
// EchoTool простой инструмент для демонстрации
type EchoTool struct{}

func (t *EchoTool) Name() string {
    return "echo"
}

func (t *EchoTool) Description() string {
    return "Повторяет переданный текст. Полезен для тестирования и отладки."
}

func (t *EchoTool) Parameters() *jsonschema.Schema {
    return &jsonschema.Schema{
        Type: "object",
        Properties: map[string]*jsonschema.Schema{
            "text": {
                Type:        "string",
                Description: "Текст для повторения",
            },
        },
        Required: []string{"text"},
    }
}

func (t *EchoTool) ResultSchema() *jsonschema.Schema {
    return &jsonschema.Schema{
        Type: "object",
        Properties: map[string]*jsonschema.Schema{
            "echoed_text": {
                Type:        "string",
                Description: "Повторённый текст",
            },
        },
    }
}

func (t *EchoTool) Call(ctx context.Context, agentCtx *AgentContext, params map[string]interface{}) (*ToolResult, error) {
    text, ok := params["text"].(string)
    if !ok {
        return &ToolResult{
            Success: false,
            Error:   "Параметр 'text' должен быть строкой",
        }, nil
    }
    
    return &ToolResult{
        Success: true,
        Output: map[string]interface{}{
            "echoed_text": text,
        },
    }, nil
}

func (t *EchoTool) ApprovalPolicy() ApprovalPolicy {
    return AutoApprove
}

func (t *EchoTool) IsAvailable(ctx context.Context, agentCtx *AgentContext) bool {
    return true
}
```

#### Расширенные возможности Tools

1. **Tool Batching** - объединение нескольких вызовов инструментов в один

```go
// BatchedTool инструмент с поддержкой batching
type BatchedTool interface {
    Tool
    
    // BatchExecute выполняет несколько запросов за один вызов
    BatchExecute(ctx context.Context, agentCtx *AgentContext, params []map[string]interface{}) ([]*ToolResult, error)
    
    // CanBatch проверяет возможность объединения
    CanBatch(requests []map[string]interface{}) bool
}
```

2. **Parallel Execution** - параллельное выполнение независимых инструментов

```go
// ParallelExecutor выполняет инструменты параллельно
type ParallelExecutor struct {
    maxConcurrent int
    timeout       time.Duration
}

// ExecuteParallel выполняет инструменты параллельно
func (pe *ParallelExecutor) ExecuteParallel(
    ctx context.Context,
    agentCtx *AgentContext,
    tools []Tool,
    params []map[string]interface{},
) ([]*ToolResult, error)
```

3. **Tool RAG** - семантический поиск инструментов

```go
// ToolRegistry реестр инструментов с поиском
type ToolRegistry interface {
    // Register регистрирует инструмент
    Register(tool Tool) error
    
    // GetByName получает инструмент по имени
    GetByName(name string) (Tool, error)
    
    // Search ищет инструменты по описанию
    Search(query string, limit int) ([]Tool, error)
    
    // GetAll возвращает все инструменты
    GetAll() []Tool
    
    // Filter фильтрует инструменты по критериям
    Filter(predicate func(Tool) bool) []Tool
}
```

4. **Tool Approval** - система подтверждений

```go
// ApprovalManager управляет подтверждениями
type ApprovalManager interface {
    // RequestApproval запрашивает подтверждение
    RequestApproval(ctx context.Context, agentCtx *AgentContext, tool Tool, params map[string]interface{}) (bool, error)
    
    // RegisterApprover регистрирует обработчик подтверждений
    RegisterApprover(approver Approver)
}

// Approver определяет интерфейс обработчика подтверждений
type Approver interface {
    // Approve обрабатывает запрос подтверждения
    Approve(ctx context.Context, req *ApprovalRequest) (*ApprovalResponse, error)
}
```

### 3.4 LLM Provider (OpenRouter)

Абстракция над LLM провайдерами с поддержкой OpenRouter.

#### Интерфейс LLM

```go
// LLM определяет интерфейс LLM провайдера
type LLM interface {
    // Generate генерирует ответ
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    
    // GenerateStream генерирует ответ со стримингом
    GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error)
    
    // GetName возвращает имя провайдера
    GetName() string
    
    // GetCapabilities возвращает возможности
    GetCapabilities() *LLMCapabilities
}

// GenerateRequest запрос на генерацию
type GenerateRequest struct {
    // Messages история сообщений
    Messages []Message
    
    // Tools доступные инструменты
    Tools []Tool
    
    // Model модель
    Model string
    
    // Temperature температура
    Temperature float64
    
    // MaxTokens максимальное количество токенов
    MaxTokens int
    
    // ThinkingBudget бюджет на размышления (для Anthropic и подобных)
    ThinkingBudget int
    
    // Stream включить стриминг
    Stream bool
    
    // CachePrompt кешировать промпт
    CachePrompt bool
    
    // Metadata метаданные
    Metadata map[string]interface{}
}

// GenerateResponse ответ генерации
type GenerateResponse struct {
    // Content текстовый контент
    Content string
    
    // ToolCalls вызовы инструментов
    ToolCalls []ToolCall
    
    // FinishReason причина завершения
    FinishReason string
    
    // Usage использование токенов
    Usage *TokenUsage
    
    // Model использованная модель
    Model string
    
    // Reasoning размышления (если поддерживается)
    Reasoning string
}

// TokenUsage использование токенов
type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CacheHit         bool
}
```

#### Поддержка OpenRouter

```go
// OpenRouterProvider провайдер OpenRouter
type OpenRouterProvider struct {
    apiKey      string
    baseURL     string
    httpClient  *http.Client
    defaultModel string
    
    // Для token efficiency
    cache       PromptCache
    router      ModelRouter
}

// OpenRouterConfig конфигурация OpenRouter
type OpenRouterConfig struct {
    APIKey       string
    BaseURL      string // опционально
    DefaultModel string
    
    // Token efficiency настройки
    EnableCaching      bool
    EnableRouting      bool
    RoutingModel       string // модель для маршрутизации
}
```

### 3.5 Agent Core

Ядро агента, координирующее все компоненты.

#### Интерфейс Agent

```go
// Agent определяет интерфейс AI агента
type Agent interface {
    // Run запускает обработку сообщения
    Run(ctx context.Context, agentCtx *AgentContext, msg *Message) (*Message, error)
    
    // RunWithTools запускает с использованием инструментов
    RunWithTools(ctx context.Context, agentCtx *AgentContext, msg *Message, tools []Tool) (*Message, error)
    
    // RegisterTool регистрирует инструмент
    RegisterTool(tool Tool) error
    
    // SetLLM устанавливает LLM провайдер
    SetLLM(llm LLM)
    
    // GetName возвращает имя агента
    GetName() string
    
    // GetHistory возвращает историю сообщений
    GetHistory(sessionID string) ([]Message, error)
}

// BaseAgent базовая реализация агента
type BaseAgent struct {
    name        string
    systemPrompt string
    llm         LLM
    tools       *ToolManager
    memory      Memory
    middleware  *MiddlewareChain
    config      *AgentConfig
}
```

#### Конфигурация Agent

```go
// AgentConfig конфигурация агента
type AgentConfig struct {
    // Name имя агента
    Name string
    
    // SystemPrompt системный промпт
    SystemPrompt string
    
    // Model модель по умолчанию
    Model string
    
    // Temperature температура
    Temperature float64
    
    // MaxTokens максимум токенов ответа
    MaxTokens int
    
    // MaxHistorySize максимум сообщений в истории
    MaxHistorySize int
    
    // EnableHistory сбрасывать историю после лимита
    EnableHistory bool
    
    // EnableSummarization включать summarization
    EnableSummarization bool
    
    // SummaryThreshold порог для summarization
    SummaryThreshold int
    
    // ToolsLimit максимум инструментов за один вызов
    ToolsLimit int
    
    // MaxIterations максимум итераций (инструменты + ответы)
    MaxIterations int
    
    // Timeout таймаут обработки
    Timeout time.Duration
}
```

### 3.6 Memory System

Управление памятью и историей агента.

```go
// Memory определяет интерфейс хранилища памяти
type Memory interface {
    // Add добавляет сообщение в историю
    Add(ctx context.Context, sessionID string, msg Message) error
    
    // GetHistory возвращает историю сессии
    GetHistory(ctx context.Context, sessionID string, limit int) ([]Message, error)
    
    // GetSummary возвращает сводку истории
    GetSummary(ctx context.Context, sessionID string) (string, error)
    
    // Clear очищает историю сессии
    Clear(ctx context.Context, sessionID string) error
    
    // GetContextWindow возвращает контекстное окно для LLM
    GetContextWindow(ctx context.Context, sessionID string) ([]Message, error)
}

// SessionMemory память одной сессии
type SessionMemory struct {
    sessionID    string
    messages     []Message
    summary      string
    metadata     map[string]interface{}
    createdAt    time.Time
    updatedAt    time.Time
}
```

### 3.7 Token Efficiency (Экономия токенов)

#### 3.7.1 Prompt Caching

```go
// PromptCache кеширует системные промпты
type PromptCache interface {
    // Get получает закэшированный промпт
    Get(key string) (string, bool)
    
    // Set устанавливает кэш
    Set(key string, prompt string, ttl time.Duration)
    
    // MarkCacheable помечает промпт как кешируемый
    MarkCacheable(prompt string) string
    
    // Invalidate инвалидирует кэш
    Invalidate(key string)
}
```

#### 3.7.2 Rolling Summarization

```go
// Summarizer сжимает историю
type Summarizer interface {
    // Summarize создаёт сводку из сообщений
    Summarize(ctx context.Context, messages []Message) (string, error)
    
    // ShouldSummarize проверяет необходимость сводки
    ShouldSummarize(messages []Message, tokenCount int) bool
    
    // Merge объединяет сводку с новыми сообщениями
    Merge(summary string, recentMessages []Message) []Message
}
```

#### 3.7.3 Model Routing

```go
// ModelRouter маршрутизирует запросы по сложности
type ModelRouter interface {
    // Route определяет модель для запроса
    Route(ctx context.Context, messages []Message) (string, error)
    
    // RegisterClassifier регистрирует классификатор
    RegisterClassifier(classifier ComplexityClassifier)
}

// ComplexityClassifier классифицирует сложность
type ComplexityClassifier interface {
    // Classify возвращает уровень сложности
    Classify(ctx context.Context, messages []Message) (ComplexityLevel, error)
}

// ComplexityLevel уровень сложности
type ComplexityLevel string

const (
    ComplexitySimple    ComplexityLevel = "simple"
    ComplexityModerate  ComplexityLevel = "moderate"
    ComplexityComplex   ComplexityLevel = "complex"
)
```

#### 3.7.4 Budget Management

```go
// ModelRouter маршрутизирует запросы по сложности
type ModelRouter interface {
    // Route определяет модель для запроса
    Route(ctx context.Context, messages []Message) (string, error)
    
    // RegisterClassifier регистрирует классификатор
    RegisterClassifier(classifier ComplexityClassifier)
}

// ComplexityClassifier классифицирует сложность
type ComplexityClassifier interface {
    // Classify возвращает уровень сложности
    Classify(ctx context.Context, messages []Message) (ComplexityLevel, error)
}

// ComplexityLevel уровень сложности
type ComplexityLevel string

const (
    ComplexitySimple    ComplexityLevel = "simple"
    ComplexityModerate  ComplexityLevel = "moderate"
    ComplexityComplex   ComplexityLevel = "complex"
)
```

### 3.8 Workflow Engine

Расширенный движок рабочих процессов для сложных агентов.

#### Основные узлы

```go
/// Node определяет интерфейс узла workflow
type Node interface {
    // ID возвращает идентификатор узла
    ID() string
    
    // Type возвращает тип узла
    Type() NodeType
    
    // Execute выполняет узел
    Execute(ctx context.Context, env *WorkflowEnv) (*NodeResult, error)
    
    // Validate валидирует конфигурацию
    Validate() error
}

// NodeType типы узлов
type NodeType string

const (
    NodeTypeLLM       NodeType = "llm"
    NodeTypeTool      NodeType = "tool"
    NodeTypeCondition NodeType = "condition"
    NodeTypeParallel  NodeType = "parallel"
    NodeTypeSequence  NodeType = "sequence"
    NodeTypeRetry     NodeType = "retry"
    NodeTypeFunc      NodeType = "function"
    NodeTypeBranch    NodeType = "branch"
)

// WorkflowEnv окружение выполнения workflow
type WorkflowEnv struct {
    // Context контекст агента
    Context *AgentContext
    
    // Variables переменные workflow
    Variables map[string]interface{}
    
    // State состояние выполнения
    State *WorkflowState
}
```

#### Типы узлов

1. **LLMNode** - генерация ответа с помощью LLM
2. **ToolNode** - вызов инструмента
3. **FuncNode** - выполнение пользовательской функции
4. **ConditionNode** - условное ветвление
5. **ParallelNode** - параллельное выполнение
6. **SequenceNode** - последовательное выполнение
7. **RetryNode** - повтор при ошибке
8. **BranchNode** - ветвление по условию

---

## 4. Модульная структура

```
/ai
├── /transport          # Интерфейсы и реализации транспортов
│   ├── transport.go    # Базовые интерфейсы
│   ├── message.go      # Структура Message
│   ├── context.go      # AgentContext
│   ├── console/        # Console транспорт (для CLI)
│   ├── telegram/       # Telegram транспорт
│   ├── discord/        # Discord транспорт
│   ├── http/           # HTTP/Webhook транспорт
│   └── stdio/          # STDIO транспорт (для CLI)
│
├── /middleware         # Система промежуточного ПО
│   ├── middleware.go   # Базовые интерфейсы
│   ├── chain.go        # MiddlewareChain
│   ├── logging.go      # LoggingMiddleware
│   ├── auth.go         # AuthMiddleware
│   ├── ratelimit.go    # RateLimitMiddleware
│   ├── validation.go   # ValidationMiddleware
│   └── metrics.go      # MetricsMiddleware
│
├── /agent              # Ядро агента
│   ├── agent.go        # Интерфейс Agent
│   ├── base.go         # Базовая реализация
│   ├── config.go       # Конфигурация
│   └── handler.go      # Обработчики
│
├── /tools              # Система инструментов
│   ├── tool.go         # Интерфейс Tool
│   ├── manager.go      # ToolManager
│   ├── registry.go     # ToolRegistry
│   ├── batch.go        # Batching
│   ├── parallel.go     # Parallel execution
│   ├── approval.go     # Система подтверждений
│   ├── rag.go          # Tool RAG
│   └── /built-in/      # Встроенные инструменты
│       ├── http_get.go
│       ├── file_read.go
│       ├── file_write.go
│       ├── folder_list.go
│       └── cli_exec.go
│
├── /llm                # LLM провайдеры
│   ├── llm.go          # Базовый интерфейс
│   ├── openrouter/     # OpenRouter провайдер
│   ├── messages.go     # Работа с сообщениями
│   └── factory.go      # Factory для создания провайдеров
│
├── /memory             # Система памяти
│   ├── memory.go       # Интерфейс Memory
│   ├── session.go      # SessionMemory
│   ├── summarizer.go   # Rolling summarization
│   └── storage/        # Слой хранилища
│
├── /efficiency         # Оптимизация токенов
│   ├── cache.go        # Prompt caching
│   ├── router.go       # Model routing
│   ├── budget.go       # Budget management
│   └── streaming.go    # Streaming with early termination
│
├── /workflow           # Workflow Engine
│   ├── workflow.go     # Интерфейс Workflow
│   ├── node.go        # Базовый Node
│   ├── nodes/         # Реализации узлов
│   ├── executor.go    # Исполнитель
│   └── state.go       # Управление состоянием
│
├── /security           # Безопасность
│   ├── auth.go        # Аутентификация
│   ├── permissions.go # Права доступа
│   ├── audit.go       # Аудит логи
│   └── masking.go     # Маскирование данных
│
└── /mcp               # Model Context Protocol (опционально)
    ├── client.go      # MCP клиент
    ├── server.go      # MCP сервер
    └── tools.go       # MCP инструменты
```

---

## 5. Примеры использования

### 5.1 Простой бот

```go
// Создание агента
agent := agent.NewAgent(&agent.AgentConfig{
    Name:         "EchoBot",
    SystemPrompt: "Ты - простой эхо-бот. Повторяй сообщения пользователя.",
    Model:        "openai/gpt-4o-mini",
})

// Создание Telegram транспорта
telegramTransport := telegram.NewTransport(&telegram.Config{
    BotToken: "YOUR_BOT_TOKEN",
})

// Регистрация обработчика
telegramTransport.Handle(func(ctx context.Context, agentCtx *agent.AgentContext, msg *transport.Message) (*transport.Message, error) {
    return agent.Run(ctx, agentCtx, msg)
})

// Запуск
ctx := context.Background()
telegramTransport.Start(ctx)
```

### 5.2 Агент с инструментами

```go
// Создание агента с инструментами
agent := agent.NewAgent(&agent.AgentConfig{
    Name:         "Assistant",
    SystemPrompt: "Ты - полезный ассистент с доступом к инструментам.",
    Model:        "openai/gpt-4o",
})

// Регистрация встроенных инструментов
agent.RegisterTool(&builtin.HTTPGetTool{})
agent.RegisterTool(&builtin.FileReadTool{})
agent.RegisterTool(&builtin.FolderListTool{})

// Регистрация пользовательского инструмента
agent.RegisterTool(&EchoTool{})

// Создание middleware цепочки
chain := middleware.NewMiddlewareChain(
    middleware.Recovery(),
    middleware.Logging(logger),
    middleware.RateLimit(rateLimiter),
)

// Обработка с middleware
handler := chain.Then(func(ctx context.Context, agentCtx *agent.AgentContext, msg *transport.Message) (*transport.Message, error) {
    return agent.RunWithTools(ctx, agentCtx, msg, nil)
})
```

### 5.3 Workflow агент

```go
// Создание workflow
wf := workflow.NewWorkflow("ResearchAgent")

// Добавление узлов
wf.AddNode(&workflow.LLMNode{
    ID:      "analyze",
    Model:   "openai/gpt-4o",
    Prompt:  "Проанализируй запрос: {{.input}}",
})

wf.AddNode(&workflow.ToolNode{
    ID:     "search",
    Tool:   &SearchTool{},
})

wf.AddNode(&workflow.LLMNode{
    ID:      "synthesize",
    Model:   "openai/gpt-4o-mini",
    Prompt:  "Синтезируй результаты: {{.search_results}}",
})

// Определение связей
wf.AddEdge("analyze", "search")
wf.AddEdge("search", "synthesize")

// Выполнение
result, err := wf.Execute(ctx, env)
```

---

## 6. Безопасность

### 6.1 Аутентификация и авторизация

- **UserID/SessionID** - уникальные идентификаторы из Transport
- **IsAdmin** - флаг администратора
- **Permissions** - список разрешений
- **RBAC** - role-based access control

### 6.2 Rate Limiting

- **Token Bucket Algorithm** - для гибкого ограничения
- **Sliding Window** - для точного ограничения
- **Per-user limits** - индивидуальные лимиты

### 6.3 Аудит

```go
// AuditLog запись аудита
type AuditLog struct {
    Timestamp    time.Time
    UserID       string
    SessionID    string
    Transport    string
    Action       string
    Resource     string
    Result       string
    IPAddress    string
    Metadata     map[string]interface{}
}
```

### 6.4 Data Masking

- Маскирование PII (Personally Identifiable Information)
- Логирование без чувствительных данных

---

## 7. Соглашения об именовании

### 7.1 Интерфейсы

- Все интерфейсы именуются в единственном числе: `Transport`, `Agent`, `Tool`
- Глагольные интерфейсы для действий: `Handler`, `Executor`

### 7.2 Реализации

- Базовые реализации: `BaseAgent`, `BaseTransport`
- Конкретные реализации: `TelegramTransport`, `OpenRouterProvider`

### 7.3 Пакеты

- Нижний регистр: `transport`, `middleware`, `agent`
- Акронимы: `llm` (не `LLM`)

---

## 8. Зависимости

### 8.1 Внешние библиотеки

- `github.com/go-telegram-bot-api/telegram-bot-api/v5` - Telegram
- `github.com/sashabaranov/go-openai` или `github.com/openrouter/openrouter-go` - LLM
- `github.com/redis/go-redis/v9` - Redis (опционально)
- `github.com/lib/pq` - PostgreSQL (опционально)
- `github.com/Masterminds/sprig/v3` - Шаблоны

### 8.2 Внутренние модули

- Все пакеты внутри `/ai` используют общие интерфейсы
- Циклические зависимости запрещены

---

## 9. Тестирование

### 9.1 Уровни тестирования

1. **Unit тесты** - отдельные компоненты
2. **Integration тесты** - взаимодействие компонентов
3. **E2E тесты** - полный флоу

### 9.2 Mock объекты

- `mocks/Transport.go`
- `mocks/LLM.go`
- `mocks/Tool.go`

---

## 10. Roadmap

### Фаза 1 (Основа)
- [x] Transport интерфейс
- [x] Message структура
- [x] AgentContext
- [x] Middleware система
- [x] Базовый Agent
- [x] Console транспорт
- [x] Telegram транспорт

### Фаза 2 (Tools & LLM)
- [x] Tool интерфейс (Call, ApprovalPolicy, ResultSchema)
- [x] Tool Manager
- [x] OpenRouter провайдер
- [x] Tool Approval система
- [x] Встроенные инструменты (http_get, file_read, file_write, folder_list, cli_exec)

### Фаза 3 (Efficiency)
- [ ] Prompt Caching
- [ ] Rolling Summarization
- [ ] Model Routing
- [ ] Budget Management

### Фаза 4 (Advanced)
- [ ] Workflow Engine
- [ ] MCP интеграция
- [ ] Multi-agent support

---

## 11. Конфигурация

### 11.1 Через код

```go
agent := agent.NewAgent(&agent.AgentConfig{
    Name:         "MyAgent",
    Model:        "openai/gpt-4o",
    Temperature:  0.7,
    MaxTokens:    1000,
})
```

### 11.2 Через YAML/JSON

```yaml
agent:
  name: MyAgent
  model: openai/gpt-4o
  temperature: 0.7
  max_tokens: 1000
  tools:
    - name: http_get
      approval_policy: require_approval
    - name: file_read
      approval_policy: require_approval
    - name: cli_exec
      approval_policy: require_admin_approval
```

### 11.3 Через Environment Variables

```bash
AGENT_MODEL=openai/gpt-4o
AGENT_TEMPERATURE=0.7
OPENROUTER_API_KEY=sk-...
```

---

## 12. Заключение

Данная архитектура обеспечивает:

1. **Расширяемость** - легкое добавление новых Transport, Tools, LLM провайдеров
2. **Безопасность** - встроенная авторизация, аудит, rate limiting
3. **Эффективность** - современные паттерны экономии токенов
4. **Модульность** - использование только необходимых компонентов
5. **Производительность** - параллельное выполнение, стриминг, early termination

---

*Документ создан на основе исследования лучших практик AI агентов (март 2026)*
