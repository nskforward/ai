package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/nskforward/ai/transport"
)

// RecoveryMiddleware восстанавливается после паник
func RecoveryMiddleware(logger Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (resp *transport.Message, err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Printf("[RECOVERY] Panic recovered | UserID: %s | Error: %v", agentCtx.UserID, r)
					err = fmt.Errorf("internal error: panic recovered")
				}
			}()

			return next(ctx, agentCtx, msg)
		}
	}
}
