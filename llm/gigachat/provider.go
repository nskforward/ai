package gigachat

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/tool"
)

// Config holds GigaChat-specific settings.
type Config struct {
	AuthKey          string // GigaChat Authorization Key (Base64 of ClientID:ClientSecret)
	Scope            string // GIGACHAT_API_PERS, GIGACHAT_API_B2B or GIGACHAT_API_CORP
	Model            string // e.g., "GigaChat-2", "GigaChat-2-Max"
	DisableSSLVerify bool   // Set to true to bypass TLS verification (useful for Mintsifra certs)
}

// Provider implements llm.Provider for GigaChat API.
type Provider struct {
	config      Config
	client      *http.Client
	accessToken string
	tokenExpiry time.Time
	mu          sync.Mutex
}

// NewProvider initializes the GigaChat client.
func NewProvider(cfg Config) *Provider {
	if cfg.Scope == "" {
		cfg.Scope = "GIGACHAT_API_PERS"
	}
	if cfg.Model == "" {
		cfg.Model = "GigaChat"
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.DisableSSLVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Provider{
		config: cfg,
		client: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second, // Long generations can take time
		},
	}
}

func (p *Provider) ensureToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Renew if less than 1 minute left
	if p.accessToken != "" && time.Now().Add(1*time.Minute).Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	data := url.Values{}
	data.Set("scope", p.config.Scope)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", generateUUID())
	req.Header.Set("Authorization", "Basic "+p.config.AuthKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gigachat oauth error: %d - %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"` // milliseconds since epoch
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	p.accessToken = res.AccessToken
	p.tokenExpiry = time.UnixMilli(res.ExpiresAt)

	return p.accessToken, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// chatRequest mirrors the GigaChat API chat/completions payload structure.
type chatRequest struct {
	Model       string                 `json:"model"`
	Messages    []apiMessage           `json:"messages"`
	Functions   []apiFunction          `json:"functions,omitempty"`
	Temperature float32                `json:"temperature,omitempty"`
	ProcessOpts map[string]interface{} `json:"-,omitempty"` // Just for internal logic handling if needed
}

type apiMessage struct {
	Role         string          `json:"role"`
	Content      string          `json:"content"`
	FunctionCall *apiFuncCall    `json:"function_call,omitempty"`
}

type apiFuncCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type apiFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// chatResponse mirrors the GigaChat API chat/completions response.
type chatResponse struct {
	Choices []struct {
		Message apiMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate sends the context to GigaChat.
func (p *Provider) Generate(ctx context.Context, history []llm.Message, tools []tool.Tool, opts *llm.GenerateOptions) (llm.Message, error) {
	token, err := p.ensureToken(ctx)
	if err != nil {
		return llm.Message{}, fmt.Errorf("failed to get token: %w", err)
	}

	reqBody := chatRequest{
		Model:       p.config.Model,
		Temperature: 0.1, // Less hallucination
	}

	// 1. Map messages
	for _, historyMsg := range history {
		role := string(historyMsg.Role)
		
		msg := apiMessage{
			Role:    role,
			Content: historyMsg.Content,
		}

		// If it's a tool output, GigaChat uses role 'function'
		if historyMsg.Role == llm.RoleTool {
			msg.Role = "function"
			// In GigaChat, for role=function, the name of the function must also be sent sometimes.
			// The content is the result. Let's stick to standard map.
			// Actually, name might be required, but we'll try sending content.
			// If not supported natively, we inject it as a user message.
			msg.Role = "user" 
			msg.Content = fmt.Sprintf("[Результат выполнения инструмента (id=%s)]: %s", historyMsg.ToolCallID, historyMsg.Content)
		}

		// If it was an assistant message that called a tool previously
		if historyMsg.Role == llm.RoleAssistant && len(historyMsg.ToolCalls) > 0 {
			tc := historyMsg.ToolCalls[0]
			
			var argsObj interface{}
			// Try to decode JSON string to object because GigaChat might expect an object, not a string
			if err := json.Unmarshal([]byte(tc.Args), &argsObj); err != nil {
				argsObj = tc.Args // fallback to string
			}

			msg.FunctionCall = &apiFuncCall{
				Name:      tc.Name,
				Arguments: argsObj,
			}
			msg.Content = "" // Content can be empty when FunctionCall is present
		}

		reqBody.Messages = append(reqBody.Messages, msg)
	}

	// 2. Map tools
	for _, t := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(t.Schema()), &schema); err != nil {
			return llm.Message{}, fmt.Errorf("failed to parse schema for tool %s: %w", t.Name(), err)
		}

		reqBody.Functions = append(reqBody.Functions, apiFunction{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  schema,
		})
	}

	// 3. Structured Output logic
	if opts != nil && opts.ResponseFormat != "" {
		// GigaChat does not natively support response_format strict schemas yet.
		// So we artificially wrap it as a forced function call!
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(opts.ResponseFormat), &schema); err == nil {
			// Add a special pseudo-tool
			reqBody.Functions = append(reqBody.Functions, apiFunction{
				Name:        "return_structured_result",
				Description: "You MUST call this function to return your final answer in the required JSON format.",
				Parameters:  schema,
			})
			
			// Force the model to call it by adding to the first system message, or as a user message at the end
			reqBody.Messages = append(reqBody.Messages, apiMessage{
				Role: "user",
				Content: "PLEASE REMEMBER: You must use the function 'return_structured_result' to output your final answer formatted as requested.",
			})
		}
	}

	// 4. Send Request
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return llm.Message{}, err
	}

	endpoint := "https://gigachat.devices.sberbank.ru/api/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return llm.Message{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return llm.Message{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyString, _ := io.ReadAll(resp.Body)
		return llm.Message{}, fmt.Errorf("gigachat completion error: %d - %s", resp.StatusCode, string(bodyString))
	}

	var res chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return llm.Message{}, err
	}

	if len(res.Choices) == 0 {
		return llm.Message{}, fmt.Errorf("gigachat returned empty choices. Error message: %s", res.Error.Message)
	}

	apiOut := res.Choices[0].Message

	// 5. Parse response
	out := llm.Message{
		Role:    llm.RoleAssistant,
		Content: apiOut.Content,
	}

	if apiOut.FunctionCall != nil {
		// Convert FunctionCall arguments (interface{}) back to a JSON string
		var argsStr string
		switch v := apiOut.FunctionCall.Arguments.(type) {
		case string:
			argsStr = v
		default:
			// It was parsed as map[string]interface{}
			b, _ := json.Marshal(v)
			argsStr = string(b)
		}

		// Is it our fake "structured output" function?
		if apiOut.FunctionCall.Name == "return_structured_result" {
			out.Content = argsStr
		} else {
			out.ToolCalls = []llm.ToolCall{
				{
					ID:   generateUUID(), // GigaChat doesn't return tool call IDs, auto-generate one
					Name: apiOut.FunctionCall.Name,
					Args: argsStr,
				},
			}
		}
	}

	return out, nil
}
