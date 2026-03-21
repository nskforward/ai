package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nskforward/ai/agent"
	"github.com/nskforward/ai/llm/gigachat"
	"github.com/nskforward/ai/logger"
	"github.com/nskforward/ai/sandbox"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

func main() {
	// Подгружаем переменные окружения из файла .env (он должен лежать рядом либо в корне)
	if err := godotenv.Load(".env"); err != nil {
		log.Println("Файл .env не найден. Будут использованы системные переменные окружения.")
	}

	// 1. Хранилище памяти и Песочница
	// LocalFS сохраняет Markdown-файлы навыков непосредственно в эту директорию.
	store, _ := storage.NewLocalFS("agent_data")

	// FSSandbox запрещает агенту (LLM) случайное или умышленное чтение/запись вне 'skills'
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 2. Драйвер ввода/вывода (Transport)
	tg, err := transport.NewTelegram(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Ошибка Telegram: %v", err)
	}

	// 3. Провайдеры LLM (Sber GigaChat)
	// Для GigaChat мы передаём Authorization Key; провайдер сам автообновляет Access Token'ы.
	// DisableSSLVerify: true используется для обхода проверки сертификатов Минцифры.
	lightProvider := gigachat.NewProvider(gigachat.Config{
		AuthKey:          os.Getenv("GIGACHAT_AUTH_KEY"),
		Model:            "GigaChat-2", // Дешёвая и быстрая модель для планирования
		DisableSSLVerify: true,
	})

	heavyProvider := gigachat.NewProvider(gigachat.Config{
		AuthKey:          os.Getenv("GIGACHAT_AUTH_KEY"),
		Model:            "GigaChat-2-Pro", // Дорогая умная модель для реализации
		DisableSSLVerify: true,
	})

	// 4. Инструменты (Tools)
	// Доступ агента к физическому миру (чтение опыта, запись навыков с подтверждением, доступ в интернет с одобрением)
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox}, // Требует !allow_save
		&tool.HttpGetTool{Whitelist: store},                   // Требует !approve <URL>
	}

	// 5. Права доступа (ACL / Admins)
	// Разрешаем сохранять навыки только конкретному администратору Telegram.
	adminID := os.Getenv("TELEGRAM_ADMIN_ID")
	if adminID == "" || adminID == "12345678" || adminID == "ВАШ_ID" {
		log.Println("Внимание: TELEGRAM_ADMIN_ID не задан или недействителен. Инструменты администратора (SaveSkillTool) недоступны.")
	}

	// 6. Конфигурация Агента
	// Добавим расширенное логирование уровня Debug, чтобы видеть скрытые мысли и аргументы инструментов
	cfg := agent.Config{
		Transport:  tg,
		Storage:    store,
		LightModel: lightProvider,
		HeavyModel: heavyProvider,
		Tools:      tools,
		Logger:     logger.NewSlogDebug(), // Подробнейшие логи в консоль
		AllowedAdmins: []tool.AdminUser{
			{Transport: "telegram", UserID: adminID},
		},
		MaxSteps: 10,
	}

	// 7. Запуск оркестратора
	myAgent := agent.New(cfg)
	log.Println("Все зависимости запущены. Ожидаю сообщения в Telegram...")

	if err := myAgent.Start(context.Background()); err != nil {
		log.Fatalf("Работа агента прервана: %v", err)
	}
}
