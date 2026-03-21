package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/logger"
	"github.com/nskforward/ai/memory"
	"github.com/nskforward/ai/session"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

const defaultSysPrompt = `Вы — специализированный AI-Агент. Ваши возможности СТРОГО ограничены списком инструментов и Оглавлением памяти (навыками).
Ваше Оглавление памяти приведено прямо в ЭТОМ системном сообщении ниже.
Если в Оглавлении памяти нет навыка для решения задачи — СТРОГО ЗАПРЕЩЕНО придумывать (галлюцинировать) URL-адреса или API-ключи.

ИЗВЕСТНЫЕ ШАБЛОНЫ API:
- Яндекс.Погода: Требует заголовок "X-Yandex-API-Key" с вашим ключом. НЕ передавайте ключ в параметрах URL.

КРИТИЧЕСКИЕ ПРАВИЛА:
1. Если выполнение инструмента (любого) вернуло ошибку "ACCESS DENIED", вы ДОЛЖНЫ сообщить об этом пользователю и запросить разрешение. КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО врать о том, что всё получилось.
2. При сохранении навыка (save_skill), он ДОЛЖЕН содержать работающий КОД и URL. КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО сохранять свои планы или просто текст как навык.
3. КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО упоминать пользователю свои внутренние планы, шаги или "оркестратора". Пользователь должен видеть только конечный результат.
4. Соблюдай краткость. После успешного вызова инструмента просто подтверди результат.`

// Handler is a function that processes a message.
type Handler func(ctx context.Context, msg transport.Message) error

// Middleware wraps a Handler with pre/post processing.
type Middleware func(ctx context.Context, msg transport.Message, next Handler) error

// Config holds the configuration and dependencies for the Agent.
type Config struct {
	// Dependencies
	Transport  transport.Transport
	Storage    storage.Storage
	LightModel llm.Provider
	HeavyModel llm.Provider
	Tools      []tool.Tool

	// Security
	AllowedAdmins []tool.AdminUser

	// Session memory
	SessionStore session.Store // nil = in-memory with 1h TTL

	// Observability
	Logger logger.Logger // nil = slog default

	// Middleware chain (executed in order)
	Middlewares []Middleware

	// Behaviour
	EnableReflection bool // if true, Light Model verifies Heavy's answer
	MaxSteps         int  // max ReAct iterations (default 10)

	// Experience management callback.
	// Called after a successful task completion.
	// Return true to trigger save_skill.
	OnTaskComplete func(task string, steps []llm.Message) bool
}

// Agent is the core framework struct managing the lifecycle.
type Agent struct {
	config     Config
	log        logger.Logger
	sessions   session.Store
	memManager *memory.Manager
	toolReg    *tool.Registry
}

// New creates a new Agent instance.
func New(cfg Config) *Agent {
	reg := tool.NewRegistry(cfg.AllowedAdmins)
	for _, t := range cfg.Tools {
		reg.Register(t)
	}

	// Defaults
	log := cfg.Logger
	if log == nil {
		log = logger.NewSlog()
	}

	ss := cfg.SessionStore
	if ss == nil {
		ss = session.NewMemoryStore(1 * time.Hour)
	}

	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}

	return &Agent{
		config:     cfg,
		log:        log,
		sessions:   ss,
		memManager: memory.NewManager(cfg.Storage),
		toolReg:    reg,
	}
}

// Start begins the agent event loop with graceful shutdown support.
func (a *Agent) Start(ctx context.Context) error {
	a.log.Info("agent started")

	for {
		select {
		case <-ctx.Done():
			a.log.Info("agent shutting down", "reason", ctx.Err())
			return ctx.Err()
		default:
		}

		msg, err := a.config.Transport.Read()
		if err != nil {
			// Check if context was cancelled while blocking on Read
			if ctx.Err() != nil {
				a.log.Info("agent shutting down", "reason", ctx.Err())
				return ctx.Err()
			}
			return fmt.Errorf("transport read error: %w", err)
		}

		a.log.Info("message received",
			"session", msg.SessionID,
			"transport", msg.TransportName,
			"user", msg.UserID,
		)
		a.log.Debug("user prompt", "text", msg.Text)

		// Build middleware chain
		handler := a.processMessage
		for i := len(a.config.Middlewares) - 1; i >= 0; i-- {
			mw := a.config.Middlewares[i]
			next := handler
			handler = func(ctx context.Context, msg transport.Message) error {
				return mw(ctx, msg, next)
			}
		}

		if err := a.handleCommand(ctx, msg); err == nil {
			// Command was handled, skip LLM processing for this message
			continue
		}

		if err := handler(ctx, msg); err != nil {
			a.log.Error("processing error", "error", err, "session", msg.SessionID)
			errMsg := msg
			errMsg.Text = fmt.Sprintf("Ошибка: %v", err)
			_ = a.config.Transport.Write(errMsg)
		}
	}
}

