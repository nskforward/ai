# Architecture

Мы разрабатываем универсальную библиотеку на Golang для построения гибких расширяемых AI агентов.

Библиотека состоит из следующих абстракций, каждая из которых находится в подмодуле.

## User

```
type User interface {
	ID            string
    SessionID     string
    TransportName string
    IsAdmin       bool
}
```

## Transport

Интерфейс, который отвечает за описание источника сообщений для агента (Console, Telegram, etc).

```
type Transport interface {
	Name() string
	ReadMessage() (Message, error)
	WriteMessage(msg Message) error
    SendEvent(session, Event) error
}

type Message struct {
    Session Session
	Text    string
}

type Event uint8

const (
    InProcessing Event = 1
)
```

## LLM

Универсальная абстракция для LLM.

```
type Role string

const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
	Tool      Role = "tool"
)

type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string // Used when Role == RoleTool to identify which call is being answered.
}

// ToolCall represents a function call requested by the LLM.
type ToolCall struct {
	ID   string
	Name string
	Args string // JSON arguments
}

// Provider represents an LLM backend.
type Provider interface {
	Generate(ctx context.Context, history []Message, tools []tool.Tool) (Message, error)
}
```

## Tool

```
type Tool interface {
	Name() string
	Description() string
	Schema() string
	Execute(ctx context.Context, args string) (string, error)
	Allow(session.Session) error
}
```

## Storage

```
type FileStorage interface {
	Read(path string) ([]byte, error)
	Write(path string, data []byte) error
	List(dir string) ([]string, error)
	Delete(path string) error
}

```

## Agent

```
type SessionStore interface {
	Get(sessionID string) []llm.Message
	Append(sessionID string, message llm.Message)
	Delete(sessionID string)
}

type ToolRegistry interface {
	Get(name string) tool.Tool
	Set(tool.Tool)
	Delete(name string)
	GetAll() []tool.Tool
}

type Config struct {
	// Dependencies
	Transport    transport.Transport
	LLM 	     llm.Provider
	Tools        []tool.Tool

	// Security
	AllowedAdmins map[string][]string // map[transport name] array of user ids

	// Middleware chain (executed in order)
	Middlewares []Middleware
}

// Handler is a function that processes a message.
type Handler func(ctx context.Context, msg transport.Message) error

// Middleware wraps a Handler with pre/post processing.
type Middleware func(ctx context.Context, msg transport.Message, next Handler) error

type Agent struct {
	config   Config
	sessions SessionStore
	tools    ToolRegistry
}

func (a *Agent) Start(ctx context.Context) error
```