package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nskforward/ai/tools"
	builtin "github.com/nskforward/ai/tools/built-in"
	"github.com/nskforward/ai/transport"
)

func main() {
	// Создаём менеджер инструментов
	tm := tools.NewToolManager()
	ctx := context.Background()

	// Создаём контекст агента
	agentCtx := &transport.AgentContext{
		UserID:        "user123",
		SessionID:     "session456",
		TransportName: "console",
		IsAdmin:       false,
		UserName:      "john_doe",
	}

	// Регистрируем инструменты
	httpGetTool := builtin.NewHTTPGetTool()
	folderListTool := builtin.NewFolderListTool()

	if err := tm.Register(httpGetTool); err != nil {
		log.Fatal(err)
	}
	if err := tm.Register(folderListTool); err != nil {
		log.Fatal(err)
	}

	// Настраиваем систему подтверждений
	approvalManager := tm.GetApprovalManager()

	// Вариант 1: Автоматическое подтверждение (для тестов)
	approvalManager.RegisterApprover(tools.RequireApproval, tools.NewAutoApprover())

	// Вариант 2: Подтверждение через консоль
	approvalManager.RegisterApprover(tools.RequireApproval, tools.NewConsoleApprover(
		func(req *tools.ApprovalRequest) bool {
			fmt.Printf("\n=== APPROVAL REQUEST ===\n")
			fmt.Printf("Tool: %s\n", req.ToolName)
			fmt.Printf("Description: %s\n", req.ToolDescription)
			fmt.Printf("Params: %v\n", req.Params)
			fmt.Printf("User: %s\n", req.AgentContext.UserID)
			fmt.Printf("Approve? (y/n): ")

			var response string
			fmt.Scanln(&response)
			return response == "y" || response == "Y"
		},
	))

	// Вариант 3: Подтверждение только для администраторов
	approvalManager.RegisterApprover(tools.RequireAdminApproval, tools.NewAdminApprover(
		func(agentCtx *transport.AgentContext) bool {
			return agentCtx.IsAdmin
		},
		func(req *tools.ApprovalRequest) bool {
			fmt.Printf("\n=== ADMIN APPROVAL REQUIRED ===\n")
			fmt.Printf("Tool: %s\n", req.ToolName)
			fmt.Printf("Admin approval required. Approve? (y/n): ")

			var response string
			fmt.Scanln(&response)
			return response == "y" || response == "Y"
		},
	))

	// Пример 1: Выполнение инструмента с AutoApprove (folder_list)
	fmt.Println("\n=== Example 1: AutoApprove (folder_list) ===")
	result, err := tm.Execute(ctx, agentCtx, "folder_list", map[string]interface{}{
		"path": ".",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Output: %v\n", result.Output)
	}

	// Пример 2: Выполнение инструмента с RequireApproval (http_get)
	fmt.Println("\n=== Example 2: RequireApproval (http_get) ===")
	result, err = tm.Execute(ctx, agentCtx, "http_get", map[string]interface{}{
		"url": "https://api.github.com",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		if result.Success {
			fmt.Printf("Status Code: %v\n", result.Output.(map[string]interface{})["status_code"])
		}
	}

	// Пример 3: Попытка выполнить инструмент без прав администратора
	fmt.Println("\n=== Example 3: RequireAdminApproval (cli_exec) - Non-admin ===")
	cliExecTool := builtin.NewCLIExecTool()
	if err := tm.Register(cliExecTool); err != nil {
		log.Fatal(err)
	}

	result, err = tm.Execute(ctx, agentCtx, "cli_exec", map[string]interface{}{
		"command": "ls -la",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		fmt.Printf("Error: %s\n", result.Error)
	}

	// Пример 4: Выполнение инструмента с правами администратора
	fmt.Println("\n=== Example 4: RequireAdminApproval (cli_exec) - Admin ===")
	adminCtx := &transport.AgentContext{
		UserID:        "admin1",
		SessionID:     "session789",
		TransportName: "console",
		IsAdmin:       true,
		UserName:      "admin",
	}

	result, err = tm.Execute(ctx, adminCtx, "cli_exec", map[string]interface{}{
		"command": "echo 'Hello from admin'",
	})
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Success: %v\n", result.Success)
		if result.Success {
			fmt.Printf("Output: %v\n", result.Output)
		}
	}

	// Просмотр истории подтверждений
	fmt.Println("\n=== Approval History ===")
	history := approvalManager.GetHistory()
	for i, req := range history {
		fmt.Printf("%d. Tool: %s, Status: %s, User: %s\n",
			i+1, req.ToolName, req.Status, req.AgentContext.UserID)
	}
}
