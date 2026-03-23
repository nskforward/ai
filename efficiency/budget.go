package efficiency

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nskforward/ai/llm"
)

// BudgetPolicy политика управления бюджетом
type BudgetPolicy string

const (
	// BudgetPolicyHard жёсткий лимит - прерывает при превышении
	BudgetPolicyHard BudgetPolicy = "hard"

	// BudgetPolicySoft мягкий лимит - предупреждает, но продолжает
	BudgetPolicySoft BudgetPolicy = "soft"

	// BudgetPolicyAdaptive адаптивный лимит - уменьшает MaxTokens
	BudgetPolicyAdaptive BudgetPolicy = "adaptive"
)

// BudgetConfig конфигурация бюджета
type BudgetConfig struct {
	// DailyLimit дневной лимит токенов
	DailyLimit int

	// MonthlyLimit месячный лимит токенов
	MonthlyLimit int

	// RequestLimit лимит на один запрос
	RequestLimit int

	// Policy политика управления
	Policy BudgetPolicy

	// WarningThreshold порог предупреждения (процент от лимита)
	WarningThreshold float64

	// ResetHour час сброса дневного лимита (0-23)
	ResetHour int
}

// DefaultBudgetConfig возвращает конфигурацию по умолчанию
func DefaultBudgetConfig() *BudgetConfig {
	return &BudgetConfig{
		DailyLimit:       100000,
		MonthlyLimit:     2000000,
		RequestLimit:     10000,
		Policy:           BudgetPolicyAdaptive,
		WarningThreshold: 0.8,
		ResetHour:        0,
	}
}

// BudgetStats статистика использования бюджета
type BudgetStats struct {
	// DailyUsed использовано за день
	DailyUsed int

	// MonthlyUsed использовано за месяц
	MonthlyUsed int

	// DailyRemaining осталось за день
	DailyRemaining int

	// MonthlyRemaining осталось за месяц
	MonthlyRemaining int

	// DailyPercent процент использования за день
	DailyPercent float64

	// MonthlyPercent процент использования за месяц
	MonthlyPercent float64

	// LastReset время последнего сброса
	LastReset time.Time
}

// BudgetManager управляет бюджетом токенов
type BudgetManager struct {
	config *BudgetConfig
	mu     sync.RWMutex

	// dailyUsed использовано за текущий день
	dailyUsed int

	// monthlyUsed использовано за текущий месяц
	monthlyUsed int

	// lastDailyReset время последнего сброса дневного лимита
	lastDailyReset time.Time

	// lastMonthlyReset время последнего сброса месячного лимита
	lastMonthlyReset time.Time

	// requestHistory история запросов
	requestHistory []RequestRecord
}

// RequestRecord запись о запросе
type RequestRecord struct {
	Timestamp  time.Time
	TokensUsed int
	Model      string
	SessionID  string
	UserID     string
}

// NewBudgetManager создаёт новый менеджер бюджета
func NewBudgetManager(config *BudgetConfig) *BudgetManager {
	if config == nil {
		config = DefaultBudgetConfig()
	}

	now := time.Now()
	return &BudgetManager{
		config:           config,
		lastDailyReset:   now,
		lastMonthlyReset: now,
		requestHistory:   make([]RequestRecord, 0),
	}
}

