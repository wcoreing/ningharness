// Package ningharness 带 SQLite 的 Agent 宿主地基（session / history / task / job / lesson）。
// 不含 turnpipe、Hub/MCP、Eino；产品管道与工具台由调用方提供。
package ningharness

import (
	"database/sql"
	"fmt"
	"strings"

	"ningharness/deskdb"
	"ningharness/job"
	"ningharness/session"
)

// Opts Open 参数。
type Opts struct {
	// DataDir 全局 desk.db 所在目录；空则 ~/.agentdesk。
	DataDir string
	// Root 当前项目根；可空，之后 UseProject。
	Root string
}

// H 地基句柄。
type H struct {
	DB      *sql.DB
	Session *session.Store
	Job     *job.Manager
	dataDir string
	root    string
}

// Open 打开台面库并装配 Session / Job（Job.Executor 须由调用方 SetExecutor）。
func Open(opts Opts) (*H, error) {
	var (
		db  *sql.DB
		err error
	)
	dir := strings.TrimSpace(opts.DataDir)
	if dir == "" {
		db, err = deskdb.Open()
	} else {
		db, err = deskdb.OpenAt(dir)
	}
	if err != nil {
		return nil, err
	}
	h := &H{
		DB:      db,
		Session: session.NewStore(),
		Job:     job.New(nil, nil),
		dataDir: dir,
	}
	if root := strings.TrimSpace(opts.Root); root != "" {
		if err := h.UseProject(root); err != nil {
			_ = h.Close()
			return nil, err
		}
	}
	return h, nil
}

// Close 关闭台面库连接缓存（进程内全局 cache）。
func (h *H) Close() error {
	if h == nil {
		return nil
	}
	if h.Job != nil {
		h.Job.Shutdown()
	}
	deskdb.CloseAll()
	h.DB = nil
	return nil
}

// Root 当前项目根。
func (h *H) Root() string {
	if h == nil {
		return ""
	}
	return h.root
}

// UseProject 登记项目、打开项目迁移，并绑定 Job 到该根。
func (h *H) UseProject(root string) error {
	if h == nil {
		return fmt.Errorf("ningharness: nil")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("ningharness: empty root")
	}
	db, err := deskdb.OpenProject(root)
	if err != nil {
		return err
	}
	h.DB = db
	h.root = root
	if h.Job != nil {
		h.Job.Bind(root)
	}
	return nil
}
