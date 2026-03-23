package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nskforward/ai/transport"
)

// Config конфигурация Telegram транспорта
type Config struct {
	BotToken string
	Debug    bool
}

// Telegram реализует Transport для Telegram
type Telegram struct {
	config  Config
	bot     *tgbotapi.BotAPI
	handler transport.MessageHandler
	mu      sync.Mutex
	done    chan struct{}
}

// New создаёт новый Telegram транспорт
func New(config Config) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = config.Debug

	return &Telegram{
		config: config,
		bot:    bot,
		done:   make(chan struct{}),
	}, nil
}

// Name возвращает имя транспорта
func (t *Telegram) Name() string {
	return "telegram"
}

// Start запускает приём сообщений
func (t *Telegram) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)

	go t.handleUpdates(ctx, updates)

	return nil
}

// Stop останавливает приём сообщений
func (t *Telegram) Stop() error {
	close(t.done)
	t.bot.StopReceivingUpdates()
	return nil
}

// Send отправляет сообщение
func (t *Telegram) Send(ctx context.Context, msg *transport.Message) error {
	chatID, err := strconv.ParseInt(msg.SessionID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid session ID (chat ID): %w", err)
	}

	tgMsg := tgbotapi.NewMessage(chatID, msg.Text)
	_, err = t.bot.Send(tgMsg)
	return err
}

// Handle регистрирует обработчик сообщений
func (t *Telegram) Handle(handler transport.MessageHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

func (t *Telegram) handleUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			t.processUpdate(ctx, update)
		}
	}
}

func (t *Telegram) processUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	// Создаём контекст агента
	userID := strconv.FormatInt(update.Message.From.ID, 10)
	sessionID := strconv.FormatInt(update.Message.Chat.ID, 10)

	agentCtx := transport.NewAgentContext(userID, sessionID, "telegram")
	agentCtx.UserName = update.Message.From.UserName
	agentCtx.DisplayName = update.Message.From.FirstName + " " + update.Message.From.LastName
	agentCtx.IsAdmin = false // TODO: Configure admin users

	// Создаём сообщение
	msg := transport.NewMessage(update.Message.Text)
	msg.ID = strconv.Itoa(update.Message.MessageID)
	msg.UserID = userID
	msg.SessionID = sessionID
	msg.IsGroup = update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup()
	msg.RawData = update

	// Обрабатываем вложения
	if update.Message.Photo != nil {
		// Берём фото наибольшего размера
		photo := update.Message.Photo[len(update.Message.Photo)-1]
		msg.Attachments = append(msg.Attachments, transport.Attachment{
			Type: transport.AttachmentTypeImage,
			URL:  photo.FileID,
			Size: int64(photo.FileSize),
		})
	}

	if update.Message.Document != nil {
		msg.Attachments = append(msg.Attachments, transport.Attachment{
			Type: transport.AttachmentTypeFile,
			URL:  update.Message.Document.FileID,
			Name: update.Message.Document.FileName,
			Size: int64(update.Message.Document.FileSize),
		})
	}

	if update.Message.Voice != nil {
		msg.Attachments = append(msg.Attachments, transport.Attachment{
			Type: transport.AttachmentTypeAudio,
			URL:  update.Message.Voice.FileID,
			Size: int64(update.Message.Voice.FileSize),
		})
	}

	// Вызываем обработчик
	t.mu.Lock()
	handler := t.handler
	t.mu.Unlock()

	if handler != nil {
		resp, err := handler(ctx, agentCtx, msg)
		if err != nil {
			errMsg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Error: %v", err))
			t.bot.Send(errMsg)
		} else if resp != nil {
			respMsg := tgbotapi.NewMessage(update.Message.Chat.ID, resp.Text)
			t.bot.Send(respMsg)
		}
	}
}
