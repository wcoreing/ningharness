# ningharness

[English](README.md) | **中文**

**纯 Go 后端框架**：Agent 宿主（SQLite + Gateway/MCP）。  
不依赖 Wails、Node、git，只需 Go 1.25+。

管世界真相与工具网关，不管模型怎么想。

## 定位

| 本库负责 | 本库不负责 |
|----------|------------|
| **Store** 持久状态（`desk.db` 等） | **客户端** UI / 产品壳 |
| 工作区 I/O + **Gateway** MCP 核工具 | 客户端专有工具与交互 |
| Skill 契约 + Lesson / Memory | 客户端 Skill 包目录与策略 |
| Job / Task（属 Store） | — |
| **Lifecycle**（默认步骤 + 事件打孔） | — |
| **可选默认**：MCP 服务 + Eino Guest | 可关闭或自行替换 |

工具真相在宿主：Guest 只能经 Gateway 改世界，禁止绕过写盘。  
一轮 Chat / Job 由 **`lifecycle`** 定义；阶段边界经 **Bus** 打孔（`On` / `Watch`），载荷为 `RunState`。MCP 工具只发生在 `run_guest` 内（经 Gateway）。

## 标准结构

```text
【客户端 Client】调用方（嵌入本库的应用 / MCP 宿主 / 示例）
        │  Open · UseProject · RunTurn/Chat · 注册工具 · 订 Bus
        ▼
【ningharness 内核】
   Workspace · Store · Gateway · Lifecycle · RunState
   可换插槽：Guest · Memory · Skill
```

- **客户端**：调用方；不在本库内实现 UI。  
- **Store**：框架内持久状态（session / history / task / job / resource / lesson…）。  
- **前馈（Feedforward）**：`RunState` 字段，在 `assemble` 写入、进模给 Guest；不是整份 RunState。

## 术语

| 术语 | 含义 |
|------|------|
| **客户端 Client** | 调用方：嵌入或调用本库的一方 |
| **Harness** | 门面：Store + Session + Job |
| **Store** | 框架内持久状态（含 session / history / task / job / resource / lesson 等） |
| **Workspace** | 项目文件世界（相对根路径） |
| **Lifecycle** | 一次 Task / 一轮 Chat 的固定阶段管道（步骤 + Bus） |
| **Bus / Event** | 生命周期事件：`Phase × before\|after`；`On` 可 Block，`Watch` 只观察 |
| **RunState** | 本轮管道上下文；步骤与 Bus 共享；**Guest 不直接读** |
| **前馈 Feedforward** | `RunState` 上进模附加上下文；`assemble` 落 user 时带上 |
| **Step** | 生命周期领域原子（如 `begin_task`、`run_guest`）；**不是**工具名 |
| **Gateway** | 工具网关 + MCP 核（注册 / 授权 / 调用）；副作用唯一入口 |
| **Guest** | 模型循环插槽：`Run(Input{Message,Feedforward})`；默认 Eino；`guest.Chat` 为无前馈糖 |
| **Memory** | 记忆插槽：`assemble` 贡献前馈；可选 `Ingester`（`FileIngest` 写 JSONL）；默认 `memory.Lesson`；`NewLessonWithFileIngest` 组合 |
| **Skill** | 方法包插槽：`List` / `Match` / `Load`；默认 `skill.Disk`；`SetSkill` / `WithoutSkill` |
| **Lesson** | Store 中的经验条目；写入即默认可用（进前馈）；ack_lesson 用于历史未认账或再次确认 |
| **Task** | 一次执行记录（Store） |
| **Job** | 队列单元（调度何时跑 Lifecycle；可跨多个 Task） |
| **Goal** | 外环 Job：反复触发 Lifecycle 直到 `GOAL.yaml` status 终态 |
| **Trace** | Task 级 append-only JSONL（`.ningharness/traces/`）；恢复=已配对 tool_call/tool_result + task_end |

## 模块（按层）

