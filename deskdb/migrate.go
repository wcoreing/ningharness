package deskdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	migratedJSONFlag = "json_migrated_v2"
	mergedLegacyFlag = "legacy_db_merged_v2"
)

func isEphemeralRoot(root string) bool {
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil && resolved != "" {
		abs = resolved
	}
	slash := filepath.ToSlash(abs)
	// go test TempDir：macOS …/T/…；Linux /tmp/…
	if strings.Contains(slash, "/T/") || strings.Contains(slash, "/tmp/") || strings.Contains(slash, "/Temp/") {
		return true
	}
	tmp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil && resolved != "" {
		tmp = resolved
	}
	sep := string(filepath.Separator)
	return abs == tmp || strings.HasPrefix(abs, tmp+sep)
}

// OpenProject 唯一库（生产 ~/.agentdesk/desk.db；测试 TempDir 内 desk.db）。
func openProjectDB(root string) (*sql.DB, error) {
	if isEphemeralRoot(root) {
		// 测试：库落在项目 .agentdesk/desk.db（隔离且被 gitignore）
		return OpenAt(filepath.Join(root, ".agentdesk"))
	}
	return Open()
}

func migrateProjectInto(db *sql.DB, root, pid string) error {
	if err := migrateLegacyProjectDB(db, root, pid); err != nil {
		return err
	}
	if err := migrateProjectJSON(db, root, pid); err != nil {
		return err
	}
	return PromoteMetaBlobs(db)
}

