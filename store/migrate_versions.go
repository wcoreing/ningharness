package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// migrateFn 单步升级：从 version-1 → version。
type migrateFn func(tx *sql.Tx) error

var schemaMigrations = map[int]migrateFn{
	3: migrateToV3,
	4: migrateToV4,
	5: migrateToV5,
	6: migrateToV6,
	7: migrateToV7,
	8:  migrateToV8,
	9:  migrateToV9,
	10: migrateToV10,
	11: migrateToV11,
	12: migrateToV12,
	13: migrateToV13,
	14: migrateToV14,
	15: migrateToV15,
	16: migrateToV16,
	17: migrateToV17,
	18: migrateToV18,
	19: migrateToV19,
	20: migrateToV20,
	21: migrateToV21,
	22: migrateToV22,
	23: migrateToV23,
	24: migrateToV24,
	25: migrateToV25,
}

func migrateToV25(tx *sql.Tx) error {
	has, err := columnExists(tx, "jobs", "goal_next")
	if err != nil {
		return err
	}
	if !has {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN goal_next TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

func migrateToV24(tx *sql.Tx) error {
	has, err := columnExists(tx, "jobs", "steer_pending")
	if err != nil {
		return err
	}
	if !has {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN steer_pending TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

func migrateToV23(tx *sql.Tx) error {
	hasMax, err := columnExists(tx, "jobs", "goal_max_rounds")
	if err != nil {
		return err
	}
	if !hasMax {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN goal_max_rounds INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	hasRound, err := columnExists(tx, "jobs", "goal_round")
	if err != nil {
		return err
	}
	if !hasRound {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN goal_round INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	return nil
}

func migrateToV22(tx *sql.Tx) error {
	drops := []string{
		`DROP TABLE IF EXISTS pins`,
		`DROP TABLE IF EXISTS pin_sessions`,
		`DROP TABLE IF EXISTS review_sessions`,
		`DROP TABLE IF EXISTS growth_reflect`,
		`DROP TABLE IF EXISTS settings_blob`,
		`DROP TABLE IF EXISTS app_state`,
		`DROP TABLE IF EXISTS product_feedback`,
		`DROP TABLE IF EXISTS library_pack`,
	}
	for _, q := range drops {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func migrateToV21(tx *sql.Tx) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='lesson_entry'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		_, err := tx.Exec(`CREATE TABLE lesson_entry (
  id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT 'skill',
  anchors TEXT NOT NULL DEFAULT '{}',
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  source_task_id TEXT NOT NULL DEFAULT '',
  parent_task_id TEXT NOT NULL DEFAULT '',
  source_session_key TEXT NOT NULL DEFAULT '',
  supersedes_id TEXT NOT NULL DEFAULT '',
  acked_at_ms INTEGER NOT NULL DEFAULT 0,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
)`)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE INDEX idx_lesson_scope_status ON lesson_entry(project_id, scope, status)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE INDEX idx_lesson_anchors_skill ON lesson_entry(project_id, json_extract(anchors, '$.skillId'), status)`); err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE INDEX idx_lesson_source_task ON lesson_entry(project_id, source_task_id)`)
		return err
	}
	var hasSkill int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('lesson_entry') WHERE name='skill_id'`).Scan(&hasSkill); err != nil {
		return err
	}
	if hasSkill == 0 {
		return nil
	}
	if _, err := tx.Exec(`CREATE TABLE lesson_entry_v21 (
  id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT 'skill',
  anchors TEXT NOT NULL DEFAULT '{}',
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  source_task_id TEXT NOT NULL DEFAULT '',
  parent_task_id TEXT NOT NULL DEFAULT '',
  source_session_key TEXT NOT NULL DEFAULT '',
  supersedes_id TEXT NOT NULL DEFAULT '',
  acked_at_ms INTEGER NOT NULL DEFAULT 0,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO lesson_entry_v21(
  id, project_id, scope, anchors, body, status,
  source_task_id, parent_task_id, source_session_key, supersedes_id,
  acked_at_ms, created_at_ms, updated_at_ms
)
SELECT id, project_id, 'skill',
  CASE WHEN trim(skill_id)='' THEN '{}' ELSE json_object('skillId', skill_id) END,
  body, status, source_task_id, parent_task_id, source_session_key, supersedes_id,
  acked_at_ms, created_at_ms, updated_at_ms
FROM lesson_entry`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE lesson_entry`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE lesson_entry_v21 RENAME TO lesson_entry`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX idx_lesson_scope_status ON lesson_entry(project_id, scope, status)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX idx_lesson_anchors_skill ON lesson_entry(project_id, json_extract(anchors, '$.skillId'), status)`); err != nil {
		return err
	}
	_, err := tx.Exec(`CREATE INDEX idx_lesson_source_task ON lesson_entry(project_id, source_task_id)`)
	return err
}

func migrateToV20(tx *sql.Tx) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='lesson_entry'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := tx.Exec(`CREATE TABLE lesson_entry (
  id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  skill_id TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  source_task_id TEXT NOT NULL DEFAULT '',
  parent_task_id TEXT NOT NULL DEFAULT '',
  source_session_key TEXT NOT NULL DEFAULT '',
  supersedes_id TEXT NOT NULL DEFAULT '',
  acked_at_ms INTEGER NOT NULL DEFAULT 0,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX idx_lesson_skill_status ON lesson_entry(project_id, skill_id, status)`); err != nil {
		return err
	}
	_, err := tx.Exec(`CREATE INDEX idx_lesson_source_task ON lesson_entry(project_id, source_task_id)`)
	return err
}

// migrateToV19: library_pack — 个人技能库索引（正文在 ~/.agentdesk/skill-library/packs/）。
func migrateToV19(tx *sql.Tx) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='library_pack'`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := tx.Exec(`CREATE TABLE library_pack (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '1',
  tags_json TEXT NOT NULL DEFAULT '[]',
  summary TEXT NOT NULL DEFAULT '',
  source_project TEXT NOT NULL DEFAULT '',
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  enabled INTEGER NOT NULL DEFAULT 1
)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_library_pack_updated ON library_pack(updated_at_ms DESC)`)
	return err
}

// migrateToV18: history_message.resource_ids_json — tool 行外置 resource 索引（与 tool_calls_json[].resource_ids 对齐）。
func migrateToV18(tx *sql.Tx) error {
	has, err := columnExists(tx, "history_message", "resource_ids_json")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE history_message ADD COLUMN resource_ids_json TEXT NOT NULL DEFAULT ''`)
	return err
}

// migrateToV17: jobs.feed_extra（练笔等人分前馈）；job_steps.prompt（节原话持久化）。
func migrateToV17(tx *sql.Tx) error {
	hasFE, err := columnExists(tx, "jobs", "feed_extra")
	if err != nil {
		return err
	}
	if !hasFE {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN feed_extra TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	hasSP, err := columnExists(tx, "job_steps", "prompt")
	if err != nil {
		return err
	}
	if !hasSP {
		if _, err := tx.Exec(`ALTER TABLE job_steps ADD COLUMN prompt TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// migrateToV16: pins.note — 人编辑钉住摘录/批注（Transport override）。
func migrateToV16(tx *sql.Tx) error {
	var tbl int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pins'`).Scan(&tbl); err != nil {
		return err
	}
	if tbl == 0 {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS pins (
  project_id TEXT NOT NULL,
  path TEXT NOT NULL,
  sort_ord INTEGER NOT NULL DEFAULT 0,
  note TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, path)
)`)
		return err
	}
	has, err := columnExists(tx, "pins", "note")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE pins ADD COLUMN note TEXT NOT NULL DEFAULT ''`)
	return err
}

// migrateToV15: history_message.feedforward；tasks 去掉 model_input_id/snapshot_id；停用 resource model_input/snapshot。
func migrateToV15(tx *sql.Tx) error {
	hasFF, err := columnExists(tx, "history_message", "feedforward")
	if err != nil {
		return err
	}
	if !hasFF {
		if _, err := tx.Exec(`ALTER TABLE history_message ADD COLUMN feedforward TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if err := migrateEmbeddedSnapshotsToFeedforward(tx); err != nil {
		return err
	}
	if err := rebuildTasksDropAuditFKV15(tx); err != nil {
		return err
	}
	_, _ = tx.Exec(`DELETE FROM resource WHERE kind IN ('model_input','snapshot')`)
	return nil
}

func migrateEmbeddedSnapshotsToFeedforward(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, content, feedforward FROM history_message WHERE role='user' AND content LIKE '%agentdesk-snapshot-start%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id  int64
		c   string
		ff  string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.c, &r.ff); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	const start = "<!-- agentdesk-snapshot-start -->"
	const end = "<!-- agentdesk-snapshot-end -->"
	for _, r := range list {
		if strings.TrimSpace(r.ff) != "" {
			continue
		}
		si := strings.Index(r.c, start)
		if si < 0 {
			continue
		}
		ei := strings.Index(r.c[si:], end)
		if ei < 0 {
			continue
		}
		endAt := si + ei + len(end)
		ff := strings.TrimSpace(r.c[si+len(start) : si+ei])
		after := strings.TrimSpace(r.c[endAt:])
		before := strings.TrimSpace(r.c[:si])
		content := after
		if content == "" {
			content = before
		}
		if _, err := tx.Exec(`UPDATE history_message SET content=?, feedforward=? WHERE id=?`, content, ff, r.id); err != nil {
			return err
		}
	}
	return nil
}

func rebuildTasksDropAuditFKV15(tx *sql.Tx) error {
	hasMI, err := columnExists(tx, "tasks", "model_input_id")
	if err != nil {
		return err
	}
	hasSnap, err := columnExists(tx, "tasks", "snapshot_id")
	if err != nil {
		return err
	}
	if !hasMI && !hasSnap {
		return nil
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tasks_v15 (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0,
  ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  tool_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);
`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO tasks_v15(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count)
SELECT project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count FROM tasks
`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS tasks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE tasks_v15 RENAME TO tasks`); err != nil {
		return err
	}
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_ended ON tasks(project_id, ended_at_ms DESC)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job ON tasks(project_id, job_id)`)
	return nil
}

// migrateToV14: tasks.snapshot_id → 关联 resource.kind=snapshot（本轮 Turn Transport）。
func migrateToV14(tx *sql.Tx) error {
	has, err := columnExists(tx, "tasks", "snapshot_id")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE tasks ADD COLUMN snapshot_id INTEGER NOT NULL DEFAULT 0`)
	return err
}

// migrateToV13: tasks.model_input_id → 关联 resource.kind=model_input。
func migrateToV13(tx *sql.Tx) error {
	has, err := columnExists(tx, "tasks", "model_input_id")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE tasks ADD COLUMN model_input_id INTEGER NOT NULL DEFAULT 0`)
	return err
}

// migrateToV3: task.trail_json → run_events，再删 trail_json。
func migrateToV3(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS run_events (
  project_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  kind TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at_ms INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, run_id, seq),
  FOREIGN KEY (project_id, run_id) REFERENCES runs(project_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_run_events_run ON run_events(project_id, run_id, seq);
`); err != nil {
		return err
	}

	hasTrail, err := columnExists(tx, "runs", "trail_json")
	if err != nil {
		return err
	}
	if hasTrail {
		rows, err := tx.Query(`SELECT project_id, id, trail_json FROM runs`)
		if err != nil {
			return err
		}
		type row struct {
			pid, id, trail string
		}
		var list []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.pid, &r.id, &r.trail); err != nil {
				_ = rows.Close()
				return err
			}
			list = append(list, r)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		now := time.Now().UnixMilli()
		for _, r := range list {
			if err := replaceRunEventsTx(tx, r.pid, r.id, r.trail, now); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`ALTER TABLE runs DROP COLUMN trail_json`); err != nil {
			// 旧 SQLite：重建表
			if err := rebuildRunsWithoutTrail(tx); err != nil {
				return fmt.Errorf("drop trail_json: %w", err)
			}
		}
	}
	return nil
}

func rebuildRunsWithoutTrail(tx *sql.Tx) error {
	if _, err := tx.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
CREATE TABLE runs_new (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0,
  ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL DEFAULT '',
  reply TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, id)
);
INSERT INTO runs_new SELECT project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, task_id, prompt, reply, status, error FROM runs;
DROP TABLE runs;
ALTER TABLE runs_new RENAME TO runs;
CREATE INDEX IF NOT EXISTS idx_runs_ended ON runs(project_id, ended_at_ms DESC);
`); err != nil {
		_, _ = tx.Exec(`PRAGMA foreign_keys=ON`)
		return err
	}
	_, err := tx.Exec(`PRAGMA foreign_keys=ON`)
	return err
}

// migrateToV4: file_timeline.payload_json → timeline_rounds + timeline_reviews。
func migrateToV4(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS timeline_rounds (
  project_id TEXT NOT NULL,
  rel TEXT NOT NULL,
  round_ord INTEGER NOT NULL,
  baseline TEXT NOT NULL DEFAULT '',
  baseline_short TEXT NOT NULL DEFAULT '',
  ended_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, rel, round_ord)
);
CREATE INDEX IF NOT EXISTS idx_timeline_rounds_rel ON timeline_rounds(project_id, rel);

CREATE TABLE IF NOT EXISTS timeline_reviews (
  project_id TEXT NOT NULL,
  rel TEXT NOT NULL,
  round_ord INTEGER NOT NULL,
  review_ord INTEGER NOT NULL,
  hash TEXT NOT NULL DEFAULT '',
  short TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  when_at TEXT NOT NULL DEFAULT '',
  idx INTEGER NOT NULL DEFAULT 0,
  accepted INTEGER NOT NULL DEFAULT 0,
  discarded INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, rel, round_ord, review_ord),
  FOREIGN KEY (project_id, rel, round_ord) REFERENCES timeline_rounds(project_id, rel, round_ord) ON DELETE CASCADE
);
`); err != nil {
		return err
	}

	exists, err := tableExists(tx, "file_timeline")
	if err != nil {
		return err
	}
	if exists {
		rows, err := tx.Query(`SELECT project_id, rel, payload_json FROM file_timeline`)
		if err != nil {
			return err
		}
		type row struct {
			pid, rel, payload string
		}
		var list []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.pid, &r.rel, &r.payload); err != nil {
				_ = rows.Close()
				return err
			}
			list = append(list, r)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, r := range list {
			if err := replaceTimelineFromPayloadTx(tx, r.pid, r.rel, r.payload); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`DROP TABLE IF EXISTS file_timeline`); err != nil {
			return err
		}
	}
	return nil
}

