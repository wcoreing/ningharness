package session

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ningharness/store"
	"ningharness/history"
)

// Message 会话消息（工作记忆壳；正文进 ContextMessages；工具经 TaskID → tasks，禁止塞进 Content）。
type Message struct {
	Role           string `json:"role"` // user | assistant | system
	Content        string `json:"content"`
	CreatedAt      int64  `json:"createdAt"`
	TaskID         string `json:"taskId,omitempty"` // 助手回合对应 tasks 轨迹
	Seq            int    `json:"seq,omitempty"`
	InModelWindow  bool   `json:"inModelWindow"`
	HasFeedforward bool   `json:"hasFeedforward"`
}

// Session 一条 UI/编排会话。
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	OrchKey   string    `json:"orchKey"`
	UpdatedAt int64     `json:"updatedAt"`
	Messages  []Message `json:"messages"`
}

// File 磁盘/库视图格式（API 兼容）。
type File struct {
	ActiveID string    `json:"activeId"`
	Sessions []Session `json:"sessions"`
}

// Store 按项目 SQLite 会话。
type Store struct {
	mu sync.Mutex
}

func NewStore() *Store { return &Store{} }

func (s *Store) db(projectRoot string) (*sql.DB, string, error) {
	db, err := store.OpenProject(projectRoot)
	return db, store.ProjectID(projectRoot), err
}

func (s *Store) load(projectRoot, projectID string) (File, error) {
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return File{}, err
	}
	rows, err := db.Query(`SELECT id, title, orch_key, updated_at_ms FROM sessions WHERE project_id=? ORDER BY updated_at_ms ASC`, pid)
	if err != nil {
		return File{}, err
	}
	type rowSess struct {
		ID, Title, OrchKey string
		UpdatedAt          int64
	}
	var heads []rowSess
	for rows.Next() {
		var r rowSess
		if err := rows.Scan(&r.ID, &r.Title, &r.OrchKey, &r.UpdatedAt); err != nil {
			rows.Close()
			return File{}, err
		}
		heads = append(heads, r)
	}
	if err := rows.Close(); err != nil {
		return File{}, err
	}
	if err := rows.Err(); err != nil {
		return File{}, err
	}
	var sessions []Session
	for _, h := range heads {
		sess := Session{
			ID: NormalizeSessionID(h.ID), Title: h.Title,
			OrchKey: OrchKey(projectID, NormalizeSessionID(h.ID)), UpdatedAt: h.UpdatedAt,
		}
		msgs, err := loadMessages(projectRoot, sess.ID)
		if err != nil {
			return File{}, err
		}
		sess.Messages = msgs
		sessions = append(sessions, sess)
	}
	active, _ := store.MetaGet(db, pid, "active_session")
	if len(sessions) == 0 {
		now := time.Now().UnixMilli()
		sid := MainSessionID
		if _, err := db.Exec(`INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,?,?,?,?)`,
			pid, sid, "main", OrchKey(projectID, sid), now); err != nil {
			return File{}, err
		}
		_ = store.MetaSet(db, pid, "active_session", sid)
		sessions = []Session{{
			ID: sid, Title: "main", OrchKey: OrchKey(projectID, sid),
			UpdatedAt: now, Messages: []Message{},
		}}
		active = sid
	}
	if NormalizeSessionID(active) == "" {
		active = sessions[0].ID
		_ = store.MetaSet(db, pid, "active_session", active)
	}
	return File{ActiveID: NormalizeSessionID(active), Sessions: sessions}, nil
}

