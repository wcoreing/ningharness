package toolgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	deskqueue "ningharness/job"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) toolEnqueueAgentTurn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func (h *Gateway) toolEnqueueGoal(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("enqueue_goal", err)
	}
	objective, err := req.RequireString("objective")
	if err != nil {
		return toolErr("enqueue_goal", err)
	}
	maxRounds := int(req.GetFloat("max_rounds", 0))
	mode := strings.TrimSpace(strings.ToLower(req.GetString("session", "")))
	sessionKey := ""
	switch mode {
	case "isolated", "once", "queue":
		sessionKey = ""
	case "", "active":
		sessionKey = h.activeSessionKey()
	default:
		sessionKey = h.activeSessionKey()
	}
	task, err := q.EnqueueGoal(deskqueue.GoalEnqueue{
		Objective:  objective,
		Driver:     req.GetString("driver", ""),
		Title:      req.GetString("title", ""),
		SessionKey: sessionKey,
		MaxRounds:  maxRounds,
	})
	if err != nil {
		return toolErr("enqueue_goal", err)
	}
	h.AnnounceQueueEnqueued(task)
	return mcp.NewToolResultText(deskqueue.FormatEnqueueOK(task)), nil
}

func (h *Gateway) toolEnqueueAgentTurnsForPaths(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func (h *Gateway) toolListQueue(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("list_queue", err)
	}
	raw, _ := json.MarshalIndent(deskqueue.FormatAgentSnapshot(q.Snapshot()), "", "  ")
	return mcp.NewToolResultText(string(raw)), nil
}

func (h *Gateway) toolCancelQueueTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func (h *Gateway) toolSteerQueueTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("steer_queue_task", err)
	}
	text, err := req.RequireString("text")
	if err != nil {
		return toolErr("steer_queue_task", err)
	}
	jobID := strings.TrimSpace(req.GetString("task_id", ""))
	job, err := q.Steer(jobID, text)
	if err != nil {
		return toolErr("steer_queue_task", err)
	}
	if note := deskqueue.FormatSteerForSession(text); note != "" {
		root, _ := h.root()
		sess := strings.TrimSpace(job.SessionKey)
		if sess == "" {
			sess = "main"
		}
		if root != "" && h.sess != nil {
			_, _ = h.sess.Append(root, "", sess, "user", note, "", "")
			if h.OnSessionChange != nil {
				h.OnSessionChange()
			}
		}
	}
	return mcp.NewToolResultText(fmt.Sprintf(
		"已排队插话 · id=%s status=%s · 下一工具结果或下一 Goal 轮送达",
		job.ID, job.Status,
	)), nil
}

func (h *Gateway) toolSetQueuePaused(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func (h *Gateway) requireQueue() (*deskqueue.Manager, error) {
	if h == nil || h.Queue == nil {
		return nil, fmt.Errorf("queue requires Job manager（执行器未挂载）")
	}
	root, err := h.root()
	if err != nil {
		return nil, err
	}
	h.Queue.Bind(root)
	return h.Queue, nil
}
