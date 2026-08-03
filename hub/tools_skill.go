package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	lesson "ningharness/lesson"
	nhskill "ningharness/skill"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Hub) toolListSkills(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("list_skills", err)
	}
	_ = lesson.EnsureImported(root)
	list, err := nhskill.ListEnabled(root)
	if err != nil {
		return toolErr("list_skills", err)
	}
	if list == nil {
		list = []nhskill.Info{}
	}
	for i := range list {
		list[i].HasLessons = lesson.HasAny(root, list[i].ID)
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (h *Hub) toolGetSkill(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("get_skill", err)
	}
	matched, err := req.RequireString("skill")
	if err != nil {
		return toolErr("get_skill", err)
	}
	_ = lesson.EnsureImported(root)
	info, body, err := nhskill.LoadBody(root, matched)
	if err != nil {
		return toolErr("get_skill", err)
	}
	if !info.Enabled {
		return toolErr("get_skill", fmt.Errorf("skill %s 已停用（frontmatter enabled: false）；人在项目菜单启用后再加载", info.ID))
	}
	entries, _ := lesson.ListBySkill(root, info.ID)
	body = strings.TrimSpace(body) + lesson.FormatForSkillBody(entries)
	text := fmt.Sprintf("# skill %s (%s)\n\n%s", info.ID, info.Name, strings.TrimSpace(body))
	return mcp.NewToolResultText(text), nil
}

func (h *Hub) toolCreateSkill(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("create_project_skill", err)
	}
	id, err := req.RequireString("id")
	if err != nil {
		return toolErr("create_project_skill", err)
	}
	name := req.GetString("name", "")
	desc := req.GetString("description", "")
	content := req.GetString("content", "")
	rel, doc, err := nhskill.RenderNew(id, name, desc, content)
	if err != nil {
		return toolErr("create_project_skill", err)
	}
	if h.pathExistsForAgent(rel) {
		return toolErr("create_project_skill", fmt.Errorf("skill already exists: %s", id))
	}
	writeID := h.mcpWriteID("mcp-skill")
	if err := h.writeAgentFile(rel, doc, writeID); err != nil {
		return toolErr("create_project_skill", err)
	}
	msg := fmt.Sprintf("created %s (writeId=%s) · 未文件打点 · 下一步 get_skill %s 自检；作者规范 get_skill skill-author；落盘用 write_file/edit，启用靠人 pin", rel, writeID, id)
	return mcp.NewToolResultText(msg), nil
}
