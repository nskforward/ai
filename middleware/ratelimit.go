package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nskforward/ai/transport"
)

// RateLimiter интерфейс для ограничения частоты запросов
type RateLimiter interface {
	// Allow проверяет, разрешён ли запрос
	Allow(key string) bool
}

// TokenBucket реализует алгоритм Token Bucket
type TokenBucket struct {
	mu       sync.Mutex
	rate     float64 // токенов в секунду
	capacity int     // максимальное количество токенов
	tokens   map[string]float64
	lastTime map[string]time.Time
}

// NewTokenBucket создаёт новый Token Bucket
func NewTokenBucket(rate float64, capacity int) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   make(map[string]float64),
		lastTime: make(map[string]time.Time),
	}
}

// Allow проверяет, разрешён ли запрос
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	last, exists := tb.lastTime[key]
	if !exists {
		last = now
		tb.tokens[key] = float64(tb.capacity)
	}

	elapsed := now.Sub(last).Seconds()
	tb.tokens[key] += elapsed * tb.rate
	if tb.tokens[key] > float64(tb.capacity) {
		tb.tokens[key] = float64(tb.capacity)
	}

	tb.lastTime[key] = now

	if tb.tokens[key] >= 1 {
		tb.tokens[key]--
		return true
	}

	return false
}

// RateLimitMiddleware ограничивает частоту запросов
func RateLimitMiddleware(limiter RateLimiter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, agentCtx *transport.AgentContext, msg *transport.Message) (*transport.Message, error) {
			key := agentCtx.UserID
			if !limiter.Allow(key) {
				return nil, fmt.Errorf("rate limit exceeded for user %s", key)
			}
			return next(ctx, agentCtx, msg)
		}
	}
}
