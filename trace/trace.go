package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	TypeTaskBegin  = "task_begin"
	TypeTaskEnd    = "task_end"
	TypeToolCall   = "tool_call"
	TypeToolResult = "tool_result"
	TypeAbort      = "abort"
	TypeNote       = "note"

	relTracesDir   = ".ningharness/traces"
	maxBriefRunes  = 500
	DefaultMaxKeep = 80
)

// Event 一条 append-only JSONL 记录。
type Event struct {
	TS          int64  `json:"ts"`
	Type        string `json:"type"`
	TaskID      string `json:"task_id,omitempty"`
	SessionKey  string `json:"session_key,omitempty"`
	JobID       string `json:"job_id,omitempty"`
	ToolCallID  string `json:"tool_call_id,omitempty"`
	Name        string `json:"name,omitempty"`
	ArgsBrief   string `json:"args_brief,omitempty"`
	ResultBrief string `json:"result_brief,omitempty"`
	OK          *bool  `json:"ok,omitempty"`
	Status      string `json:"status,omitempty"`
	Error       string `json:"error,omitempty"`
	Round       int    `json:"round,omitempty"`
}

// ResumeState 恢复契约：只认已配对事件；未完成 tool_call 视为需中止补记。
type ResumeState struct {
	Complete      bool
	TaskID        string
	Path          string
	UnpairedCalls []string
	EndedStatus   string
	EventCount    int
}

// Writer 单 Task 的 JSONL 追加写入。
// Eino 会并行调工具，Append 必须串行，否则丢行（曾漏 list_pins/pin_path）。
type Writer struct {
	root   string
	taskID string
	path   string
	mu     sync.Mutex
}

func Dir(root string) string {
	return filepath.Join(strings.TrimSpace(root), relTracesDir)
}

func PathFor(root, taskID string, at time.Time) string {
	root = strings.TrimSpace(root)
	taskID = sanitizeID(taskID)
	if root == "" || taskID == "" {
		return ""
	}
	if at.IsZero() {
		at = time.Now()
	}
	day := at.Format("2006-01-02")
	return filepath.Join(Dir(root), day, taskID+".jsonl")
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "\\", "_")
	id = strings.ReplaceAll(id, "..", "_")
	return id
}

// Begin 创建 Writer 并写入 task_begin。
func Begin(root, taskID, sessionKey, jobID string) (*Writer, error) {
	root = strings.TrimSpace(root)
	taskID = sanitizeID(taskID)
	if root == "" || taskID == "" {
		return nil, fmt.Errorf("trace: empty root or task_id")
	}
	w := &Writer{root: root, taskID: taskID, path: PathFor(root, taskID, time.Now())}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return nil, err
	}
	if err := w.Append(Event{
		Type:       TypeTaskBegin,
		TaskID:     taskID,
		SessionKey: strings.TrimSpace(sessionKey),
		JobID:      strings.TrimSpace(jobID),
	}); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

func (w *Writer) TaskID() string {
	if w == nil {
		return ""
	}
	return w.taskID
}

func (w *Writer) Append(ev Event) error {
	if w == nil || w.path == "" {
		return fmt.Errorf("trace: nil writer")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if ev.TS <= 0 {
		ev.TS = time.Now().UnixMilli()
	}
	if ev.TaskID == "" {
		ev.TaskID = w.taskID
	}
	ev.ArgsBrief = trimBrief(ev.ArgsBrief)
	ev.ResultBrief = trimBrief(ev.ResultBrief)
	raw, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

func (w *Writer) ToolCall(callID, name, argsBrief string) error {
	return w.Append(Event{
		Type:       TypeToolCall,
		ToolCallID: strings.TrimSpace(callID),
		Name:       strings.TrimSpace(name),
		ArgsBrief:  argsBrief,
	})
}

func (w *Writer) ToolResult(callID, name, resultBrief string, ok bool) error {
	o := ok
	return w.Append(Event{
		Type:        TypeToolResult,
		ToolCallID:  strings.TrimSpace(callID),
		Name:        strings.TrimSpace(name),
		ResultBrief: resultBrief,
		OK:          &o,
	})
}

func (w *Writer) End(status, errMsg string) error {
	if w == nil {
		return nil
	}
	st := strings.TrimSpace(status)
	if st == "" {
		if strings.TrimSpace(errMsg) != "" {
			st = "error"
		} else {
			st = "ok"
		}
	}
	if err := w.Append(Event{Type: TypeTaskEnd, Status: st, Error: strings.TrimSpace(errMsg)}); err != nil {
		return err
	}
	_ = Prune(w.root, DefaultMaxKeep)
	return nil
}

func (w *Writer) Abort(errMsg string) error {
	if w == nil {
		return nil
	}
	return w.Append(Event{Type: TypeAbort, Error: strings.TrimSpace(errMsg)})
}

func trimBrief(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxBriefRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxBriefRunes]) + "…"
}

