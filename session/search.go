package session

import (
	"fmt"
	"strings"

	"ningharness/history"
)

const (
	searchDefaultLimit = 12
	searchMaxLimit     = 40
	searchSnippetRunes = 160
)

// SearchOptions 会话对话检索（FTS5）。
type SearchOptions struct {
	Query     string
	Limit     int
	SessionID string // 空=全部会话
}

// Search 按 FTS5 检索 history_message。
func (s *Store) Search(projectRoot, projectID string, opt SearchOptions) (string, error) {
	_ = projectID
	q := strings.TrimSpace(opt.Query)
	if q == "" {
		return "", fmt.Errorf("query required")
	}
	limit := opt.Limit
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return "", err
	}
	ftsQ := buildFTSQuery(q)
	if ftsQ == "" {
		return "", fmt.Errorf("query empty after tokenize")
	}
	wantSess := NormalizeSessionID(opt.SessionID)
	sqlQ := `SELECT m.session_key, m.role, m.content, m.task_id, bm25(history_message_fts) AS score
		FROM history_message_fts
		JOIN history_message m ON m.id = history_message_fts.rowid
		WHERE m.project_id=? AND history_message_fts MATCH ?`
	args := []any{pid, ftsQ}
	if wantSess != "" {
		sqlQ += ` AND m.session_key = ?`
		args = append(args, wantSess)
	}
	sqlQ += ` ORDER BY score LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(sqlQ, args...)
	if err != nil {
		return s.searchLike(projectRoot, pid, q, wantSess, limit)
	}
	defer rows.Close()
	type hit struct {
		sid, role, content, taskID string
		score                      float64
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.sid, &h.role, &h.content, &h.taskID, &h.score); err != nil {
			return "", err
		}
		hits = append(hits, h)
	}
	if len(hits) == 0 {
		return s.searchLike(projectRoot, pid, q, wantSess, limit)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "session_search fts5 query=%q hits=%d（对话摘录；细节用 get_task_summary）\n", q, len(hits))
	for _, h := range hits {
		extra := ""
		if h.taskID != "" {
			extra = " task=" + h.taskID
		}
		snippet := history.ContentForUI(h.content)
		fmt.Fprintf(&b, "- [%s/%s]%s score=%.3f\n  %s\n", h.sid, h.role, extra, -h.score, trimRunes(snippet, searchSnippetRunes))
	}
	return strings.TrimSpace(b.String()), nil
}

func (s *Store) searchLike(projectRoot, pid, q, wantSess string, limit int) (string, error) {
	db, _, err := s.db(projectRoot)
	if err != nil {
		return "", err
	}
	sqlQ := `SELECT session_key, role, content, task_id FROM history_message WHERE project_id=? AND content LIKE ?`
	args := []any{pid, "%" + q + "%"}
	if wantSess != "" {
		sqlQ += ` AND session_key = ?`
		args = append(args, wantSess)
	}
	sqlQ += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(sqlQ, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var b strings.Builder
	fmt.Fprintf(&b, "session_search like query=%q hits=", q)
	n := 0
	var body strings.Builder
	for rows.Next() {
		var sid, role, content, taskID string
		if err := rows.Scan(&sid, &role, &content, &taskID); err != nil {
			return "", err
		}
		n++
		extra := ""
		if taskID != "" {
			extra = " task=" + taskID
		}
		snippet := history.ContentForUI(content)
		fmt.Fprintf(&body, "- [%s/%s]%s\n  %s\n", sid, role, extra, trimRunes(snippet, searchSnippetRunes))
	}
	fmt.Fprintf(&b, "%d（对话摘录；细节用 get_task_summary）\n", n)
	if n == 0 {
		b.WriteString("（无命中；可换关键词，或 recall_project_context 查项目文件）\n")
		return b.String(), nil
	}
	b.WriteString(body.String())
	return strings.TrimSpace(b.String()), nil
}

func buildFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	parts := strings.Fields(q)
	if len(parts) == 1 {
		p := escapeFTS(parts[0])
		if p == "" {
			return ""
		}
		return p
	}
	var out []string
	for _, p := range parts {
		p = escapeFTS(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, " OR ")
}

func escapeFTS(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	tok = strings.ReplaceAll(tok, `"`, "")
	tok = strings.ReplaceAll(tok, `'`, "")
	if tok == "" {
		return ""
	}
	if strings.ContainsAny(tok, " *^:") {
		return `"` + tok + `"`
	}
	return tok
}
