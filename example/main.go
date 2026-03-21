package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/nskforward/ai/agent"
	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/sandbox"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

func main() {
	fmt.Println("Инициализация AI Агента...")

	// 1. Storage
	store, err := storage.NewLocalFS("agent_data")
	if err != nil {
		log.Fatalf("Ошибка создания хранилища: %v", err)
	}

	// Seed a sample skill for TOC
	_ = store.Write("skills/how_to_hello.md", []byte("# Приветствие\nШаг 1: Скажи привет.\nШаг 2: Спроси как дела."))

	// 2. Transport
	console := transport.NewConsole()

	// 3. LLM Providers (Mock)
	light := &llm.MockProvider{Name: "Light"}
	heavy := &llm.MockProvider{Name: "Heavy"}

	// 4. Sandbox
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 5. Tools
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox},
	}

	// 6. Example middleware: log every message
	logMiddleware := func(ctx context.Context, msg transport.Message, next agent.Handler) error {
		fmt.Printf("[middleware] Получено сообщение от %s:%s\n", msg.TransportName, msg.UserID)
		return next(ctx, msg)
	}

	// 7. Config
	cfg := agent.Config{
		Transport:  console,
		Storage:    store,
		LightModel: light,
		HeavyModel: heavy,
		Tools:      tools,
		AllowedAdmins: []tool.AdminUser{
			{Transport: "console", UserID: "admin"},
		},
		Middlewares:      []agent.Middleware{logMiddleware},
		EnableReflection: false, // set true to enable self-check
		MaxSteps:         10,
	}

	myAgent := agent.New(cfg)

	// Graceful shutdown via SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("Агент готов. Команды: 'test read', 'test save'. Ctrl+C для выхода.")

	if err := myAgent.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Критическая ошибка: %v", err)
	}

	fmt.Println("Агент остановлен.")
}
