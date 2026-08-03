package deskdb

// CurrentSchemaVersion 当前统一库版本；升级走 numbered migrations。
const CurrentSchemaVersion = 21

// unifiedSchemaSQL 全新库：jobs=队列，tasks=执行台账；对话 SSOT=history_message；正文外置=resource。
const unifiedSchemaSQL = `
CREATE TABLE IF NOT EXISTS schema_version (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  root TEXT NOT NULL,
  updated_at_ms INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS meta (
  project_id TEXT NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (project_id, key)
);

CREATE TABLE IF NOT EXISTS sessions (
  project_id TEXT NOT NULL,
  id TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  orch_key TEXT NOT NULL DEFAULT '',
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);

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
  tool_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);
CREATE INDEX IF NOT EXISTS idx_tasks_ended ON tasks(project_id, ended_at_ms DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_job ON tasks(project_id, job_id);

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
  feed_extra TEXT NOT NULL DEFAULT '',
  step_done INTEGER NOT NULL DEFAULT 0,
  step_total INTEGER NOT NULL DEFAULT 0,
  progress_hint TEXT NOT NULL DEFAULT '',
  sort_ord INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (project_id, id)
);

CREATE TABLE IF NOT EXISTS job_steps (
  project_id TEXT NOT NULL,
  job_id TEXT NOT NULL,
  idx INTEGER NOT NULL,
  rel TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, job_id, idx),
  FOREIGN KEY (project_id, job_id) REFERENCES jobs(project_id, id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pins (
  project_id TEXT NOT NULL,
  path TEXT NOT NULL,
  sort_ord INTEGER NOT NULL DEFAULT 0,
  note TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (project_id, path)
);

CREATE TABLE IF NOT EXISTS pin_sessions (
  project_id TEXT PRIMARY KEY,
  baseline TEXT NOT NULL DEFAULT '',
  started TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS history_message (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id TEXT NOT NULL,
  session_key TEXT NOT NULL,
  seq INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  feedforward TEXT NOT NULL DEFAULT '',
  tool_call_id TEXT NOT NULL DEFAULT '',
  tool_calls_json TEXT NOT NULL DEFAULT '',
  resource_ids_json TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL DEFAULT 0,
  UNIQUE(project_id, session_key, seq)
);
CREATE INDEX IF NOT EXISTS idx_history_message_session ON history_message(project_id, session_key, seq);
CREATE INDEX IF NOT EXISTS idx_history_message_task ON history_message(project_id, task_id);

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
CREATE INDEX IF NOT EXISTS idx_resource_session ON resource(project_id, session_key, created_at_ms);
CREATE INDEX IF NOT EXISTS idx_resource_call ON resource(project_id, tool_call_id);
CREATE INDEX IF NOT EXISTS idx_resource_path ON resource(project_id, rel_path);
CREATE INDEX IF NOT EXISTS idx_resource_kind ON resource(project_id, kind);

CREATE TABLE IF NOT EXISTS growth_reflect (
  project_id TEXT PRIMARY KEY,
  at_ms INTEGER NOT NULL DEFAULT 0,
  task_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  lesson_rels_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS settings_blob (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  last_project TEXT NOT NULL DEFAULT '',
  recent_json TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS product_feedback (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  category TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  suggested_fix TEXT NOT NULL DEFAULT '',
  project_root TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_feedback_status ON product_feedback(status);

CREATE TABLE IF NOT EXISTS library_pack (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '1',
  tags_json TEXT NOT NULL DEFAULT '[]',
  summary TEXT NOT NULL DEFAULT '',
  source_project TEXT NOT NULL DEFAULT '',
  updated_at_ms INTEGER NOT NULL DEFAULT 0,
  enabled INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_library_pack_updated ON library_pack(updated_at_ms DESC);

CREATE TABLE IF NOT EXISTS lesson_entry (
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
);
CREATE INDEX IF NOT EXISTS idx_lesson_scope_status ON lesson_entry(project_id, scope, status);
CREATE INDEX IF NOT EXISTS idx_lesson_anchors_skill ON lesson_entry(project_id, json_extract(anchors, '$.skillId'), status);
CREATE INDEX IF NOT EXISTS idx_lesson_source_task ON lesson_entry(project_id, source_task_id);
`