// migrateToV5: meta 大 blob → review_sessions / growth_reflect。
func migrateToV5(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS review_sessions (
  project_id TEXT PRIMARY KEY,
  baseline TEXT NOT NULL DEFAULT '',
  started TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS growth_reflect (
  project_id TEXT PRIMARY KEY,
  at_ms INTEGER NOT NULL DEFAULT 0,
  run_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  lesson_rels_json TEXT NOT NULL DEFAULT '[]'
);
`); err != nil {
		return err
	}
	return promoteMetaBlobsTx(tx)
}

// migrateToV6: review_sessions → pin_sessions（文件打点模型命名）。
func migrateToV6(tx *sql.Tx) error {
	var name string
	err := tx.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='review_sessions'`).Scan(&name)
	if err == sql.ErrNoRows {
		// 已是新名或空库：确保 pin_sessions 存在
		_, err = tx.Exec(`
CREATE TABLE IF NOT EXISTS pin_sessions (
  project_id TEXT PRIMARY KEY,
  baseline TEXT NOT NULL DEFAULT '',
  started TEXT NOT NULL DEFAULT ''
)`)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`ALTER TABLE review_sessions RENAME TO pin_sessions`)
	return err
}

// migrateToV7: 模型侧连贯上下文 history_message。
func migrateToV7(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS history_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id TEXT NOT NULL,
  session_key TEXT NOT NULL,
  seq INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  tool_call_id TEXT NOT NULL DEFAULT '',
  tool_calls_json TEXT NOT NULL DEFAULT '',
  run_id TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL DEFAULT 0,
  UNIQUE(project_id, session_key, seq)
);
CREATE INDEX IF NOT EXISTS idx_history_message_session ON history_message(project_id, session_key, seq);
CREATE INDEX IF NOT EXISTS idx_history_message_run ON history_message(project_id, run_id);
`)
	return err
}

// migrateToV8: 工具产物全文索引（进模用 summary，正文按需 recall）。
func migrateToV8(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tool_payload (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id TEXT NOT NULL,
  session_key TEXT NOT NULL DEFAULT '',
  run_id TEXT NOT NULL DEFAULT '',
  tool_call_id TEXT NOT NULL DEFAULT '',
  tool_name TEXT NOT NULL DEFAULT '',
  phase TEXT NOT NULL DEFAULT '',
  rel_path TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_tool_payload_session ON tool_payload(project_id, session_key, created_at_ms);
CREATE INDEX IF NOT EXISTS idx_tool_payload_call ON tool_payload(project_id, tool_call_id);
CREATE INDEX IF NOT EXISTS idx_tool_payload_path ON tool_payload(project_id, rel_path);
`)
	return err
}


