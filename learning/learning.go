// Package learning 自学习闭环：Evidence → Candidate → Policy → Commit(MD)。
// 成败策略只实现为 Policy；默认 NoopPolicy 只记候选。
package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wcoreing/ningharness/lesson"
)

const (
	defaultCandidateRelDir = ".ningharness/memory-ingest"
	candidateBodyMaxRunes  = 2000
)

// Evidence 本轮可学习事实快照（不判断好坏）。
type Evidence struct {
	Root         string
	SessionKey   string
	TaskID       string
	Prompt       string
	Reply        string
	Feedforward  string
	SkillIDs     []string
	Notes        string
	Values       map[string]any
}

// Candidate 未权威持久记录。
type Candidate struct {
	ID         string   `json:"id"`
	AtMs       int64    `json:"atMs"`
	SessionKey string   `json:"sessionKey,omitempty"`
	TaskID     string   `json:"taskId,omitempty"`
	SkillIDs   []string `json:"skillIds,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	Reply      string   `json:"reply,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// CandidateStore 候选仓。
type CandidateStore interface {
	Append(ctx context.Context, ev Evidence) (Candidate, error)
}

// Decision Policy 产出；Accept 时写入 MD。
type Decision struct {
	Accept       bool
	Scope        string // project|personal|skill
	SkillID      string
	Body         string
	SupersedesID string
}

// Policy 业务策略插槽（成败规则放这里）。
type Policy interface {
	Decide(ctx context.Context, ev Evidence, c Candidate) (Decision, error)
}

// NoopPolicy 只记候选，不 Commit。
type NoopPolicy struct{}

func (NoopPolicy) Decide(ctx context.Context, ev Evidence, c Candidate) (Decision, error) {
	_ = ctx
	_ = ev
	_ = c
	return Decision{Accept: false}, nil
}

// FileCandidate JSONL 候选仓（非 DB）。
type FileCandidate struct {
	RelDir string
}

func (f *FileCandidate) Append(ctx context.Context, ev Evidence) (Candidate, error) {
	_ = ctx
	root := strings.TrimSpace(ev.Root)
	if root == "" {
		return Candidate{}, nil
	}
	rel := strings.TrimSpace(f.RelDir)
	if rel == "" {
		rel = defaultCandidateRelDir
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Candidate{}, fmt.Errorf("learning candidate: %w", err)
	}
	day := time.Now().UTC().Format("2006-01-02")
	path := filepath.Join(dir, day+".jsonl")
	c := Candidate{
		ID:         fmt.Sprintf("cand_%d", time.Now().UnixNano()),
		AtMs:       time.Now().UnixMilli(),
		SessionKey: ev.SessionKey,
		TaskID:     ev.TaskID,
		SkillIDs:   ev.SkillIDs,
		Prompt:     trimRunes(ev.Prompt, candidateBodyMaxRunes),
		Reply:      trimRunes(ev.Reply, candidateBodyMaxRunes),
		Notes:      trimRunes(ev.Notes, candidateBodyMaxRunes),
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return Candidate{}, err
	}
	fp, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Candidate{}, err
	}
	defer fp.Close()
	if _, err := fp.Write(append(raw, '\n')); err != nil {
		return Candidate{}, err
	}
	return c, nil
}

// Commit 将 Decision 写入 MD SSOT。
func Commit(root string, d Decision) error {
	if !d.Accept {
		return nil
	}
	body := strings.TrimSpace(d.Body)
	if body == "" {
		return nil
	}
	scope := strings.TrimSpace(d.Scope)
	if scope == "" {
		scope = lesson.ScopeProject
	}
	_, err := lesson.Append(lesson.AppendInput{
		Root:         root,
		Scope:        scope,
		SkillID:      strings.TrimSpace(d.SkillID),
		Body:         body,
		SupersedesID: strings.TrimSpace(d.SupersedesID),
	})
	return err
}

// Learner 跑一轮学习。
type Learner struct {
	Candidates CandidateStore
	Policy     Policy // nil = Noop
}

// Result 本轮学习结果（可写入 RunState.Values）。
type Result struct {
	Candidate Candidate
	Decision  Decision
	Committed bool
}

// Run Evidence → Candidate → Policy → 可选 Commit。
func (l *Learner) Run(ctx context.Context, ev Evidence) (Result, error) {
	var out Result
	if strings.TrimSpace(ev.Root) == "" {
		return out, nil
	}
	if strings.TrimSpace(ev.Reply) == "" && strings.TrimSpace(ev.Notes) == "" {
		return out, nil
	}
	store := l.Candidates
	if store == nil {
		store = &FileCandidate{}
	}
	c, err := store.Append(ctx, ev)
	if err != nil {
		return out, err
	}
	out.Candidate = c
	pol := l.Policy
	if pol == nil {
		pol = NoopPolicy{}
	}
	d, err := pol.Decide(ctx, ev, c)
	if err != nil {
		return out, err
	}
	out.Decision = d
	if !d.Accept {
		return out, nil
	}
	if err := Commit(ev.Root, d); err != nil {
		return out, err
	}
	out.Committed = true
	return out, nil
}

// NewDefault 默认：JSONL 候选 + NoopPolicy。
func NewDefault() *Learner {
	return &Learner{
		Candidates: &FileCandidate{},
		Policy:     NoopPolicy{},
	}
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

var _ CandidateStore = (*FileCandidate)(nil)
var _ Policy = NoopPolicy{}
