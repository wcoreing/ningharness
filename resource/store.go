// Package resource 外置正文索引：入库不截断，进模用 summary，按需召回。
package resource

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ningharness/task"
	"ningharness/store"
)

const (
	// HistoryInlineMax 不超过此 rune 数时，history 直接带全文；仍写入本表便于检索。
	HistoryInlineMax = 480
	searchDefault    = 12
	searchMax        = 40
	snippetRunes     = 160

	KindToolCall   = "tool_call"
	KindToolResult = "tool_result"
	KindDiff       = "diff"
)

// PutInput 写入一项外置资源。
type PutInput struct {
	SessionKey string
	TaskID     string
	ToolCallID string
	ToolName   string
	Kind       string // tool_call | tool_result | diff；空则按 Phase 推导
	Phase      string // call | result（兼容；Kind 优先）
	RelPath    string // 可空；空则从 Body 猜测
	Body       string
}

// Record 一行 resource。
type Record struct {
	ID          int64
	SessionKey  string
	TaskID      string
	ToolCallID  string
	ToolName    string
	Kind        string
	Phase       string
	RelPath     string
	Status      string
	Summary     string
	Body        string
	CreatedAtMs int64
}

// SearchOptions 检索资源。
type SearchOptions struct {
	ResourceID int64
	ToolCallID string
	RelPath    string
	Query      string
	Phase      string // 空=不限；call|result
	Kind       string // 空=不限
	SessionKey string
	Limit      int
}

func openDB(root string) (*sql.DB, string, error) {
	db, err := store.OpenProject(root)
	if err != nil {
		return nil, "", err
	}
	return db, store.ProjectID(root), nil
}

func resolveKind(kind, phase string) (string, string) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	phase = strings.ToLower(strings.TrimSpace(phase))
	if kind == "" {
		switch phase {
		case "call":
			kind = KindToolCall
		case "diff":
			kind = KindDiff
		default:
			kind = KindToolResult
			if phase == "" {
				phase = "result"
			}
		}
	}
	if phase == "" {
		switch kind {
		case KindToolCall:
			phase = "call"
		case KindDiff:
			phase = "diff"
		default:
			phase = "result"
		}
	}
	return kind, phase
}