// migrateToV9: jobs/tasks 定名；删 run_tools/run_events/timeline_*；run_id→task_id。
func migrateToV9(tx *sql.Tx) error {
	// 1) runs → tasks（trail 并入 trail_json；去掉 prompt/reply/run_tools/run_events）
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tasks (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0,
  ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  trail_json TEXT NOT NULL DEFAULT '[]',
  tools_json TEXT NOT NULL DEFAULT '[]',
  PRIMARY KEY (project_id, id)
)`); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, task_id, status, error FROM runs`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, id, driver, sess, purpose, jobID, status, errStr string
			var started, ended int64
			if err := rows.Scan(&pid, &id, &started, &ended, &driver, &sess, &purpose, &jobID, &status, &errStr); err != nil {
				return err
			}
			trail := "[]"
			if raw, err := loadRunTrailTx(tx, pid, id); err == nil {
				s := strings.TrimSpace(string(raw))
				if s != "" && s != "[]" && s != "null" {
					trail = s
				}
			}
			if trail == "[]" {
				var oldTrail string
				_ = tx.QueryRow(`SELECT trail_json FROM runs WHERE project_id=? AND id=?`, pid, id).Scan(&oldTrail)
				if s := strings.TrimSpace(oldTrail); s != "" && s != "[]" {
					trail = s
				}
			}
			tools := "[]"
			if raw, err := loadRunToolsJSONTx(tx, pid, id); err == nil && raw != "" && raw != "[]" {
				tools = raw
			}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO tasks(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, trail_json, tools_json)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, pid, id, started, ended, driver, sess, purpose, jobID, status, errStr, trail, tools); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`DROP TABLE IF EXISTS run_events`,
		`DROP TABLE IF EXISTS run_tools`,
		`DROP TABLE IF EXISTS runs`,
		`DROP TABLE IF EXISTS timeline_reviews`,
		`DROP TABLE IF EXISTS timeline_rounds`,
		`DROP TABLE IF EXISTS file_timeline`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_ended ON tasks(project_id, ended_at_ms DESC)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job ON tasks(project_id, job_id)`); err != nil {
		return err
	}

	// 2) queue_tasks → jobs；queue_steps → job_steps
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL DEFAULT '',
  driver TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  target_rel TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  batch_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  started_at INTEGER NOT NULL DEFAULT 0,
  finished_at INTEGER NOT NULL DEFAULT 0,
  session_key TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  step_done INTEGER NOT NULL DEFAULT 0,
  step_total INTEGER NOT NULL DEFAULT 0,
  progress_hint TEXT NOT NULL DEFAULT '',
  sort_ord INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO jobs SELECT project_id, id, type, title, prompt, driver, model, target_rel, status, run_id, error, last_error, retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose, step_done, step_total, progress_hint, sort_ord FROM queue_tasks`); err != nil {
		// queue_tasks 可能已不存在（新库跳过）
		_ = err
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS job_steps (
  project_id TEXT NOT NULL,
  job_id TEXT NOT NULL,
  idx INTEGER NOT NULL,
  rel TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, job_id, idx),
  FOREIGN KEY (project_id, job_id) REFERENCES jobs(project_id, id) ON DELETE CASCADE
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO job_steps(project_id, job_id, idx, rel, title, status, task_id, error)
		SELECT project_id, task_id, idx, rel, title, status, run_id, error FROM queue_steps`); err != nil {
		_ = err
	}
	_, _ = tx.Exec(`DROP TABLE IF EXISTS queue_steps`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS queue_tasks`)

	// 3) rename run_id → task_id on message tables
	if err := renameColumnIfExists(tx, "history_message", "run_id", "task_id"); err != nil {
		return err
	}
	_, _ = tx.Exec(`DROP INDEX IF EXISTS idx_history_message_run`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_history_message_task ON history_message(project_id, task_id)`)
	if err := renameColumnIfExists(tx, "tool_payload", "run_id", "task_id"); err != nil {
		return err
	}
	if err := renameColumnIfExists(tx, "messages", "run_id", "task_id"); err != nil {
		return err
	}
	if err := renameColumnIfExists(tx, "growth_reflect", "run_id", "task_id"); err != nil {
		return err
	}
	return nil
}

// migrateToV10: 对话 SSOT=history_message；FTS 迁过去；删 messages。
func migrateToV10(tx *sql.Tx) error {
	// 若 history 尚无 user/assistant，把旧 messages 灌入（避免纯 UI 会话丢失）。
	rows, err := tx.Query(`SELECT project_id, session_id FROM messages GROUP BY project_id, session_id`)
	if err == nil {
		type sk struct{ pid, sid string }
		var keys []sk
		for rows.Next() {
			var k sk
			if err := rows.Scan(&k.pid, &k.sid); err != nil {
				rows.Close()
				return err
			}
			keys = append(keys, k)
		}
		rows.Close()
		for _, k := range keys {
			var n int
			_ = tx.QueryRow(`SELECT COUNT(1) FROM history_message WHERE project_id=? AND session_key=? AND role IN ('user','assistant') AND IFNULL(content,'')!=''`,
				k.pid, k.sid).Scan(&n)
			if n > 0 {
				continue
			}
			mrows, err := tx.Query(`SELECT role, content, created_at_ms, task_id FROM messages WHERE project_id=? AND session_id=? AND role IN ('user','assistant') ORDER BY seq ASC, id ASC`,
				k.pid, k.sid)
			if err != nil {
				return err
			}
			var maxSeq sql.NullInt64
			_ = tx.QueryRow(`SELECT MAX(seq) FROM history_message WHERE project_id=? AND session_key=?`, k.pid, k.sid).Scan(&maxSeq)
			seq := 0
			if maxSeq.Valid {
				seq = int(maxSeq.Int64)
			}
			for mrows.Next() {
				var role, content, taskID string
				var created int64
				if err := mrows.Scan(&role, &content, &created, &taskID); err != nil {
					mrows.Close()
					return err
				}
				seq++
				if _, err := tx.Exec(`INSERT INTO history_message(project_id, session_key, seq, role, content, tool_call_id, tool_calls_json, task_id, created_at_ms)
					VALUES(?,?,?,?,?,'','',?,?)`, k.pid, k.sid, seq, role, content, taskID, created); err != nil {
					mrows.Close()
					return err
				}
			}
			mrows.Close()
			if err := mrows.Err(); err != nil {
				return err
			}
		}
	}

	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS messages_ai`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS messages_ad`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS messages_au`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS messages_fts`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS messages`)

	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS history_message_ai`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS history_message_ad`)
	_, _ = tx.Exec(`DROP TRIGGER IF EXISTS history_message_au`)
	_, _ = tx.Exec(`DROP TABLE IF EXISTS history_message_fts`)

	if _, err := tx.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS history_message_fts USING fts5(
  content,
  content='history_message',
  content_rowid='id',
  tokenize='unicode61'
);
CREATE TRIGGER IF NOT EXISTS history_message_ai AFTER INSERT ON history_message BEGIN
  INSERT INTO history_message_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER IF NOT EXISTS history_message_ad AFTER DELETE ON history_message BEGIN
  INSERT INTO history_message_fts(history_message_fts, rowid, content) VALUES('delete', old.id, old.content);
