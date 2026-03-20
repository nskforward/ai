package main

import (
	"context"
	"fmt"
	"log"

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

	// Создаем тестовую инструкцию для генерации Оглавления (TOC)
	_ = store.Write("skills/how_to_hello.md", []byte("Шаг 1: Скажи привет."))

	// 2. Transport
	console := transport.NewConsole()

	// 3. Providers (Mock)
	light := &llm.MockProvider{Name: "Light-Model"}
	heavy := &llm.MockProvider{Name: "Heavy-Model"}

	// 4. Sandbox
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 5. Tools
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox},
	}

	// 6. Config
	cfg := agent.Config{
		AllowedAdmins: []string{"admin"}, // Пользователь консоли по умолчанию
	}

	// 7. Core
	myAgent := agent.New(cfg, console, store, light, heavy, tools)

	fmt.Println("Агент готов. Для проверки можете написать 'test read' или 'test save'. Нажмите Ctrl+C для выхода.")

	ctx := context.Background()
	if err := myAgent.Start(ctx); err != nil {
		log.Fatalf("Критическая ошибка: %v", err)
	}
}
