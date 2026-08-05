# ningharness

[English](README.md) | **中文**

**纯 Go 后端框架**：Agent 宿主（SQLite + Gateway/MCP）。  
不依赖 Wails、Node、git，只需 Go 1.25+。

管世界真相与工具网关，不管模型怎么想。

## 定位

| 本库负责 | 本库不负责 |
|----------|------------|
| `store` 持久状态（`desk.db`） | 桌面 / UI 壳 |
| 工作区 I/O + **Gateway** MCP 核工具 | 产品扩展（版本管理 UI、pins 等） |
| Skill 契约 + Lesson 经验 | 产品 Skill 包目录 |
| Job / Task 台账 | — |
| **Lifecycle**（默认步骤 + 事件打孔） | — |
| **可选默认**：MCP 服务 + Eino Guest | 可关闭或自行替换 |

工具真相在宿主：Guest 只能经 Gateway 改世界，禁止绕过写盘。经验需人 ack。  
一轮 Chat / Job 执行由包 **`lifecycle`** 定义；阶段边界经 **事件总线**打孔（`On` Gate / `Watch` 观察），载荷为管道上下文 `RunState`。MCP 工具不在生命周期层，只发生在 `run_guest` 内。

## 术语

| 术语 | 含义 |
|------|------|
| **Harness** | 门面：DB + Session + Job |
| **Lifecycle** | 一次 Task / 一轮 Chat 的固定阶段管道（有序领域步骤 + Bus） |
| **Bus / Event** | 生命周期事件：`Phase × before\|after`；`On` 可 Block，`Watch` 只观察 |
| **RunState** | 一轮管道上下文（Turn Context）；事件与步骤共享 |
| **Step** | 生命周期领域原子（如 `begin_task`、`run_guest`）；**不是**工具名 |
| **Gateway** | 工具网关 + MCP 核（注册 / 授权 / 调用） |
| **Guest** | 模型循环（默认 Eino；可换） |
| **Task** | 一次执行台账 |
| **Job** | 队列单元（调度何时跑生命周期；可跨多个 Task） |
| **Goal** | 外环 Job：反复触发生命周期直到 `GOAL.yaml` status 终态（不是预知 N 节批写） |
| **Trace** | Task 级 append-only JSONL（`.ningharness/traces/`）；恢复契约=已配对 tool_call/tool_result + task_end |
| **Skill** | 落盘方法包契约 |
| **Lesson** | 经验条目（ack 后才算认账） |

## 模块（按层）

```text
ningharness/           门面 Open/Close/UseProject
  lifecycle/           一轮：步骤 + Bus + RunState（state.go）+ Runner
  toolgateway/         Gateway + MCP + 工具 Registry；turn* = RunState 投影
  workspace/ protocol/ 文件世界 / 跨包 DTO
  store/ session/ history/ resource/
  task/ job/ goal/ trace/
  skill/ lesson/       台账与记忆
  guest/ (+ eino/)     模型怎么想
  defaults/            装配层（Host 投影同步 ToolGateway + MCP + Eino）
  examples/
```

依赖方向：`defaults` → lifecycle / toolgateway / guest；`lifecycle` 不 import `toolgateway`（Host 反向注入）。

一轮投影：`begin_task` → `Gateway.ProjectTurn`；收尾唯一入口 `OnExit` → `FinishTurn`（`end_task` 只作 Bus 钩子）。  
`runLifecycle` 可为 Host 步骤 `WithRunState`；**Guest 不感知 RunState**——工具侧本轮身份只认 Gateway 投影，不经模型栈透传 ctx。  
`assemble_context` 落 user，并支持 `RunState.Feedforward`（Job 的 `FeedExtra` 会填入）。  
工具分发：`RegisterHandler` / 核工具 `ensureCoreHandlers`；产品扩展不必改 `CallNamedTool`。

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

> 纯 Go Agent 宿主：台面库、Gateway/MCP 核工具、Skill/经验；可选 Eino Guest。无 UI。
