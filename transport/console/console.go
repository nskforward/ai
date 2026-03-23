package console

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/nskforward/ai/transport"
)

// Console реализует Transport для интерактивного CLI
type Console struct {
	handler transport.MessageHandler
	scanner *bufio.Scanner
	mu      sync.Mutex
	done    chan struct{}
}

// New создаёт новый Console транспорт
func New() *Console {
	return &Console{
		scanner: bufio.NewScanner(os.Stdin),
		done:    make(chan struct{}),
	}
}

// Name возвращает имя транспорта
func (c *Console) Name() string {
	return "console"
}

// Start запускает приём сообщений
func (c *Console) Start(ctx context.Context) error {
	fmt.Println("Console transport started. Type your messages (Ctrl+C to exit):")
	fmt.Println("---")

	go c.readLoop(ctx)
	return nil
}

// Stop останавливает приём сообщений
func (c *Console) Stop() error {
	close(c.done)
	return nil
}

// Send отправляет сообщение
func (c *Console) Send(ctx context.Context, msg *transport.Message) error {
	fmt.Printf("\n[Agent]: %s\n\n", msg.Text)
	return nil
}

// Handle регистрирует обработчик сообщений
func (c *Console) Handle(handler transport.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

func (c *Console) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
			fmt.Print("[You]: ")
			if !c.scanner.Scan() {
				return
			}

			text := strings.TrimSpace(c.scanner.Text())
			if text == "" {
				continue
			}

			// Создаём контекст агента
			agentCtx := transport.NewAgentContext("console-user", "console-session", "console")
			agentCtx.UserName = "Console User"
			agentCtx.DisplayName = "Console User"

			// Создаём сообщение
			msg := transport.NewMessage(text)
			msg.UserID = agentCtx.UserID
			msg.SessionID = agentCtx.SessionID

			// Вызываем обработчик
			c.mu.Lock()
			handler := c.handler
			c.mu.Unlock()

			if handler != nil {
				resp, err := handler(ctx, agentCtx, msg)
				if err != nil {
					fmt.Printf("\n[Error]: %v\n\n", err)
				} else if resp != nil {
					fmt.Printf("\n[Agent]: %s\n\n", resp.Text)
				}
			}
		}
	}
}
