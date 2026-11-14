// Package store 台面持久层：默认 SQLite（desk.db）；可切换 MySQL（表前缀隔离，供嵌入方同实例共库）。
// 项目数据用 project_id 隔离；认账记忆（正文 / LESSONS）仍可在文件系统。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wcoreing/ningharness/metadir"

	_ "github.com/go-sql-driver/mysql"
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

type cachedConn struct {
	db    *sql.DB
	owned bool // false=宿主注入，CloseAll 不关
}

var (
	mu    sync.Mutex
	cache = map[string]*cachedConn{}

	embedMu      sync.Mutex
	embedDataDir string
)

// SetEmbedDataDir 嵌入方指定 desk.db 目录；空则清除。
// OpenProject / history / session 将与此目录对齐，避免误写家目录默认库。
func SetEmbedDataDir(dir string) {
	embedMu.Lock()
	defer embedMu.Unlock()
	embedDataDir = strings.TrimSpace(dir)
}

// EmbedDataDir 当前嵌入库目录（只读）。
func EmbedDataDir() string {
	embedMu.Lock()
	defer embedMu.Unlock()
	return embedDataDir
}

// ProjectPath 旧项目旁路库路径（仅迁移用）：root/.ningharness/desk.db。
func ProjectPath(root string) string {
	return filepath.Join(strings.TrimSpace(root), metadir.Name, FileName)
}

// Path 唯一库路径：~/.ningharness/desk.db。
func Path() (string, error) {
	dir, err := metadir.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// ProjectID 项目键。优先 Config.ProjectKey；无路径分隔的 root 视为逻辑键；否则绝对路径。
func ProjectID(root string) string {
	if k := configuredProjectKey(); k != "" {
		return k
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if !strings.ContainsAny(root, `/\`) {
		return root
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return filepath.Clean(abs)
}

// Open 打开唯一台面库（按 ActiveConfig）。
func Open() (*sql.DB, error) {
	cfg := ActiveConfig()
	if cfg.Driver == DriverMySQL {
		return openMySQL(cfg)
	}
	if dir := strings.TrimSpace(cfg.DataDir); dir != "" {
		return OpenAt(dir)
	}
	if dir := EmbedDataDir(); dir != "" {
		return OpenAt(dir)
	}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return openCachedSQLite(cacheKey, path)
}

// OpenAt 打开指定目录下的 desk.db（sqlite / 测试）。
func OpenAt(dir string) (*sql.DB, error) {
	if IsMySQL() {
		return Open()
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Open()
	}
	return openCachedSQLite("at:"+dir, filepath.Join(dir, FileName))
}

// OpenProject 打开唯一库并为该项目完成迁移（旧项目 desk.db / JSON；mysql 仅 EnsureProject）。
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
	if !IsMySQL() {
		if err := migrateProjectInto(db, root, pid); err != nil {
			return nil, fmt.Errorf("deskdb migrate project: %w", err)
		}
	}
	return db, nil
}

// OpenProjectAt 测试：库在 dir（sqlite）；mysql 忽略 dir。
func OpenProjectAt(dir, root string) (*sql.DB, error) {
	var (
		db  *sql.DB
		err error
	)
	if IsMySQL() {
		db, err = Open()
	} else {
		db, err = OpenAt(dir)
	}
	if err != nil {
		return nil, err
	}
	if err := EnsureProject(db, root); err != nil {
		return nil, err
	}
	pid := ProjectID(root)
	if !IsMySQL() {
		if err := migrateProjectInto(db, root, pid); err != nil {
			return nil, err
		}
	}
	return db, nil
}

// EnsureProject 登记项目。
func EnsureProject(db *sql.DB, root string) error {
	pid := ProjectID(root)
	if pid == "" {
		return fmt.Errorf("store: empty project id")
	}
	rootVal := strings.TrimSpace(root)
	if rootVal == "" {
		rootVal = pid
	}
	q := Upsert("projects",
		[]string{"id", "root", "updated_at_ms"},
		[]string{"id"},
		[]string{"root", "updated_at_ms"},
	)
	_, err := db.Exec(q, pid, rootVal, time.Now().UnixMilli())
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
	for k, c := range cache {
		if c.owned {
			_ = c.db.Close()
		}
		delete(cache, k)
	}
	SetEmbedDataDir("")
}
func ResetCacheForTest() {
	CloseAll()
	Configure(Config{})
}

func openMySQL(cfg Config) (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()
	key := "mysql:" + cfg.TablePrefix + ":" + cfg.DSN
	if cfg.DB != nil {
		key = fmt.Sprintf("mysql-inject:%p:%s", cfg.DB, cfg.TablePrefix)
	}
	if c, ok := cache[key]; ok {
		return c.db, nil
	}
	var (
		db    *sql.DB
		owned bool
		err   error
	)
	if cfg.DB != nil {
		db = cfg.DB
		owned = false
	} else {
		if cfg.DSN == "" {
			return nil, fmt.Errorf("store: mysql requires DSN or DB")
		}
		db, err = sql.Open("mysql", cfg.DSN)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Hour)
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, err
		}
		owned = true
	}
	if err := ensureSchemaMySQL(db); err != nil {
		if owned {
			_ = db.Close()
		}
		return nil, err
	}
	cache[key] = &cachedConn{db: db, owned: owned}
	return db, nil
}

func openCachedSQLite(key, path string) (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := cache[key]; ok {
		return c.db, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	cache[key] = &cachedConn{db: db, owned: true}
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
		`DROP TABLE IF EXISTS lesson_entry`,
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
	err := db.QueryRow(
		fmt.Sprintf(`SELECT value FROM %s WHERE project_id=? AND %s=?`, T("meta"), quoteMetaKeyCol()),
		projectID, key,
	).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func MetaSet(db *sql.DB, projectID, key, value string) error {
	q := Upsert("meta",
		[]string{"project_id", quoteMetaKeyCol(), "value"},
		[]string{"project_id", quoteMetaKeyCol()},
		[]string{"value"},
	)
	_, err := db.Exec(q, projectID, key, value)
	return err
}

func quoteMetaKeyCol() string {
	if IsMySQL() {
		return "`key`"
	}
	return "key"
}
