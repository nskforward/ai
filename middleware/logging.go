package middleware

import (
	"context"
	"log"
	"time"

	"github.com/nskforward/ai/transport"
)

// Logger интерфейс для логирования
type Logger interface {
	Printf(format string, v ...interface{})
}

// LoggingMiddleware логирует входящие и исходящие сообщения
func LoggingMiddleware(logger Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
			start := time.Now()

			logger.Printf("[LOG] Incoming message | UserID: %s | SessionID: %s | Transport: %s | Text: %s",
				agentCtx.UserID,
				agentCtx.SessionID,
				agentCtx.TransportName,
				truncate(msg.Text, 100),
			)

			resp, err := next(ctx, agentCtx, msg)

			duration := time.Since(start)

			if err != nil {
				logger.Printf("[LOG] Error | Duration: %v | Error: %v", duration, err)
			} else if resp != nil {
				logger.Printf("[LOG] Response | Duration: %v | Text: %s", duration, truncate(resp.Text, 100))
			} else {
				logger.Printf("[LOG] Completed | Duration: %v | No response", duration)
			}

			return resp, err
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
