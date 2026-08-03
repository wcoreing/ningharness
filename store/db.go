// Package store 台面 SQLite（默认文件名仍为 desk.db；路径可由 DataDir 配置）。
// 项目数据用 project_id 隔离；认账记忆（正文 / LESSONS）仍在文件系统。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	FileName      = "desk.db"
	cacheKey      = "__desk__"
	busyTimeoutMS = 15000
)

// sqliteDSN 单文件库：busy_timeout / WAL / FK 写入 DSN，保证池内每个连接都生效。
// MaxOpenConns 必须为 1：多连接并发写同一 desk.db 会 SQLITE_BUSY，导致入队后会话气泡丢失。
func sqliteDSN(path string) string {
	return fmt.Sprintf(
		"%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
		path, busyTimeoutMS,
	)
}

var (
	mu    sync.Mutex
	cache = map[string]*sql.DB{}
)

// ProjectPath 旧项目库路径（仅迁移用）。
func ProjectPath(root string) string {
	return filepath.Join(strings.TrimSpace(root), ".agentdesk", FileName)
}

// Path 唯一库路径 ~/.agentdesk/desk.db
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentdesk", FileName), nil
}

// ProjectID 项目键：绝对路径（稳定、可跨表过滤）。
func ProjectID(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return filepath.Clean(abs)
}

// Open 打开唯一台面库。
func Open() (*sql.DB, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	db, err := openCached(cacheKey, path)
	if err != nil {
		return nil, err
	}
	if err := migrateHomeJSON(db); err != nil {
		return nil, fmt.Errorf("deskdb migrate home json: %w", err)
	}
	return db, nil
}

// OpenAt 打开指定目录下的 desk.db（测试）。
func OpenAt(dir string) (*sql.DB, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Open()
	}
	return openCached("at:"+dir, filepath.Join(dir, FileName))
}

// OpenProject 打开唯一库并为该项目完成迁移（旧项目 desk.db / JSON）。
func OpenProject(root string) (*sql.DB, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("store: empty root")
	}
	db, err := openProjectDB(root)
	if err != nil {
		return nil, err
	}
	pid := ProjectID(root)
	if err := EnsureProject(db, root); err != nil {
		return nil, err
	}
	if err := migrateProjectInto(db, root, pid); err != nil {
		return nil, fmt.Errorf("deskdb migrate project: %w", err)
	}
	return db, nil
}

// OpenProjectAt 测试：库在 dir。
func OpenProjectAt(dir, root string) (*sql.DB, error) {
	db, err := OpenAt(dir)
	if err != nil {
		return nil, err
	}
	if err := EnsureProject(db, root); err != nil {
		return nil, err
	}
	pid := ProjectID(root)
	if err := migrateProjectInto(db, root, pid); err != nil {
		return nil, err
	}
	return db, nil
}

// EnsureProject 登记项目。
func EnsureProject(db *sql.DB, root string) error {
	pid := ProjectID(root)
	if pid == "" {
		return fmt.Errorf("store: empty project id")
	}
	_, err := db.Exec(`INSERT INTO projects(id, root, updated_at_ms) VALUES(?,?,strftime('%s','now')*1000)
		ON CONFLICT(id) DO UPDATE SET root=excluded.root, updated_at_ms=excluded.updated_at_ms`, pid, pid)
	return err
}

// OpenGlobal / OpenGlobalAt 兼容旧调用 → 唯一库。
func OpenGlobal() (*sql.DB, error) { return Open() }
func OpenGlobalAt(dir string) (*sql.DB, error) {
	return OpenAt(dir)
}

// CloseAll / ResetCacheForTest
func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for k, db := range cache {
		_ = db.Close()
		delete(cache, k)
	}
}
func ResetCacheForTest() { CloseAll() }

func openCached(key, path string) (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()
	if db, ok := cache[key]; ok {
		return db, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	// 唯一台面库：串行化访问。多 conn 时 busy_timeout 也挡不住写写冲突风暴。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	cache[key] = db
	return db, nil
}

func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		version INTEGER NOT NULL
	)`); err != nil {
		return err
	}
	var ver int
	err := db.QueryRow(`SELECT version FROM schema_version WHERE id=1`).Scan(&ver)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		ver = 0
	}
	if ver > CurrentSchemaVersion {
		return fmt.Errorf("store: schema version %d newer than supported %d", ver, CurrentSchemaVersion)
	}
	// 统一库前的残版：丢掉台面表后按最新 schema 重建（settings/app_state/feedback 保留）
	if ver > 0 && ver < 2 {
		if err := wipeIncompatible(db); err != nil {
			return err
		}
		ver = 0
	}
	if ver == 0 {
		if _, err := db.Exec(unifiedSchemaSQL); err != nil {
			return err
		}
		_, err = db.Exec(`INSERT INTO schema_version(id, version) VALUES(1, ?)
			ON CONFLICT(id) DO UPDATE SET version=excluded.version`, CurrentSchemaVersion)
		return err
	}
	for ver < CurrentSchemaVersion {
		next := ver + 1
		fn := schemaMigrations[next]
		if fn == nil {
			return fmt.Errorf("store: missing migration to v%d", next)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("deskdb migrate to v%d: %w", next, err)
		}
		if _, err := tx.Exec(`UPDATE schema_version SET version=? WHERE id=1`, next); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		ver = next
	}
	return nil
}

func wipeIncompatible(db *sql.DB) error {
	drops := []string{
		`DROP TABLE IF EXISTS history_message_fts`,
		`DROP TRIGGER IF EXISTS history_message_ai`,
		`DROP TRIGGER IF EXISTS history_message_ad`,
		`DROP TRIGGER IF EXISTS history_message_au`,
		`DROP TABLE IF EXISTS messages_fts`,
		`DROP TRIGGER IF EXISTS messages_ai`,
		`DROP TRIGGER IF EXISTS messages_ad`,
		`DROP TRIGGER IF EXISTS messages_au`,
		`DROP TABLE IF EXISTS messages`,
		`DROP TABLE IF EXISTS history_message`,
		`DROP TABLE IF EXISTS tool_payload`,
		`DROP TABLE IF EXISTS resource`,
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS run_events`,
		`DROP TABLE IF EXISTS run_tools`,
		`DROP TABLE IF EXISTS runs`,
		`DROP TABLE IF EXISTS tasks`,
		`DROP TABLE IF EXISTS job_steps`,
		`DROP TABLE IF EXISTS jobs`,
		`DROP TABLE IF EXISTS queue_steps`,
		`DROP TABLE IF EXISTS queue_tasks`,
		`DROP TABLE IF EXISTS pins`,
		`DROP TABLE IF EXISTS timeline_reviews`,
		`DROP TABLE IF EXISTS timeline_rounds`,
		`DROP TABLE IF EXISTS file_timeline`,
		`DROP TABLE IF EXISTS pin_sessions`,
		`DROP TABLE IF EXISTS review_sessions`,
		`DROP TABLE IF EXISTS growth_reflect`,
		`DROP TABLE IF EXISTS meta`,
		`DROP TABLE IF EXISTS projects`,
	}
	for _, q := range drops {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// MetaGet / MetaSet 按项目键值。
func MetaGet(db *sql.DB, projectID, key string) (string, error) {
	var v string
	err := db.QueryRow(`SELECT value FROM meta WHERE project_id=? AND key=?`, projectID, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func MetaSet(db *sql.DB, projectID, key, value string) error {
	_, err := db.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?,?,?)
		ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, projectID, key, value)
	return err
}
