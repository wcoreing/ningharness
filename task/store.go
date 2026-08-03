package task

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ningharness/store"
	"ningharness/history"
)

const MaxKeep = 50

// ToolCall 单次工具调用摘要（可由 steps 推导）。
type ToolCall struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Path   string `json:"path,omitempty"`
}

// ToolEvent Agent 流里的一次 tool 事件。
type ToolEvent struct {
	Name  string
	Text  string
	Phase string
}

// Record 一轮执行台账（元数据在 tasks；过程在 history_message；正文在 resource）。
// Steps / Tools 为 Get 时从 history 推导的 DTO，不落盘到 tasks。
type Record struct {
	ID            string          `json:"id"`
	StartedAtMs   int64           `json:"startedAtMs"`
	EndedAtMs     int64           `json:"endedAtMs"`
	Driver        string          `json:"driver"`
	SessionID     string          `json:"sessionId,omitempty"`
	Purpose       string          `json:"purpose,omitempty"`
	JobID         string          `json:"jobId,omitempty"` // 关联 deskqueue.Job；非队列 once:…
	Tools         []ToolCall      `json:"tools,omitempty"`
	Steps         []Step          `json:"steps,omitempty"`
	Status        string          `json:"status"`
	Error         string          `json:"error,omitempty"`
	Feedforward   string          `json:"feedforward,omitempty"` // 本轮 user 前馈（来自 history_message）
	ResourceIDs   []int64         `json:"resourceIds,omitempty"` // Get 时从 history 汇总
}

// IndexEntry 索引元数据。
type IndexEntry struct {
	ID        string `json:"id"`
	EndedAtMs int64  `json:"endedAtMs"`
	Driver    string `json:"driver"`
	Status    string `json:"status"`
	Purpose   string `json:"purpose,omitempty"`
	ToolCount int    `json:"toolCount"`
	SessionID string `json:"sessionId,omitempty"`
	JobID     string `json:"jobId,omitempty"`
}

func openDB(root string) (*sql.DB, string, error) {
	db, err := store.OpenProject(root)
	return db, store.ProjectID(root), err
}

