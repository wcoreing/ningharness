package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestMigrateV2ToV5 模拟旧版 v2 库逐步升到 v5。
func TestMigrateV2ToV5(t *testing.T) {
	ResetCacheForTest()
	defer ResetCacheForTest()

	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.Exec(`PRAGMA foreign_keys=ON`)

	// 手工建 v2 形态
	if _, err := db.Exec(`
CREATE TABLE schema_version (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
INSERT INTO schema_version(id, version) VALUES(1, 2);
CREATE TABLE projects (id TEXT PRIMARY KEY, root TEXT NOT NULL, updated_at_ms INTEGER NOT NULL DEFAULT 0);
CREATE TABLE meta (project_id TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY (project_id, key));
CREATE TABLE runs (
  project_id TEXT NOT NULL, id TEXT NOT NULL,
  started_at_ms INTEGER NOT NULL DEFAULT 0, ended_at_ms INTEGER NOT NULL DEFAULT 0,
  driver TEXT NOT NULL DEFAULT '', session_id TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT '', task_id TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL DEFAULT '', reply TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok', error TEXT NOT NULL DEFAULT '',
  trail_json TEXT NOT NULL DEFAULT '[]',
  PRIMARY KEY (project_id, id)
);
CREATE TABLE file_timeline (
  project_id TEXT NOT NULL, rel TEXT NOT NULL, payload_json TEXT NOT NULL,
  PRIMARY KEY (project_id, rel)
);
`); err != nil {
		t.Fatal(err)
	}
	pid := "/tmp/proj-a"
	trail := `[{"kind":"text","text":"hi"},{"kind":"tool","name":"write_file","ok":true}]`
	if _, err := db.Exec(`INSERT INTO projects(id, root, updated_at_ms) VALUES(?,?,?)`, pid, pid, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO runs(project_id, id, trail_json, status) VALUES(?,?,?,?)`, pid, "r1", trail, "ok"); err != nil {
		t.Fatal(err)
	}
	payload := `{"rel":"a.md","rounds":[{"baseline":"abc","baselineShort":"abc","endedAt":"t","reviews":[{"hash":"h1","short":"h1","subject":"s","when":"w","index":1,"accepted":true}]}]}`
	if _, err := db.Exec(`INSERT INTO file_timeline(project_id, rel, payload_json) VALUES(?,?,?)`, pid, "a.md", payload); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)`, pid, "review_session_json", `{"baseline":"b","started":"s"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)`, pid, "last_reflect_json", `{"atMs":1,"summary":"grow","lessonRels":["skills/x/LESSONS.md"]}`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	// 经 OpenAt 触发 ensureSchema
	odb, err := OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ver int
	if err := odb.QueryRow(`SELECT version FROM schema_version WHERE id=1`).Scan(&ver); err != nil || ver != CurrentSchemaVersion {
		t.Fatalf("version=%d err=%v want %d", ver, err, CurrentSchemaVersion)
	}
	var toolCount int
	if err := odb.QueryRow(`SELECT tool_count FROM tasks WHERE project_id=? AND id=?`, pid, "r1").Scan(&toolCount); err != nil {
		t.Fatalf("tasks row missing err=%v", err)
	}
	if toolCount < 1 {
		t.Fatalf("tool_count=%d", toolCount)
	}
	var hasStepsCol int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='steps_json'`).Scan(&hasStepsCol)
	if hasStepsCol != 0 {
		t.Fatal("tasks.steps_json should be gone")
	}
	var hasSnapshotCol int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='snapshot_id'`).Scan(&hasSnapshotCol)
	if hasSnapshotCol != 0 {
		t.Fatal("tasks.snapshot_id should be gone")
	}
	var hasModelInputCol int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='model_input_id'`).Scan(&hasModelInputCol)
	if hasModelInputCol != 0 {
		t.Fatal("tasks.model_input_id should be gone")
	}
	var hasFF int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('history_message') WHERE name='feedforward'`).Scan(&hasFF)
	if hasFF != 1 {
		t.Fatal("history_message.feedforward missing")
	}
	var hasTrailCol int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='trail_json'`).Scan(&hasTrailCol)
	if hasTrailCol != 0 {
		t.Fatal("tasks.trail_json should be gone")
	}
	var hasToolsCol int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name='tools_json'`).Scan(&hasToolsCol)
	if hasToolsCol != 0 {
		t.Fatal("tasks.tools_json should be gone")
	}
	var hasTP int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tool_payload'`).Scan(&hasTP)
	if hasTP != 0 {
		t.Fatal("tool_payload should be gone")
	}
	var hasRes int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='resource'`).Scan(&hasRes)
	if hasRes == 0 {
		t.Fatal("resource table missing")
	}
	var histN int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM history_message WHERE project_id=? AND task_id=?`, pid, "r1").Scan(&histN)
	if histN < 1 {
		t.Fatalf("steps should migrate into history_message, histN=%d", histN)
	}
	var hasTrail int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('runs') WHERE name='trail_json'`).Scan(&hasTrail)
	if hasTrail != 0 {
		t.Fatal("trail_json should be gone")
	}
	var tl int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='timeline_rounds'`).Scan(&tl)
	if tl != 0 {
		t.Fatalf("timeline_rounds should be dropped, count=%d", tl)
	}
	var ft int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='file_timeline'`).Scan(&ft)
	if ft != 0 {
		t.Fatal("file_timeline should be dropped")
	}
	for _, tbl := range []string{"pin_sessions", "growth_reflect", "pins", "settings_blob", "app_state", "product_feedback", "library_pack"} {
		var n int
		_ = odb.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&n)
		if n != 0 {
			t.Fatalf("product table %s should not exist in framework schema", tbl)
		}
	}
	var metaBlob int
	_ = odb.QueryRow(`SELECT COUNT(*) FROM meta WHERE project_id=? AND key IN ('review_session_json','last_reflect_json')`, pid).Scan(&metaBlob)
	if metaBlob != 0 {
		t.Fatal("meta blobs should be promoted away")
	}
}
