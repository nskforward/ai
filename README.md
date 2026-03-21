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

Добавьте фреймворк и все его зависимости в ваш проект:
```bash
go get github.com/nskforward/ai
```

Экспортируйте обязательные переменные окружения:
```bash
# Токен бота, полученный у @BotFather в Telegram
export TELEGRAM_BOT_TOKEN="ВАШ_TELEGRAM_ТОКЕН"

# Учетные данные GigaChat (из личного кабинета)
export GIGACHAT_CLIENT_ID="ВАШ_CLIENT_ID"
export GIGACHAT_CLIENT_SECRET="ВАШ_CLIENT_SECRET"

# Опционально: Ваш User ID в Telegram (через @userinfobot),
# нужен для выдачи админ-прав (разрешает агенту сохранять опыт).
export TELEGRAM_ADMIN_ID="123456789"
```

### 2. Код `main.go`

Минимальный самодостаточный пример сборки агента с подробными комментариями:

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
	// LocalFS сохраняет Markdown-файлы навыков (решённых задач) прямо на диске
	store, _ := storage.NewLocalFS("agent_data")
	
	// FSSandbox запрещает агенту (LLM) выходить за пределы указанных директорий
	// В данном случае мы разрешаем чтение и запись только в папку "skills"
	fsSandbox := sandbox.NewFSSandbox([]string{"skills"})

	// 2. Драйвер ввода/вывода (Transport)
	// Создаем Telegram Transport для работы в формате бота. 
	// Он сам настроит long polling и маршрутизацию сообщений.
	tg, err := transport.NewTelegram(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("Ошибка Telegram: %v", err)
	}

	// 3. Провайдер LLM (Sber GigaChat)
	// Для GigaChat мы передаём ClientID и ClientSecret; провайдер сам 
	// сгенерирует Base64-токен и будет управлять циклами его обновления (OAuth).
	// DisableSSLVerify: true отключает проверку сертификатов Минцифры,
	// которых по умолчанию нет в системе на Windows/macOS.
	provider := gigachat.NewProvider(gigachat.Config{
		ClientID:         os.Getenv("GIGACHAT_CLIENT_ID"),
		ClientSecret:     os.Getenv("GIGACHAT_CLIENT_SECRET"),
		Model:            "GigaChat", 
		DisableSSLVerify: true,       
	})

	// 4. Инструменты (Tools)
	// Предоставляем агенту список того, что он умеет делать физически:
	// - Вспоминать: чтение ранее сохраненных навыков
	// - Изучать: сохранение нового опыта успешного решения задачи в память.
	// Инструмент SaveSkillTool внутри себя возвращает RequiresAdmin() == true, 
	// поэтому его смогут вызвать только пользователи из белого списка.
	tools := []tool.Tool{
		&tool.ReadFileTool{Store: store, Sandbox: fsSandbox},
		&tool.SaveSkillTool{Store: store, Sandbox: fsSandbox}, 
	}

	// 5. Права доступа (ACL / Admins)
	// Связываем идентификатор пользователя ("UserID" из Telegram) с транспортом.
	adminID := os.Getenv("TELEGRAM_ADMIN_ID")
	if adminID == "" {
		log.Println("Внимание: TELEGRAM_ADMIN_ID не задан. Инструменты администратора (SaveSkillTool) недоступны.")
	}

	// 6. Конфигурация Агента
	// Сборка всех компонентов воедино
	cfg := agent.Config{
		Transport:  tg,             // Как общаемся (Telegram)
		Storage:    store,          // Как храним файлы (FS)
		LightModel: provider,       // Дешёвая модель (для планирования / роутинга)
		HeavyModel: provider,       // Дорогая модель (для выполнения сложных задач)
		Tools:      tools,          // Список доступных инструментов
		AllowedAdmins: []tool.AdminUser{
			{Transport: "telegram", UserID: adminID}, // Белый список
		},
		MaxSteps:   10,             // Предохранитель от бесконечного цикла мыслей
	}

	// 7. Запуск оркестратора
	// Инициализируем агента и блокируем горутину функцией Start, которая
	// будет слушать Telegram и обрабатывать запросы пользователей.
	myAgent := agent.New(cfg)
	log.Println("Бот запущен. Ожидаю сообщения в Telegram...")
	
	if err := myAgent.Start(context.Background()); err != nil {
		log.Fatalf("Работа агента прервана: %v", err)
	}
}
```

Все архитектурные детали описаны в корневом файле [ARCHITECTURE.md](./ARCHITECTURE.md).