func migrateLegacyProjectDB(db *sql.DB, root, pid string) error {
	// 测试库就在 project/.agentdesk/desk.db，无独立旧库可迁。
	if isEphemeralRoot(root) {
		return nil
	}
	legacy := ProjectPath(root)
	if !fileExists(legacy) {
		return nil
	}
	if v, _ := MetaGet(db, pid, mergedLegacyFlag); v == "1" {
		_ = os.Remove(legacy)
		_ = os.Remove(legacy + "-wal")
		_ = os.Remove(legacy + "-shm")
		return nil
	}
	// 避免 ATTACH 自己（含测试：库就在 project/.agentdesk/desk.db）
	if abs, err := filepath.Abs(legacy); err == nil {
		var mainFile string
		_ = db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&mainFile)
		if mainFile != "" {
			if ma, err := filepath.Abs(mainFile); err == nil && ma == abs {
				return MetaSet(db, pid, mergedLegacyFlag, "1")
			}
		}
		if p, err := Path(); err == nil {
			if pa, err := filepath.Abs(p); err == nil && pa == abs {
				return MetaSet(db, pid, mergedLegacyFlag, "1")
			}
		}
	}

	if _, err := db.Exec(`ATTACH DATABASE ? AS leg`, legacy); err != nil {
		// 旧库损坏则跳过
		_ = MetaSet(db, pid, mergedLegacyFlag, "1")
		_ = os.Remove(legacy)
		return nil
	}
	defer func() { _, _ = db.Exec(`DETACH DATABASE leg`) }()

	// 旧库可能无 project_id
	copySQL := []struct {
		check string
		exec  string
	}{
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='sessions' LIMIT 1`,
			`INSERT OR IGNORE INTO sessions(project_id, id, title, orch_key, updated_at_ms)
			 SELECT ?, id, title, orch_key, updated_at_ms FROM leg.sessions`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='history_message' LIMIT 1`,
			`INSERT OR IGNORE INTO history_message(project_id, session_key, seq, role, content, tool_call_id, tool_calls_json, task_id, created_at_ms)
			 SELECT ?, session_key, seq, role, content, IFNULL(tool_call_id,''), IFNULL(tool_calls_json,''), '', created_at_ms FROM leg.history_message`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='messages' LIMIT 1`,
			`INSERT INTO history_message(project_id, session_key, seq, role, content, tool_call_id, tool_calls_json, task_id, created_at_ms)
			 SELECT ?, session_id, seq, role, content, '', '', '', created_at_ms
			 FROM leg.messages m
			 WHERE role IN ('user','assistant')
			 AND NOT EXISTS (
			   SELECT 1 FROM history_message h WHERE h.project_id=? AND h.session_key=m.session_id AND h.role IN ('user','assistant') AND IFNULL(h.content,'')!=''
			 )`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='runs' LIMIT 1`,
			`INSERT OR IGNORE INTO tasks(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count)
			 SELECT ?, id, started_at_ms, ended_at_ms, driver, session_id, purpose, task_id, status, error, 0 FROM leg.runs`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='queue_tasks' LIMIT 1`,
			`INSERT OR IGNORE INTO jobs(project_id, id, type, title, prompt, driver, model, target_rel, status, task_id, error, last_error, retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose, step_done, step_total, progress_hint, sort_ord)
			 SELECT ?, id, type, title, prompt, driver, model, target_rel, status, run_id, error, last_error, retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose, step_done, step_total, progress_hint, sort_ord FROM leg.queue_tasks`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='queue_steps' LIMIT 1`,
			`INSERT OR IGNORE INTO job_steps(project_id, job_id, idx, rel, title, status, task_id, error)
			 SELECT ?, task_id, idx, rel, title, status, run_id, error FROM leg.queue_steps`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='pins' LIMIT 1`,
			`INSERT OR IGNORE INTO pins(project_id, path, sort_ord) SELECT ?, path, sort_ord FROM leg.pins`},
		{`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='meta' LIMIT 1`,
			`INSERT OR IGNORE INTO meta(project_id, key, value) SELECT ?, key, value FROM leg.meta`},
	}
	for _, c := range copySQL {
		var one int
		if err := db.QueryRow(c.check).Scan(&one); err != nil {
			continue
		}
		// messages→history 需要两个 pid 占位
		if strings.Contains(c.exec, "NOT EXISTS") {
			if _, err := db.Exec(c.exec, pid, pid); err != nil {
				_, _ = db.Exec(`DETACH DATABASE leg`)
				return fmt.Errorf("copy legacy: %w", err)
			}
			continue
		}
		if _, err := db.Exec(c.exec, pid); err != nil {
			_, _ = db.Exec(`DETACH DATABASE leg`)
			return fmt.Errorf("copy legacy: %w", err)
		}
	}
	// v9+：不再迁 run_events / timeline_*（执行步骤在 history_message；文件打点在 filegit）
	_ = copyLegacyRunEvents
	_ = copyLegacyTimelines
	_, _ = db.Exec(`DETACH DATABASE leg`)
	_ = MetaSet(db, pid, mergedLegacyFlag, "1")
	_ = os.Remove(legacy)
	_ = os.Remove(legacy + "-wal")
	_ = os.Remove(legacy + "-shm")
	return nil
}

func copyLegacyRunEvents(db *sql.DB, pid string) error {
	var one int
	if err := db.QueryRow(`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='runs' LIMIT 1`).Scan(&one); err != nil {
		return nil
	}
	hasTrail := false
	cols, err := db.Query(`PRAGMA leg.table_info(runs)`)
	if err == nil {
		for cols.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				_ = cols.Close()
				return err
			}
			if name == "trail_json" {
				hasTrail = true
			}
		}
		_ = cols.Close()
	}
	if hasTrail {
		rows, err := db.Query(`SELECT id, trail_json FROM leg.runs`)
		if err != nil {
			return err
		}
		type pair struct{ id, trail string }
		var list []pair
		for rows.Next() {
			var p pair
			if err := rows.Scan(&p.id, &p.trail); err != nil {
				_ = rows.Close()
				return err
			}
			list = append(list, p)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		now := time.Now().UnixMilli()
		for _, p := range list {
			if err := replaceRunEventsTx(tx, pid, p.id, p.trail, now); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	if err := db.QueryRow(`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='run_events' LIMIT 1`).Scan(&one); err != nil {
		return nil
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO run_events(project_id, run_id, seq, kind, payload_json, created_at_ms)
		SELECT ?, run_id, seq, kind, payload_json, created_at_ms FROM leg.run_events`, pid)
	return err
}

