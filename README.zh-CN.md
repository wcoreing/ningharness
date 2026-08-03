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

MCP 与 Guest **互不绑定**：只起 MCP **不需要** API key；key 只在启用 Eino Guest 时用。

### 只起 MCP（不要 Guest，不要 key）

```bash
go run ./examples/mcp /path/to/project
# 终端会打印 MCP 地址 + Cursor 配置片段
# 指定端口: NINGHARNESS_MCP_ADDR=127.0.0.1:51021
```

把打印的 URL 写进 Cursor（`~/.cursor/mcp.json` 或项目 `.cursor/mcp.json`）：

```json
{
  "mcpServers": {
    "ningharness": {
      "url": "http://127.0.0.1:51020/mcp"
    }
  }
}
```

URL 以终端打印为准（端口被占用时会变）。配好后 Cursor 走自己的模型 key；本进程不设 key。

```go
rt, err := defaults.Open(defaults.Opts{
	Opts:        ningharness.Opts{DataDir: "./data", Root: root},
	WithoutEino: true, // 不创建 Guest，不读 API key
})
// rt.MCPURL() → 填进上面 url 字段
```

### 可选：启用 Eino Guest（才需要 key + 可选链接）

```bash
export NINGHARNESS_API_KEY=sk-...              # 或 OPENAI_API_KEY
export NINGHARNESS_BASE_URL=https://api.openai.com/v1   # 或 OPENAI_BASE_URL；兼容网关也写这里
# 可选: NINGHARNESS_MODEL=gpt-4o-mini

go run ./examples/chat /path/to/project "List files briefly."
```

代码里：

```go
rt, err := defaults.Open(defaults.Opts{
	Opts:    ningharness.Opts{DataDir: "./data", Root: root},
	MCPAddr: "off",
	Eino: eino.Opts{
		APIKey:  "sk-...",
		BaseURL: "https://api.openai.com/v1", // 空则读 NINGHARNESS_BASE_URL / OPENAI_BASE_URL
		Model:   "gpt-4o-mini",
	},
})
reply, err := rt.Chat(ctx, "List files briefly.")
```

自备 Guest（不绑 Eino）：`WithoutEino: true` 后 `rt.SetGuest(myGuest)`。

### 不用默认

```go
// 只要地基 — 无 MCP、无 Eino
h, _ := ningharness.Open(ningharness.Opts{DataDir: "./data", Root: root})
defer h.Close()

// 保留 MCP，不要 Eino
rt, _ := defaults.Open(defaults.Opts{
	Opts:        ningharness.Opts{DataDir: "./data", Root: root},
	WithoutEino: true,
})

// 不要 MCP HTTP
rt, _ := defaults.Open(defaults.Opts{
	Opts:    ningharness.Opts{DataDir: "./data", Root: root},
	MCPAddr: "off",
})

// 替换 Guest
rt.SetGuest(myGuest)
```

### 默认装配了什么

1. **ToolHost** + 核工具：`list_tree`、`read_file`、`write_file`、`grep`、`edit`，以及 session / skill / lesson / task / queue…
2. **MCP HTTP**（`/mcp`）— 同一套工具给 Cursor 或其他 MCP 客户端
3. **Eino Guest** — 经 `ToolHost.Invoke` 对上述部分工具做 ReAct

环境变量（仅 Eino Guest）：`NINGHARNESS_API_KEY` / `OPENAI_API_KEY`，`NINGHARNESS_BASE_URL` / `OPENAI_BASE_URL`，可选 `NINGHARNESS_MODEL`。MCP 本身不读 key。

## 接入

```go
require ningharness v0.0.0

// 本地旁路检出：
replace ningharness => ../ningharness
```

嵌入 `toolhost.ToolHost` 挂产品工具，或用 `defaults.Open` 再 `SetGuest` 换模型循环。

## 开发

```bash
go test ./...
```

只需 Go 1.25+（无 Wails / Node / git）。

## 许可证

[MIT](LICENSE)

---

**GitHub About**

> 纯 Go Agent 宿主：台面库、ToolHost/MCP 核工具、Skill/经验；可选 Eino Guest。无 UI。
