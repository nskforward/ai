package efficiency

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// CacheEntry запись в кэше
type CacheEntry struct {
	// Value значение
	Value string

	// CreatedAt время создания
	CreatedAt time.Time

	// ExpiresAt время истечения
	ExpiresAt time.Time

	// HitCount количество обращений
	HitCount int64
}

// PromptCache кеширует системные промпты
type PromptCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	maxSize int
	ttl     time.Duration
}

// NewPromptCache создаёт новый кэш промптов
func NewPromptCache(maxSize int, ttl time.Duration) *PromptCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}

	cache := &PromptCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// Запускаем очистку истёкших записей
	go cache.cleanup()

	return cache
}

// Get получает закэшированный промпт
func (c *PromptCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return "", false
	}

	// Проверяем срок действия
	if time.Now().After(entry.ExpiresAt) {
		return "", false
	}

	entry.HitCount++
	return entry.Value, true
}

// Set устанавливает кэш
func (c *PromptCache) Set(key string, prompt string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		ttl = c.ttl
	}

	// Если достигнут лимит, удаляем старые записи
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &CacheEntry{
		Value:     prompt,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		HitCount:  0,
	}
}

// MarkCacheable помечает промпт как кешируемый и возвращает ключ
func (c *PromptCache) MarkCacheable(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	key := fmt.Sprintf("%x", hash)

	// Автоматически кэшируем
	c.Set(key, prompt, c.ttl)

	return key
}

// Invalidate инвалидирует кэш
func (c *PromptCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// InvalidateAll очищает весь кэш
func (c *PromptCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// GetStats возвращает статистику кэша
func (c *PromptCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalHits int64
	validEntries := 0

	for _, entry := range c.entries {
		if time.Now().Before(entry.ExpiresAt) {
			validEntries++
			totalHits += entry.HitCount
		}
	}

	return CacheStats{
		TotalEntries: len(c.entries),
		ValidEntries: validEntries,
		TotalHits:    totalHits,
		MaxSize:      c.maxSize,
	}
}

// CacheStats статистика кэша
type CacheStats struct {
	TotalEntries int
	ValidEntries int
	TotalHits    int64
	MaxSize      int
}

// evictOldest удаляет самую старую запись
func (c *PromptCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanup периодически очищает истёкшие записи
func (c *PromptCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}
