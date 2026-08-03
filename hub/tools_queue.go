package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	deskqueue "ningharness/job"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Hub) toolEnqueueAgentTurn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("enqueue_agent_turn", err)
	}
	prompt, err := req.RequireString("prompt")
	if err != nil {
		return toolErr("enqueue_agent_turn", err)
	}
	driver := req.GetString("driver", "")
	title := req.GetString("title", "")
	target := req.GetString("target_rel", "")
	// session: 空/active → 侧栏当前会话（对话栏可见，与练笔/章节入队一致）；
	// isolated → once:queue:{id}（隐藏，并行批不串台）。
	mode := strings.TrimSpace(strings.ToLower(req.GetString("session", "")))
	var task deskqueue.Job
	switch mode {
	case "isolated", "once", "queue":
		task, err = q.EnqueueIsolated(prompt, driver, title, target)
	default:
		task, err = q.EnqueueSession(prompt, driver, title, target, h.activeSessionKey(), "", "")
	}
	if err != nil {
		return toolErr("enqueue_agent_turn", err)
	}
	h.AnnounceQueueEnqueued(task)
	return mcp.NewToolResultText(deskqueue.FormatEnqueueOK(task)), nil
}

func (h *Hub) toolEnqueueAgentTurnsForPaths(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("enqueue_agent_turns_for_paths", err)
	}
	rawPaths, err := req.RequireStringSlice("rel_paths")
	if err != nil {
		return toolErr("enqueue_agent_turns_for_paths", err)
	}
	tasks, err := q.EnqueuePaths(rawPaths, req.GetString("prompt_template", ""), req.GetString("driver", ""), "", "")
	if err != nil {
		return toolErr("enqueue_agent_turns_for_paths", err)
	}
	if len(tasks) == 0 {
		return toolErr("enqueue_agent_turns_for_paths", fmt.Errorf("no tasks"))
	}
	for _, t := range tasks {
		h.AnnounceQueueEnqueued(t)
	}
	msg := deskqueue.FormatEnqueueOK(tasks[0])
	if len(tasks) > 1 {
		msg = fmt.Sprintf("%s · batchJobs=%d", msg, len(tasks))
	}
	return mcp.NewToolResultText(msg), nil
}

func (h *Hub) toolListQueue(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("list_queue", err)
	}
	raw, _ := json.MarshalIndent(deskqueue.FormatAgentSnapshot(q.Snapshot()), "", "  ")
	return mcp.NewToolResultText(string(raw)), nil
}

func (h *Hub) toolCancelQueueTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("cancel_queue_task", err)
	}
	id, err := req.RequireString("task_id")
	if err != nil {
		return toolErr("cancel_queue_task", err)
	}
	if err := q.Cancel(id); err != nil {
		return toolErr("cancel_queue_task", err)
	}
	return mcp.NewToolResultText("cancelled " + strings.TrimSpace(id)), nil
}

func (h *Hub) toolSetQueuePaused(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("set_queue_paused", err)
	}
	paused := req.GetBool("paused", true)
	if err := q.SetPaused(paused); err != nil {
		return toolErr("set_queue_paused", err)
	}
	return mcp.NewToolResultText(fmt.Sprintf("paused=%v", paused)), nil
}

func (h *Hub) requireQueue() (*deskqueue.Manager, error) {
	if h == nil || h.Queue == nil {
		return nil, fmt.Errorf("queue requires AgentDesk App（执行器未挂载）")
	}
	root, err := h.root()
	if err != nil {
		return nil, err
	}
	h.Queue.Bind(root)
	return h.Queue, nil
}
