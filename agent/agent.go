package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/logger"
	"github.com/nskforward/ai/memory"
	"github.com/nskforward/ai/session"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

const defaultSysPrompt = `Вы — AI-Агент. Перед выполнением задачи проверьте Оглавление памяти ниже.
Если есть подходящий навык — прочитайте файл через read_file.
Если задача сложная и незнакомая — составьте план, выполните его, затем сохраните опыт через save_skill.`

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

		// Build middleware chain
		handler := a.processMessage
		for i := len(a.config.Middlewares) - 1; i >= 0; i-- {
			mw := a.config.Middlewares[i]
			next := handler
			handler = func(ctx context.Context, msg transport.Message) error {
				return mw(ctx, msg, next)
			}
		}

		if err := handler(ctx, msg); err != nil {
			a.log.Error("processing error", "error", err, "session", msg.SessionID)
			errMsg := msg
			errMsg.Text = fmt.Sprintf("Ошибка: %v", err)
			_ = a.config.Transport.Write(errMsg)
		}
	}
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

	availableTools := a.toolReg.GetTools()

	// 2.5 Planning Phase (Orchestrator)
	planPrompt := append(history, llm.Message{
		Role: llm.RoleSystem,
		Content: "Оцени задачу пользователя. Если она простая — верни is_complex: false. Если сложная — верни is_complex: true и распиши шаги (steps).",
	})

	a.log.Debug("orchestrator: requesting plan")
	planMsg, err := a.config.LightModel.Generate(ctx, planPrompt, nil, &llm.GenerateOptions{ResponseFormat: PlanSchema})
	if err != nil {
		a.log.Error("planning phase error", "error", err)
	}

	var plan Plan
	if err == nil {
		if err := json.Unmarshal([]byte(planMsg.Content), &plan); err == nil && plan.IsComplex {
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
					a.log.Debug("subagent completed", "step", step.ID)
					aggregatedContext += fmt.Sprintf("\nШаг %d результат:\n%s\n", step.ID, res)
				}
			}

			// Inject aggregated context into the main history so Heavy model can finalize it
			history = append(history, llm.Message{
				Role: llm.RoleSystem,
				Content: fmt.Sprintf("Субагенты выполнили задачу. Их результаты:\n%s\nСформируй финальный ответ пользователю на основе этих результатов.", aggregatedContext),
			})
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
			start := time.Now()
			toolResult, err := a.toolReg.Execute(ctx, tc.Name, msg.TransportName, msg.UserID, tc.Args)
			elapsed := time.Since(start)

			if err != nil {
				a.log.Error("tool error", "tool", tc.Name, "error", err, "elapsed", elapsed)
				toolResult = fmt.Sprintf("Error: %v", err)
			} else {
				a.log.Debug("tool executed", "tool", tc.Name, "elapsed", elapsed)
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
			a.log.Debug("reflection applied")
			finalResponse = reflected.Content
			history = append(history, reflected)
		}
	}

	// 5. Experience management
	if a.config.OnTaskComplete != nil && a.config.OnTaskComplete(msg.Text, history) {
		a.log.Info("experience management triggered", "task", msg.Text)
		// The callback returned true — ask Heavy Model to formulate a skill
		expHistory := append(history, llm.Message{
			Role:    llm.RoleUser,
			Content: "Сформулируй пошаговую инструкцию по решению этой задачи и сохрани её через save_skill. Имя файла должно быть коротким и описательным.",
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
