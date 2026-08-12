package store

import (
	"database/sql"
	"fmt"
)

// ensureSchemaMySQL 在同实例共库场景下建 nh_* 表；不跑 SQLite 版本链 / ATTACH 遗留迁移。
func ensureSchemaMySQL(db *sql.DB) error {
	verTable := T("schema_version")
	if _, err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT NOT NULL PRIMARY KEY,
		version INT NOT NULL,
		CONSTRAINT chk_nh_schema_one CHECK (id = 1)
	)`, verTable)); err != nil {
		return err
	}
	var ver int
	err := db.QueryRow(fmt.Sprintf(`SELECT version FROM %s WHERE id=1`, verTable)).Scan(&ver)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		ver = 0
	}
	if ver > CurrentSchemaVersion {
		return fmt.Errorf("store: schema version %d newer than supported %d", ver, CurrentSchemaVersion)
	}
	if ver == 0 {
		if err := execMySQLSchema(db); err != nil {
			return err
		}
		_, err = db.Exec(fmt.Sprintf(
			`INSERT INTO %s(id, version) VALUES(1, ?) ON DUPLICATE KEY UPDATE version=VALUES(version)`,
			verTable), CurrentSchemaVersion)
		return err
	}
	if ver < CurrentSchemaVersion {
		// 一期：直接升到当前版本号（表已用最新 DDL）；后续按需加 mysqlMigrations。
		_, err = db.Exec(fmt.Sprintf(`UPDATE %s SET version=? WHERE id=1`, verTable), CurrentSchemaVersion)
		return err
	}
	return nil
}

func execMySQLSchema(db *sql.DB) error {
	stmts := mysqlSchemaStmts()
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("mysql schema: %w\nSQL: %s", err, q)
		}
	}
	return nil
}

func mysqlSchemaStmts() []string {
	p := TablePrefix()
	t := func(name string) string { return p + name }
	return []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id VARCHAR(512) NOT NULL PRIMARY KEY,
  root VARCHAR(1024) NOT NULL,
  updated_at_ms BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("projects")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  project_id VARCHAR(512) NOT NULL,
  `+"`key`"+` VARCHAR(191) NOT NULL,
  value MEDIUMTEXT NOT NULL,
  PRIMARY KEY (project_id, `+"`key`"+`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("meta")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  project_id VARCHAR(512) NOT NULL,
  id VARCHAR(191) NOT NULL,
  title VARCHAR(512) NOT NULL DEFAULT '',
  orch_key VARCHAR(512) NOT NULL DEFAULT '',
  updated_at_ms BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("sessions")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  project_id VARCHAR(512) NOT NULL,
  id VARCHAR(191) NOT NULL,
  started_at_ms BIGINT NOT NULL DEFAULT 0,
  ended_at_ms BIGINT NOT NULL DEFAULT 0,
  driver VARCHAR(64) NOT NULL DEFAULT '',
  session_id VARCHAR(191) NOT NULL DEFAULT '',
  purpose VARCHAR(512) NOT NULL DEFAULT '',
  job_id VARCHAR(191) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'ok',
  error TEXT NOT NULL,
  tool_count INT NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id),
  KEY idx_tasks_ended (project_id, ended_at_ms),
  KEY idx_tasks_job (project_id, job_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("tasks")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  project_id VARCHAR(512) NOT NULL,
  id VARCHAR(191) NOT NULL,
  type VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(512) NOT NULL DEFAULT '',
  prompt MEDIUMTEXT NOT NULL,
  driver VARCHAR(64) NOT NULL DEFAULT '',
  model VARCHAR(128) NOT NULL DEFAULT '',
  target_rel VARCHAR(1024) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT '',
  task_id VARCHAR(191) NOT NULL DEFAULT '',
  error TEXT NOT NULL,
  last_error TEXT NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  batch_id VARCHAR(191) NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  started_at BIGINT NOT NULL DEFAULT 0,
  finished_at BIGINT NOT NULL DEFAULT 0,
  session_key VARCHAR(191) NOT NULL DEFAULT '',
  purpose VARCHAR(512) NOT NULL DEFAULT '',
  feed_extra MEDIUMTEXT NOT NULL,
  step_done INT NOT NULL DEFAULT 0,
  step_total INT NOT NULL DEFAULT 0,
  progress_hint VARCHAR(512) NOT NULL DEFAULT '',
  goal_max_rounds INT NOT NULL DEFAULT 0,
  goal_round INT NOT NULL DEFAULT 0,
  goal_next TEXT NOT NULL,
  steer_pending TEXT NOT NULL,
  sort_ord INT NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("jobs")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  project_id VARCHAR(512) NOT NULL,
  job_id VARCHAR(191) NOT NULL,
  idx INT NOT NULL,
  rel VARCHAR(1024) NOT NULL DEFAULT '',
  title VARCHAR(512) NOT NULL DEFAULT '',
  prompt MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT '',
  task_id VARCHAR(191) NOT NULL DEFAULT '',
  error TEXT NOT NULL,
  PRIMARY KEY (project_id, job_id, idx),
  CONSTRAINT fk_%s_jobs FOREIGN KEY (project_id, job_id) REFERENCES %s(project_id, id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("job_steps"), t("job_steps"), t("jobs")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  project_id VARCHAR(512) NOT NULL,
  session_key VARCHAR(191) NOT NULL,
  seq INT NOT NULL,
  role VARCHAR(32) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  feedforward MEDIUMTEXT NOT NULL,
  tool_call_id VARCHAR(191) NOT NULL DEFAULT '',
  tool_calls_json MEDIUMTEXT NOT NULL,
  resource_ids_json MEDIUMTEXT NOT NULL,
  task_id VARCHAR(191) NOT NULL DEFAULT '',
  created_at_ms BIGINT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_hist_sess_seq (project_id, session_key, seq),
  KEY idx_history_message_session (project_id, session_key, seq),
  KEY idx_history_message_task (project_id, task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("history_message")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  project_id VARCHAR(512) NOT NULL,
  session_key VARCHAR(191) NOT NULL DEFAULT '',
  task_id VARCHAR(191) NOT NULL DEFAULT '',
  tool_call_id VARCHAR(191) NOT NULL DEFAULT '',
  tool_name VARCHAR(128) NOT NULL DEFAULT '',
  kind VARCHAR(64) NOT NULL DEFAULT '',
  phase VARCHAR(64) NOT NULL DEFAULT '',
  rel_path VARCHAR(1024) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT '',
  summary TEXT NOT NULL,
  body MEDIUMTEXT NOT NULL,
  created_at_ms BIGINT NOT NULL DEFAULT 0,
  KEY idx_resource_session (project_id, session_key, created_at_ms),
  KEY idx_resource_call (project_id, tool_call_id),
  KEY idx_resource_path (project_id, rel_path(191)),
  KEY idx_resource_kind (project_id, kind)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("resource")),

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id VARCHAR(191) NOT NULL,
  project_id VARCHAR(512) NOT NULL,
  scope VARCHAR(64) NOT NULL DEFAULT 'skill',
  anchors JSON NOT NULL,
  body MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  source_task_id VARCHAR(191) NOT NULL DEFAULT '',
  parent_task_id VARCHAR(191) NOT NULL DEFAULT '',
  source_session_key VARCHAR(191) NOT NULL DEFAULT '',
  supersedes_id VARCHAR(191) NOT NULL DEFAULT '',
  acked_at_ms BIGINT NOT NULL DEFAULT 0,
  created_at_ms BIGINT NOT NULL,
  updated_at_ms BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id),
  KEY idx_lesson_scope_status (project_id, scope, status),
  KEY idx_lesson_source_task (project_id, source_task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, t("lesson_entry")),
	}
}
