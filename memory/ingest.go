package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultIngestRelDir = ".ningharness/memory-ingest"
	ingestBodyMaxRunes  = 2000
)

// FileIngest 把回合摘要追加到项目内 JSONL（候选沉淀；不自动变 lesson）。
// 可单独使用，或包在 Chain 里与 Lesson Assemble 组合。
type FileIngest struct {
	// RelDir 相对项目根；空则 .ningharness/memory-ingest
	RelDir string
}

// Ingest 写入一行 JSON（prompt/reply 有界截断）。
func (f *FileIngest) Ingest(ctx context.Context, in IngestInput) error {
	_ = ctx
	root := strings.TrimSpace(in.Root)
	if root == "" {
		return nil
	}
	rel := strings.TrimSpace(f.RelDir)
	if rel == "" {
		rel = defaultIngestRelDir
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("memory ingest: %w", err)
	}
	day := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, day+".jsonl")
	rec := ingestRec{
		AtMs:       time.Now().UnixMilli(),
		SessionKey: in.SessionKey,
		TaskID:     in.TaskID,
		SkillIDs:   in.SkillIDs,
		Prompt:     trimRunes(in.Prompt, ingestBodyMaxRunes),
		Reply:      trimRunes(in.Reply, ingestBodyMaxRunes),
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	fp, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fp.Close()
	if _, err := fp.Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

type ingestRec struct {
	AtMs       int64    `json:"atMs"`
	SessionKey string   `json:"sessionKey,omitempty"`
	TaskID     string   `json:"taskId,omitempty"`
	SkillIDs   []string `json:"skillIds,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	Reply      string   `json:"reply,omitempty"`
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

// Bundle 组合 Assemble（主 Memory）与可选 Ingest。
type Bundle struct {
	Memory
	Ingester Ingester
}

// Ingest 转调内嵌 Ingester；无则 no-op。
func (b Bundle) Ingest(ctx context.Context, in IngestInput) error {
	if b.Ingester == nil {
		return nil
	}
	return b.Ingester.Ingest(ctx, in)
}

// NewLessonWithFileIngest 默认 Lesson 前馈 + 回合 JSONL 沉淀。
func NewLessonWithFileIngest() Bundle {
	return Bundle{Memory: NewLesson(), Ingester: &FileIngest{}}
}

var _ Memory = Bundle{}
var _ Ingester = Bundle{}