func loadMessages(projectRoot, sessionID string) ([]Message, error) {
	all, err := history.Load(projectRoot, sessionID)
	if err != nil {
		return nil, err
	}
	win := history.ContextWindow(all, history.DefaultBudget())
	var out []Message
	for _, m := range all {
		if !history.IsUIRole(m) {
			continue
		}
		out = append(out, Message{
			Role:           m.Role,
			Content:        history.ContentForUI(m.Content),
			CreatedAt:      m.CreatedAtMs,
			TaskID:         m.TaskID,
			Seq:            m.Seq,
			InModelWindow:  win.InWindowSeq[m.Seq],
			HasFeedforward: strings.TrimSpace(m.Role) == "user" && strings.TrimSpace(m.Feedforward) != "",
		})
	}
	return out, nil
}

func (s *Store) saveActive(db *sql.DB, projectKey, activeID string) error {
	return store.MetaSet(db, projectKey, "active_session", NormalizeSessionID(activeID))
}

func upsertSession(tx *sql.Tx, projectKey string, sess Session) error {
	_, err := tx.Exec(`INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,?,?,?,?)
		ON CONFLICT(project_id, id) DO UPDATE SET title=excluded.title, orch_key=excluded.orch_key, updated_at_ms=excluded.updated_at_ms`,
		projectKey, sess.ID, sess.Title, sess.OrchKey, sess.UpdatedAt)
	return err
}