func copyLegacyTimelines(db *sql.DB, pid string) error {
	var one int
	if err := db.QueryRow(`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='file_timeline' LIMIT 1`).Scan(&one); err == nil {
		rows, err := db.Query(`SELECT rel, payload_json FROM leg.file_timeline`)
		if err != nil {
			return err
		}
		type pair struct{ rel, payload string }
		var list []pair
		for rows.Next() {
			var p pair
			if err := rows.Scan(&p.rel, &p.payload); err != nil {
				_ = rows.Close()
				return err
			}
			list = append(list, p)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		for _, p := range list {
			if err := replaceTimelineFromPayloadTx(tx, pid, p.rel, p.payload); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	if err := db.QueryRow(`SELECT 1 FROM leg.sqlite_master WHERE type='table' AND name='timeline_rounds' LIMIT 1`).Scan(&one); err != nil {
		return nil
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO timeline_rounds(project_id, rel, round_ord, baseline, baseline_short, ended_at)
		SELECT ?, rel, round_ord, baseline, baseline_short, ended_at FROM leg.timeline_rounds`, pid); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT OR IGNORE INTO timeline_reviews(project_id, rel, round_ord, review_ord, hash, short, subject, when_at, idx, accepted, discarded)
		SELECT ?, rel, round_ord, review_ord, hash, short, subject, when_at, idx, accepted, discarded FROM leg.timeline_reviews`, pid)
	return err
}

func migrateProjectJSON(db *sql.DB, root, pid string) error {
	if v, _ := MetaGet(db, pid, migratedJSONFlag); v == "1" {
		return nil
	}
	hasJSON := projectHasLegacyJSON(root)
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE project_id=?`, pid).Scan(&n)
	if n > 0 && !hasJSON {
		return MetaSet(db, pid, migratedJSONFlag, "1")
	}
	if !hasJSON {
		return MetaSet(db, pid, migratedJSONFlag, "1")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := migrateSessionsTx(tx, root, pid); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	if err := migrateRunsTx(tx, root, pid); err != nil {
		return fmt.Errorf("runs: %w", err)
	}
	if err := migrateQueueTx(tx, root, pid); err != nil {
		return fmt.Errorf("queue: %w", err)
	}
	if err := migratePinsTx(tx, root, pid); err != nil {
		return fmt.Errorf("pins: %w", err)
	}
	if err := migrateReflectTx(tx, root, pid); err != nil {
		return fmt.Errorf("reflect: %w", err)
	}
	if err := migrateReviewTx(tx, root, pid); err != nil {
		return fmt.Errorf("review: %w", err)
	}
	if err := migrateTimelineTx(tx, root, pid); err != nil {
		return fmt.Errorf("timeline: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)
		ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, pid, migratedJSONFlag, "1"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	removeProjectLegacyJSON(root)
	return nil
}

func projectHasLegacyJSON(root string) bool {
	paths := []string{
		filepath.Join(root, ".agentdesk", "sessions.json"),
		filepath.Join(root, ".agentdesk", "runs-index.json"),
		filepath.Join(root, ".agentdesk", "queue.json"),
		filepath.Join(root, ".agentdesk", "pins.json"),
		filepath.Join(root, ".agentdesk", "last-reflect.json"),
		filepath.Join(root, ".agentdesk", "review-session.json"),
	}
	for _, p := range paths {
		if fileExists(p) {
			return true
		}
	}
	for _, sub := range []string{"runs", "file-timeline"} {
		dir := filepath.Join(root, ".agentdesk", sub)
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				return true
			}
		}
	}
	return false
}

func removeProjectLegacyJSON(root string) {
	_ = os.Remove(filepath.Join(root, ".agentdesk", "sessions.json"))
	_ = os.Remove(filepath.Join(root, ".agentdesk", "runs-index.json"))
	_ = os.Remove(filepath.Join(root, ".agentdesk", "queue.json"))
	_ = os.Remove(filepath.Join(root, ".agentdesk", "pins.json"))
	_ = os.Remove(filepath.Join(root, ".agentdesk", "last-reflect.json"))
	_ = os.Remove(filepath.Join(root, ".agentdesk", "review-session.json"))
	_ = os.RemoveAll(filepath.Join(root, ".agentdesk", "runs"))
	_ = os.RemoveAll(filepath.Join(root, ".agentdesk", "file-timeline"))
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func migrateSessionsTx(tx *sql.Tx, root, pid string) error {
	p := filepath.Join(root, ".agentdesk", "sessions.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f struct {
		ActiveID string `json:"activeId"`
		Sessions []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			OrchKey   string `json:"orchKey"`
			UpdatedAt int64  `json:"updatedAt"`
			Messages  []struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				CreatedAt int64  `json:"createdAt"`
				RunID     string `json:"runId"`
			} `json:"messages"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	for _, sess := range f.Sessions {
		id := strings.TrimSpace(sess.ID)
		if id == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,?,?,?,?)
			ON CONFLICT(project_id, id) DO UPDATE SET title=excluded.title, orch_key=excluded.orch_key, updated_at_ms=excluded.updated_at_ms`,
			pid, id, sess.Title, sess.OrchKey, sess.UpdatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM history_message WHERE project_id=? AND session_key=?`, pid, id); err != nil {
			return err
		}
		for i, m := range sess.Messages {
			role := strings.TrimSpace(m.Role)
			if role != "user" && role != "assistant" {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO history_message(project_id, session_key, seq, role, content, tool_call_id, tool_calls_json, task_id, created_at_ms) VALUES(?,?,?,?,?,'','',?,?)`,
				pid, id, i+1, role, m.Content, m.RunID, m.CreatedAt); err != nil {
				return err
			}
		}
	}
	if aid := strings.TrimSpace(f.ActiveID); aid != "" {
		if _, err := tx.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)
			ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, pid, "active_session", aid); err != nil {
			return err
		}
	}
	return nil
}

func migrateRunsTx(tx *sql.Tx, root, pid string) error {
	dir := filepath.Join(root, ".agentdesk", "runs")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec struct {
			ID          string `json:"id"`
			StartedAtMs int64  `json:"startedAtMs"`
			EndedAtMs   int64  `json:"endedAtMs"`
			Driver      string `json:"driver"`
			SessionID   string `json:"sessionId"`
			Purpose     string `json:"purpose"`
			TaskID      string `json:"taskId"`
			Prompt      string `json:"prompt"`
			Reply       string `json:"reply"`
			Status      string `json:"status"`
			Error       string `json:"error"`
			Tools       []struct {
				Name   string `json:"name"`
				OK     bool   `json:"ok"`
				Detail string `json:"detail"`
				Path   string `json:"path"`
			} `json:"tools"`
			Trail json.RawMessage `json:"trail"`
		}
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		id := strings.TrimSpace(rec.ID)
		if id == "" {
			continue
		}
		trail := "[]"
		if len(rec.Trail) > 0 {
			trail = string(rec.Trail)
		}
		toolCount := 0
		if len(rec.Tools) > 0 {
			toolCount = len(rec.Tools)
		} else if len(rec.Trail) > 0 {
			var steps []struct {
				Kind string `json:"kind"`
			}
			if err := json.Unmarshal(rec.Trail, &steps); err == nil {
				for _, s := range steps {
					if s.Kind == "tool" {
						toolCount++
					}
				}
			}
		}
		if _, err := tx.Exec(`INSERT INTO tasks(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(project_id, id) DO UPDATE SET
				started_at_ms=excluded.started_at_ms, ended_at_ms=excluded.ended_at_ms, driver=excluded.driver,
				session_id=excluded.session_id, purpose=excluded.purpose, job_id=excluded.job_id,
				status=excluded.status, error=excluded.error, tool_count=excluded.tool_count`,
			pid, id, rec.StartedAtMs, rec.EndedAtMs, rec.Driver, rec.SessionID, rec.Purpose, rec.TaskID,
			rec.Status, rec.Error, toolCount); err != nil {
			return err
		}
		_ = trail // 旧 trail 不再写入 tasks；过程行由运行时写入 history_message
	}
	return nil
}

func migrateQueueTx(tx *sql.Tx, root, pid string) error {
	p := filepath.Join(root, ".agentdesk", "queue.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f struct {
		Paused       bool              `json:"paused"`
		PauseOnError bool              `json:"pauseOnError"`
		PauseReason  string            `json:"pauseReason"`
		MaxParallel  int               `json:"maxParallel"`
		Tasks        []json.RawMessage `json:"tasks"`
		History      []json.RawMessage `json:"history"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	metas := map[string]string{
		"queue_paused":         boolStr(f.Paused),
		"queue_pause_on_error": boolStr(f.PauseOnError),
		"queue_pause_reason":   f.PauseReason,
		"queue_max_parallel":   fmt.Sprintf("%d", f.MaxParallel),
	}
	for k, v := range metas {
		if _, err := tx.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)
			ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, pid, k, v); err != nil {
			return err
		}
	}
	all := append([]json.RawMessage{}, f.Tasks...)
	all = append(all, f.History...)
	for i, rawT := range all {
		var t queueTaskJSON
		if err := json.Unmarshal(rawT, &t); err != nil || strings.TrimSpace(t.ID) == "" {
			continue
		}
		if err := upsertQueueTaskTx(tx, pid, t, i); err != nil {
			return err
		}
	}
	return nil
}

type queueTaskJSON struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Prompt       string `json:"prompt"`
	Driver       string `json:"driver"`
	Model        string `json:"model"`
	TargetRel    string `json:"targetRel"`
	Status       string `json:"status"`
	RunID        string `json:"runId"`
	Error        string `json:"error"`
	LastError    string `json:"lastError"`
	RetryCount   int    `json:"retryCount"`
	BatchID      string `json:"batchId"`
	CreatedAt    int64  `json:"createdAt"`
	StartedAt    int64  `json:"startedAt"`
	FinishedAt   int64  `json:"finishedAt"`
	SessionKey   string `json:"sessionKey"`
	Purpose      string `json:"purpose"`
	StepDone     int    `json:"stepDone"`
	StepTotal    int    `json:"stepTotal"`
	ProgressHint string `json:"progressHint"`
	Steps        []struct {
		Rel    string `json:"rel"`
		Title  string `json:"title"`
		Status string `json:"status"`
		RunID  string `json:"runId"`
		Error  string `json:"error"`
	} `json:"steps"`
}

func upsertQueueTaskTx(tx *sql.Tx, pid string, t queueTaskJSON, ord int) error {
	if _, err := tx.Exec(`INSERT INTO jobs(
		project_id, id, type, title, prompt, driver, model, target_rel, status, task_id, error, last_error,
		retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose,
		step_done, step_total, progress_hint, sort_ord)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(project_id, id) DO UPDATE SET
			type=excluded.type, title=excluded.title, prompt=excluded.prompt, driver=excluded.driver,
			model=excluded.model, target_rel=excluded.target_rel, status=excluded.status, task_id=excluded.task_id,
			error=excluded.error, last_error=excluded.last_error, retry_count=excluded.retry_count,
			batch_id=excluded.batch_id, created_at=excluded.created_at, started_at=excluded.started_at,
			finished_at=excluded.finished_at, session_key=excluded.session_key, purpose=excluded.purpose,
			step_done=excluded.step_done, step_total=excluded.step_total, progress_hint=excluded.progress_hint,
			sort_ord=excluded.sort_ord`,
		pid, t.ID, t.Type, t.Title, t.Prompt, t.Driver, t.Model, t.TargetRel, t.Status, t.RunID, t.Error, t.LastError,
		t.RetryCount, t.BatchID, t.CreatedAt, t.StartedAt, t.FinishedAt, t.SessionKey, t.Purpose,
		t.StepDone, t.StepTotal, t.ProgressHint, ord); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM job_steps WHERE project_id=? AND job_id=?`, pid, t.ID); err != nil {
		return err
	}
	for i, s := range t.Steps {
		if _, err := tx.Exec(`INSERT INTO job_steps(project_id, job_id, idx, rel, title, status, task_id, error) VALUES(?,?,?,?,?,?,?,?)`,
			pid, t.ID, i, s.Rel, s.Title, s.Status, s.RunID, s.Error); err != nil {
			return err
		}
	}
	return nil
}

func migratePinsTx(tx *sql.Tx, root, pid string) error {
	p := filepath.Join(root, ".agentdesk", "pins.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f struct {
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	for i, path := range f.Paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO pins(project_id, path, sort_ord) VALUES(?,?,?)
			ON CONFLICT(project_id, path) DO UPDATE SET sort_ord=excluded.sort_ord`, pid, path, i); err != nil {
			return err
		}
	}
	return nil
}