// Save 写入一轮元数据；超出 MaxKeep 删最旧。过程行已在 history_message；此处只写 tasks。
func Save(root string, rec Record) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("agenttask: empty root")
	}
	if strings.TrimSpace(rec.ID) == "" {
		return fmt.Errorf("agenttask: empty id")
	}
	if rec.EndedAtMs <= 0 {
		rec.EndedAtMs = time.Now().UnixMilli()
	}
	if rec.StartedAtMs <= 0 {
		rec.StartedAtMs = rec.EndedAtMs
	}
	for i := range rec.Tools {
		rec.Tools[i].Name = strings.TrimSpace(rec.Tools[i].Name)
	}
	if rec.Status == "" {
		if rec.Error != "" {
			rec.Status = "error"
		} else {
			rec.Status = "ok"
		}
	}
	if len(rec.Tools) == 0 && len(rec.Steps) > 0 {
		rec.Tools = ToolsFromSteps(rec.Steps)
	}
	toolCount := len(rec.Tools)

	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO tasks(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(project_id, id) DO UPDATE SET
			started_at_ms=excluded.started_at_ms, ended_at_ms=excluded.ended_at_ms, driver=excluded.driver,
			session_id=excluded.session_id, purpose=excluded.purpose, job_id=excluded.job_id,
			status=excluded.status, error=excluded.error, tool_count=excluded.tool_count`,
		pid, rec.ID, rec.StartedAtMs, rec.EndedAtMs, rec.Driver, rec.SessionID, rec.Purpose, rec.JobID,
		rec.Status, rec.Error, toolCount); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return pruneOld(db, pid)
}

func pruneOld(db *sql.DB, pid string) error {
	rows, err := db.Query(`SELECT id FROM tasks WHERE project_id=? ORDER BY ended_at_ms DESC`, pid)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if len(ids) <= MaxKeep {
		return nil
	}
	for _, id := range ids[MaxKeep:] {
		if _, err := db.Exec(`DELETE FROM tasks WHERE project_id=? AND id=?`, pid, id); err != nil {
			return err
		}
	}
	return nil
}

// List 返回索引（新→旧）。usersOnly 时跳过 purpose=reflect。
func List(root string, limit int, usersOnly bool) ([]IndexEntry, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("agenttask: empty root")
	}
	if limit <= 0 {
		limit = 20
	}
	db, pid, err := openDB(root)
	if err != nil {
		return nil, err
	}
	q := `SELECT id, ended_at_ms, driver, status, purpose, session_id, job_id, tool_count
		FROM tasks WHERE project_id=?`
	args := []any{pid}
	if usersOnly {
		q += ` AND TRIM(purpose) != 'reflect'`
	}
	q += ` ORDER BY ended_at_ms DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IndexEntry
	for rows.Next() {
		var e IndexEntry
		if err := rows.Scan(&e.ID, &e.EndedAtMs, &e.Driver, &e.Status, &e.Purpose, &e.SessionID, &e.JobID, &e.ToolCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get 读单轮元数据；Steps/Tools 由 history_message 推导。id 空则最近一轮用户执行。
func Get(root, id string) (*Record, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("agenttask: empty root")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		list, err := List(root, 1, true)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("agenttask: none")
		}
		id = list[0].ID
	}
	db, pid, err := openDB(root)
	if err != nil {
		return nil, err
	}
	var rec Record
	err = db.QueryRow(`SELECT id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error
		FROM tasks WHERE project_id=? AND id=?`, pid, id).Scan(
		&rec.ID, &rec.StartedAtMs, &rec.EndedAtMs, &rec.Driver, &rec.SessionID, &rec.Purpose, &rec.JobID,
		&rec.Status, &rec.Error)
	if err != nil {
		return nil, err
	}
	msgs, _ := history.LoadByTask(root, id)
	rec.Steps = StepsFromHistory(msgs)
	rec.Tools = ToolsFromSteps(rec.Steps)
	rec.ResourceIDs = history.CollectResourceIDs(msgs)
	for _, m := range msgs {
		if strings.TrimSpace(m.Role) != "user" {
			continue
		}
		if ff := strings.TrimSpace(m.Feedforward); ff != "" {
			rec.Feedforward = ff
			break
		}
		// 兼容未迁移的 content 内嵌快照
		if ui := history.ContentForUI(m.Content); ui != m.Content {
			// ContentForUI 剥掉了外壳；前馈取中间段
			if start := strings.Index(m.Content, history.SnapshotStartMarker); start >= 0 {
				if endRel := strings.Index(m.Content[start:], history.SnapshotEndMarker); endRel >= 0 {
					body := strings.TrimSpace(m.Content[start+len(history.SnapshotStartMarker) : start+endRel])
					if body != "" {
						rec.Feedforward = body
						break
					}
				}
			}
		}
	}
	return &rec, nil
}

// ShortSummary 给 Turn / Reflect 的一行短摘要。
func ShortSummary(root string) string {
	rec, err := Get(root, "")
	if err != nil || rec == nil {
		return ""
	}
	names := make([]string, 0, len(rec.Tools))
	fail := 0
	for _, t := range rec.Tools {
		if t.Name == "" {
			continue
		}
		if !t.OK {
			fail++
			names = append(names, t.Name+"!")
		} else {
			names = append(names, t.Name)
		}
	}
	toolPart := "无工具"
	if len(names) > 0 {
		if len(names) > 8 {
			names = names[:8]
			toolPart = strings.Join(names, ",") + "…"
		} else {
			toolPart = strings.Join(names, ",")
		}
	}
	extra := ""
	if fail > 0 {
		extra = fmt.Sprintf(" · %d 失败", fail)
	}
	return fmt.Sprintf("`%s` %s · %s · %s%s", rec.ID, rec.Driver, rec.Status, toolPart, extra)
}

// Count 当前库内 tasks 条数。
func Count(root string) int {
	db, pid, err := openDB(root)
	if err != nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id=?`, pid).Scan(&n)
	return n
}

// ToolsFromSteps 从步骤块推导工具摘要。
func ToolsFromSteps(blocks []Step) []ToolCall {
	evs := make([]ToolEvent, 0, len(blocks))
	for _, b := range blocks {
		if b.Kind != "tool" {
			continue
		}
		text := b.Result
		if text == "" {
			text = b.Args
		}
		phase := "result"
		if !b.Done {
			phase = "call"
		}
		evs = append(evs, ToolEvent{Name: b.Name, Text: text, Phase: phase})
	}
	return BuildToolsFromEvents(evs)
}

// BuildToolsFromEvents 聚合工具事件：保留 path、失败 detail。
func BuildToolsFromEvents(evs []ToolEvent) []ToolCall {
	type acc struct {
		name   string
		ok     bool
		detail string
		path   string
		order  int
	}
	byName := map[string]*acc{}
	order := 0
	for _, ev := range evs {
		n := strings.TrimSpace(ev.Name)
		if n == "" {
			continue
		}
		a := byName[n]
		if a == nil {
			a = &acc{name: n, ok: true, order: order}
			byName[n] = a
			order++
		}
		text := strings.TrimSpace(ev.Text)
		if text == "" {
			continue
		}
		if p := GuessRelPath(text); p != "" {
			a.path = p
		}
		if strings.EqualFold(ev.Phase, "call") {
			continue
		}
		if strings.HasPrefix(text, "error:") || strings.Contains(text, ": error:") {
			a.ok = false
			a.detail = text
			continue
		}
		a.detail = text
	}
	out := make([]ToolCall, order)
	for _, a := range byName {
		out[a.order] = ToolCall{
			Name:   a.name,
			OK:     a.ok,
			Detail: a.detail,
			Path:   a.path,
		}
	}
	compact := make([]ToolCall, 0, len(out))
	for _, t := range out {
		if t.Name == "" {
			continue
		}
		compact = append(compact, t)
	}
	return compact
}

// GuessRelPath 从工具文本猜测相对路径。
func GuessRelPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "error:") {
		return ""
	}
	if i := strings.Index(s, `"path"`); i >= 0 {
		rest := s[i+6:]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[j+1:]
			if k := strings.Index(rest, `"`); k >= 0 {
				p := filepath.ToSlash(strings.TrimSpace(rest[:k]))
				if looksLikeRelPath(p) {
					return p
				}
			}
		}
	}
	if i := strings.Index(s, `"rel_path"`); i >= 0 {
		rest := s[i+10:]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[j+1:]
			if k := strings.Index(rest, `"`); k >= 0 {
				p := filepath.ToSlash(strings.TrimSpace(rest[:k]))
				if looksLikeRelPath(p) {
					return p
				}
			}
		}
	}
	for _, prefix := range []string{"Successfully wrote", "Successfully edited"} {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		if i := strings.Index(s, "'"); i >= 0 {
			rest := s[i+1:]
			if j := strings.Index(rest, "'"); j >= 0 {
				p := filepath.ToSlash(strings.TrimSpace(rest[:j]))
				if looksLikeRelPath(p) {
					return p
				}
			}
		}
	}
	if strings.HasPrefix(s, "wrote ") {
		rest := strings.TrimPrefix(s, "wrote ")
		rest = strings.TrimPrefix(rest, "draft ")
		if i := strings.Index(rest, " "); i > 0 {
			rest = rest[:i]
		}
		rest = strings.Trim(rest, "()`'")
		p := filepath.ToSlash(rest)
		if looksLikeRelPath(p) {
			return p
		}
	}
	if looksLikeRelPath(s) {
		return filepath.ToSlash(s)
	}
	return ""
}

func looksLikeRelPath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 260 {
		return false
	}
	if strings.ContainsAny(s, " \n\t\"'{") {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "http") {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	if strings.Contains(s, "/") {
		return true
	}
	return strings.Contains(s, ".") && !strings.HasPrefix(s, ".")
}