// persistSessionsMeta 只写 sessions/active；不重写 history_message（避免 UI 视图冲掉 tool/system）。
func (s *Store) persistSessionsMeta(projectRoot, projectID string, f File) error {
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	existing := map[string]struct{}{}
	rows, err := tx.Query(`SELECT id FROM sessions WHERE project_id=?`, pid)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		existing[id] = struct{}{}
	}
	rows.Close()

	keep := map[string]struct{}{}
	for _, sess := range f.Sessions {
		sess.ID = NormalizeSessionID(sess.ID)
		sess.OrchKey = OrchKey(projectID, sess.ID)
		keep[sess.ID] = struct{}{}
		if err := upsertSession(tx, pid, sess); err != nil {
			return err
		}
	}
	for id := range existing {
		if _, ok := keep[id]; !ok {
			if _, err := tx.Exec(`DELETE FROM history_message WHERE project_id=? AND session_key=?`, pid, id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM sessions WHERE project_id=? AND id=?`, pid, id); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)
		ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, pid, "active_session", NormalizeSessionID(f.ActiveID)); err != nil {
		return err
	}
	return tx.Commit()
}

// Snapshot 读写当前文件视图。
func (s *Store) Snapshot(projectRoot, projectID string) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(projectRoot, projectID)
}

// SetActive 切换 active 会话。
func (s *Store) SetActive(projectRoot, projectID, sessionID string) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return File{}, err
	}
	sid := NormalizeSessionID(sessionID)
	found := false
	for _, sess := range f.Sessions {
		if sess.ID == sid {
			found = true
			break
		}
	}
	if !found {
		return File{}, fmt.Errorf("session not found: %s", sid)
	}
	f.ActiveID = sid
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return File{}, err
	}
	if err := s.saveActive(db, pid, sid); err != nil {
		return File{}, err
	}
	return f, nil
}

// Create 新建 sess-*。
func (s *Store) Create(projectRoot, projectID, title string) (File, Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return File{}, Session{}, err
	}
	now := time.Now().UnixMilli()
	sid := fmt.Sprintf("sess-%d", now)
	t := strings.TrimSpace(title)
	if t == "" {
		t = sid
	}
	sess := Session{
		ID: sid, Title: t, OrchKey: OrchKey(projectID, sid),
		UpdatedAt: now, Messages: []Message{},
	}
	f.Sessions = append(f.Sessions, sess)
	f.ActiveID = sid
	if err := s.persistSessionsMeta(projectRoot, projectID, f); err != nil {
		return File{}, Session{}, err
	}
	return f, sess, nil
}

// Clear 清空指定会话消息；删会话时若清的是 active 则回落 main。
func (s *Store) Clear(projectRoot, projectID, sessionID string, deleteSession bool) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return File{}, err
	}
	sid := NormalizeSessionID(sessionID)
	next := f.Sessions[:0]
	for _, sess := range f.Sessions {
		if sess.ID != sid {
			next = append(next, sess)
			continue
		}
		if deleteSession && sid != MainSessionID {
			_ = history.ClearSession(projectRoot, sid)
			continue
		}
		if err := history.ClearSession(projectRoot, sid); err != nil {
			return File{}, err
		}
		sess.Messages = []Message{}
		sess.UpdatedAt = time.Now().UnixMilli()
		next = append(next, sess)
	}
	f.Sessions = next
	if len(f.Sessions) == 0 {
		now := time.Now().UnixMilli()
		f.Sessions = []Session{{
			ID: MainSessionID, Title: "main", OrchKey: OrchKey(projectID, MainSessionID),
			UpdatedAt: now, Messages: []Message{},
		}}
	}
	still := false
	for _, sess := range f.Sessions {
		if sess.ID == f.ActiveID {
			still = true
			break
		}
	}
	if !still {
		f.ActiveID = f.Sessions[0].ID
	}
	if err := s.persistSessionsMeta(projectRoot, projectID, f); err != nil {
		return File{}, err
	}
	return s.load(projectRoot, projectID)
}

// Append 往 active（或指定）会话追加消息；写入 history_message（对话 SSOT）。
func (s *Store) Append(projectRoot, projectID, sessionID, role, content, taskID, feedforward string) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return File{}, err
	}
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return File{}, err
	}
	sid := NormalizeSessionID(sessionID)
	if sid == "" {
		sid = NormalizeSessionID(f.ActiveID)
	}
	role = strings.TrimSpace(role)
	content = strings.TrimSpace(content)
	if role == "" || content == "" {
		return File{}, fmt.Errorf("role and content required")
	}
	now := time.Now().UnixMilli()
	found := false
	for i := range f.Sessions {
		if f.Sessions[i].ID != sid {
			continue
		}
		f.Sessions[i].UpdatedAt = now
		found = true
		break
	}
	if !found {
		title := sid
		if IsHiddenSession(sid) {
			title = "queue"
		}
		f.Sessions = append(f.Sessions, Session{
			ID: sid, Title: title, OrchKey: OrchKey(projectID, sid),
			UpdatedAt: now, Messages: nil,
		})
	}
	tx, err := db.Begin()
	if err != nil {
		return File{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var sess Session
	for _, x := range f.Sessions {
		if x.ID == sid {
			sess = x
			break
		}
	}
	if err := upsertSession(tx, pid, sess); err != nil {
		return File{}, err
	}
	if err := tx.Commit(); err != nil {
		return File{}, err
	}
	ff := ""
	if role == "user" {
		ff = strings.TrimSpace(feedforward)
	}
	if err := history.Append(projectRoot, sid, history.Msg{
		Role: role, Content: content, Feedforward: ff, TaskID: strings.TrimSpace(taskID), CreatedAtMs: now,
	}); err != nil {
		return File{}, err
	}
	return s.load(projectRoot, projectID)
}

// Ensure 确保会话存在；不切换 ActiveID。
func (s *Store) Ensure(projectRoot, projectID, sessionID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return err
	}
	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return fmt.Errorf("session id required")
	}
	for _, sess := range f.Sessions {
		if sess.ID == sid {
			return nil
		}
	}
	now := time.Now().UnixMilli()
	t := strings.TrimSpace(title)
	if t == "" {
		t = sid
	}
	db, pid, err := s.db(projectRoot)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,?,?,?,?)`,
		pid, sid, t, OrchKey(projectID, sid), now)
	return err
}

// VisibleSnapshot 侧栏可见会话（过滤 once: / skill-reflect:）。
func (s *Store) VisibleSnapshot(projectRoot, projectID string) (File, error) {
	f, err := s.Snapshot(projectRoot, projectID)
	if err != nil {
		return File{}, err
	}
	vis := make([]Session, 0, len(f.Sessions))
	for _, sess := range f.Sessions {
		if IsHiddenSession(sess.ID) {
			continue
		}
		vis = append(vis, sess)
	}
	f.Sessions = vis
	if len(f.Sessions) == 0 {
		return f, nil
	}
	still := false
	for _, sess := range f.Sessions {
		if sess.ID == f.ActiveID {
			still = true
			break
		}
	}
	if !still {
		f.ActiveID = f.Sessions[0].ID
	}
	return f, nil
}

// Rewind 截断会话消息，仅保留前 keep 条 UI 可见消息（及之前的 tool/system）。
func (s *Store) Rewind(projectRoot, projectID, sessionID string, keep int) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.load(projectRoot, projectID)
	if err != nil {
		return File{}, err
	}
	sid := NormalizeSessionID(sessionID)
	if sid == "" {
		sid = NormalizeSessionID(f.ActiveID)
	}
	if keep < 0 {
		keep = 0
	}
	found := false
	for i := range f.Sessions {
		if f.Sessions[i].ID != sid {
			continue
		}
		found = true
		f.Sessions[i].UpdatedAt = time.Now().UnixMilli()
		break
	}
	if !found {
		return File{}, fmt.Errorf("session not found: %s", sid)
	}
	if err := history.TruncateAfterUIKeep(projectRoot, sid, keep); err != nil {
		return File{}, err
	}
	if err := s.persistSessionsMeta(projectRoot, projectID, f); err != nil {
		return File{}, err
	}
	return s.load(projectRoot, projectID)
}

