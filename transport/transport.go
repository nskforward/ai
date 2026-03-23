package transport

import "context"

// MessageHandler определяет функцию обработки сообщений
type MessageHandler func(ctx context.Context, agentCtx *AgentContext, msg *Message) (*Message, error)

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
}