// tryAutoApprove checks if the current message is an affirmative response to a previous request.
func (a *Agent) tryAutoApprove(ctx context.Context, msg transport.Message, history []llm.Message) {
	text := strings.ToLower(strings.TrimSpace(msg.Text))
	affirmative := []string{
		"ок", "ok", "да", "yes", "хорошо", "разрешаю", "одобряю", "вперед", "действуй", "согласен", "agree",
		"allow", "approve", "allow_save", "разрешить", "разрешаю", "можно", "давай",
	}
	
	isAffirmative := false
	for _, word := range affirmative {
		if text == word || strings.HasPrefix(text, word+" ") || strings.HasPrefix(text, word+"!") || strings.HasPrefix(text, word+".") {
			isAffirmative = true
			break
		}
	}

	if !isAffirmative {
		return
	}

	// Look for the most recent assistant message that looks like a permission request
	var lastRequest string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == llm.RoleAssistant {
			content := strings.ToLower(history[i].Content)
			if strings.Contains(content, "адресу") || strings.Contains(content, "навык") || strings.Contains(content, "save_skill") {
				lastRequest = history[i].Content
				break
			}
		}
	}

	if lastRequest == "" {
		return
	}

	// 1. Check for Path approval (only if the assistant asked for an address)
	if strings.Contains(strings.ToLower(lastRequest), "адресу") {
		// Extract potential HOST/PATH from the message
		words := strings.Fields(lastRequest)
		for _, word := range words {
			word = strings.Trim(word, ".,\"'()<>")
			if strings.Contains(word, ".") && !strings.Contains(word, "навык") {
				// Try to parse as URL to clean it up (remove query params if any)
				authString := word
				if u, err := url.Parse(word); err == nil && u.Host != "" {
					authString = u.Host + u.Path
				} else if strings.Contains(word, "/") {
					authString = strings.Split(word, "?")[0]
				}
				
				hash := sha256.Sum256([]byte(authString))
				pathKey := hex.EncodeToString(hash[:])

				// Determine permission type from user response
				isPersistent := strings.Contains(text, "белый список") || strings.Contains(text, "whitelist") || strings.Contains(text, "всегда") || strings.Contains(text, "always")
				isDomain := strings.Contains(text, "домен") || strings.Contains(text, "domain") || strings.Contains(text, "хост") || strings.Contains(text, "host")

				if isDomain {
					host := authString
					if idx := strings.Index(authString, "/"); idx != -1 {
						host = authString[:idx]
					}
					h := sha256.Sum256([]byte(host))
					hostKey := hex.EncodeToString(h[:])
					_ = a.config.Storage.Write("approved_urls/hosts/"+hostKey, []byte("approved"))
					a.log.Info("contextual Host persistent approval granted", "host", host, "session", msg.SessionID)
				} else if isPersistent {
					_ = a.config.Storage.Write("approved_urls/paths/"+pathKey, []byte("approved"))
					a.log.Info("contextual Path persistent approval granted", "path", authString, "session", msg.SessionID)
				} else {
					// Default: ONE-TIME session permission
					_ = a.config.Storage.Write("permissions/"+msg.SessionID+"/urls/"+pathKey, []byte("granted"))
					a.log.Info("contextual Path ONE-TIME permission granted", "path", authString, "session", msg.SessionID)
				}
			}
		}
	}

	// 2. Check for Skill save approval (broad match for any skill mentions)
	lowerReq := strings.ToLower(lastRequest)
	if strings.Contains(lowerReq, "навык") || strings.Contains(lowerReq, "save_skill") {
		_ = a.config.Storage.Write("permissions/"+msg.SessionID+"/save_skill", []byte("granted"))
		a.log.Info("contextual save permission granted", "session", msg.SessionID)
	}
}

