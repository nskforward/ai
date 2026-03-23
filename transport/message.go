package transport

import "time"

// AttachmentType тип вложения
type AttachmentType string

const (
	AttachmentTypeImage AttachmentType = "image"
	AttachmentTypeAudio AttachmentType = "audio"
	AttachmentTypeVideo AttachmentType = "video"
	AttachmentTypeFile  AttachmentType = "file"
	AttachmentTypeText  AttachmentType = "text"
)

// Attachment представляет вложение
type Attachment struct {
	Type    AttachmentType
	URL     string
	Content []byte
	Name    string
	Size    int64
}

// Message представляет универсальное сообщение
type Message struct {
	// ID уникальный идентификатор сообщения
	ID string

	// Text текст сообщения
	Text string

	// RawText оригинальный текст (до обработки)
	RawText string

	// UserID идентификатор пользователя (из Transport)
	UserID string

	// SessionID идентификатор сессии
	SessionID string

	// IsGroup указывает, что сообщение из группы
	IsGroup bool

	// Metadata дополнительные метаданные
	Metadata map[string]interface{}

	// Attachments вложенные файлы
	Attachments []Attachment

	// Timestamp временная метка
	Timestamp time.Time

	// RawData сырые данные от транспорта
	RawData interface{}
}

// NewMessage создаёт новое сообщение
func NewMessage(text string) *Message {
	return &Message{
		Text:      text,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}
