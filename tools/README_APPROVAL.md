# Tool Approval System

Система подтверждений позволяет контролировать вызовы инструментов с различными уровнями безопасности.

## Overview

Система подтверждений включает:

- **ApprovalManager** - менеджер подтверждений
- **Approver** - интерфейс обработчика подтверждений
- **ApprovalRequest/ApprovalResponse** - структуры запросов

## Approval Policies

### AutoApprove
Автоматическое подтверждение без запроса. Используется для безопасных операций.

```go
tool := &MyTool{}
tool.SetApprovalPolicy(tools.AutoApprove)
```

### RequireApproval
Требует подтверждения от пользователя. Используется для операций среднего уровня безопасности.

```go
tool := &HTTPGetTool{}
tool.SetApprovalPolicy(tools.RequireApproval)
```

### RequireAdminApproval
Требует подтверждения от администратора. Используется для опасных операций.

```go
tool := &CLIExecTool{}
tool.SetApprovalPolicy(tools.RequireAdminApproval)
```

### Deny
Запрещает использование инструмента.

```go
tool := &DangerousTool{}
tool.SetApprovalPolicy(tools.Deny)
```

## Built-in Approvers

### AutoApprover
Автоматически подтверждает все запросы. Используется для тестов.

```go
approvalManager.RegisterApprover(tools.RequireApproval, tools.NewAutoApprover())
```

### ConsoleApprover
Запрашивает подтверждение через консоль.

```go
approvalManager.RegisterApprover(tools.RequireApproval, tools.NewConsoleApprover(
    func(req *tools.ApprovalRequest) bool {
        fmt.Printf("Tool: %s\n", req.ToolName)
        fmt.Printf("Params: %v\n", req.Params)
        fmt.Printf("Approve? (y/n): ")
        
        var response string
        fmt.Scanln(&response)
        return response == "y" || response == "Y"
    },
))
```

### AdminApprover
Проверяет права администратора и запрашивает подтверждение.

```go
approvalManager.RegisterApprover(tools.RequireAdminApproval, tools.NewAdminApprover(
    func(agentCtx *transport.AgentContext) bool {
        return agentCtx.IsAdmin
    },
    func(req *tools.ApprovalRequest) bool {
        fmt.Printf("Admin approval required for: %s\n", req.ToolName)
        // Admin approval logic
        return true
    },
))
```

## Usage Example

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/nskforward/ai/tools"
    "github.com/nskforward/ai/tools/built-in"
    "github.com/nskforward/ai/transport"
)

func main() {
    // Create tool manager
    tm := tools.NewToolManager()
    ctx := context.Background()
    
    // Create agent context
    agentCtx := &transport.AgentContext{
        UserID:    "user123",
        SessionID: "session456",
        IsAdmin:   false,
    }
    
    // Register tools
    httpGetTool := builtin.NewHTTPGetTool()
    tm.Register(httpGetTool)
    
    // Configure approval system
    approvalManager := tm.GetApprovalManager()
    
    // Register console approver for RequireApproval policy
    approvalManager.RegisterApprover(tools.RequireApproval, tools.NewConsoleApprover(
        func(req *tools.ApprovalRequest) bool {
            fmt.Printf("Approve tool %s? (y/n): ", req.ToolName)
            var response string
            fmt.Scanln(&response)
            return response == "y"
        },
    ))
    
    // Execute tool - will prompt for approval
    result, err := tm.Execute(ctx, agentCtx, "http_get", map[string]interface{}{
        "url": "https://api.github.com",
    })
    
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Printf("Success: %v\n", result.Success)
    }
}
```

## Custom Approver

Создайте свой обработчик подтверждений:

```go
type CustomApprover struct {
    // Custom fields
}

func (ca *CustomApprover) Approve(ctx context.Context, req *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
    // Custom approval logic
    // - Check user permissions
    // - Validate parameters
    // - Log request
    // - Return approval decision
    
    approved := true // Your logic here
    
    return &tools.ApprovalResponse{
        RequestID:   req.ID,
        Approved:    approved,
        RespondedBy: "custom-approver",
    }, nil
}

// Register custom approver
approvalManager.RegisterApprover(tools.RequireApproval, &CustomApprover{})
```

## Approval History

Получение истории подтверждений:

```go
history := approvalManager.GetHistory()
for _, req := range history {
    fmt.Printf("Tool: %s, Status: %s, User: %s\n",
        req.ToolName, req.Status, req.AgentContext.UserID)
}
```

## Pending Requests

Получение ожидающих подтверждения запросов:

```go
pending := approvalManager.GetPendingRequests()
for _, req := range pending {
    fmt.Printf("Pending: %s (ID: %s)\n", req.ToolName, req.ID)
}