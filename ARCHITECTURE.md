# Архитектура AI-Агента (Go Framework)

Данный документ описывает устройство и жизненный цикл универсального AI-агента, разработанного на базе этого фреймворка. Архитектура построена с акцентом на автономность, расширяемость и безопасность.

---

## 1. Основные компоненты

### 1.1 Драйвер ввода-вывода (Transport)
Агент не привязан к конкретному источнику данных. Взаимодействие абстрагировано через интерфейс:
```go
type Message struct {
    SessionID     string
    UserID        string
    TransportName string
    Text          string
}

type Transport interface {
    Name() string
    Read() (Message, error)
    Write(msg Message) error
}
```
Фреймворк из коробки предоставляет `ConsoleTransport` для тестирования.

### 1.2 Хранилище (Storage)
Абстракция для переключения между локальными файлами и облаком:
```go
type Storage interface {
    Read(path string) ([]byte, error)
    Write(path string, data []byte) error
    List(dir string) ([]string, error)
}
```
По умолчанию: `storage.LocalFS`.

### 1.3 Системный промпт
Хранится в `Storage` как `sysprompt.md`. При запуске агент проверяет наличие файла, при отсутствии — создаёт дефолтный. Ручные правки на диске подхватываются "на лету".

### 1.4 Две модели (Light & Heavy)
- **Light Model**: Triage — анализ запроса, чтение Оглавления, рефлексия, простые ответы.
- **Heavy Model**: Сложные рассуждения, написание кода, генерация нового опыта.

### 1.5 Логгер (Observability)
Для отслеживания поведения агента в продакшене. Логирует: входящие сообщения, вызовы инструментов (имя, аргументы, результат, время), переключения между моделями, ACL-отказы.
```go
type Logger interface {
    Info(msg string, args ...any)
    Error(msg string, args ...any)
    Debug(msg string, args ...any)
}
```
По умолчанию — `log/slog` из стандартной библиотеки Go.

---

## 2. Память

### 2.1 Краткосрочная (Сессии)
Хранит историю диалога в рамках одной сессии (`SessionID`). Реализуется через интерфейс:
```go
type SessionStore interface {
    Load(sessionID string) ([]llm.Message, error)
    Save(sessionID string, history []llm.Message) error
}
```
По умолчанию — in-memory `map` с автоочисткой по TTL. Пользователь библиотеки может подставить Redis/БД.

### 2.2 Долгосрочная (Skills / Опыт)
Агент нативно умеет обучаться. Механизм:
1. **Оглавление (TOC)**: Фреймворк сканирует `skills/` в Storage и строит список заголовков.
2. **Инъекция в промпт**: TOC динамически вшивается в Системный Промпт.
3. **Чтение опыта**: Light Model вызывает `read_file` для загрузки нужной инструкции.
4. **Сохранение нового опыта**: После сложной задачи Heavy Model вызывает `save_skill`.

### 2.3 Стратегия обучения (Experience Management)
Формализуется через callback:
```go
type Config struct {
    // OnTaskComplete вызывается после успешного завершения задачи.
    // Агент может решить, стоит ли сохранять опыт.
    OnTaskComplete func(task string, steps []llm.Message) bool
}
```
По умолчанию — агент спрашивает у Heavy Model: *"Стоит ли сохранить решение как навык?"*

---

## 3. Пайплайн обработки сообщения

### 3.1 Middleware
Расширяемая цепочка обработчиков (по аналогии с `http.Handler`):
```go
type Handler func(ctx context.Context, msg transport.Message) error
type Middleware func(ctx context.Context, msg transport.Message, next Handler) error
```
Через `Config.Middlewares` пользователь добавляет: логирование, rate-limiter, модерацию контента и т.д.

### 3.2 Полный цикл обработки
```
Transport.Read() → Middleware chain → ProcessMessage:
  1. Загрузить историю сессии (SessionStore.Load)
  2. Загрузить sysprompt.md + TOC
  3. Light Model (triage):
     a. Простой запрос → ответить сразу
     b. Есть опыт в TOC → прочитать файл, делегировать Heavy
     c. Нет опыта, сложная задача → делегировать Heavy
  4. Heavy Model (ReAct loop, max N шагов):
     Thought → Action (Tool) → Observation → ...
  5. Reflection (опционально):
     Light Model проверяет финальный ответ
  6. Experience Management:
     OnTaskComplete callback → save_skill (при необходимости)
  7. Сохранить историю (SessionStore.Save)
  8. Transport.Write(response)
```

### 3.3 Рефлексия (Self-Reflection)
Опциональный шаг перед отправкой ответа. Light Model получает финальный ответ Heavy Model с промптом: *"Проверь ответ на ошибки и полноту."* Включается через `Config.EnableReflection`.

### 3.4 Планирование (Planning)
Для сложных задач Light Model может создать **план** (список шагов) перед началом выполнения. Heavy Model затем выполняет его шаг за шагом, адаптируя план при необходимости. На начальном этапе решается через промпт-инженерию в `sysprompt.md`.

### 3.5 Structured Output
Для надёжного роутинга и парсинга, `llm.Provider` поддерживает запрос структурированного JSON-ответа:
```go
type GenerateOptions struct {
    ResponseFormat *JSONSchema // опционально
}
```

---

## 4. Безопасность

### 4.1 Авторизация (Admin ACL)
Белый список администраторов: связка `TransportName:UserID`.
```go
type AdminUser struct {
    Transport string
    UserID    string
}
```
Инструменты с `RequiresAdmin() == true` проверяют эту связку и возвращают `Permission Denied` при несовпадении.

### 4.2 Песочница (Sandbox)
- **Storage Sandbox**: Запрещён выход из каталога (`../`).
- **Network Sandbox**: Белый список разрешённых доменов для HTTP.

---

## 5. Жизненный цикл агента

### 5.1 Запуск
```go
agent := agent.New(cfg)
agent.Start(ctx)
```
`Start` запускает цикл ожидания `Transport.Read()`.

### 5.2 Graceful Shutdown
При отмене `context`:
1. Агент завершает обработку текущего сообщения.
2. Перестаёт принимать новые.
3. Освобождает ресурсы (сессии, файлы).
