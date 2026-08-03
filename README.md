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

MCP and Guest are **independent**. MCP alone needs **no API key**. A key is only for the optional Eino Guest.

### MCP only (no Guest, no key)

```bash
go run ./examples/mcp /path/to/project
# prints MCP URL + a Cursor config snippet
# override: NINGHARNESS_MCP_ADDR=127.0.0.1:51021
```

Paste the printed URL into Cursor (`~/.cursor/mcp.json` or project `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "ningharness": {
      "url": "http://127.0.0.1:51020/mcp"
    }
  }
}
```

Use the URL from the terminal (port may change if 51020 is busy). Cursor uses its own model key; this process needs none.

```go
rt, err := defaults.Open(defaults.Opts{
	Opts:        ningharness.Opts{DataDir: "./data", Root: root},
	WithoutEino: true, // no Guest, no API key
})
// rt.MCPURL() → put in the url field above
```

### Optional: Eino Guest (needs key + optional base URL)

```bash
export NINGHARNESS_API_KEY=sk-...
export NINGHARNESS_BASE_URL=https://api.openai.com/v1   # or OPENAI_BASE_URL; gateways go here
# optional: NINGHARNESS_MODEL=gpt-4o-mini

go run ./examples/chat /path/to/project "List files briefly."
```

Or in code:

```go
rt, err := defaults.Open(defaults.Opts{
	Opts:    ningharness.Opts{DataDir: "./data", Root: root},
	MCPAddr: "off",
	Eino: eino.Opts{
		APIKey:  "sk-...",
		BaseURL: "https://api.openai.com/v1", // empty → NINGHARNESS_BASE_URL / OPENAI_BASE_URL
		Model:   "gpt-4o-mini",
	},
})
reply, err := rt.Chat(ctx, "List files briefly.")
```

Bring your own Guest (no Eino): `WithoutEino: true`, then `rt.SetGuest(myGuest)`.

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

Env (Eino Guest only): `NINGHARNESS_API_KEY` / `OPENAI_API_KEY`, `NINGHARNESS_BASE_URL` / `OPENAI_BASE_URL`, optional `NINGHARNESS_MODEL`. MCP itself never reads a key.

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

Requires Go 1.25+ only (no Wails / Node / git).

## License

[MIT](LICENSE)

---

**GitHub About**

> Pure Go agent host: desk.db, ToolHost/MCP core tools, skill/lesson; optional Eino Guest. No UI.