// ActiveBrief 供 MCP / UI / Turn Snapshot 短摘要。
func (s *Store) ActiveBrief(projectRoot, projectID string) (string, error) {
	f, err := s.Snapshot(projectRoot, projectID)
	if err != nil {
		return "", err
	}
	var cur *Session
	for i := range f.Sessions {
		if f.Sessions[i].ID == NormalizeSessionID(f.ActiveID) {
			cur = &f.Sessions[i]
			break
		}
	}
	if cur == nil {
		return "no active session", nil
	}
	n := len(cur.Messages)
	last := ""
	if n > 0 {
		m := cur.Messages[n-1]
		const maxLast = 80
		c := strings.TrimSpace(m.Content)
		if utf8.RuneCountInString(c) > maxLast {
			last = fmt.Sprintf("%s：（%d字 · search_session）", m.Role, utf8.RuneCountInString(c))
		} else {
			last = m.Role + ": " + c
		}
	}
	return fmt.Sprintf("active=%s orch=%s messages=%d last=%s", cur.ID, cur.OrchKey, n, last), nil
}

// FormatThreadBrief 最近用户原话。
func (s *Store) FormatThreadBrief(projectRoot, projectID string, usersOnly bool, maxUser int) (string, error) {
	f, err := s.Snapshot(projectRoot, projectID)
	if err != nil {
		return "", err
	}
	var cur *Session
	for i := range f.Sessions {
		if f.Sessions[i].ID == NormalizeSessionID(f.ActiveID) {
			cur = &f.Sessions[i]
			break
		}
	}
	if cur == nil {
		return "no active session", nil
	}
	if maxUser <= 0 {
		maxUser = 8
	}
	head, err := s.ActiveBrief(projectRoot, projectID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString("\n\n最近消息：\n")
	msgs := cur.Messages
	start := 0
	if len(msgs) > 20 {
		start = len(msgs) - 20
	}
	shown := 0
	for _, m := range msgs[start:] {
		if usersOnly && m.Role != "user" {
			continue
		}
		if usersOnly {
			shown++
			if shown > maxUser {
				break
			}
		}
		fmt.Fprintf(&b, "- %s: %s\n", m.Role, trimRunes(m.Content, 160))
	}
	if usersOnly && shown == 0 {
		b.WriteString("- （无 user 句）\n")
	}
	return strings.TrimSpace(b.String()), nil
}

func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
