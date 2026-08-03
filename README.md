# ningharness

**English** | [中文](README.zh-CN.md)

**Pure Go backend framework** for hosting agents (SQLite + ToolHost/MCP).  
No Wails, Node, or git required—only Go 1.25+.

Owns world truth and the tool gate—not how the model thinks.

## Positioning

| In scope | Out of scope |
|----------|--------------|
| Durable state in `store` (`desk.db`) | Desktop / UI shells |
| Workspace I/O + **ToolHost** MCP core tools | Product-specific extensions (VCS UI, pins, …) |
| Skill contract + Lesson memory | Product Skill pack catalogs |
| Job / Task ledgers | — |
| **Optional defaults**: MCP server + Eino Guest | You may replace or disable them |

Tool truth lives in the host: Guests change the world only through ToolHost—no bypass disk writes. Lessons need human ack.

## Glossary

| Term | Meaning |
|------|---------|
| **Harness** | Facade: DB + Session + Job |
| **ToolHost** | Tool gate + MCP core (register / arm / invoke) |
| **Guest** | Model loop (default: Eino; swappable) |
| **Task** | One execution ledger |
| **Job** | Queue unit (may span multiple tasks) |
| **Skill** | On-disk method pack contract |
| **Lesson** | Experience entry (ack to own) |

## Packages

```text
ningharness/           Harness Open/Close/UseProject
  store                SQLite (file still named desk.db)
  session history …    working memory
  task job lesson skill
  workspace toolhost protocol
  guest/               Guest interface
  guest/eino/          optional default Eino Guest
  defaults/            wire ToolHost + MCP + Eino (opt-in)
```

## Quick start

One complete path: edit Eino config → start MCP + Guest → send one message → paste Cursor URL.

Edit [`examples/chat/main.go`](examples/chat/main.go):

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
	APIKey:  "sk-...",                    // required for Guest
	BaseURL: "https://api.openai.com/v1", // gateway goes here
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
		// WithoutEino: true  → MCP only (no key)
		// MCPAddr: "off"     → Chat only (no HTTP)
		// MCPAddr: "127.0.0.1:51021" → fixed port
	})
	if err != nil {
		panic(err)
	}
	defer rt.Close()

	fmt.Println("MCP:", rt.MCPURL())
	// paste into ~/.cursor/mcp.json → {"mcpServers":{"ningharness":{"url":"<MCP URL>"}}}

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

Run:

```bash
go run ./examples/chat /path/to/project "List files briefly."
```

Defaults wire **ToolHost** core tools + **MCP HTTP** (`/mcp`) + **Eino Guest** (ReAct via `ToolHost.Invoke`).  
`WithoutEino` / `MCPAddr: "off"` / `SetGuest` turn pieces off or replace them. Empty Eino fields fall back to `NINGHARNESS_API_KEY`, `NINGHARNESS_BASE_URL`, `NINGHARNESS_MODEL` (or `OPENAI_*`).

## Integrate

```go
require ningharness v0.0.0
replace ningharness => ../ningharness
```

Embed `toolhost.ToolHost` for product tools, or `defaults.Open` + `SetGuest`.

## Develop

```bash
go test ./...
```

Requires Go 1.25+ only.

## License

[MIT](LICENSE)

---

**GitHub About**

> Pure Go agent host: desk.db, ToolHost/MCP core tools, skill/lesson; optional Eino Guest. No UI.