// handleCommand checks for special commands starting with ! and executes them if authorized.
func (a *Agent) handleCommand(ctx context.Context, msg transport.Message) error {
	trimmed := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(trimmed, "!") {
		return fmt.Errorf("not a command")
	}

	// Permission grants are handled and then allowed to continue to LLM ReAct loop
	parts := strings.Fields(trimmed)
	cmd := parts[0]

	switch cmd {
	case "!approve", "!allow_save":
		if !a.isAdmin(msg) {
			_ = a.config.Transport.Write(transport.Message{
				SessionID:     msg.SessionID,
				TransportName: msg.TransportName,
				UserID:        msg.UserID,
				Text:          "Ошибка: у вас нет прав администратора для этой команды.",
			})
			return fmt.Errorf("unauthorized")
		}
		if cmd == "!allow_save" {
			_ = a.config.Storage.Write("permissions/"+msg.SessionID+"/save_skill", []byte("granted"))
			a.log.Info("explicit save permission granted via command", "session", msg.SessionID)
		} else {
			if len(parts) < 2 {
				_ = a.config.Transport.Write(transport.Message{
					SessionID:     msg.SessionID,
					TransportName: msg.TransportName,
					UserID:        msg.UserID,
					Text:          "Использование: !approve <URL>",
				})
				return fmt.Errorf("invalid arguments")
			}
			url := parts[1]
			hash := sha256.Sum256([]byte(url))
			key := hex.EncodeToString(hash[:])
			_ = a.config.Storage.Write("approved_urls/"+key, []byte("approved"))
			a.log.Info("explicit URL approval via command", "url", url)
		}
		// Return nil to indicate we handled the permission, but let Agent.Start continue to processMessage
		return nil
	}

	// Other commands might still want to break execution (standard behavior)
	return a.handleStandardCommand(ctx, msg)
}

func (a *Agent) isAdmin(msg transport.Message) bool {
	for _, admin := range a.config.AllowedAdmins {
		if admin.Transport == msg.TransportName && admin.UserID == msg.UserID {
			return true
		}
	}
	return false
}

// handleStandardCommand handles non-permission commands and returns an error to signal execution break.
func (a *Agent) handleStandardCommand(ctx context.Context, msg transport.Message) error {
	trimmed := strings.TrimSpace(msg.Text)
	parts := strings.Fields(trimmed)
	cmd := parts[0]

	// Check if sender is admin for all commands
	if !a.isAdmin(msg) {
		return a.config.Transport.Write(transport.Message{
			SessionID:     msg.SessionID,
			TransportName: msg.TransportName,
			UserID:        msg.UserID,
			Text:          "Ошибка: У вас нет прав администратора для выполнения этой команды.",
		})
	}

	switch cmd {
	// Add other commands here if needed
	}

	return fmt.Errorf("unknown command")
}

