# ningharness

**English** | [中文](README.zh-CN.md)

**SQLite-backed agent host foundation.**

Owns world truth and the tool gate—not how the model thinks.

## Positioning

| In scope | Out of scope |
|----------|--------------|
| Durable state in `store` (`desk.db`) | Product turnpipe / UI |
| Workspace I/O + **ToolHost** MCP core tools | Product-specific extensions (git, pins, …) |
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

## Quick start — send one message

Defaults include **MCP core tools** + **Eino Guest**. Turn either off if you do not want them.

```bash
export NINGHARNESS_API_KEY=sk-...    # or OPENAI_API_KEY
# optional: OPENAI_BASE_URL  NINGHARNESS_MODEL=gpt-4o-mini

go run ./examples/chat /path/to/project "List files briefly."
```

Or in code:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"ningharness"
	"ningharness/defaults"
)

func main() {
	rt, err := defaults.Open(defaults.Opts{
		Opts: ningharness.Opts{
			DataDir: "./data",           // desk.db directory
			Root:    "/path/to/project", // project workspace
		},
		// MCPAddr: ""  → start MCP at 127.0.0.1:51020 with core tools
		// MCPAddr: "off" → no MCP HTTP
		// WithoutEino: true → no default Guest (bring your own)
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()

	fmt.Println("MCP endpoint:", rt.MCPURL()) // for Cursor etc.

	reply, err := rt.Chat(context.Background(), "List the project tree in one short paragraph.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(reply)
}
```

### Opt out

```go
// Core only — no MCP, no Eino
h, _ := ningharness.Open(ningharness.Opts{DataDir: "./data", Root: root})
defer h.Close()

// Defaults but no Eino (keep MCP tools for external agents)
rt, _ := defaults.Open(defaults.Opts{
	Opts:        ningharness.Opts{DataDir: "./data", Root: root},
	WithoutEino: true,
})

// Defaults but no MCP HTTP
rt, _ := defaults.Open(defaults.Opts{
	Opts:    ningharness.Opts{DataDir: "./data", Root: root},
	MCPAddr: "off",
})

// Replace Guest
rt.SetGuest(myGuest)
```

### What defaults wire

1. **ToolHost** + core tools: `list_tree`, `read_file`, `write_file`, `grep`, `edit`, session / skill / lesson / task / queue…
2. **MCP HTTP** (`/mcp`) — same tools for Cursor or other MCP clients
3. **Eino Guest** — ReAct over a subset of those tools via `ToolHost.Invoke`

Env: `NINGHARNESS_API_KEY` or `OPENAI_API_KEY`; optional `OPENAI_BASE_URL`, `NINGHARNESS_MODEL`.

## Integrate

```go
require ningharness v0.0.0

// local sibling checkout:
replace ningharness => ../ningharness
```

Embed `toolhost.ToolHost` for product-specific tools, or call `defaults.Open` and swap Guest with `SetGuest`.

## Develop

```bash
go test ./...
```

Requires Go 1.25+.

## License

[MIT](LICENSE)

---

**GitHub About**

> SQLite-backed agent host: desk.db, ToolHost/MCP core tools, skill/lesson; optional Eino Guest.
