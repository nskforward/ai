package transport

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Telegram implements the Transport interface for Telegram bots via long polling.
type Telegram struct {
	bot     *tgbotapi.BotAPI
	updates tgbotapi.UpdatesChannel
}

// NewTelegram creates a new Telegram transport using the provided bot token.
func NewTelegram(token string) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to init telegram bot: %w", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	return &Telegram{
		bot:     bot,
		updates: updates,
	}, nil
}

func (t *Telegram) Name() string {
	return "telegram"
}

func (t *Telegram) Read() (Message, error) {
	for update := range t.updates {
		if update.Message == nil {
			// Ignore any non-Message updates for now
			continue
		}

		chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
		userID := strconv.FormatInt(update.Message.From.ID, 10)

		return Message{
			SessionID:     chatID,
			UserID:        userID,
			TransportName: "telegram",
			Text:          update.Message.Text,
		}, nil
	}
	return Message{}, fmt.Errorf("telegram updates channel closed")
}

func (t *Telegram) Write(msg Message) error {
	chatID, err := strconv.ParseInt(msg.SessionID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID format: %w", err)
	}

	tgMsg := tgbotapi.NewMessage(chatID, msg.Text)
	_, err = t.bot.Send(tgMsg)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}

	return nil
}

func (t *Telegram) SendTyping(sessionID string) error {
	chatID, err := strconv.ParseInt(sessionID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID format: %w", err)
	}

	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, err = t.bot.Request(action)
	return err
}
