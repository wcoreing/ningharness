package lesson

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"ningharness/deskdb"
)

const (
	StatusActive     = "active"
	StatusSuperseded = "superseded"
	StatusExpired    = "expired"

	ScopeSkill    = "skill"
	ScopeProject  = "project"
	ScopePersonal = "personal"

	// PersonalProjectID personal scope 行挂在全局库哨兵项目下（与项目绝对路径隔离）。
	PersonalProjectID = "_personal"

	metaImported   = "lessons_db_imported"
	InjectMaxRunes = 900
	skillsRoot     = "system/skills"
	lessonsFile    = "LESSONS.md"
)

// Anchors 挂点 JSON（skill scope 必有 skillId）。
type Anchors struct {
	SkillID string `json:"skillId,omitempty"`
}

type Entry struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"projectId"`
	Scope            string  `json:"scope"`
	Anchors          Anchors `json:"anchors"`
	SkillID          string  `json:"skillId,omitempty"` // 兼容：= anchors.skillId
	Body             string  `json:"body"`
	Status           string  `json:"status"`
	SourceTaskID     string  `json:"sourceTaskId,omitempty"`
	ParentTaskID     string  `json:"parentTaskId,omitempty"`
	SourceSessionKey string  `json:"sourceSessionKey,omitempty"`
	SupersedesID     string  `json:"supersedesId,omitempty"`
	AckedAtMs        int64   `json:"ackedAtMs,omitempty"`
	CreatedAtMs      int64   `json:"createdAtMs"`
	UpdatedAtMs      int64   `json:"updatedAtMs,omitempty"`
}

type AppendInput struct {
	Root             string
	Scope            string // skill|project|personal；空=skill
	SkillID          string // scope=skill 必填
	Body             string
	SourceTaskID     string
	ParentTaskID     string
	SourceSessionKey string
	SupersedesID     string
}

func Append(in AppendInput) (Entry, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return Entry{}, fmt.Errorf("lesson: empty body")
	}
	scope := normalizeScope(in.Scope)
	if scope == "" {
		scope = ScopeSkill
	}
	skillID := strings.TrimSpace(in.SkillID)
	anchors := Anchors{}
	switch scope {
	case ScopeSkill:
		if skillID == "" {
			return Entry{}, fmt.Errorf("lesson: skill scope requires skill")
		}
		anchors.SkillID = skillID
	case ScopeProject, ScopePersonal:
		// anchors 可空
	default:
		return Entry{}, fmt.Errorf("lesson: bad scope")
	}

	var db *sql.DB
	var pid string
	var err error
	if scope == ScopePersonal {
		db, err = deskdb.Open()
		if err != nil {
			return Entry{}, err
		}
		pid = PersonalProjectID
	} else {
		root := strings.TrimSpace(in.Root)
		if root == "" {
			return Entry{}, fmt.Errorf("lesson: empty root")
		}
		db, pid, err = open(root)
		if err != nil {
			return Entry{}, err
		}
	}

	now := time.Now().UnixMilli()
	e := Entry{
		ID:               newID(),
		ProjectID:        pid,
		Scope:            scope,
		Anchors:          anchors,
		SkillID:          anchors.SkillID,
		Body:             body,
		Status:           StatusActive,
		SourceTaskID:     strings.TrimSpace(in.SourceTaskID),
		ParentTaskID:     strings.TrimSpace(in.ParentTaskID),
		SourceSessionKey: strings.TrimSpace(in.SourceSessionKey),
		SupersedesID:     strings.TrimSpace(in.SupersedesID),
		CreatedAtMs:      now,
		UpdatedAtMs:      now,
	}
	aj, err := json.Marshal(anchors)
	if err != nil {
		return Entry{}, err
	}
	_, err = db.Exec(`INSERT INTO lesson_entry(
		id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.ProjectID, e.Scope, string(aj), e.Body, e.Status,
		e.SourceTaskID, e.ParentTaskID, e.SourceSessionKey, e.SupersedesID,
		0, e.CreatedAtMs, e.UpdatedAtMs)
	if err != nil {
		return Entry{}, err
	}
	if e.SupersedesID != "" {
		_ = SetStatus(in.Root, e.SupersedesID, StatusSuperseded)
	}
	return e, nil
}