END;
CREATE TRIGGER IF NOT EXISTS history_message_au AFTER UPDATE ON history_message BEGIN
  INSERT INTO history_message_fts(history_message_fts, rowid, content) VALUES('delete', old.id, old.content);
  INSERT INTO history_message_fts(rowid, content) VALUES (new.id, new.content);
END;
`); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO history_message_fts(history_message_fts) VALUES('rebuild')`)
	return err
}

// migrateToV11: trail_json → steps_json；tools_json → tool_count；重建 tasks。
func migrateToV11(tx *sql.Tx) error {
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tasks_v11 (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0,
  ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  steps_json TEXT NOT NULL DEFAULT '[]',
  tool_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);
`); err != nil {
		return err
	}

	hasTrail, err := columnExists(tx, "tasks", "trail_json")
	if err != nil {
		return err
	}
	hasTools, err := columnExists(tx, "tasks", "tools_json")
	if err != nil {
		return err
	}
	hasSteps, err := columnExists(tx, "tasks", "steps_json")
	if err != nil {
		return err
	}
	if hasSteps && !hasTrail {
		// 已是新形态（例如部分升级中断后重跑）
		_, _ = tx.Exec(`DROP TABLE IF EXISTS tasks_v11`)
		return nil
	}

	sel := `SELECT project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error`
	if hasTrail {
		sel += `, trail_json`
	} else if hasSteps {
		sel += `, steps_json`
	} else {
		sel += `, '[]'`
	}
	if hasTools {
		sel += `, tools_json`
	} else {
		sel += `, '[]'`
	}
	sel += ` FROM tasks`
	rows, err := tx.Query(sel)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid, id, driver, sessionID, purpose, jobID, status, errStr, stepsJSON, toolsJSON string
		var started, ended int64
		if err := rows.Scan(&pid, &id, &started, &ended, &driver, &sessionID, &purpose, &jobID, &status, &errStr, &stepsJSON, &toolsJSON); err != nil {
			return err
		}
		if strings.TrimSpace(stepsJSON) == "" {
			stepsJSON = "[]"
		}
		toolCount := toolCountFromToolsJSON(toolsJSON)
		if toolCount == 0 {
			toolCount = toolCountFromStepsJSON(stepsJSON)
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO tasks_v11(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, steps_json, tool_count)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			pid, id, started, ended, driver, sessionID, purpose, jobID, status, errStr, stepsJSON, toolCount); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS tasks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE tasks_v11 RENAME TO tasks`); err != nil {
		return err
	}
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_ended ON tasks(project_id, ended_at_ms DESC)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job ON tasks(project_id, job_id)`)
	return nil
}

// migrateToV12: tool_payload→resource；steps_json→history_message；tasks 去掉 steps_json。
func migrateToV12(tx *sql.Tx) error {
	if err := migrateToolPayloadToResourceV12(tx); err != nil {
		return err
	}
	if err := migrateStepsJSONToHistoryV12(tx); err != nil {
		return err
	}
	return rebuildTasksDropStepsJSONV12(tx)
}

func migrateToolPayloadToResourceV12(tx *sql.Tx) error {
	hasResource, err := tableExists(tx, "resource")
	if err != nil {
		return err
	}
	if !hasResource {
		if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS resource (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id TEXT NOT NULL,
  session_key TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  tool_call_id TEXT NOT NULL DEFAULT '',
  tool_name TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  phase TEXT NOT NULL DEFAULT '',
  rel_path TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL DEFAULT 0
);
`); err != nil {
			return err
		}
	}
	hasTP, err := tableExists(tx, "tool_payload")
	if err != nil {
		return err
	}
	if hasTP {
		rows, err := tx.Query(`SELECT id, project_id, session_key, task_id, tool_call_id, tool_name, phase, rel_path, status, summary, body, created_at_ms FROM tool_payload`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id, created int64
			var pid, sess, taskID, callID, name, phase, rel, status, summary, body string
			if err := rows.Scan(&id, &pid, &sess, &taskID, &callID, &name, &phase, &rel, &status, &summary, &body, &created); err != nil {
				return err
			}
			kind := "tool_result"
			ph := strings.ToLower(strings.TrimSpace(phase))
			if ph == "call" {
				kind = "tool_call"
			}
			summary = strings.ReplaceAll(summary, "tool_payload#", "resource#")
			summary = strings.ReplaceAll(summary, "recall_tool_result", "recall_resource")
			summary = strings.ReplaceAll(summary, "payload_id=", "resource_id=")
			if _, err := tx.Exec(`INSERT OR IGNORE INTO resource(id, project_id, session_key, task_id, tool_call_id, tool_name, kind, phase, rel_path, status, summary, body, created_at_ms)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				id, pid, sess, taskID, callID, name, kind, phase, rel, status, summary, body, created); err != nil {
				return err
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE IF EXISTS tool_payload`); err != nil {
			return err
		}
	}
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_resource_session ON resource(project_id, session_key, created_at_ms)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_resource_call ON resource(project_id, tool_call_id)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_resource_path ON resource(project_id, rel_path)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_resource_kind ON resource(project_id, kind)`)
	return nil
}

type stepMigrateV12 struct {
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Name   string `json:"name"`
	CallID string `json:"callId"`
	Args   string `json:"args"`
	Result string `json:"result"`
	OK     bool   `json:"ok"`
	Done   bool   `json:"done"`
	Diff   *struct {
		Path  string `json:"path"`
		Add   int    `json:"add"`
		Del   int    `json:"del"`
		Lines []struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		} `json:"lines"`
	} `json:"diff"`
}

func migrateStepsJSONToHistoryV12(tx *sql.Tx) error {
	hasSteps, err := columnExists(tx, "tasks", "steps_json")
	if err != nil || !hasSteps {
		return err
	}
	rows, err := tx.Query(`SELECT project_id, id, session_id, ended_at_ms, steps_json FROM tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type taskRow struct {
		pid, id, sess string
		ended         int64
		stepsJSON     string
	}
	var list []taskRow
	for rows.Next() {
		var r taskRow
		if err := rows.Scan(&r.pid, &r.id, &r.sess, &r.ended, &r.stepsJSON); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range list {
		if err := migrateOneTaskStepsToHistoryV12(tx, r.pid, r.id, r.sess, r.ended, r.stepsJSON); err != nil {
			return err
		}
	}
	return nil
}

func migrateOneTaskStepsToHistoryV12(tx *sql.Tx, pid, taskID, sess string, endedMs int64, stepsJSON string) error {
	stepsJSON = strings.TrimSpace(stepsJSON)
	if stepsJSON == "" || stepsJSON == "[]" || stepsJSON == "null" {
		return nil
	}
	var steps []stepMigrateV12
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return nil // 坏 JSON 跳过
	}
	sess = strings.TrimSpace(sess)
	if sess == "" {
		sess = "main"
	}
	var thinkN, toolN int
	_ = tx.QueryRow(`SELECT COUNT(1) FROM history_message WHERE project_id=? AND task_id=? AND role='thinking'`, pid, taskID).Scan(&thinkN)
	_ = tx.QueryRow(`SELECT COUNT(1) FROM history_message WHERE project_id=? AND task_id=? AND role='tool'`, pid, taskID).Scan(&toolN)

	created := endedMs
	if created <= 0 {
		created = time.Now().UnixMilli()
	}
	for _, s := range steps {
		switch strings.TrimSpace(s.Kind) {
		case "thinking":
			if thinkN > 0 {
				continue
			}
			text := s.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			if err := insertHistoryMsgTx(tx, pid, sess, "thinking", text, "", "", taskID, created); err != nil {
				return err
			}
		case "tool":
			if toolN > 0 {
				continue
			}
			name := strings.TrimSpace(s.Name)
			if name == "" {
				continue
			}
			callID := strings.TrimSpace(s.CallID)
			if callID == "" {
				callID = name
			}
			args := strings.TrimSpace(s.Args)
			result := strings.TrimSpace(s.Result)
			if d := s.Diff; d != nil && strings.TrimSpace(d.Path) != "" {
				raw, _ := json.Marshal(d)
				diffID, err := insertResourceTx(tx, pid, sess, taskID, callID, name, "diff", "diff", d.Path, "ok", string(raw), created)
				if err != nil {
					return err
				}
				if diffID > 0 {
					hint := fmt.Sprintf("resource#%d · diff · %s", diffID, d.Path)
					if result != "" {
						result = result + " · " + hint
					} else {
						result = hint
					}
				}
			}
			args = strings.ReplaceAll(args, "tool_payload#", "resource#")
			result = strings.ReplaceAll(result, "tool_payload#", "resource#")
			if args != "" || !s.Done {
				calls, _ := json.Marshal([]map[string]string{{
					"id": callID, "name": name, "arguments": args,
				}})
				if err := insertHistoryMsgTx(tx, pid, sess, "assistant", "", "", string(calls), taskID, created); err != nil {
					return err
				}
			}
			if result != "" || s.Done {
				if err := insertHistoryMsgTx(tx, pid, sess, "tool", result, callID, "", taskID, created); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func insertHistoryMsgTx(tx *sql.Tx, pid, sessionKey, role, content, toolCallID, toolCallsJSON, taskID string, createdAtMs int64) error {
	var maxSeq sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(seq) FROM history_message WHERE project_id=? AND session_key=?`, pid, sessionKey).Scan(&maxSeq); err != nil {
		return err
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}
	_, err := tx.Exec(`INSERT INTO history_message(project_id, session_key, seq, role, content, tool_call_id, tool_calls_json, task_id, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		pid, sessionKey, seq, role, content, toolCallID, toolCallsJSON, taskID, createdAtMs)
	return err
}

func insertResourceTx(tx *sql.Tx, pid, sess, taskID, callID, name, kind, phase, rel, status, body string, created int64) (int64, error) {
	summary := fmt.Sprintf("resource · %s", kind)
	if rel != "" {
		summary += " · " + rel
	}
	res, err := tx.Exec(`INSERT INTO resource(project_id, session_key, task_id, tool_call_id, tool_name, kind, phase, rel_path, status, summary, body, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		pid, sess, taskID, callID, name, kind, phase, rel, status, summary, body, created)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	summary = fmt.Sprintf("resource#%d · %s · %s · 全文 desk_session recall_resource resource_id=%d", id, kind, rel, id)
	_, _ = tx.Exec(`UPDATE resource SET summary=? WHERE id=? AND project_id=?`, summary, id, pid)
	return id, nil
}

func rebuildTasksDropStepsJSONV12(tx *sql.Tx) error {
	hasSteps, err := columnExists(tx, "tasks", "steps_json")
	if err != nil {
		return err
	}
	if !hasSteps {
		return nil
	}
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tasks_v12 (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0,
  ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL DEFAULT '',
  tool_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);
`); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, steps_json, tool_count FROM tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid, id, driver, sessionID, purpose, jobID, status, errStr, stepsJSON string
		var started, ended int64
		var toolCount int
		if err := rows.Scan(&pid, &id, &started, &ended, &driver, &sessionID, &purpose, &jobID, &status, &errStr, &stepsJSON, &toolCount); err != nil {
			return err
		}
		if toolCount == 0 {
			toolCount = toolCountFromStepsJSON(stepsJSON)
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO tasks_v12(project_id, id, started_at_ms, ended_at_ms, driver, session_id, purpose, job_id, status, error, tool_count)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			pid, id, started, ended, driver, sessionID, purpose, jobID, status, errStr, toolCount); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS tasks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE tasks_v12 RENAME TO tasks`); err != nil {
		return err
	}
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_ended ON tasks(project_id, ended_at_ms DESC)`)
	_, _ = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_job ON tasks(project_id, job_id)`)
	return nil
}

func toolCountFromToolsJSON(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return 0
	}
	var tools []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &tools); err != nil {
		return 0
	}
	return len(tools)
}

func toolCountFromStepsJSON(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return 0
	}
	var steps []struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return 0
	}
	n := 0
	for _, s := range steps {
		if s.Kind == "tool" {
			n++
		}
	}
	return n
}

func loadRunTrailTx(tx *sql.Tx, pid, runID string) ([]byte, error) {
	rows, err := tx.Query(`SELECT payload_json FROM run_events WHERE project_id=? AND run_id=? ORDER BY seq ASC`, pid, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var parts []json.RawMessage
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		parts = append(parts, json.RawMessage(payload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(parts)
}

func loadRunToolsJSONTx(tx *sql.Tx, pid, runID string) (string, error) {
	rows, err := tx.Query(`SELECT name, ok, detail, path FROM run_tools WHERE project_id=? AND run_id=? ORDER BY id ASC`, pid, runID)
	if err != nil {
		return "[]", err
	}
	defer rows.Close()
	type tool struct {
		Name   string `json:"name"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail,omitempty"`
		Path   string `json:"path,omitempty"`
	}
	var out []tool
	for rows.Next() {
		var t tool
		var okInt int
		if err := rows.Scan(&t.Name, &okInt, &t.Detail, &t.Path); err != nil {
			return "[]", err
		}
		t.OK = okInt != 0
		out = append(out, t)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]", err
	}
	return string(b), rows.Err()
}

func renameColumnIfExists(tx *sql.Tx, table, from, to string) error {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil // 表不存在
	}
	defer rows.Close()
	hasFrom, hasTo := false, false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == from {
			hasFrom = true
		}
		if name == to {
			hasTo = true
		}
	}
	if !hasFrom || hasTo {
		return nil
	}
	_, err = tx.Exec(`ALTER TABLE ` + table + ` RENAME COLUMN ` + from + ` TO ` + to)
	return err
}

