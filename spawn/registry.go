package spawn

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wcoreing/ningharness/metadir"
	"gopkg.in/yaml.v3"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusError     = "error"
	StatusCancelled = "cancelled"
)

// Record 子代理登记（控制面 YAML；结构化元数据，非经验正文）。
type Record struct {
	ID            string   `yaml:"id" json:"id"`
	SessionKey    string   `yaml:"sessionKey" json:"sessionKey"`
	ParentSession string   `yaml:"parentSession,omitempty" json:"parentSession,omitempty"`
	ParentTask    string   `yaml:"parentTask,omitempty" json:"parentTask,omitempty"`
	ParentJob     string   `yaml:"parentJob,omitempty" json:"parentJob,omitempty"`
	Goal          string   `yaml:"goal" json:"goal"`
	Context       string   `yaml:"context,omitempty" json:"context,omitempty"`
	Artifacts     []string `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Role          string   `yaml:"role,omitempty" json:"role,omitempty"`
	Depth         int      `yaml:"depth" json:"depth"`
	MaxDepth      int      `yaml:"maxDepth,omitempty" json:"maxDepth,omitempty"`
	Status        string   `yaml:"status" json:"status"`
	Summary       string   `yaml:"summary,omitempty" json:"summary,omitempty"`
	Error         string   `yaml:"error,omitempty" json:"error,omitempty"`
	JobID         string   `yaml:"jobId,omitempty" json:"jobId,omitempty"`
	CreatedAtMs   int64    `yaml:"createdAtMs" json:"createdAtMs"`
	UpdatedAtMs   int64    `yaml:"updatedAtMs,omitempty" json:"updatedAtMs,omitempty"`
	FinishedAtMs  int64    `yaml:"finishedAtMs,omitempty" json:"finishedAtMs,omitempty"`
}

func dir(root, id string) string {
	return metadir.JoinProject(root, "spawns", strings.TrimSpace(id))
}

func controlPath(root, id string) string {
	return filepath.Join(dir(root, id), "SPAWN.yaml")
}

// Put 写入/覆盖登记。
func Put(root string, rec Record) error {
	root = strings.TrimSpace(root)
	id := strings.TrimSpace(rec.ID)
	if root == "" || id == "" {
		return fmt.Errorf("spawn: empty root/id")
	}
	rec.ID = id
	if rec.SessionKey == "" {
		rec.SessionKey = SessionKeyFor(id)
	}
	if rec.Status == "" {
		rec.Status = StatusQueued
	}
	now := time.Now().UnixMilli()
	if rec.CreatedAtMs == 0 {
		rec.CreatedAtMs = now
	}
	rec.UpdatedAtMs = now
	if err := os.MkdirAll(dir(root, id), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(controlPath(root, id), b, 0o644)
}

// Get 读登记。
func Get(root, id string) (Record, error) {
	raw, err := os.ReadFile(controlPath(root, id))
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := yaml.Unmarshal(raw, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// List 列子代理；parentSession 非空则过滤；limit≤0 默认 50。
func List(root, parentSession string, limit int) ([]Record, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("spawn: empty root")
	}
	if limit <= 0 {
		limit = 50
	}
	base := metadir.JoinProject(root, "spawns")
	ents, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	wantParent := strings.TrimSpace(parentSession)
	out := make([]Record, 0, len(ents))
	for i := len(ents) - 1; i >= 0; i-- {
		ent := ents[i]
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		rec, err := Get(root, ent.Name())
		if err != nil {
			continue
		}
		if wantParent != "" && strings.TrimSpace(rec.ParentSession) != wantParent {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// MarkRunning 标记执行中。
func MarkRunning(root, id, jobID string) error {
	rec, err := Get(root, id)
	if err != nil {
		return err
	}
	rec.Status = StatusRunning
	if j := strings.TrimSpace(jobID); j != "" {
		rec.JobID = j
	}
	return Put(root, rec)
}

// Complete 写入终态与摘要。
func Complete(root, id, status, summary, errMsg string) error {
	rec, err := Get(root, id)
	if err != nil {
		return err
	}
	st := strings.TrimSpace(status)
	if st == "" {
		st = StatusDone
	}
	rec.Status = st
	rec.Summary = strings.TrimSpace(summary)
	rec.Error = strings.TrimSpace(errMsg)
	rec.FinishedAtMs = time.Now().UnixMilli()
	return Put(root, rec)
}

// BriefFrom 从登记生成回传。
func BriefFrom(rec Record) Brief {
	return Brief{
		SpawnID:    rec.ID,
		Status:     rec.Status,
		Summary:    rec.Summary,
		Artifacts:  rec.Artifacts,
		Error:      rec.Error,
		SessionKey: rec.SessionKey,
	}
}

// RegisterFromPacket 创建登记（id=jobID）。
func RegisterFromPacket(root, jobID string, p Packet) (Record, error) {
	if err := p.Validate(); err != nil {
		return Record{}, err
	}
	jobID = strings.TrimSpace(jobID)
	rec := Record{
		ID:            jobID,
		SessionKey:    SessionKeyFor(jobID),
		ParentSession: strings.TrimSpace(p.ParentSession),
		ParentTask:    strings.TrimSpace(p.ParentTask),
		ParentJob:     strings.TrimSpace(p.ParentJob),
		Goal:          strings.TrimSpace(p.Goal),
		Context:       strings.TrimSpace(p.Context),
		Artifacts:     cleanArtifacts(p.Artifacts),
		Role:          p.NormalizeRole(),
		Depth:         p.ChildDepth(),
		MaxDepth:      p.MaxDepthOrDefault(),
		Status:        StatusQueued,
		JobID:         jobID,
	}
	if err := Put(root, rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}