// Put 全文入库并返回 id 与进模用 summary。
func Put(root string, in PutInput) (id int64, summary string, err error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0, "", fmt.Errorf("resource: empty root")
	}
	body := in.Body
	name := strings.TrimSpace(in.ToolName)
	kind, phase := resolveKind(in.Kind, in.Phase)
	rel := strings.TrimSpace(in.RelPath)
	if rel == "" {
		rel = task.GuessRelPath(body)
	}
	status := "ok"
	if strings.HasPrefix(strings.TrimSpace(body), "error:") {
		status = "error"
	}
	summary = BuildSummary(0, name, kind, phase, status, rel, body)
	db, pid, err := openDB(root)
	if err != nil {
		return 0, summary, err
	}
	now := time.Now().UnixMilli()
	res, err := db.Exec(`INSERT INTO resource(
		project_id, session_key, task_id, tool_call_id, tool_name, kind, phase, rel_path, status, summary, body, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		pid, strings.TrimSpace(in.SessionKey), strings.TrimSpace(in.TaskID), strings.TrimSpace(in.ToolCallID),
		name, kind, phase, rel, status, summary, body, now)
	if err != nil {
		return 0, summary, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, summary, err
	}
	summary = BuildSummary(id, name, kind, phase, status, rel, body)
	_, _ = db.Exec(`UPDATE resource SET summary=? WHERE id=? AND project_id=?`, summary, id, pid)
	return id, summary, nil
}

// HistoryContent history_message.content；resource_ids 另存，不在正文重复 〔resource#N〕。
func HistoryContent(id int64, name, kind, phase, status, rel, body string) string {
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	phase = strings.TrimSpace(phase)
	status = strings.TrimSpace(status)
	rel = strings.TrimSpace(rel)
	trimmed := strings.TrimSpace(body)
	n := utf8.RuneCountInString(trimmed)
	if kind != KindDiff && n <= HistoryInlineMax && trimmed != "" {
		return trimmed
	}
	var b strings.Builder
	if name != "" {
		b.WriteString(name)
	}
	if kind != "" {
		if b.Len() > 0 {
			b.WriteString(" · ")
		}
		fmt.Fprintf(&b, "%s", kind)
	}
	if phase != "" && phase != kind {
		fmt.Fprintf(&b, " · %s", phase)
	}
	if status != "" {
		fmt.Fprintf(&b, " · %s", status)
	}
	if rel != "" {
		fmt.Fprintf(&b, " · %s", rel)
	}
	if n > 0 {
		fmt.Fprintf(&b, " · %d字", n)
	}
	return b.String()
}

// BuildSummary resource 表 summary 与台账短摘要；进模 recall 提示由 history resource_ids 承担。
func BuildSummary(id int64, name, kind, phase, status, rel, body string) string {
	name = strings.TrimSpace(name)
	kind = strings.TrimSpace(kind)
	phase = strings.TrimSpace(phase)
	status = strings.TrimSpace(status)
	rel = strings.TrimSpace(rel)
	trimmed := strings.TrimSpace(body)
	n := utf8.RuneCountInString(trimmed)
	if kind != KindDiff && n <= HistoryInlineMax && trimmed != "" {
		if id > 0 {
			return fmt.Sprintf("%s\n〔resource#%d〕", trimmed, id)
		}
		return trimmed
	}
	var b strings.Builder
	if id > 0 {
		fmt.Fprintf(&b, "resource#%d", id)
	} else {
		b.WriteString("resource")
	}
	if kind != "" {
		fmt.Fprintf(&b, " · %s", kind)
	}
	if name != "" {
		fmt.Fprintf(&b, " · %s", name)
	}
	if phase != "" && phase != kind {
		fmt.Fprintf(&b, " · %s", phase)
	}
	if status != "" {
		fmt.Fprintf(&b, " · %s", status)
	}
	if rel != "" {
		fmt.Fprintf(&b, " · %s", rel)
	}
	if n > 0 {
		fmt.Fprintf(&b, " · %d字", n)
	}
	if id > 0 {
		fmt.Fprintf(&b, " · 全文 desk_session recall_resource resource_id=%d", id)
	} else {
		b.WriteString(" · 全文 desk_session recall_resource")
	}
	return b.String()
}

// Get 按 id 取全文。
func Get(root string, id int64) (*Record, error) {
	if id <= 0 {
		return nil, fmt.Errorf("resource: invalid id")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return nil, err
	}
	var r Record
	err = db.QueryRow(`SELECT id, session_key, task_id, tool_call_id, tool_name, kind, phase, rel_path, status, summary, body, created_at_ms
		FROM resource WHERE project_id=? AND id=?`, pid, id).Scan(
		&r.ID, &r.SessionKey, &r.TaskID, &r.ToolCallID, &r.ToolName, &r.Kind, &r.Phase, &r.RelPath, &r.Status, &r.Summary, &r.Body, &r.CreatedAtMs)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("resource: id %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Search 检索；resource_id 优先返回全文，否则返回命中列表（含摘要）。
func Search(root string, opt SearchOptions) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("resource: empty root")
	}
	if opt.ResourceID > 0 {
		r, err := Get(root, opt.ResourceID)
		if err != nil {
			return "", err
		}
		return formatFull(r), nil
	}
	db, pid, err := openDB(root)
	if err != nil {
		return "", err
	}
	limit := opt.Limit
	if limit <= 0 {
		limit = searchDefault
	}
	if limit > searchMax {
		limit = searchMax
	}
	q := strings.TrimSpace(opt.Query)
	callID := strings.TrimSpace(opt.ToolCallID)
	rel := strings.TrimSpace(opt.RelPath)
	phase := strings.ToLower(strings.TrimSpace(opt.Phase))
	kind := strings.ToLower(strings.TrimSpace(opt.Kind))
	sess := strings.TrimSpace(opt.SessionKey)
	if q == "" && callID == "" && rel == "" {
		return "", fmt.Errorf("recall_resource: 需要 resource_id / tool_call_id / rel_path / query 之一")
	}

	sqlQ := `SELECT id, session_key, task_id, tool_call_id, tool_name, kind, phase, rel_path, status, summary, body, created_at_ms
		FROM resource WHERE project_id=?`
	args := []any{pid}
	if callID != "" {
		sqlQ += ` AND tool_call_id=?`
		args = append(args, callID)
	}
	if rel != "" {
		sqlQ += ` AND rel_path=?`
		args = append(args, rel)
	}
	if phase != "" {
		sqlQ += ` AND phase=?`
		args = append(args, phase)
	}
	if kind != "" {
		sqlQ += ` AND kind=?`
		args = append(args, kind)
	}
	if sess != "" {
		sqlQ += ` AND session_key=?`
		args = append(args, sess)
	}
	if q != "" {
		like := "%" + q + "%"
		sqlQ += ` AND (summary LIKE ? OR body LIKE ? OR tool_name LIKE ? OR rel_path LIKE ?)`
		args = append(args, like, like, like, like)
	}
	sqlQ += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(sqlQ, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var hits []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.SessionKey, &r.TaskID, &r.ToolCallID, &r.ToolName, &r.Kind, &r.Phase, &r.RelPath, &r.Status, &r.Summary, &r.Body, &r.CreatedAtMs); err != nil {
			return "", err
		}
		hits = append(hits, r)
	}
	if len(hits) == 0 {
		return "resource: 无命中", nil
	}
	if len(hits) == 1 && callID != "" && q == "" {
		return formatFull(&hits[0]), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "resource hits=%d（取全文用 resource_id）\n", len(hits))
	for _, h := range hits {
		snip := h.Summary
		if snip == "" {
			snip = trimRunes(h.Body, snippetRunes)
		}
		fmt.Fprintf(&b, "- id=%d · %s · %s · %s · %s · %s\n  %s\n",
			h.ID, h.Kind, h.ToolName, h.Phase, h.Status, h.RelPath, snip)
	}
	return strings.TrimSpace(b.String()), nil
}

func formatFull(r *Record) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "resource#%d · %s · %s · %s", r.ID, r.Kind, r.ToolName, r.Status)
	if r.RelPath != "" {
		fmt.Fprintf(&b, " · %s", r.RelPath)
	}
	if r.ToolCallID != "" {
		fmt.Fprintf(&b, " · call_id=%s", r.ToolCallID)
	}
	b.WriteString("\n---\n")
	b.WriteString(r.Body)
	return b.String()
}

func trimRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