// promoteMetaBlobsTx 把 meta 里的大 JSON 迁到专用表并删键（幂等）。
func promoteMetaBlobsTx(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT project_id, key, value FROM meta WHERE key IN ('review_session_json','last_reflect_json')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		pid, key, value string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.pid, &r.key, &r.value); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range list {
		switch r.key {
		case "review_session_json":
			if err := upsertReviewSessionTx(tx, r.pid, r.value); err != nil {
				return err
			}
		case "last_reflect_json":
			if err := upsertGrowthReflectTx(tx, r.pid, r.value); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`DELETE FROM meta WHERE project_id=? AND key=?`, r.pid, r.key); err != nil {
			return err
		}
	}
	return nil
}

// PromoteMetaBlobs 项目迁入后兜底（legacy/json 可能又写入 meta blob）。
func PromoteMetaBlobs(db *sql.DB) error {
	return nil
}

// ReplaceRunEvents 用 trail JSON 数组覆写某 run 的事件行（供 task.Save / 迁移共用）。
func ReplaceRunEvents(tx *sql.Tx, pid, runID, trailJSON string, createdAtMs int64) error {
	return replaceRunEventsTx(tx, pid, runID, trailJSON, createdAtMs)
}

// LoadRunTrail 按 seq 拼回 trail JSON 数组字节。
func LoadRunTrail(db *sql.DB, pid, runID string) ([]byte, error) {
	rows, err := db.Query(`SELECT payload_json FROM run_events WHERE project_id=? AND run_id=? ORDER BY seq ASC`, pid, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var parts []json.RawMessage
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		parts = append(parts, json.RawMessage(payload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(parts)
}

func replaceRunEventsTx(tx *sql.Tx, pid, runID, trailJSON string, createdAtMs int64) error {
	if _, err := tx.Exec(`DELETE FROM run_events WHERE project_id=? AND run_id=?`, pid, runID); err != nil {
		return err
	}
	trailJSON = strings.TrimSpace(trailJSON)
	if trailJSON == "" || trailJSON == "[]" || trailJSON == "null" {
		return nil
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal([]byte(trailJSON), &blocks); err != nil {
		// 整段当一个 unknown 事件保留
		payload := trailJSON
		if _, err := tx.Exec(`INSERT INTO run_events(project_id, run_id, seq, kind, payload_json, created_at_ms) VALUES(?,?,?,?,?,?)`,
			pid, runID, 0, "raw", payload, createdAtMs); err != nil {
			return err
		}
		return nil
	}
	for i, raw := range blocks {
		kind := "block"
		var peek struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(raw, &peek)
		if k := strings.TrimSpace(peek.Kind); k != "" {
			kind = k
		}
		if _, err := tx.Exec(`INSERT INTO run_events(project_id, run_id, seq, kind, payload_json, created_at_ms) VALUES(?,?,?,?,?,?)`,
			pid, runID, i, kind, string(raw), createdAtMs); err != nil {
			return err
		}
	}
	return nil
}

func replaceTimelineFromPayloadTx(tx *sql.Tx, pid, rel, payload string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil
	}
	var idx struct {
		Rel    string `json:"rel"`
		Rounds []struct {
			Baseline      string `json:"baseline"`
			BaselineShort string `json:"baselineShort"`
			EndedAt       string `json:"endedAt"`
			Reviews       []struct {
				Hash      string `json:"hash"`
				Short     string `json:"short"`
				Subject   string `json:"subject"`
				When      string `json:"when"`
				Index     int    `json:"index"`
				Accepted  bool   `json:"accepted"`
				Discarded bool   `json:"discarded"`
			} `json:"reviews"`
		} `json:"rounds"`
	}
	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &idx); err != nil {
			return err
		}
	}
	if r := strings.TrimSpace(idx.Rel); r != "" {
		rel = r
	}
	if _, err := tx.Exec(`DELETE FROM timeline_reviews WHERE project_id=? AND rel=?`, pid, rel); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM timeline_rounds WHERE project_id=? AND rel=?`, pid, rel); err != nil {
		return err
	}
	for i, round := range idx.Rounds {
		if _, err := tx.Exec(`INSERT INTO timeline_rounds(project_id, rel, round_ord, baseline, baseline_short, ended_at) VALUES(?,?,?,?,?,?)`,
			pid, rel, i, round.Baseline, round.BaselineShort, round.EndedAt); err != nil {
			return err
		}
		for j, rev := range round.Reviews {
			acc, disc := 0, 0
			if rev.Accepted {
				acc = 1
			}
			if rev.Discarded {
				disc = 1
			}
			if _, err := tx.Exec(`INSERT INTO timeline_reviews(project_id, rel, round_ord, review_ord, hash, short, subject, when_at, idx, accepted, discarded)
				VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
				pid, rel, i, j, rev.Hash, rev.Short, rev.Subject, rev.When, rev.Index, acc, disc); err != nil {
				return err
			}
		}
	}
	return nil
}

