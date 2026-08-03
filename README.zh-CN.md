# ningharness

[English](README.md) | **中文**

**纯 Go 后端框架**：Agent 宿主（SQLite + ToolHost/MCP）。  
不依赖 Wails、Node、git，只需 Go 1.25+。

管世界真相与工具闸，不管模型怎么想。

## 定位

| 本库负责 | 本库不负责 |
|----------|------------|
| `store` 持久状态（`desk.db`） | 桌面 / UI 壳 |
| 工作区 I/O + **ToolHost** MCP 核工具 | 产品扩展（版本管理 UI、pins 等） |
| Skill 契约 + Lesson 经验 | 产品 Skill 包目录 |
| Job / Task 台账 | — |
| **可选默认**：MCP 服务 + Eino Guest | 可关闭或自行替换 |

工具真相在宿主：Guest 只能经 ToolHost 改世界，禁止绕过写盘。经验需人 ack。

## 术语

| 术语 | 含义 |
|------|------|
| **Harness** | 门面：DB + Session + Job |
| **ToolHost** | 工具闸 + MCP 核（注册 / 授权 / 调用） |
| **Guest** | 模型循环（默认 Eino；可换） |
| **Task** | 一次执行台账 |
| **Job** | 队列单元（可跨多个 Task） |
| **Skill** | 落盘方法包契约 |
| **Lesson** | 经验条目（ack 后才算认账） |

## 模块

```text
ningharness/           Harness Open/Close/UseProject
  store                SQLite（文件名仍为 desk.db）
  session history …    工作记忆
  task job lesson skill
  workspace toolhost protocol
  guest/               Guest 接口
  guest/eino/          可选默认 Eino Guest
  defaults/            装配 ToolHost + MCP + Eino（可选启用）
```

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

默认装配 **ToolHost** 核工具 + **MCP HTTP**（`/mcp`）+ **Eino Guest**（经 `ToolHost.Invoke` 做 ReAct）。  
`WithoutEino` / `MCPAddr: "off"` / `SetGuest` 可关掉或替换。Eino 字段留空则读 `NINGHARNESS_API_KEY`、`NINGHARNESS_BASE_URL`、`NINGHARNESS_MODEL`（或 `OPENAI_*`）。

## 接入

```go
require ningharness v0.0.0
replace ningharness => ../ningharness
```

嵌入 `toolhost.ToolHost` 挂产品工具，或 `defaults.Open` + `SetGuest`。

## 开发

```bash
go test ./...
```

只需 Go 1.25+。

## 许可证

[MIT](LICENSE)

---

**GitHub About**

> 纯 Go Agent 宿主：台面库、ToolHost/MCP 核工具、Skill/经验；可选 Eino Guest。无 UI。