func migrateReflectTx(tx *sql.Tx, root, pid string) error {
	p := filepath.Join(root, ".agentdesk", "last-reflect.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return upsertGrowthReflectTx(tx, pid, string(raw))
}

func migrateReviewTx(tx *sql.Tx, root, pid string) error {
	p := filepath.Join(root, ".agentdesk", "review-session.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return upsertReviewSessionTx(tx, pid, string(raw))
}

func migrateTimelineTx(tx *sql.Tx, root, pid string) error {
	dir := filepath.Join(root, ".agentdesk", "file-timeline")
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var idx struct {
			Rel string `json:"rel"`
		}
		_ = json.Unmarshal(raw, &idx)
		rel := strings.TrimSpace(idx.Rel)
		if rel == "" {
			rel = strings.TrimSuffix(e.Name(), ".json")
		}
		if err := replaceTimelineFromPayloadTx(tx, pid, rel, string(raw)); err != nil {
			return err
		}
	}
	return nil
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func migrateHomeJSON(db *sql.DB) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	base := filepath.Join(home, ".agentdesk")

	if fileExists(filepath.Join(base, "settings.json")) {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM settings_blob`).Scan(&n)
		if n == 0 {
			if raw, err := os.ReadFile(filepath.Join(base, "settings.json")); err == nil {
				if _, err := db.Exec(`INSERT INTO settings_blob(id, json) VALUES(1, ?)
					ON CONFLICT(id) DO UPDATE SET json=excluded.json`, string(raw)); err != nil {
					return err
				}
				_ = os.Remove(filepath.Join(base, "settings.json"))
			}
		}
	}
	if fileExists(filepath.Join(base, "state.json")) {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM app_state`).Scan(&n)
		if n == 0 {
			if raw, err := os.ReadFile(filepath.Join(base, "state.json")); err == nil {
				var st struct {
					LastProject    string   `json:"lastProject"`
					RecentProjects []string `json:"recentProjects"`
				}
				if json.Unmarshal(raw, &st) == nil {
					rj, _ := json.Marshal(st.RecentProjects)
					if _, err := db.Exec(`INSERT INTO app_state(id, last_project, recent_json) VALUES(1, ?, ?)
						ON CONFLICT(id) DO UPDATE SET last_project=excluded.last_project, recent_json=excluded.recent_json`,
						st.LastProject, string(rj)); err != nil {
						return err
					}
					_ = os.Remove(filepath.Join(base, "state.json"))
				}
			}
		}
	}
	if fileExists(filepath.Join(base, "product-feedback.json")) {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM product_feedback`).Scan(&n)
		if n == 0 {
			if raw, err := os.ReadFile(filepath.Join(base, "product-feedback.json")); err == nil {
				var f struct {
					Entries []struct {
						ID           string `json:"id"`
						Status       string `json:"status"`
						Category     string `json:"category"`
						Title        string `json:"title"`
						Detail       string `json:"detail"`
						SuggestedFix string `json:"suggestedFix"`
						ProjectRoot  string `json:"projectRoot"`
						TaskID       string `json:"taskId"`
						Kind         string `json:"kind"`
						CreatedAtMs  int64  `json:"createdAtMs"`
						UpdatedAtMs  int64  `json:"updatedAtMs"`
					} `json:"entries"`
				}
				if json.Unmarshal(raw, &f) == nil {
					for _, e := range f.Entries {
						if strings.TrimSpace(e.ID) == "" {
							continue
						}
						if _, err := db.Exec(`INSERT INTO product_feedback(
							id, status, category, title, detail, suggested_fix, project_root, task_id, kind, created_at_ms, updated_at_ms)
							VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
							e.ID, e.Status, e.Category, e.Title, e.Detail, e.SuggestedFix, e.ProjectRoot, e.TaskID, e.Kind, e.CreatedAtMs, e.UpdatedAtMs); err != nil {
							return err
						}
					}
					_ = os.Remove(filepath.Join(base, "product-feedback.json"))
				}
			}
		}
	}
	return nil
}
