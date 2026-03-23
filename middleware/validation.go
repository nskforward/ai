package middleware

import (
	"context"
	"fmt"

	"github.com/nskforward/ai/transport"
)

// ValidationRule определяет правило валидации
type ValidationRule func(msg *transport.Message) error

// ValidationMiddleware валидирует входящие сообщения
func ValidationMiddleware(rules ...ValidationRule) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
			for _, rule := range rules {
				if err := rule(msg); err != nil {
					return nil, fmt.Errorf("validation failed: %w", err)
				}
			}
			return next(ctx, agentCtx, msg)
		}
	}
}

// NotEmpty проверяет, что текст сообщения не пустой
func NotEmpty() ValidationRule {
	return func(msg *transport.Message) error {
		if msg.Text == "" {
			return fmt.Errorf("message text cannot be empty")
		}
		return nil
	}
}

// MaxLength проверяет максимальную длину текста
func MaxLength(maxLen int) ValidationRule {
	return func(msg *transport.Message) error {
		if len(msg.Text) > maxLen {
			return fmt.Errorf("message text exceeds maximum length of %d characters", maxLen)
		}
		return nil
	}
}
