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

func (h *Hub) toolAppendSkillNote(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	note, err := req.RequireString("note")
	if err != nil {
		return toolErr("append_project_skill_note", err)
	}
	scope := strings.TrimSpace(req.GetString("scope", lesson.ScopeSkill))
	if scope == "" {
		scope = lesson.ScopeSkill
	}
	root, _ := h.root()
	skillID := ""
	if scope == lesson.ScopeSkill {
		matched, err := req.RequireString("skill")
		if err != nil {
			return toolErr("append_project_skill_note", err)
		}
		if root == "" {
			return toolErr("append_project_skill_note", fmt.Errorf("no project root"))
		}
		info, _, err := nhskill.LoadBody(root, matched)
		if err != nil {
			return toolErr("append_project_skill_note", err)
		}
		skillID = info.ID
	} else if scope == lesson.ScopeProject && root == "" {
		return toolErr("append_project_skill_note", fmt.Errorf("no project root"))
	}
	taskID, sessKey, parentTask := h.TurnContext()
	if parentTask == "" && strings.HasPrefix(sessKey, "skill-reflect:") {
		parentTask = strings.TrimPrefix(sessKey, "skill-reflect:")
	}
	e, err := lesson.Append(lesson.AppendInput{
		Root:             root,
		Scope:            scope,
		SkillID:          skillID,
		Body:             note,
		SourceTaskID:     taskID,
		ParentTaskID:     parentTask,
		SourceSessionKey: sessKey,
		SupersedesID:     req.GetString("supersedes", ""),
	})
	if err != nil {
		return toolErr("append_project_skill_note", err)
	}
	msg := fmt.Sprintf("appended lesson %s scope=%s", e.ID, e.Scope)
	if e.SkillID != "" {
		msg += " skill=" + e.SkillID
	}
	if e.SourceTaskID != "" {
		msg += " source_task=" + e.SourceTaskID
	}
	msg += "（未认账）"
	if e.ParentTaskID != "" {
		msg += " parent_task=" + e.ParentTaskID
	}
	return mcp.NewToolResultText(msg), nil
}

func (h *Hub) toolSetLessonStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("set_lesson_status", err)
	}
	id, err := req.RequireString("id")
	if err != nil {
		return toolErr("set_lesson_status", err)
	}
	status, err := req.RequireString("status")
	if err != nil {
		return toolErr("set_lesson_status", err)
	}
	if err := lesson.SetStatus(root, id, status); err != nil {
		return toolErr("set_lesson_status", err)
	}
	return mcp.NewToolResultText(fmt.Sprintf("lesson %s status=%s", strings.TrimSpace(id), strings.TrimSpace(status))), nil
}

func (h *Hub) toolListLessons(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, _ := h.root()
	if root != "" {
		_ = lesson.EnsureImported(root)
	}
	skill := strings.TrimSpace(req.GetString("skill", ""))
	scope := strings.TrimSpace(req.GetString("scope", ""))
	var list []lesson.Entry
	var err error
	switch {
	case skill != "":
		if root == "" {
			return toolErr("list_lessons", fmt.Errorf("no project root"))
		}
		info, _, err := nhskill.LoadBody(root, skill)
		if err != nil {
			return toolErr("list_lessons", err)
		}
		list, err = lesson.ListBySkill(root, info.ID)
	case scope != "":
		list, err = lesson.ListByScope(root, scope, 50)
	default:
		if root == "" {
			return toolErr("list_lessons", fmt.Errorf("no project root"))
		}
		list, err = lesson.ListActive(root, nil, 50)
	}
	if err != nil {
		return toolErr("list_lessons", err)
	}
	if list == nil {
		list = []lesson.Entry{}
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (h *Hub) toolAckLesson(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("ack_lesson", err)
	}
	id, err := req.RequireString("id")
	if err != nil {
		return toolErr("ack_lesson", err)
	}
	if err := lesson.Ack(root, id); err != nil {
		return toolErr("ack_lesson", err)
	}
	return mcp.NewToolResultText(fmt.Sprintf("acked lesson %s", strings.TrimSpace(id))), nil
}
