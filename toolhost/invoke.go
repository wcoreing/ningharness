package toolhost

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *ToolHost) CallNamedTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if h == nil {
		return nil, fmt.Errorf("hub nil")
	}
	name = strings.TrimSpace(name)
	if err := h.checkToolInterceptor(name); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	args = FlattenArguments(args)
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	var (
		res *mcp.CallToolResult
		err error
	)
	switch name {
	case "list_tree":
		res, err = h.toolListTree(ctx, req)
	case "search_session":
		res, err = h.toolSearchSession(ctx, req)
	case "recall_resource":
		res, err = h.toolRecallResource(ctx, req)
	case "read_file":
		res, err = h.toolReadFile(ctx, req)
	case "grep":
		res, err = h.toolGrep(ctx, req)
	case "write_file":
		res, err = h.toolWriteFile(ctx, req)
	case "edit":
		res, err = h.toolEdit(ctx, req)
	case "mkdir":
		res, err = h.toolMkdir(ctx, req)
	case "create_file":
		res, err = h.toolCreateFile(ctx, req)
	case "rename_path":
		res, err = h.toolRenamePath(ctx, req)
	case "move_path":
		res, err = h.toolMovePath(ctx, req)
	case "copy_path":
		res, err = h.toolCopyPath(ctx, req)
	case "delete_path":
		res, err = h.toolDeletePath(ctx, req)
	case "batch_delete_paths":
		res, err = h.toolBatchDeletePaths(ctx, req)
	case "batch_move_paths":
		res, err = h.toolBatchMovePaths(ctx, req)
	case "append_session_message":
		res, err = h.toolAppendSession(ctx, req)
	case "get_session_brief":
		res, err = h.toolSessionBrief(ctx, req)
	case "list_skills":
		res, err = h.toolListSkills(ctx, req)
	case "get_skill":
		res, err = h.toolGetSkill(ctx, req)
	case "create_project_skill":
		res, err = h.toolCreateSkill(ctx, req)
	case "append_project_skill_note":
		res, err = h.toolAppendSkillNote(ctx, req)
	case "set_lesson_status":
		res, err = h.toolSetLessonStatus(ctx, req)
	case "list_lessons":
		res, err = h.toolListLessons(ctx, req)
	case "ack_lesson":
		res, err = h.toolAckLesson(ctx, req)
	case "list_tasks":
		res, err = h.toolListTasks(ctx, req)
	case "get_task_summary":
		res, err = h.toolGetTaskSummary(ctx, req)
	case "enqueue_agent_turn":
		res, err = h.toolEnqueueAgentTurn(ctx, req)
	case "enqueue_agent_turns_for_paths":
		res, err = h.toolEnqueueAgentTurnsForPaths(ctx, req)
	case "list_queue":
		res, err = h.toolListQueue(ctx, req)
	case "cancel_queue_task":
		res, err = h.toolCancelQueueTask(ctx, req)
	case "set_queue_paused":
		res, err = h.toolSetQueuePaused(ctx, req)
	default:
		return nil, fmt.Errorf("unknown tool %s", name)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (h *ToolHost) Invoke(ctx context.Context, name, argsJSON string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("hub nil")
	}
	name = strings.TrimSpace(name)
	args := map[string]any{}
	if s := strings.TrimSpace(argsJSON); s != "" && s != "null" {
		s = UnwrapArgumentsJSON(s)
		if name == "write_file" {
			path, content, err := ParseWriteFile(s)
			if err != nil {
				return "", err
			}
			args["rel_path"] = path
			args["content"] = content
		} else if err := json.Unmarshal([]byte(s), &args); err != nil {
			return "", fmt.Errorf("args: %w", err)
		}
	}
	res, err := h.CallNamedTool(ctx, name, args)
	if err != nil {
		return "", err
	}
	return FormatToolResult(res), nil
}

func FormatToolResult(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		switch t := c.(type) {
		case mcp.TextContent:
			b.WriteString(t.Text)
		default:
			raw, _ := json.Marshal(c)
			b.Write(raw)
		}
	}
	out := strings.TrimSpace(b.String())
	if res.IsError {
		if out == "" {
			out = "tool error"
		}
		return "error: " + out
	}
	return out
}
