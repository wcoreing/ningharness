package job

import (
	"fmt"
	"strings"
	"time"

	"github.com/wcoreing/ningharness/spawn"
)

func spawnMarkRunning(root, id string) error {
	return spawn.MarkRunning(root, id, id)
}

func spawnSyncFinish(root string, done Job) {
	if strings.TrimSpace(done.Type) != JobTypeSpawn {
		return
	}
	id := strings.TrimSpace(done.ID)
	if id == "" {
		return
	}
	st := spawn.StatusDone
	errMsg := ""
	switch done.Status {
	case StatusCancelled:
		st = spawn.StatusCancelled
		errMsg = "cancelled"
	case StatusError:
		st = spawn.StatusError
		errMsg = strings.TrimSpace(done.Error)
	case StatusDone:
		st = spawn.StatusDone
	default:
		return
	}
	rec, err := spawn.Get(root, id)
	summary := ""
	if err == nil {
		summary = strings.TrimSpace(rec.Summary)
		if summary != "" && st == spawn.StatusDone && rec.Status == spawn.StatusDone {
			return
		}
	}
	_ = spawn.Complete(root, id, st, summary, errMsg)
}

// EnqueueSpawn 入队子代理：独立 session、Packet 前馈、Registry 登记。
func (m *Manager) EnqueueSpawn(in SpawnEnqueue) (Job, error) {
	pkt := spawn.Packet{
		Goal:          in.Goal,
		Context:       in.Context,
		Artifacts:     in.Artifacts,
		ParentSession: in.ParentSession,
		ParentTask:    in.ParentTask,
		ParentJob:     in.ParentJob,
		ParentDepth:   in.ParentDepth,
		Role:          in.Role,
		Title:         in.Title,
		MaxDepth:      in.MaxDepth,
	}
	if err := pkt.Validate(); err != nil {
		return Job{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	root := strings.TrimSpace(m.root)
	if root == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	id := newID("q")
	ff := spawn.BuildFeedforward(pkt)
	if extra := strings.TrimSpace(in.FeedExtra); extra != "" {
		ff = ff + "\n\n" + extra
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = titleFromPrompt(pkt.Goal, "")
	}
	t := Job{
		ID:         id,
		Type:       JobTypeSpawn,
		Title:      title,
		Prompt:     spawn.BuildPrompt(pkt),
		Driver:     strings.TrimSpace(in.Driver),
		Model:      strings.TrimSpace(in.Model),
		Status:     StatusQueued,
		CreatedAt:  now,
		SessionKey: spawn.SessionKeyFor(id),
		Purpose:    pkt.Purpose(),
		FeedExtra:  ff,
	}
	if _, err := spawn.RegisterFromPacket(root, id, pkt); err != nil {
		return Job{}, err
	}
	m.file.Jobs = append(m.file.Jobs, t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return t, nil
}
