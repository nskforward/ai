package agent

import (
	"context"
	"fmt"

	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/memory"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

const defaultSysPrompt = "Вы - умный AI-Агент. Ваш код состоит из Light и Heavy моделей. Используйте инструмент read_file для получения накопленного опыта (из Оглавления ниже), прежде чем выполнять сложные задачи."

// Config holds the configuration for the Agent.
type Config struct {
	AllowedAdmins []string
}

// Agent is the core framework struct managing the lifecycle.
type Agent struct {
	config     Config
	transport  transport.Transport
	store      storage.Storage
	lightModel llm.Provider
	heavyModel llm.Provider
	memManager *memory.Manager
	toolReg    *tool.Registry
}

// New creates a new Agent instance.
func New(cfg Config, tp transport.Transport, st storage.Storage, light, heavy llm.Provider, tools []tool.Tool) *Agent {
	reg := tool.NewRegistry(cfg.AllowedAdmins)
	for _, t := range tools {
		reg.Register(t)
	}

	return &Agent{
		config:     cfg,
		transport:  tp,
		store:      st,
		lightModel: light,
		heavyModel: heavy,
		memManager: memory.NewManager(st),
		toolReg:    reg,
	}
}

// getSystemPrompt retrieves the prompt from storage or initializes it.
func (a *Agent) getSystemPrompt() (string, error) {
	data, err := a.store.Read("sysprompt.md")
	if err != nil {
		if err == storage.ErrNotFound {
			// Create default sysprompt.md
			err = a.store.Write("sysprompt.md", []byte(defaultSysPrompt))
			if err != nil {
				return "", err
			}
			return defaultSysPrompt, nil
		}
		return "", err
	}
	return string(data), nil
}

// Start begins the agent event loop.
func (a *Agent) Start(ctx context.Context) error {
	for {
		msg, err := a.transport.Read()
		if err != nil {
			return fmt.Errorf("transport read error: %w", err)
		}

		err = a.ProcessMessage(ctx, msg)
		if err != nil {
			_ = a.transport.Write(msg.SessionID, fmt.Sprintf("System Error: %v", err))
		}
	}
}

// ProcessMessage handles a single input request with Dual-Model routing and ReAct execution.
func (a *Agent) ProcessMessage(ctx context.Context, msg transport.Message) error {
	sysPrompt, err := a.getSystemPrompt()
	if err != nil {
		return fmt.Errorf("failed to get system prompt: %w", err)
	}

	toc, err := a.memManager.GenerateTOC()
	if err != nil {
		return fmt.Errorf("failed to generate TOC: %w", err)
	}

	fullSysPrompt := fmt.Sprintf("%s\n\n--- ОГЛАВЛЕНИЕ ПАМЯТИ ---\n%s", sysPrompt, toc)

	history := []llm.Message{
		{Role: llm.RoleSystem, Content: fullSysPrompt},
		{Role: llm.RoleUser, Content: msg.Text},
	}

	availableTools := a.toolReg.GetTools()

	// MAX 10 Steps ReAct loop
	for step := 0; step < 10; step++ {
		// Dual Model Routing: Light model for step 0 (triage), Heavy Model for subsequent tools
		provider := a.lightModel
		if step > 0 {
			provider = a.heavyModel
		}

		resp, err := provider.Generate(ctx, history, availableTools)
		if err != nil {
			return fmt.Errorf("LLM error: %w", err)
		}

		history = append(history, resp)

		// Terminate if model did not call tools
		if len(resp.ToolCalls) == 0 {
			return a.transport.Write(msg.SessionID, resp.Content)
		}

		// Execute tools
		for _, tc := range resp.ToolCalls {
			toolResult, err := a.toolReg.Execute(ctx, tc.Name, msg.UserID, tc.Args)
			if err != nil {
				toolResult = fmt.Sprintf("Error: %v", err)
			}
			history = append(history, llm.Message{
				Role:       llm.RoleTool,
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}
	}

	return a.transport.Write(msg.SessionID, "Error: Max execution steps reached.")
}