// Load 读取 JSONL；容忍末行截断。
func Load(path string) ([]Event, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("trace: empty path")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if strings.TrimSpace(ev.Type) == "" {
			continue
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

// FindByTaskID 在 traces 树下找 task 的 jsonl（取 mtime 最新）。
func FindByTaskID(root, taskID string) (string, error) {
	root = strings.TrimSpace(root)
	taskID = sanitizeID(taskID)
	if root == "" || taskID == "" {
		return "", fmt.Errorf("trace: empty root or task_id")
	}
	want := taskID + ".jsonl"
	var best string
	var bestMod time.Time
	err := filepath.WalkDir(Dir(root), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() != want {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if best == "" || info.ModTime().After(bestMod) {
			best = path
			bestMod = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if best == "" {
		return "", fmt.Errorf("trace: not found for %s", taskID)
	}
	return best, nil
}

// Inspect 应用恢复契约。
func Inspect(events []Event) ResumeState {
	st := ResumeState{EventCount: len(events)}
	open := map[string]bool{}
	var order []string
	for _, ev := range events {
		if st.TaskID == "" && ev.TaskID != "" {
			st.TaskID = ev.TaskID
		}
		switch ev.Type {
		case TypeToolCall:
			id := strings.TrimSpace(ev.ToolCallID)
			if id == "" {
				id = fmt.Sprintf("anon:%d", ev.TS)
			}
			if !open[id] {
				order = append(order, id)
			}
			open[id] = true
		case TypeToolResult:
			id := strings.TrimSpace(ev.ToolCallID)
			if id == "" {
				continue
			}
			delete(open, id)
		case TypeTaskEnd:
			st.Complete = true
			st.EndedStatus = ev.Status
		case TypeAbort:
			st.Complete = false
		}
	}
	for _, id := range order {
		if open[id] {
			st.UnpairedCalls = append(st.UnpairedCalls, id)
		}
	}
	if len(st.UnpairedCalls) > 0 {
		st.Complete = false
	}
	return st
}

// InspectFile Load + Inspect。
func InspectFile(path string) (ResumeState, []Event, error) {
	evs, err := Load(path)
	if err != nil {
		return ResumeState{}, nil, err
	}
	st := Inspect(evs)
	st.Path = path
	return st, evs, nil
}

// Prune 按文件 mtime 保留最近 keep 个 jsonl。
func Prune(root string, keep int) error {
	if keep < 1 {
		keep = DefaultMaxKeep
	}
	dir := Dir(root)
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if len(files) <= keep {
		return nil
	}
	type fm struct {
		path string
		mod  time.Time
	}
	list := make([]fm, 0, len(files))
	for _, p := range files {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		list = append(list, fm{path: p, mod: info.ModTime()})
	}
	for i := 0; i < len(list); i++ {
		best := i
		for j := i + 1; j < len(list); j++ {
			if list[j].mod.After(list[best].mod) {
				best = j
			}
		}
		list[i], list[best] = list[best], list[i]
	}
	for i := keep; i < len(list); i++ {
		_ = os.Remove(list[i].path)
	}
	return nil
}