```text
ningharness/           门面 Open/Close/UseProject
  lifecycle/           一轮：步骤 + Bus + RunState + Runner
  toolgateway/         Gateway + MCP + 工具 Registry；turn* = RunState 投影
  workspace/ protocol/ 文件世界 / 跨包 DTO
  store/ session/ history/ resource/   Store
  task/ job/ goal/ trace/
  skill/ lesson/       Skill Slot + Lesson（Store）
  memory/              Memory 插槽（Assemble + 可选 Ingest）
  guest/ (+ eino/)     Guest 插槽（Run）
  defaults/            装配（Lifecycle Host + Gateway 投影 + MCP + Eino + Memory + Skill）
  examples/            示例客户端
```

依赖方向：`defaults` → lifecycle / toolgateway / guest / memory / skill；`lifecycle` 不 import `toolgateway`（Host 反向注入）。

一轮投影：`begin_task` → `Gateway.ProjectTurn`；收尾唯一入口 `OnExit` → `FinishTurn`（`end_task` 只作 Bus 钩子）。  
`runLifecycle` 可为 Host 步骤 `WithRunState`；**Guest 不感知 RunState**——工具侧本轮身份只认 Gateway 投影。  
`assemble_context`：`skill.paths` 匹配补 skill id → 调用方前馈 + **Memory.Assemble** → `RunState.Feedforward` → 落 user。  
`run_guest`：`Guest.Run`（`guest.Wire` 合并前馈进模）。  
`persist_turn`：落 assistant；若 Memory 实现 **Ingester** 则 `Ingest`。  
工具分发：`RegisterHandler` / 核工具 `ensureCoreHandlers`；客户端扩展不必改 `CallNamedTool`。

## 快速开始

一条完整路径：改 Eino 配置 → 起 MCP + Guest → 发一句话 → 把 URL 贴进 Cursor。

编辑 [`examples/chat/main.go`](examples/chat/main.go)：

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ningharness"
	"ningharness/defaults"
	"ningharness/guest/eino"
)

var einoCfg = eino.Opts{
	APIKey:  "sk-...",                    // Guest 必填
	BaseURL: "https://api.openai.com/v1", // 网关写这里
	Model:   "gpt-4o-mini",
}

func main() {
	root, msg := ".", "List the project files in one short paragraph."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if len(os.Args) > 2 {
		msg = os.Args[2]
	}
	abs, _ := filepath.Abs(root)

	rt, err := defaults.Open(defaults.Opts{
		Opts: ningharness.Opts{DataDir: filepath.Join(abs, ".ningharness-data"), Root: abs},
		Eino: einoCfg,
		// WithoutEino: true  → 只要 MCP（不用 key）
		// MCPAddr: "off"     → 只要 Chat（不起 HTTP）
		// MCPAddr: "127.0.0.1:51021" → 固定端口
	})
	if err != nil {
		panic(err)
	}
	defer rt.Close()

	fmt.Println("MCP:", rt.MCPURL())
	// 贴进 ~/.cursor/mcp.json → {"mcpServers":{"ningharness":{"url":"<MCP URL>"}}}

	reply, err := rt.Chat(context.Background(), msg)
	if err != nil {
		panic(err)
	}
	fmt.Println(reply)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
```

运行：

```bash
go run ./examples/chat /path/to/project "List files briefly."
```

默认装配 **Gateway** 核工具 + **MCP HTTP**（`/mcp`）+ **Eino Guest**（经 `Gateway.Invoke` 做 ReAct）。  
`WithoutEino` / `MCPAddr: "off"` / `SetGuest` 可关掉或替换。Eino 字段留空则读 `NINGHARNESS_API_KEY`、`NINGHARNESS_BASE_URL`、`NINGHARNESS_MODEL`（或 `OPENAI_*`）。

## 接入

```go
require ningharness v0.0.0
replace ningharness => ../ningharness
```

嵌入 `toolgateway.Gateway` 挂产品工具，或 `defaults.Open` + `SetGuest`。

## 开发

```bash
go test ./...
```

只需 Go 1.25+。

## 许可证

[MIT](LICENSE)

---

**GitHub About**

> 纯 Go Agent 宿主：Store、Gateway/MCP 核工具、Skill/Memory；可选 Eino Guest。客户端自备 UI。
