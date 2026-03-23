package transport

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage("Hello, World!")

	if msg.Text != "Hello, World!" {
		t.Errorf("expected text 'Hello, World!', got '%s'", msg.Text)
	}

	if msg.Metadata == nil {
		t.Error("expected metadata to be initialized")
	}

	if msg.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestMessageFields(t *testing.T) {
	msg := NewMessage("test")
	msg.ID = "msg-123"
	msg.UserID = "user-456"
	msg.SessionID = "session-789"
	msg.IsGroup = true
	msg.RawText = "raw test"
	msg.Metadata["key"] = "value"

	if msg.ID != "msg-123" {
		t.Errorf("expected ID 'msg-123', got '%s'", msg.ID)
	}

	if msg.UserID != "user-456" {
		t.Errorf("expected UserID 'user-456', got '%s'", msg.UserID)
	}

	if msg.SessionID != "session-789" {
		t.Errorf("expected SessionID 'session-789', got '%s'", msg.SessionID)
	}

	if !msg.IsGroup {
		t.Error("expected IsGroup to be true")
	}

	if msg.RawText != "raw test" {
		t.Errorf("expected RawText 'raw test', got '%s'", msg.RawText)
	}

	if msg.Metadata["key"] != "value" {
		t.Errorf("expected metadata key 'value', got '%v'", msg.Metadata["key"])
	}
}

func TestAttachment(t *testing.T) {
	msg := NewMessage("test")
	msg.Attachments = []Attachment{
		{
			Type:    AttachmentTypeImage,
			URL:     "https://example.com/image.jpg",
			Name:    "image.jpg",
			Size:    1024,
			Content: []byte("image data"),
		},
	}

	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	if att.Type != AttachmentTypeImage {
		t.Errorf("expected type 'image', got '%s'", att.Type)
	}

	if att.URL != "https://example.com/image.jpg" {
		t.Errorf("expected URL 'https://example.com/image.jpg', got '%s'", att.URL)
	}

	if att.Name != "image.jpg" {
		t.Errorf("expected name 'image.jpg', got '%s'", att.Name)
	}

	if att.Size != 1024 {
		t.Errorf("expected size 1024, got %d", att.Size)
	}
}

func TestAttachmentTypes(t *testing.T) {
	tests := []struct {
		name     string
		attType  AttachmentType
		expected string
	}{
		{"Image", AttachmentTypeImage, "image"},
		{"Audio", AttachmentTypeAudio, "audio"},
		{"Video", AttachmentTypeVideo, "video"},
		{"File", AttachmentTypeFile, "file"},
		{"Text", AttachmentTypeText, "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.attType) != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, string(tt.attType))
			}
		})
	}
}
