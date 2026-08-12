package store

import (
	"database/sql"
	"strings"
	"sync"
)

// Driver 名称（通用，不含业务含义）。
const (
	DriverSQLite = "sqlite"
	DriverMySQL  = "mysql"
)

// Config 台面库配置。由嵌入方在 Open 前通过 Configure / OpenWith 注入。
// 禁止出现产品/业务字段；同实例共库时用 TablePrefix 隔离。
type Config struct {
	// Driver sqlite（默认）| mysql
	Driver string
	// DSN mysql 连接串；Driver=mysql 且 DB==nil 时使用。
	DSN string
	// DB 可选：由宿主注入的连接池（如产品已打开的 *sql.DB）。
	// 注入时 harness 不 Close 该池。
	DB *sql.DB
	// TablePrefix 表前缀。mysql 默认 "nh_"；sqlite 默认空（保持 desk.db 兼容）。
	TablePrefix string
	// DataDir sqlite 文件目录；mysql 忽略。
	DataDir string
	// ProjectKey 逻辑项目键；非空时覆盖 ProjectID(root) 的路径算法（嵌入多实例同库用）。
	ProjectKey string
}

var (
	cfgMu     sync.Mutex
	activeCfg Config
)

// Configure 设置进程内默认台面配置（Open / OpenProject 生效）。
// 传零值 Config 可复位为 sqlite 默认。
func Configure(c Config) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	activeCfg = normalizeConfig(c)
}

// ActiveConfig 当前配置副本。
func ActiveConfig() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return activeCfg
}

func normalizeConfig(c Config) Config {
	d := strings.ToLower(strings.TrimSpace(c.Driver))
	switch d {
	case "", DriverSQLite, "sqlite3":
		c.Driver = DriverSQLite
	case DriverMySQL, "mariadb":
		c.Driver = DriverMySQL
	default:
		c.Driver = d
	}
	c.DSN = strings.TrimSpace(c.DSN)
	c.DataDir = strings.TrimSpace(c.DataDir)
	c.ProjectKey = strings.TrimSpace(c.ProjectKey)
	c.TablePrefix = strings.TrimSpace(c.TablePrefix)
	if c.Driver == DriverMySQL && c.TablePrefix == "" {
		c.TablePrefix = "nh_"
	}
	return c
}

// DriverName 当前驱动。
func DriverName() string {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if activeCfg.Driver == "" {
		return DriverSQLite
	}
	return activeCfg.Driver
}

// IsMySQL 当前是否 MySQL 方言。
func IsMySQL() bool { return DriverName() == DriverMySQL }

// TablePrefix 当前表前缀。
func TablePrefix() string {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return activeCfg.TablePrefix
}

// T 逻辑表名 → 物理表名（加前缀）。
func T(logical string) string {
	p := TablePrefix()
	if p == "" {
		return logical
	}
	return p + logical
}

func configuredProjectKey() string {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return activeCfg.ProjectKey
}