func SetStatus(root, id, status string) error {
	status = normalizeStatus(status)
	if status == "" {
		return fmt.Errorf("lesson: bad status")
	}
	return mutateByID(root, id, func(db *sql.DB, pid string) error {
		res, err := db.Exec(`UPDATE lesson_entry SET status=?, updated_at_ms=?
			WHERE id=? AND (project_id=? OR project_id=?)`,
			status, time.Now().UnixMilli(), strings.TrimSpace(id), pid, PersonalProjectID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("lesson: not found %s", strings.TrimSpace(id))
		}
		return nil
	})
}

func Ack(root, id string) error {
	return mutateByID(root, id, func(db *sql.DB, pid string) error {
		now := time.Now().UnixMilli()
		res, err := db.Exec(`UPDATE lesson_entry SET acked_at_ms=?, updated_at_ms=?
			WHERE id=? AND (project_id=? OR project_id=?)`,
			now, now, strings.TrimSpace(id), pid, PersonalProjectID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("lesson: not found %s", strings.TrimSpace(id))
		}
		return nil
	})
}

func ListActive(root string, skillIDs []string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	db, pid, err := open(root)
	if err != nil {
		return nil, err
	}
	want := map[string]struct{}{}
	for _, id := range skillIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}
	rows, err := db.Query(`SELECT id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
		FROM lesson_entry WHERE project_id=? AND status=? AND scope=? ORDER BY created_at_ms DESC`,
		pid, StatusActive, ScopeSkill)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		if len(want) > 0 {
			if _, ok := want[e.SkillID]; !ok {
				continue
			}
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

func ListBySkill(root, skillID string) ([]Entry, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil, fmt.Errorf("lesson: empty skill")
	}
	db, pid, err := open(root)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
		FROM lesson_entry WHERE project_id=? AND scope=? AND json_extract(anchors,'$.skillId')=?
		ORDER BY created_at_ms ASC`,
		pid, ScopeSkill, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func ListByScope(root, scope string, limit int) ([]Entry, error) {
	scope = normalizeScope(scope)
	if scope == "" {
		return nil, fmt.Errorf("lesson: bad scope")
	}
	if limit <= 0 {
		limit = 100
	}
	var db *sql.DB
	var pid string
	var err error
	if scope == ScopePersonal {
		db, err = deskdb.Open()
		if err != nil {
			return nil, err
		}
		pid = PersonalProjectID
	} else {
		db, pid, err = open(root)
		if err != nil {
			return nil, err
		}
	}
	rows, err := db.Query(`SELECT id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
		FROM lesson_entry WHERE project_id=? AND scope=? ORDER BY created_at_ms DESC LIMIT ?`,
		pid, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func ListActivePersonal(limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	db, err := deskdb.Open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
		FROM lesson_entry WHERE project_id=? AND scope=? AND status=? AND acked_at_ms>0
		ORDER BY created_at_ms DESC LIMIT ?`,
		PersonalProjectID, ScopePersonal, StatusActive, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func ListActiveProject(root string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	db, pid, err := open(root)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, project_id, scope, anchors, body, status,
		source_task_id, parent_task_id, source_session_key, supersedes_id,
		acked_at_ms, created_at_ms, updated_at_ms
		FROM lesson_entry WHERE project_id=? AND scope=? AND status=?
		ORDER BY created_at_ms DESC LIMIT ?`,
		pid, ScopeProject, StatusActive, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func HasAny(root, skillID string) bool {
	list, err := ListBySkill(root, skillID)
	return err == nil && len(list) > 0
}

func CountProject(root string) int {
	db, pid, err := open(root)
	if err != nil {
		return 0
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM lesson_entry WHERE project_id=?`, pid).Scan(&n)
	return n
}