// CheckBudget проверяет, можно ли выполнить запрос
func (bm *BudgetManager) CheckBudget(ctx context.Context, estimatedTokens int) (*BudgetCheckResult, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Проверяем необходимость сброса
	bm.checkReset()

	result := &BudgetCheckResult{
		Allowed:         true,
		EstimatedTokens: estimatedTokens,
	}

	// Проверяем лимит на запрос
	if estimatedTokens > bm.config.RequestLimit {
		if bm.config.Policy == BudgetPolicyHard {
			result.Allowed = false
			result.Reason = fmt.Sprintf("Request limit exceeded: %d > %d", estimatedTokens, bm.config.RequestLimit)
			return result, nil
		}
		result.Warning = fmt.Sprintf("Request limit exceeded: %d > %d", estimatedTokens, bm.config.RequestLimit)
	}

	// Проверяем дневной лимит
	if bm.dailyUsed+estimatedTokens > bm.config.DailyLimit {
		switch bm.config.Policy {
		case BudgetPolicyHard:
			result.Allowed = false
			result.Reason = fmt.Sprintf("Daily limit exceeded: %d + %d > %d", bm.dailyUsed, estimatedTokens, bm.config.DailyLimit)
			return result, nil
		case BudgetPolicySoft:
			result.Warning = fmt.Sprintf("Daily limit exceeded: %d + %d > %d", bm.dailyUsed, estimatedTokens, bm.config.DailyLimit)
		case BudgetPolicyAdaptive:
			// Уменьшаем количество токенов
			allowedTokens := bm.config.DailyLimit - bm.dailyUsed
			if allowedTokens > 0 {
				result.AdjustedTokens = allowedTokens
				result.Warning = fmt.Sprintf("Tokens reduced from %d to %d due to daily limit", estimatedTokens, allowedTokens)
			} else {
				result.Allowed = false
				result.Reason = "Daily limit exhausted"
				return result, nil
			}
		}
	}

	// Проверяем месячный лимит
	if bm.monthlyUsed+estimatedTokens > bm.config.MonthlyLimit {
		switch bm.config.Policy {
		case BudgetPolicyHard:
			result.Allowed = false
			result.Reason = fmt.Sprintf("Monthly limit exceeded: %d + %d > %d", bm.monthlyUsed, estimatedTokens, bm.config.MonthlyLimit)
			return result, nil
		case BudgetPolicySoft:
			result.Warning = fmt.Sprintf("Monthly limit exceeded: %d + %d > %d", bm.monthlyUsed, estimatedTokens, bm.config.MonthlyLimit)
		case BudgetPolicyAdaptive:
			allowedTokens := bm.config.MonthlyLimit - bm.monthlyUsed
			if allowedTokens > 0 {
				if result.AdjustedTokens == 0 || allowedTokens < result.AdjustedTokens {
					result.AdjustedTokens = allowedTokens
					result.Warning = fmt.Sprintf("Tokens reduced from %d to %d due to monthly limit", estimatedTokens, allowedTokens)
				}
			} else {
				result.Allowed = false
				result.Reason = "Monthly limit exhausted"
				return result, nil
			}
		}
	}

	// Проверяем порог предупреждения
	dailyPercent := float64(bm.dailyUsed) / float64(bm.config.DailyLimit)
	if dailyPercent >= bm.config.WarningThreshold {
		result.Warning = fmt.Sprintf("Daily usage at %.1f%% (%d/%d tokens)", dailyPercent*100, bm.dailyUsed, bm.config.DailyLimit)
	}

	return result, nil
}

// RecordUsage записывает использование токенов
func (bm *BudgetManager) RecordUsage(tokens int, model string, sessionID string, userID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.dailyUsed += tokens
	bm.monthlyUsed += tokens

	bm.requestHistory = append(bm.requestHistory, RequestRecord{
		Timestamp:  time.Now(),
		TokensUsed: tokens,
		Model:      model,
		SessionID:  sessionID,
		UserID:     userID,
	})

	// Ограничиваем историю
	if len(bm.requestHistory) > 1000 {
		bm.requestHistory = bm.requestHistory[len(bm.requestHistory)-1000:]
	}
}

// GetStats возвращает статистику использования
func (bm *BudgetManager) GetStats() BudgetStats {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bm.checkReset()

	return BudgetStats{
		DailyUsed:        bm.dailyUsed,
		MonthlyUsed:      bm.monthlyUsed,
		DailyRemaining:   bm.config.DailyLimit - bm.dailyUsed,
		MonthlyRemaining: bm.config.MonthlyLimit - bm.monthlyUsed,
		DailyPercent:     float64(bm.dailyUsed) / float64(bm.config.DailyLimit) * 100,
		MonthlyPercent:   float64(bm.monthlyUsed) / float64(bm.config.MonthlyLimit) * 100,
		LastReset:        bm.lastDailyReset,
	}
}

// ResetDaily сбрасывает дневной лимит
func (bm *BudgetManager) ResetDaily() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.dailyUsed = 0
	bm.lastDailyReset = time.Now()
}

// ResetMonthly сбрасывает месячный лимит
func (bm *BudgetManager) ResetMonthly() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.monthlyUsed = 0
	bm.lastMonthlyReset = time.Now()
}

// checkReset проверяет необходимость сброса лимитов
func (bm *BudgetManager) checkReset() {
	now := time.Now()

	// Проверяем сброс дневного лимита
	if now.Hour() >= bm.config.ResetHour && bm.lastDailyReset.Hour() < bm.config.ResetHour {
		bm.dailyUsed = 0
		bm.lastDailyReset = now
	}

	// Проверяем сброс месячного лимита
	if now.Month() != bm.lastMonthlyReset.Month() || now.Year() != bm.lastMonthlyReset.Year() {
		bm.monthlyUsed = 0
		bm.lastMonthlyReset = now
	}
}

