package agent

import (
	"context"

	"github.com/nskforward/ai/transport"
)

// Agent определяет интерфейс AI агента
type Agent interface {
	// Run запускает обработку сообщения
	Run(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error)

	// GetName возвращает имя агента
	GetName() string

	// GetConfig возвращает конфигурацию
	GetConfig() *AgentConfig
}