func InjectBrief(root string, skillIDs []string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = InjectMaxRunes
	}
	_ = EnsureImported(root)

	var b strings.Builder
	used := 0
	writeSection := func(title string, entries []Entry) {
		if len(entries) == 0 {
			return
		}
		var picked []Entry
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			line := formatInjectLine(e)
			n := utf8.RuneCountInString(line) + 1
			if used+n > maxRunes && len(picked) > 0 {
				break
			}
			picked = append(picked, e)
			used += n
			if used >= maxRunes {
				break
			}
		}
		if len(picked) == 0 {
			return
		}
		for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
			picked[i], picked[j] = picked[j], picked[i]
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(title)
		b.WriteByte('\n')
		for _, e := range picked {
			b.WriteString(formatInjectLine(e))
			b.WriteByte('\n')
		}
	}

	personal, _ := ListActivePersonal(8)
	writeSection("## 个人经验（lesson_entry · personal · 已认账）", personal)

	proj, _ := ListActiveProject(root, 8)
	writeSection("## 项目经验（lesson_entry · project · active）", proj)

	if len(skillIDs) == 0 {
		n := CountProject(root)
		if b.Len() == 0 && n == 0 {
			return ""
		}
		if b.Len() == 0 {
			return fmt.Sprintf("## 项目经验\n%d 条 lesson_entry；本轮无路径匹配 skill，**不注入 skill 摘录**。需要时 list_lessons / get_skill。", n)
		}
		if n > 0 {
			fmt.Fprintf(&b, "\n（另有项目 skill 经验；本轮无路径匹配，不注入 skill 摘录 · list_lessons / get_skill）\n")
		}
		return strings.TrimSpace(b.String())
	}

	entries, err := ListActive(root, skillIDs, 40)
	if err != nil || len(entries) == 0 {
		if b.Len() == 0 {
			return "## 项目经验\n匹配 skill 无 active 条目；全文 list_lessons / get_skill。"
		}
		fmt.Fprintf(&b, "\n## Skill 经验\n匹配 skill 无 active 条目。\n")
		return strings.TrimSpace(b.String())
	}
	pctBudget := used
	_ = pctBudget
	title := fmt.Sprintf("## Skill 经验（lesson_entry · skill · 仅 active）· 匹配 `%s`", strings.Join(skillIDs, "` · `"))
	writeSection(title, entries)
	return strings.TrimSpace(b.String())
}

