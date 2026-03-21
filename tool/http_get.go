package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nskforward/ai/storage"
)

// HttpGetTool provides the agent with the ability to fetch data from the web.
type HttpGetTool struct {
	Whitelist storage.Storage
}

func (t *HttpGetTool) Name() string { return "http_get" }
func (t *HttpGetTool) Description() string {
	return "Makes an HTTP GET request to a URL. Support custom headers. The HOST (DOMAIN) must be whitelisted. Use headers for API keys if required."
}
func (t *HttpGetTool) Schema() string {
	return `{"type":"object","properties":{"url":{"type":"string","description":"URL to fetch"},"headers":{"type":"object","properties":{},"additionalProperties":{"type":"string"},"description":"Optional HTTP headers (e.g. {\"X-Yandex-API-Key\": \"...\"})"}},"required":["url"]}`
}
func (t *HttpGetTool) RequiresAdmin() bool { return false }

func (t *HttpGetTool) Execute(ctx context.Context, transportName string, userID string, args string) (string, error) {
	var input struct {
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	// Clean URL from common LLM escaping artifacts
	cleanURL := input.URL
	cleanURL = strings.ReplaceAll(cleanURL, "\\u0026", "&")
	cleanURL = strings.ReplaceAll(cleanURL, "\u0026", "&")
	input.URL = cleanURL

	u, err := url.Parse(input.URL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Get sessionID from context for one-time permissions
	sessionID, _ := ctx.Value("sessionID").(string)

	// Check whitelist if storage is provided
	if t.Whitelist != nil {
		authString := u.Host + u.Path
		host := u.Host
		
		hostHash := hex.EncodeToString(sha256.New().Sum([]byte(host))) // Note: I'll use a helper or just Sum256 for brevity
		_ = hostHash // placeholder for real hash
		
		// 1. Check Persistent Host
		h := sha256.Sum256([]byte(host))
		hostKey := hex.EncodeToString(h[:])
		if _, err := t.Whitelist.Read("approved_urls/hosts/" + hostKey); err == nil {
			goto authorized
		}

		// 2. Check Persistent Path
		p := sha256.Sum256([]byte(authString))
		pathKey := hex.EncodeToString(p[:])
		if _, err := t.Whitelist.Read("approved_urls/paths/" + pathKey); err == nil {
			goto authorized
		}

		// 3. Check One-time Session Path
		if sessionID != "" {
			oneTimeKey := "permissions/" + sessionID + "/urls/" + pathKey
			if _, err := t.Whitelist.Read(oneTimeKey); err == nil {
				// Authorized for one time!
				// Consume it IMMEDIATELY to prevent replay if the request is slow 
				// (though we should probably do it after success, 
				// but the user said "razo-vo" and security-wise consumption is better before/early).
				_ = t.Whitelist.Delete(oneTimeKey)
				goto authorized
			}
		}

		return fmt.Sprintf("ACCESS DENIED: The address %s is not approved. Request permission from the administrator. To grant ONE-TIME permission, the admin should say 'ok'.", authString), nil
	}

authorized:

	// 10 second timeout for external API requests
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, input.URL, nil)
	if err != nil {
		return "", err
	}

	// Add custom headers
	for k, v := range input.Headers {
		req.Header.Set(k, v)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Limit response size to prevent context overflow
	bodyStr := string(data)
	if len(bodyStr) > 4000 {
		bodyStr = bodyStr[:4000] + "\n... [TRUNCATED]"
	}

	return bodyStr, nil
}
