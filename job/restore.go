package job

import (
	"path/filepath"
	"strings"

	"ningharness/task"
)

// restoreJobsFromTasks 从 tasks 台账重建已结束 Job（无队列行时兜底）。
func restoreJobsFromTasks(root string) []Job {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	list, err := task.List(root, maxFinishedKeep, true)
	if err != nil || len(list) == 0 {
		return nil
	}
	seenJob := map[string]struct{}{}
	out := make([]Job, 0, len(list))
	for _, e := range list {
		rec, err := task.Get(root, e.ID)
		if err != nil || rec == nil {
			continue
		}
		jid := strings.TrimSpace(rec.JobID)
		if jid == "" {
			jid = "from-" + strings.TrimSpace(rec.ID)
		}
		if _, ok := seenJob[jid]; ok {
			continue
		}
		seenJob[jid] = struct{}{}
		st := StatusDone
		switch strings.TrimSpace(rec.Status) {
		case "error":
			st = StatusError
		case "cancelled":
			st = StatusCancelled
		default:
			st = StatusDone
		}
		title := "task " + rec.ID
		if p := firstToolPath(rec); p != "" {
			title = filepath.Base(p)
		}
		out = append(out, Job{
			ID:         jid,
			Type:       JobTypeAgentTurn,
			Title:      title,
			Prompt:     "(无 prompt 摘要)",
			Status:     st,
			TaskID:     strings.TrimSpace(rec.ID),
			Error:      strings.TrimSpace(rec.Error),
			CreatedAt:  rec.StartedAtMs,
			StartedAt:  rec.StartedAtMs,
			FinishedAt: rec.EndedAtMs,
		})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func firstToolPath(rec *task.Record) string {
	if rec == nil {
		return ""
	}
	for _, t := range rec.Tools {
		if p := strings.TrimSpace(t.Path); p != "" {
			return p
		}
	}
	return ""
}
