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
func (t *ReadFileTool) Description() string { return "Reads the content of a file. Use this to read files from the skills/ folder listed in the TOC." }
func (t *ReadFileTool) Schema() string {
	return `{"type":"object","properties":{"path":{"type":"string","description":"Relative path to the file, e.g. skills/how_to_deploy.md"}},"required":["path"]}`
}
func (t *ReadFileTool) RequiresAdmin() bool { return false }

func (t *ReadFileTool) Execute(ctx context.Context, userID string, args string) (string, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	if t.Sandbox != nil {
		if err := t.Sandbox.CheckRead(input.Path); err != nil {
			return "", err
		}
	}

	data, err := t.Store.Read(input.Path)
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
func (t *SaveSkillTool) Description() string { return "Saves a detailed step-by-step instruction on how to solve a task into a markdown file." }
func (t *SaveSkillTool) Schema() string {
	return `{"type":"object","properties":{"filename":{"type":"string","description":"Filename to save, e.g. deploy_to_aws.md"},"content":{"type":"string","description":"The markdown content with instructions"}},"required":["filename","content"]}`
}
// Writing new core skills requires admin invocation of the agent session.
func (t *SaveSkillTool) RequiresAdmin() bool { return true }

func (t *SaveSkillTool) Execute(ctx context.Context, userID string, args string) (string, error) {
	var input struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	path := filepath.Join("skills", input.Filename)

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