// processMessage handles a single request with Dual-Model routing, session memory, and reflection.
func (a *Agent) processMessage(ctx context.Context, msg transport.Message) error {
	// 1. Load system prompt + TOC
	sysPrompt, err := a.getSystemPrompt()
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	toc, err := a.memManager.GenerateTOC()
	if err != nil {
		return fmt.Errorf("TOC generation: %w", err)
	}

	fullSysPrompt := fmt.Sprintf("%s\n\n--- ОГЛАВЛЕНИЕ ПАМЯТИ ---\n%s", sysPrompt, toc)

	// 2. Load session history (or start fresh)
	history, err := a.sessions.Load(msg.SessionID)
	if err != nil {
		return fmt.Errorf("session load: %w", err)
	}

	// Try auto-approve from natural language before continuing
	a.tryAutoApprove(ctx, msg, history)

	// Start typing indicator in background until execution finishes
	typingCtx, cancelTyping := context.WithCancel(ctx)
	defer cancelTyping()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		// Send initial indicator
		_ = a.config.Transport.SendTyping(msg.SessionID)
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				_ = a.config.Transport.SendTyping(msg.SessionID)
			}
		}
	}()

	if len(history) == 0 {
		// New session — inject system prompt
		history = []llm.Message{
			{Role: llm.RoleSystem, Content: fullSysPrompt},
		}
	} else {
		// Update system prompt (may have new TOC entries)
		history[0] = llm.Message{Role: llm.RoleSystem, Content: fullSysPrompt}
	}

	// Append user message
	history = append(history, llm.Message{Role: llm.RoleUser, Content: msg.Text})

	// Inject sessionID into context for tools (like SaveSkillTool)
	ctx = context.WithValue(ctx, "sessionID", msg.SessionID)

	availableTools := a.toolReg.GetTools()

	// 2.5 Planning Phase (Orchestrator)
	// Create a temporary prompt for planning, ensuring system message is first.
	// The planning instruction is added as a user message to avoid multiple system messages.
	planPrompt := make([]llm.Message, len(history))
	copy(planPrompt, history)
	planPrompt = append(planPrompt, llm.Message{
		Role:    llm.RoleUser,
		Content: "[ИНСТРУКЦИЯ ОРКЕСТРАТОРА]: Проверь историю и 'ОГЛАВЛЕНИЕ ПАМЯТИ'. \n1. Если в запросе есть URL, ПЕРВЫМ ШАГОМ плана поставь 'использовать http_get для чтения documentation [URL]'.\n2. КАТЕГОРИЧЕСКИ ЗАПРЕЩЕНО изменять URL-адреса. Копируй их из запроса пользователя СИМВОЛ В СИМВОЛ.\n3. Шаги ДОЛЖНЫ содержать названия инструментов: 'использовать save_skill для...', 'использовать http_get для...'.\n4. БУДЬ СТРОГ: если в ОГЛАВЛЕНИИ лежит просто описание без кода — считай, что навыка НЕТ.",
	})

	a.log.Debug("orchestrator: requesting plan")
	planMsg, err := a.config.LightModel.Generate(ctx, planPrompt, nil, &llm.GenerateOptions{ResponseFormat: PlanSchema})
	if err != nil {
		a.log.Error("planning phase error", "error", err)
	}

	var plan Plan
	if err == nil {
		a.log.Debug("orchestrator: received raw plan", "content", planMsg.Content)
		if err := json.Unmarshal([]byte(planMsg.Content), &plan); err == nil {
			if plan.IsComplex {
				a.log.Info("orchestrator: complex task detected, spinning up subagents", "reasoning", plan.Reasoning, "steps", len(plan.Steps))
				
				subAgent := &SubAgent{
					provider: a.config.HeavyModel, // Heavy model used for deep thinking in subagents
					tools:    a.toolReg,
					maxSteps: a.config.MaxSteps,
					log:      a.log,
				}

				var aggregatedContext string
				
				// Sequential execution of sub-agents
				for _, step := range plan.Steps {
					a.log.Info("executing step", "id", step.ID, "desc", step.Description)
					
					res, err := subAgent.Execute(ctx, step.Description, aggregatedContext, msg.TransportName, msg.UserID)
					if err != nil {
						a.log.Error("subagent failed", "step", step.ID, "error", err)
						aggregatedContext += fmt.Sprintf("\nШаг %d ошибка: %v\n", step.ID, err)
					} else {
						a.log.Debug("subagent completed", "step", step.ID, "raw_result", res)
						aggregatedContext += fmt.Sprintf("\nШаг %d результат:\n%s\n", step.ID, res)
					}
				}

				// Analyze results for failures or ACCESS DENIED
				hasError := strings.Contains(strings.ToUpper(aggregatedContext), "ERROR") ||
					strings.Contains(strings.ToUpper(aggregatedContext), "ACCESS DENIED") ||
					strings.Contains(strings.ToLower(aggregatedContext), "permission required")

				statusMsg := "[СИСТЕМНОЕ УВЕДОМЛЕНИЕ]: Все шаги плана успешно выполнены субагентами. Подтверди завершение пользователю ОДНИМ кратким предложением."
				if hasError {
					statusMsg = fmt.Sprintf("[СИСТЕМНОЕ УВЕДОМЛЕНИЕ]: Некоторые шаги НЕ ВЫПОЛНЕНЫ из-за ошибок или отсутствия доступа. Результаты:\n%s\nТы ДОЛЖЕН сообщить пользователю о проблеме и запросить необходимые разрешения. НЕ ГОВОРИ, что всё успешно.", aggregatedContext)
				}

				// Notify the main agent
				history = append(history, llm.Message{
					Role:    llm.RoleUser,
					Content: statusMsg,
				})
			}
		}
	}

	// 3. ReAct loop (Either for simple task directly, or finalizing the complex task)
	var finalResponse string
	for step := 0; step < a.config.MaxSteps; step++ {
		// Dual Model Routing: Light for step 0, Heavy afterward
		provider := a.config.LightModel
		modelName := "light"
		if step > 0 {
			provider = a.config.HeavyModel
			modelName = "heavy"
		}

		a.log.Debug("LLM call", "model", modelName, "step", step)

		resp, err := provider.Generate(ctx, history, availableTools, nil)
		if err != nil {
			return fmt.Errorf("LLM error (step %d): %w", step, err)
		}

		history = append(history, resp)

		// No tool calls = final answer
		if len(resp.ToolCalls) == 0 {
			finalResponse = resp.Content
			break
		}

		// Execute tools
		for _, tc := range resp.ToolCalls {
			a.log.Debug("executing tool", "tool", tc.Name, "args", tc.Args)
			start := time.Now()
			toolResult, err := a.toolReg.Execute(ctx, tc.Name, msg.TransportName, msg.UserID, tc.Args)
			elapsed := time.Since(start)

			if err != nil {
				a.log.Error("tool error", "tool", tc.Name, "error", err, "elapsed", elapsed)
				toolResult = fmt.Sprintf("Error: %v", err)
			} else {
				a.log.Debug("tool executed", "tool", tc.Name, "result", toolResult, "elapsed", elapsed)
			}

			history = append(history, llm.Message{
				Role:       llm.RoleTool,
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}
	}

	if finalResponse == "" {
		finalResponse = "Ошибка: превышен лимит шагов выполнения."
	}

	// 4. Reflection (optional)
	if a.config.EnableReflection && finalResponse != "" {
		a.log.Debug("reflection step")
		reflectHistory := append(history, llm.Message{
			Role:    llm.RoleUser,
			Content: "Проверь свой последний ответ на ошибки, неточности и полноту. Если всё корректно, верни его без изменений. Если есть ошибки — исправь.",
		})

		reflected, err := a.config.LightModel.Generate(ctx, reflectHistory, nil, nil)
		if err == nil && reflected.Content != "" {
			a.log.Debug("reflection applied", "raw_critique", reflected.Content)
			finalResponse = reflected.Content
			history = append(history, reflected)
		}
	}

	// 5. Experience management
	if a.config.OnTaskComplete != nil && a.config.OnTaskComplete(msg.Text, history) {
		a.log.Info("experience management triggered", "task", msg.Text)
		// Experience Management: Formulate skill instruction
		expHistory := append(history, llm.Message{
			Role:    llm.RoleUser,
			Content: "Сформулируй пошаговую инструкцию по решению этой задачи и сохрани её через save_skill. Имя файла должно быть коротким и описательным. [ВАЖНО]: В финальном ответе пользователю ТОЛЬКО подтверди, что навык сохранен, НЕ дублируй содержимое навыка в чат.",
		})
		resp, err := a.config.HeavyModel.Generate(ctx, expHistory, availableTools, nil)
		if err == nil {
			// Execute any tool calls from the experience save
			for _, tc := range resp.ToolCalls {
				_, _ = a.toolReg.Execute(ctx, tc.Name, msg.TransportName, msg.UserID, tc.Args)
			}
		}
	}

	// 6. Save session
	if err := a.sessions.Save(msg.SessionID, history); err != nil {
		a.log.Error("session save error", "error", err)
	}

	// 7. Send response
	outMsg := msg
	outMsg.Text = finalResponse
	a.log.Debug("outgoing message", "text", finalResponse)
	return a.config.Transport.Write(outMsg)
}

// getSystemPrompt retrieves the prompt from storage or initializes it.
func (a *Agent) getSystemPrompt() (string, error) {
	data, err := a.config.Storage.Read("sysprompt.md")
	if err != nil {
		if err == storage.ErrNotFound {
			if writeErr := a.config.Storage.Write("sysprompt.md", []byte(defaultSysPrompt)); writeErr != nil {
				return "", writeErr
			}
			return defaultSysPrompt, nil
		}
		return "", err
	}
	return string(data), nil
}