// BudgetCheckResult результат проверки бюджета
type BudgetCheckResult struct {
	// Allowed разрешено ли выполнение
	Allowed bool

	// Reason причина отказа
	Reason string

	// Warning предупреждение
	Warning string

	// EstimatedTokens оценка токенов
	EstimatedTokens int

	// AdjustedTokens скорректированное количество токенов
	AdjustedTokens int
}

// BudgetMiddleware middleware для проверки бюджета
type BudgetMiddleware struct {
	manager *BudgetManager
}

// NewBudgetMiddleware создаёт новый middleware
func NewBudgetMiddleware(manager *BudgetManager) *BudgetMiddleware {
	return &BudgetMiddleware{
		manager: manager,
	}
}

// WrapRequest оборачивает запрос с проверкой бюджета
func (bm *BudgetMiddleware) WrapRequest(ctx context.Context, estimatedTokens int, handler func(adjustedTokens int) (*llm.GenerateResponse, error)) (*llm.GenerateResponse, error) {
	// Проверяем бюджет
	result, err := bm.manager.CheckBudget(ctx, estimatedTokens)
	if err != nil {
		return nil, fmt.Errorf("budget check failed: %w", err)
	}

	if !result.Allowed {
		return nil, fmt.Errorf("budget exceeded: %s", result.Reason)
	}

	// Определяем количество токенов
	tokens := estimatedTokens
	if result.AdjustedTokens > 0 {
		tokens = result.AdjustedTokens
	}

	// Выполняем запрос
	resp, err := handler(tokens)
	if err != nil {
		return nil, err
	}

	// Записываем использование
	if resp.Usage != nil {
		bm.manager.RecordUsage(resp.Usage.TotalTokens, resp.Model, "", "")
	}

	return resp, nil
}

// BudgetLimiter интерфейс для ограничения бюджета
type BudgetLimiter interface {
	// CheckBudget проверяет бюджет
	CheckBudget(ctx context.Context, estimatedTokens int) (*BudgetCheckResult, error)

	// RecordUsage записывает использование
	RecordUsage(tokens int, model string, sessionID string, userID string)

	// GetStats возвращает статистику
	GetStats() BudgetStats
}

// SessionBudgetManager менеджер бюджета для сессий
type SessionBudgetManager struct {
	globalManager *BudgetManager
	sessionLimits map[string]int
	mu            sync.RWMutex
}

// NewSessionBudgetManager создаёт новый менеджер бюджета для сессий
func NewSessionBudgetManager(globalManager *BudgetManager) *SessionBudgetManager {
	return &SessionBudgetManager{
		globalManager: globalManager,
		sessionLimits: make(map[string]int),
	}
}

// SetSessionLimit устанавливает лимит для сессии
func (sbm *SessionBudgetManager) SetSessionLimit(sessionID string, limit int) {
	sbm.mu.Lock()
	defer sbm.mu.Unlock()
	sbm.sessionLimits[sessionID] = limit
}

// CheckSessionBudget проверяет бюджет сессии
func (sbm *SessionBudgetManager) CheckSessionBudget(ctx context.Context, sessionID string, estimatedTokens int) (*BudgetCheckResult, error) {
	// Проверяем лимит сессии
	sbm.mu.RLock()
	sessionLimit, exists := sbm.sessionLimits[sessionID]
	sbm.mu.RUnlock()

	if exists && estimatedTokens > sessionLimit {
		if sbm.globalManager.config.Policy == BudgetPolicyHard {
			return &BudgetCheckResult{
				Allowed:         false,
				Reason:          fmt.Sprintf("Session limit exceeded: %d > %d", estimatedTokens, sessionLimit),
				EstimatedTokens: estimatedTokens,
			}, nil
		}
		// Для мягкой политики возвращаем предупреждение
		result := &BudgetCheckResult{
			Allowed:         true,
			Warning:         fmt.Sprintf("Session limit exceeded: %d > %d", estimatedTokens, sessionLimit),
			EstimatedTokens: estimatedTokens,
		}
		return result, nil
	}

	// Сначала проверяем глобальный бюджет
	result, err := sbm.globalManager.CheckBudget(ctx, estimatedTokens)
	if err != nil {
		return nil, err
	}

	return result, nil
}