func upsertReviewSessionTx(tx *sql.Tx, pid, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		_, err := tx.Exec(`DELETE FROM review_sessions WHERE project_id=?`, pid)
		return err
	}
	var s struct {
		Baseline string `json:"baseline"`
		Started  string `json:"started"`
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO review_sessions(project_id, baseline, started) VALUES(?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET baseline=excluded.baseline, started=excluded.started`,
		pid, s.Baseline, s.Started)
	return err
}

func upsertGrowthReflectTx(tx *sql.Tx, pid, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		_, err := tx.Exec(`DELETE FROM growth_reflect WHERE project_id=?`, pid)
		return err
	}
	var rec struct {
		AtMs       int64    `json:"atMs"`
		RunID      string   `json:"runId"`
		Status     string   `json:"status"`
		Summary    string   `json:"summary"`
		LessonRels []string `json:"lessonRels"`
		Thinking   string   `json:"thinking"`
		Lessons    string   `json:"lessons"`
	}
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return err
	}
	summary := strings.TrimSpace(rec.Summary)
	if summary == "" {
		summary = strings.TrimSpace(rec.Lessons)
	}
	if summary == "" {
		summary = strings.TrimSpace(rec.Thinking)
	}
	rels := rec.LessonRels
	if rels == nil {
		rels = []string{}
	}
	relsRaw, err := json.Marshal(rels)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO growth_reflect(project_id, at_ms, run_id, status, summary, lesson_rels_json) VALUES(?,?,?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET
			at_ms=excluded.at_ms, run_id=excluded.run_id, status=excluded.status,
			summary=excluded.summary, lesson_rels_json=excluded.lesson_rels_json`,
		pid, rec.AtMs, rec.RunID, rec.Status, summary, string(relsRaw))
	return err
}

func tableExists(tx *sql.Tx, name string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n > 0, err
}

func columnExists(tx *sql.Tx, table, col string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}
