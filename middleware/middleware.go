package middleware

import (
	"context"

	"github.com/nskforward/ai/transport"
)

// Handler определяет функцию обработки с контекстом агента
type Handler func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error)

// Middleware определяет интерфейс промежуточного ПО
type Middleware func(next Handler) Handler
