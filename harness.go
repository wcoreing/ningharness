// Package ningharness 是 Agent 宿主门面：Open / UseProject / Close（Store + Session + Job）。
//
// 层划分（扁平包，靠依赖方向表达架构）：
//   - lifecycle：一轮怎么跑（步骤 + Bus + RunState）；前馈见 RunState.Feedforward
//   - toolgateway / workspace：Gateway 与文件世界（turn* 为 RunState 投影）
//   - store / session / history / resource / task / job / goal / trace：Store
//   - skill / lesson / memory：Skill Slot + Memory 插槽（默认 Disk / Lesson）
//   - guest：Guest 插槽（Run；前馈经 guest.Wire）
//   - defaults：可选装配（Lifecycle Host + MCP + Eino + Memory + Skill）；examples 为示例客户端
package ningharness

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wcoreing/ningharness/store"
	"github.com/wcoreing/ningharness/job"
	"github.com/wcoreing/ningharness/session"
)

// Opts Open 参数（通用宿主配置；不含业务域名）。
type Opts struct {
	// DataDir sqlite 下 desk.db 目录；也可作嵌入方工作区根。mysql 忽略库路径。
	DataDir string
	// Root 当前项目根；可空，之后 UseProject。
	Root string
	// Driver sqlite（默认）| mysql。
	Driver string
	// DSN mysql 连接串；与 DB 二选一。
	DSN string
	// DB 宿主注入的连接池（如与产品同实例）；不由此 Close。
	DB *sql.DB
	// TablePrefix 表前缀；mysql 默认 nh_，与业务表隔离。
	TablePrefix string
	// ProjectKey 逻辑项目键；嵌入同库时建议设稳定键，避免本机路径当 id。
	ProjectKey string
}

// Harness 地基句柄。
type Harness struct {
	DB      *sql.DB
	Session *session.Store
	Job     *job.Manager
	dataDir string
	root    string
}

// Open 打开台面库并装配 Session / Job（Job.Executor 须由调用方 SetExecutor）。
func Open(opts Opts) (*Harness, error) {
	store.Configure(store.Config{
		Driver:      opts.Driver,
		DSN:         opts.DSN,
		DB:          opts.DB,
		TablePrefix: opts.TablePrefix,
		DataDir:     opts.DataDir,
		ProjectKey:  opts.ProjectKey,
	})
	var (
		db  *sql.DB
		err error
	)
	dir := strings.TrimSpace(opts.DataDir)
	if store.IsMySQL() {
		store.SetEmbedDataDir(dir) // 工作区仍可对齐 DataDir；库走 mysql
		db, err = store.Open()
	} else if dir == "" {
		store.SetEmbedDataDir("")
		db, err = store.Open()
	} else {
		store.SetEmbedDataDir(dir)
		db, err = store.OpenAt(dir)
	}
	if err != nil {
		store.SetEmbedDataDir("")
		return nil, err
	}
	h := &Harness{
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
func (h *Harness) Close() error {
	if h == nil {
		return nil
	}
	if h.Job != nil {
		h.Job.Shutdown()
	}
	store.CloseAll()
	h.DB = nil
	return nil
}

// Root 当前项目根。
func (h *Harness) Root() string {
	if h == nil {
		return ""
	}
	return h.root
}

// UseProject 登记项目、打开项目迁移，并绑定 Job 到该根。
// 若 Open 时带了 DataDir，则库落在该目录（OpenProjectAt），避免嵌入方误写入家目录默认库。
func (h *Harness) UseProject(root string) error {
	if h == nil {
		return fmt.Errorf("ningharness: nil")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("ningharness: empty root")
	}
	var (
		db  *sql.DB
		err error
	)
	if dir := strings.TrimSpace(h.dataDir); dir != "" {
		db, err = store.OpenProjectAt(dir, root)
	} else {
		db, err = store.OpenProject(root)
	}
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
