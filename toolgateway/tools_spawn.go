package toolgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	nhjob "github.com/wcoreing/ningharness/job"
	"github.com/wcoreing/ningharness/spawn"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) toolSpawnAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = ctx
	if err := h.refuseSpawnIfLeaf(); err != nil {
		return toolErr("spawn_agent", err)
	}
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("spawn_agent", err)
	}
	goal, err := req.RequireString("goal")
	if err != nil {
		return toolErr("spawn_agent", err)
	}
	root, _ := h.root()
	parentSess := h.activeSessionKey()
	parentTask, _, _ := h.TurnContext()
	parentDepth := 0
	if sid := spawn.IDFromSession(parentSess); sid != "" {
		if rec, err := spawn.Get(root, sid); err == nil {
			parentDepth = rec.Depth
		} else {
			parentDepth = 1
		}
	}
	role := strings.TrimSpace(req.GetString("role", spawn.RoleLeaf))
	arts := splitCSV(req.GetString("artifacts", ""))
	job, err := q.EnqueueSpawn(nhjob.SpawnEnqueue{
		Goal:          goal,
		Context:       req.GetString("context", ""),
		Artifacts:     arts,
		ParentSession: parentSess,
		ParentTask:    parentTask,
		ParentJob:     "",
		ParentDepth:   parentDepth,
		Role:          role,
		Title:         req.GetString("title", ""),
		Driver:        req.GetString("driver", ""),
		Model:         req.GetString("model", ""),
		MaxDepth:      int(req.GetFloat("max_depth", float64(spawn.DefaultMaxDepth))),
	})
	if err != nil {
		return toolErr("spawn_agent", err)
	}
	h.AnnounceQueueEnqueued(job)
	rec, _ := spawn.Get(root, job.ID)
	brief := spawn.BriefFrom(rec)
	brief.Status = spawn.StatusQueued
	raw, _ := json.MarshalIndent(brief, "", "  ")
	return mcp.NewToolResultText(nhjob.FormatEnqueueOK(job) + "\n" + string(raw)), nil
}

func (h *Gateway) toolListSpawns(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("list_spawns", err)
	}
	parent := strings.TrimSpace(req.GetString("parent_session", ""))
	if parent == "" && req.GetString("mine", "true") != "false" {
		parent = h.activeSessionKey()
	}
	list, err := spawn.List(root, parent, int(req.GetFloat("limit", 30)))
	if err != nil {
		return toolErr("list_spawns", err)
	}
	if list == nil {
		list = []spawn.Record{}
	}
	briefs := make([]spawn.Brief, 0, len(list))
	for _, r := range list {
		briefs = append(briefs, spawn.BriefFrom(r))
	}
	raw, _ := json.MarshalIndent(briefs, "", "  ")
	return mcp.NewToolResultText(string(raw)), nil
}

func (h *Gateway) toolGetSpawn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("get_spawn", err)
	}
	id, err := req.RequireString("spawn_id")
	if err != nil {
		return toolErr("get_spawn", err)
	}
	rec, err := spawn.Get(root, id)
	if err != nil {
		return toolErr("get_spawn", err)
	}
	raw, _ := json.MarshalIndent(spawn.BriefFrom(rec), "", "  ")
	return mcp.NewToolResultText(string(raw)), nil
}

func (h *Gateway) toolSteerSpawn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("steer_spawn", err)
	}
	id, err := req.RequireString("spawn_id")
	if err != nil {
		return toolErr("steer_spawn", err)
	}
	text, err := req.RequireString("message")
	if err != nil {
		return toolErr("steer_spawn", err)
	}
	job, err := q.Steer(id, text)
	if err != nil {
		return toolErr("steer_spawn", err)
	}
	return mcp.NewToolResultText(fmt.Sprintf("steered spawn %s status=%s", job.ID, job.Status)), nil
}

func (h *Gateway) toolStopSpawn(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := h.requireQueue()
	if err != nil {
		return toolErr("stop_spawn", err)
	}
	root, _ := h.root()
	id, err := req.RequireString("spawn_id")
	if err != nil {
		return toolErr("stop_spawn", err)
	}
	if err := q.Cancel(id); err != nil {
		return toolErr("stop_spawn", err)
	}
	_ = spawn.Complete(root, id, spawn.StatusCancelled, "", "cancelled")
	return mcp.NewToolResultText(fmt.Sprintf("stopped spawn %s", strings.TrimSpace(id))), nil
}

func (h *Gateway) refuseSpawnIfLeaf() error {
	_, sess, _ := h.TurnContext()
	if !spawn.IsSpawnSession(sess) {
		return nil
	}
	root, _ := h.root()
	sid := spawn.IDFromSession(sess)
	if sid == "" {
		return nil
	}
	rec, err := spawn.Get(root, sid)
	if err != nil {
		return fmt.Errorf("spawn: leaf subagent cannot spawn")
	}
	maxD := rec.MaxDepth
	if maxD < 1 {
		maxD = spawn.DefaultMaxDepth
	}
	if rec.Depth >= maxD {
		return fmt.Errorf("spawn: depth limit %d", maxD)
	}
	if rec.Role != spawn.RoleOrchestrator {
		return fmt.Errorf("spawn: leaf subagent cannot spawn (depth=%d)", rec.Depth)
	}
	return nil
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
