# ningharness

带 SQLite 的 Agent 宿主地基（Go module）。

## 含什么

- `deskdb` — 唯一台面库
- `session` / `history` / `resource` — 工作记忆
- `task` / `job` — 执行台账与队列调度（Executor 外置）
- `lesson` — 经验 SSOT（需人 ack）
- `skill` — Skill 磁盘契约（不含产品 packs）
- `workspace` / `writetoken` / `pathsort` / `docwords` / `toolargs` / `contract` — 工作区与契约
- `hub` — MCP 核（Arm/Invoke/HTTP + 文件/session/skill/lesson/task/job 核工具）
- `Open` / `Close` / `UseProject` — 门面

产品扩展工具（git、pins、内置 packs、语义召回等）由调用方 Register；turnpipe / Eino 不在本模块。

## 使用

```go
import "ningharness"

h, err := ningharness.Open(ningharness.Opts{DataDir: "./data"})
if err != nil { ... }
defer h.Close()
_ = h.UseProject("/path/to/project")
h.Job.SetExecutor(func(ctx context.Context, j job.Job) (string, error) {
    return taskID, nil
})
```

AgentDesk：`replace ningharness => ../ningharness`，`mcpserver` embed `hub.Hub` 并注册 Desk 扩展工具。
