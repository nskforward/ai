package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nskforward/ai/agent"
	"github.com/nskforward/ai/llm"
	"github.com/nskforward/ai/llm/gigachat"
	"github.com/nskforward/ai/sandbox"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Требуется переменная окружения TELEGRAM_BOT_TOKEN")
	}

	fmt.Println("Инициализация Telegram AI Агента...")

	// 1. Storage
	store, err := storage.NewLocalFS("agent_data")
	if err != nil {
		log.Fatalf("Ошибка создания хранилища: %v", err)
	}

	// 2. Transport (Telegram)
	tg, err := transport.NewTelegram(token)
	if err != nil {
		log.Fatalf("Ошибка Telegram: %v", err)
	}

	// 3. LLM Providers
	var light, heavy llm.Provider
	
	if clientID := os.Getenv("GIGACHAT_CLIENT_ID"); clientID != "" {
		fmt.Println("Используется реальная модель GigaChat")
		
		secret := os.Getenv("GIGACHAT_CLIENT_SECRET")
		model := os.Getenv("GIGACHAT_MODEL")
		if model == "" {
			model = "GigaChat"
		}
		
		light = gigachat.NewProvider(gigachat.Config{
			ClientID:         clientID,
			ClientSecret:     secret,
			Model:            "GigaChat",       // For planning
			DisableSSLVerify: true,             // Bypass TLS for Mintsifra
		})
		
		heavy = gigachat.NewProvider(gigachat.Config{
			ClientID:         clientID,
			ClientSecret:     secret,
			Model:            model,            // For execution
			DisableSSLVerify: true,
		})
	} else {
		fmt.Println("Используются Mock-модели (для разработки без ключей). Задайте GIGACHAT_CLIENT_ID для реального API.")
		light = &llm.MockProvider{Name: "Light"}
		heavy = &llm.MockProvider{Name: "Heavy"}
	}

	// 4. Sandbox
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 5. Tools
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox},
	}

	// Пример настройки администратора по Telegram UserID
	// Укажите свой ID вместо 12345678, чтобы получить права на запись
	adminID := os.Getenv("TELEGRAM_ADMIN_ID")
	if adminID == "" {
		fmt.Println("Внимание: TELEGRAM_ADMIN_ID не задан. Никто не сможет сохранять системные навыки.")
		adminID = "000000"
	}

	// 6. Config
	cfg := agent.Config{
		Transport:  tg,
		Storage:    store,
		LightModel: light,
		HeavyModel: heavy,
		Tools:      tools,
		AllowedAdmins: []tool.AdminUser{
			{Transport: "telegram", UserID: adminID},
		},
		EnableReflection: false,
		MaxSteps:         10,
	}

	myAgent := agent.New(cfg)

	// Graceful shutdown via SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Println("Агент запущен и ожидает сообщений в Telegram. Нажмите Ctrl+C для выхода.")

	if err := myAgent.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Критическая ошибка: %v", err)
	}

	fmt.Println("Агент остановлен.")
}
