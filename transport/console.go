package transport

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Console implements the Transport interface for CLI usage.
type Console struct {
	scanner *bufio.Scanner
}

// NewConsole creates a new standard-input based transport.
func NewConsole() *Console {
	return &Console{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

func (c *Console) Name() string { return "console" }

func (c *Console) Read() (Message, error) {
	fmt.Print("User> ")
	if !c.scanner.Scan() {
		return Message{}, c.scanner.Err()
	}
	text := strings.TrimSpace(c.scanner.Text())
	
	return Message{
		SessionID:     "cli-session",
		UserID:        "admin", // Console user is admin by default
		TransportName: c.Name(),
		Text:          text,
	}, nil
}

func (c *Console) Write(msg Message) error {
	fmt.Printf("Agent> %s\n", msg.Text)
	return nil
}

func (c *Console) SendTyping(sessionID string) error {
	return nil
}
