# ningharness

**SQLite-backed agent host foundation** / **带 SQLite 的 Agent 宿主地基.**

Owns world truth and the tool gate—not how the model thinks.  
管世界真相与工具闸，不管模型怎么想。

Products (e.g. AgentDesk) layer turn pipelines, UI, and Guests (wnai / Cursor / …) on top. This module can also embed in a CLI or another desktop shell.  
产品（如 AgentDesk）在上面挂管道、UI、Guest（wnai / Cursor / …）；本库也可嵌入 CLI 或其它桌面壳。

## Positioning / 定位

| In scope / 本库负责 | Out of scope / 本库不负责 |
|---------------------|---------------------------|
| Durable state in `desk.db` / desk.db 耐久状态 | Turn pipeline (turnpipe) / 回合管道 |
| Workspace I/O and MCP core tools / 工作区写盘与 MCP 核工具 | Model loop / Eino / 模型循环 |
| Skill on-disk contract, lesson memory / Skill 磁盘契约、lesson 经验 | Product Skill pack copy / 产品 Skill packs 文案 |
| Job / Task scheduling and ledgers / Job·Task 调度与台账 | Git, pins, semantic recall, … / Git、pins、语义召回等产品扩展 |

Tool truth lives in the host: Guests change the world only through ToolHost/MCP—no bypass writes to disk. Lessons require human ack before they count as owned experience—growth over silent automation.  
工具真相在宿主：Guest 经 ToolHost/MCP 动世界，禁止旁路直写盘。经验（lesson）需人 ack 后才算认账——引导成长，而非无人值守代劳。

## Glossary / 术语

| Term | 含义 |
|------|------|
| **Task** | 单次 Agent 执行台账（history、steps、status）；一次模型回合或一次工具链执行的可追溯记录 |
| **Job** | 队列中的调度单元；可含多步 prompt，由 Executor 驱动 Task |
| **Skill** | 磁盘上的方法包契约（`system/skills/<id>/`）；流程与约束，非内置 packs |
| **Lesson** | 经验条目（skill / project / personal 作用域）；人 ack 后才计入认账经验 |

## Packages / 模块

```text
ningharness/
  Open · Close · UseProject     Harness facade / Harness 门面
  deskdb                        single desk database / 唯一台面库
  session · history · resource  working memory / 工作记忆
  task · job                    run ledger / queue (Executor injected) / 台账与队列
  lesson                        experience SSOT (skill|project|personal) / 经验 SSOT
  skill                         system/skills contract (no builtin packs) / 磁盘契约（无内置 packs）
  workspace                     workspace I/O (writetoken, pathsort, docwords) / 工作区
  toolhost                      MCP: Arm/Invoke/HTTP + core tools / MCP 核
  protocol                      shared event & tree types / 共享事件与树类型
```

Core tools: files, session, skill contract, lesson, task, queue.  
核工具：文件、session、skill 契约、lesson、task、queue。

Extensions (git, pins, image gen, builtin packs, …) are `Register`ed by the product.  
扩展工具（git、pins、生图、内置 packs…）由产品 `Register`。

## Quick start / 快速使用

```go
h, err := ningharness.Open(ningharness.Opts{DataDir: "./data"})
if err != nil {
    log.Fatal(err)
}
defer h.Close()

if err := h.UseProject("/path/to/project"); err != nil {
    log.Fatal(err)
}

h.Job.SetExecutor(func(ctx context.Context, j job.Job) (taskID string, err error) {
    // caller's turn (pipeline / external agent)
    // 调用方自己的一回合（管道 / 外置 Agent）
    return taskID, nil
})

// optional MCP / 可选 MCP
th := toolhost.New(workspace.New(), h.Session)
th.Queue = h.Job
toolhost.RegisterCoreTools(mcpServer, th)
// product registers extensions, then Listen HTTP
// 产品再 Register 扩展工具后 Listen HTTP
```

## With AgentDesk / 与 AgentDesk

Local development / 开发期：

```go
// agentdesk/go.mod
require ningharness v0.0.0
replace ningharness => ../ningharness
```

Desk calls `ningharness.Open`, embeds `toolhost.ToolHost`, and keeps turnpipe plus product tools in-tree.  
Desk：`ningharness.Open` + embed `toolhost.ToolHost`；turnpipe 与产品工具留在 Desk。

## Develop / 开发

```bash
go test ./...
```

Requires Go 1.25+. / 需要 Go 1.25+。

---

**GitHub About**

> SQLite-backed agent host: desk.db, workspace, MCP core tools, skill/lesson contracts—not the model loop or product UI.  
> 带 SQLite 的 Agent 宿主地基：desk.db、工作区、MCP 核工具、Skill/经验契约；不管模型循环与产品 UI。
