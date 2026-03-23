package transport

import "time"

// AgentContext содержит информацию о пользователе и сессии
type AgentContext struct {
	// UserID уникальный идентификатор пользователя
	UserID string

	// SessionID идентификатор сессии/диалога
	SessionID string

	// TransportName имя транспорта, от которого получено сообщение
	TransportName string

	// IsAdmin флаг администратора
	IsAdmin bool

	// UserName имя пользователя (опционально)
	UserName string

	// DisplayName отображаемое имя
	DisplayName string

	// Locale языковой стандарт пользователя
	Locale string

	// Metadata дополнительные данные авторизации
	Metadata map[string]interface{}

	// Permissions права доступа
	Permissions []string

	// createdAt время создания контекста
	createdAt time.Time
}

// NewAgentContext создаёт новый контекст агента
func NewAgentContext(userID, sessionID, transportName string) *AgentContext {
	return &AgentContext{
		UserID:        userID,
		SessionID:     sessionID,
		TransportName: transportName,
		Metadata:      make(map[string]interface{}),
		Permissions:   make([]string, 0),
		createdAt:     time.Now(),
	}
}

// HasPermission проверяет наличие права доступа
func (ac *AgentContext) HasPermission(permission string) bool {
	for _, p := range ac.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// GetCreatedAt возвращает время создания контекста
func (ac *AgentContext) GetCreatedAt() time.Time {
	return ac.createdAt
}
