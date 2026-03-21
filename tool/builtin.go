package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/nskforward/ai/sandbox"
	"github.com/nskforward/ai/storage"
)

// ReadFileTool allows the agent to read local files, specifically skills or experiences.
type ReadFileTool struct {
	Store   storage.Storage
	Sandbox sandbox.FSSandbox
}

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string { return "Reads the content of a file. Use this to read skills listed in the TOC." }
func (t *ReadFileTool) Schema() string {
	return `{"type":"object","properties":{"filename":{"type":"string","description":"Exact filename from the TOC, e.g. deploy_to_aws.md"}},"required":["filename"]}`
}
func (t *ReadFileTool) RequiresAdmin() bool { return false }

func (t *ReadFileTool) Execute(ctx context.Context, transportName string, userID string, args string) (string, error) {
	var input struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	// Safe path construction (prevents LLM from prepending "skills/")
	cleanName := filepath.Base(input.Filename)
	path := filepath.Join("skills", cleanName)

	if t.Sandbox != nil {
		if err := t.Sandbox.CheckRead(path); err != nil {
			return "", err
		}
	}

	data, err := t.Store.Read(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveSkillTool allows the agent to write new experiences into the directory.
type SaveSkillTool struct {
	Store   storage.Storage
	Sandbox sandbox.FSSandbox
}

func (t *SaveSkillTool) Name() string { return "save_skill" }
func (t *SaveSkillTool) Description() string {
	return "Saves a detailed step-by-step instruction on how to solve a task into a markdown file. REQUIRES MANUAL PERMISSION (e.g. 'ok', 'разрешаю') from administrator for each call."
}
func (t *SaveSkillTool) Schema() string {
	return `{"type":"object","properties":{"filename":{"type":"string","description":"Filename to save, e.g. deploy_to_aws.md"},"content":{"type":"string","description":"REQUIRED: detailed, comprehensive step-by-step instruction on how to solve the task. DO NOT USE PLACEHOLDERS. Write actual real steps."}},"required":["filename","content"]}`
}
// Writing new core skills requires admin invocation of the agent session.
func (t *SaveSkillTool) RequiresAdmin() bool { return true }

func (t *SaveSkillTool) Execute(ctx context.Context, transportName string, userID string, args string) (string, error) {
	var input struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	// Hard manual permission check
	sessionID, _ := ctx.Value("sessionID").(string)
	permPath := "permissions/" + sessionID + "/save_skill"
	if _, err := t.Store.Read(permPath); err != nil {
		if err == storage.ErrNotFound {
			return "ACCESS DENIED: Manual save permission required. Ask the administrator for permission (e.g. say 'ok', 'да' or 'разрешаю').", nil
		}
		return "", err
	}
	// Consume the permission
	_ = t.Store.Delete(permPath)

	// Safe path construction (prevents LLM from prepending "skills/")
	cleanName := filepath.Base(input.Filename)
	path := filepath.Join("skills", cleanName)

	if t.Sandbox != nil {
		if err := t.Sandbox.CheckWrite(path); err != nil {
			return "", err
		}
	}

	if err := t.Store.Write(path, []byte(input.Content)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully saved skill to %s", path), nil
}