func FormatForSkillBody(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 项目经验（lesson_entry · 全文供审；前馈只采信 active）\n\n")
	for _, e := range entries {
		ack := "未认账"
		if e.AckedAtMs > 0 {
			ack = "已认账"
		}
		fmt.Fprintf(&b, "### %s · %s · %s · %s\n\n", e.ID, e.Scope, e.Status, ack)
		if e.SourceTaskID != "" || e.ParentTaskID != "" {
			fmt.Fprintf(&b, "source_task: %s", e.SourceTaskID)
			if e.ParentTaskID != "" {
				fmt.Fprintf(&b, " · parent_task: %s", e.ParentTaskID)
			}
			b.WriteString("\n\n")
		}
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func ExportMarkdown(entries []Entry) string {
	var b strings.Builder
	b.WriteString("# LESSONS\n\n")
	for _, e := range entries {
		if e.Scope != "" && e.Scope != ScopeSkill {
			continue
		}
		st := e.Status
		if st == "" {
			st = StatusActive
		}
		ts := time.UnixMilli(e.CreatedAtMs).Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "## %s · %s\n\n", ts, st)
		fmt.Fprintf(&b, "<!-- lesson:%s -->\n", e.ID)
		if e.SourceTaskID != "" {
			fmt.Fprintf(&b, "source_task: %s\n", e.SourceTaskID)
		}
		if e.ParentTaskID != "" {
			fmt.Fprintf(&b, "parent_task: %s\n", e.ParentTaskID)
		}
		if e.SupersedesID != "" {
			fmt.Fprintf(&b, "supersedes: %s\n", e.SupersedesID)
		}
		b.WriteString("\n")
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	return b.String()
}

func ImportFromLessonsFile(root, skillID, md string) (int, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return 0, fmt.Errorf("lesson: empty skill")
	}
	entries := parseLegacyMarkdown(md)
	if len(entries) == 0 {
		return 0, nil
	}
	db, pid, err := open(root)
	if err != nil {
		return 0, err
	}
	n := 0
	now := time.Now().UnixMilli()
	aj, _ := json.Marshal(Anchors{SkillID: skillID})
	for _, e := range entries {
		id := e.ID
		if id == "" {
			id = newID()
		}
		res, err := db.Exec(`INSERT OR IGNORE INTO lesson_entry(
			id, project_id, scope, anchors, body, status,
			source_task_id, parent_task_id, source_session_key, supersedes_id,
			acked_at_ms, created_at_ms, updated_at_ms
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, pid, ScopeSkill, string(aj), e.Body, e.Status,
			e.SourceTaskID, e.ParentTaskID, "", e.SupersedesID,
			0, now, now)
		if err != nil {
			return n, err
		}
		if aff, _ := res.RowsAffected(); aff > 0 {
			n++
		}
	}
	return n, nil
}

func EnsureImported(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	db, err := deskdb.OpenProject(root)
	if err != nil {
		return err
	}
	pid := deskdb.ProjectID(root)
	v, err := deskdb.MetaGet(db, pid, metaImported)
	if err == nil && v == "1" {
		return nil
	}
	base := filepath.Join(root, skillsRoot)
	ents, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return deskdb.MetaSet(db, pid, metaImported, "1")
		}
		return err
	}
	for _, ent := range ents {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(base, ent.Name(), lessonsFile))
		if err != nil {
			continue
		}
		if _, err := ImportFromLessonsFile(root, ent.Name(), string(raw)); err != nil {
			return err
		}
	}
	return deskdb.MetaSet(db, pid, metaImported, "1")
}

func open(root string) (*sql.DB, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, "", fmt.Errorf("lesson: empty root")
	}
	db, err := deskdb.OpenProject(root)
	if err != nil {
		return nil, "", err
	}
	return db, deskdb.ProjectID(root), nil
}

func mutateByID(root, id string, fn func(db *sql.DB, pid string) error) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("lesson: empty id")
	}
	root = strings.TrimSpace(root)
	var db *sql.DB
	var pid string
	var err error
	if root != "" {
		db, pid, err = open(root)
	} else {
		db, err = deskdb.Open()
		pid = PersonalProjectID
	}
	if err != nil {
		return err
	}
	return fn(db, pid)
}

func scanAll(rows *sql.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanEntry(rows *sql.Rows) (Entry, error) {
	var e Entry
	var anchorsRaw string
	err := rows.Scan(
		&e.ID, &e.ProjectID, &e.Scope, &anchorsRaw, &e.Body, &e.Status,
		&e.SourceTaskID, &e.ParentTaskID, &e.SourceSessionKey, &e.SupersedesID,
		&e.AckedAtMs, &e.CreatedAtMs, &e.UpdatedAtMs,
	)
	if err != nil {
		return e, err
	}
	_ = json.Unmarshal([]byte(anchorsRaw), &e.Anchors)
	e.SkillID = e.Anchors.SkillID
	if e.Scope == "" {
		e.Scope = ScopeSkill
	}
	return e, nil
}

func newID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return "les_" + hex.EncodeToString(b[:])
}

func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StatusActive, StatusSuperseded, StatusExpired:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func normalizeScope(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ScopeSkill, ScopeProject, ScopePersonal:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func formatInjectLine(e Entry) string {
	snip := trimRunes(e.Body, 220)
	ack := ""
	if e.AckedAtMs == 0 {
		ack = " · 未认账"
	}
	label := e.Scope
	if e.SkillID != "" {
		label = e.SkillID
	}
	return fmt.Sprintf("- `%s` · %s%s：%s", label, e.ID, ack, snip)
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}
