# AI Agent Framework

Универсальный, легко расширяемый каркас (фреймворк) на языке Golang для создания автономных AI-агентов. Ваша архитектура агентов может работать локально, накапливать опыт и адаптироваться к незнакомым задачам.

## Ключевые возможности

- **Dual-Model Routing**: Автоматическое делегирование (маршрутизация). «Легкие» LLM (Light) используются для планирования и ответов на простые вопросы, а «тяжелые» (Heavy) — для сложной генерации кода и аналитики.
- **Orchestrator & Sub-agents**: Способность разбивать сложные задачи на пошаговый план (`Plan`) и запускать изолированных субагентов для каждого шага. Это значительно экономит контекст и токены.
- **Самообучение (Memory)**: Агенты сохраняют решения сложных задач в виде Markdown-файлов на диск. Динамическое "Оглавление" (TOC) вшивается в системный промпт, чтобы агент вспомнил опыт при встрече с похожей задачей.
- **Инструменты и безопасности (Tools & Sandbox)**: Строгий ACL, валидация путей в файловой песочнице и сетевой доступ.
- **Plug-and-play Транспорты**: Включает встроенные транспорты (консоль, Telegram-бот). Легко дописать транспорт под Slack, Discord и др.
- **GigaChat API (Out of the box)**: Нативная поддержка Сбер GigaChat с Function Calling, автоматическим обновлением Access Token (OAuth) и поддержкой Structured Output.

---

## Быстрый старт (End-to-End: Telegram + GigaChat)

Пример ниже демонстрирует создание AI-бота в Telegram, который "думает" на базе GigaChat и сохраняет свой "мозг" (навыки) локально в папке `agent_data`.

### 1. Подготовка

Убедитесь, что вы скачали зависимость для транспорта Telegram:
```bash
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
```

Экспортируйте обязательные переменные окружения:
```bash
# Токен бота, полученный у @BotFather в Telegram
export TELEGRAM_BOT_TOKEN="ВАШ_TELEGRAM_ТОКЕН"

# Ключ авторизации GigaChat (base64 строка ClientId:ClientSecret)
export GIGACHAT_AUTH_KEY="ВАШ_GIGACHAT_КЛЮЧ"
```

### 2. Код `main.go`

Минимальный самодостаточный пример сборки агента:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/nskforward/ai/agent"
	"github.com/nskforward/ai/llm/gigachat"
	"github.com/nskforward/ai/sandbox"
	"github.com/nskforward/ai/storage"
	"github.com/nskforward/ai/tool"
	"github.com/nskforward/ai/transport"
)

func main() {
	// 1. Хранилище памяти и Песочница
	store, _ := storage.NewLocalFS("agent_data")
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 2. Драйвер ввода/вывода (Telegram bBot)
	tg, err := transport.NewTelegram(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Ошибка Telegram: %v", err)
	}

	// 3. Провайдер LLM (Sber GigaChat)
	// Insecure: true используется для обхода проверки сертификатов Минцифры на локальных машинах.
	authKey := os.Getenv("GIGACHAT_AUTH_KEY")
	provider := gigachat.NewProvider(gigachat.Config{
		AuthKey:  authKey,
		Model:    "GigaChat", 
		Insecure: true,       
	})

	// 4. Инструменты (чтение файлов памяти и запись нового опыта)
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox}, // Требует Admin права для использования
	}

	// 5. Конфигурация Агента
	cfg := agent.Config{
		Transport:  tg,             // Транспорт
		Storage:    store,          // Файловая система
		LightModel: provider,       // Модель для планирования и triage
		HeavyModel: provider,       // Модель для исполнения сложных задач
		Tools:      tools,          // Список доступных инструментов
		MaxSteps:   10,             // Защита от бесконечного цикла ReAct
	}

	// 6. Запуск оркестратора
	myAgent := agent.New(cfg)
	log.Println("Бот запущен. Ожидаю сообщения в Telegram...")
	
	if err := myAgent.Start(context.Background()); err != nil {
		log.Fatalf("Работа агента прервана: %v", err)
	}
}
```

Все архитектурные детали описаны в корневом файле [ARCHITECTURE.md](./ARCHITECTURE.md).
