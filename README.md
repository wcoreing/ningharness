# ningharness

**English** | [中文](README.zh-CN.md)

**Pure Go backend framework** for hosting agents (SQLite + Gateway/MCP).  
No Wails, Node, or git required—only Go 1.25+.

Owns world truth and the tool gateway—not how the model thinks.

## Positioning

| In scope | Out of scope |
|----------|--------------|
| Durable state in `store` (`desk.db`) | Desktop / UI shells |
| Workspace I/O + **Gateway** MCP core tools | Product-specific extensions (VCS UI, pins, …) |
| Skill contract + Lesson memory | Product Skill pack catalogs |
| Job / Task ledgers | — |
| **Lifecycle** (default steps + event bus) | — |
| **Optional defaults**: MCP server + Eino Guest | You may replace or disable them |

Tool truth lives in the host: Guests change the world only through Gateway—no bypass disk writes. Lessons need human ack.  
Each Chat / Job run follows package **`lifecycle`**. Phase boundaries emit on an **event bus** (`On` gate / `Watch` observe) with turn context `RunState`. MCP tools are not lifecycle steps—they run inside `run_guest` via Gateway.

## Glossary

| Term | Meaning |
|------|---------|
| **Harness** | Facade: DB + Session + Job |
| **Lifecycle** | Fixed-phase pipeline for one Task / Chat turn (ordered steps + Bus) |
| **Bus / Event** | Lifecycle events: `Phase × before\|after`; `On` may Block, `Watch` observes only |
| **RunState** | Per-turn pipeline context shared by steps and handlers |
| **Step** | Domain atom (e.g. `begin_task`, `run_guest`); **not** a tool name |
| **Gateway** | Tool gateway + MCP core (register / authorize / invoke) |
| **Guest** | Model loop (default: Eino; swappable) |
| **Task** | One execution ledger |
| **Job** | Queue unit (schedules when to run a lifecycle; may span multiple tasks) |
| **Goal** | Outer-loop Job: re-trigger the lifecycle until `GOAL.yaml` status is terminal |
| **Trace** | Append-only JSONL under `.ningharness/traces/` per Task; resume = paired tool_call/tool_result + task_end |
| **Skill** | On-disk method pack contract |
| **Lesson** | Experience entry (ack to own) |

## Packages (by layer)

```text
ningharness/           facade Open/Close/UseProject
  lifecycle/           one turn: steps + Bus + RunState (state.go) + Runner
  toolgateway/         Gateway + MCP + tool Registry; turn* = projection of RunState
  workspace/ protocol/ files / shared DTOs
  store/ session/ history/ resource/
  task/ job/ goal/ trace/
  skill/ lesson/       ledgers & memory
  guest/ (+ eino/)     how the model thinks
  defaults/            wiring (Host projects RunState → ToolGateway + MCP + Eino)
  examples/
```

Dependency rule: `defaults` → lifecycle / toolgateway / guest; `lifecycle` does not import `toolgateway` (Host injected).

Turn projection: `begin_task` → `Gateway.ProjectTurn`; teardown only via `OnExit` → `FinishTurn` (`end_task` is a Bus hook).  
`runLifecycle` may wrap ctx with `WithRunState` for Host steps; **Guest stays free of RunState** — tool turn identity is the Gateway projection, not ctx through the model stack.  
`assemble_context` persists the user row and honors `RunState.Feedforward` (Job `FeedExtra` is mapped in).  
Tool dispatch: `RegisterHandler` + core `ensureCoreHandlers`; product tools need not edit `CallNamedTool`.

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

Defaults wire **Gateway** core tools + **MCP HTTP** (`/mcp`) + **Eino Guest** (ReAct via `Gateway.Invoke`).  
`WithoutEino` / `MCPAddr: "off"` / `SetGuest` turn pieces off or replace them. Empty Eino fields fall back to `NINGHARNESS_API_KEY`, `NINGHARNESS_BASE_URL`, `NINGHARNESS_MODEL` (or `OPENAI_*`).

## Integrate

```go
require ningharness v0.0.0
replace ningharness => ../ningharness
```

Embed `toolgateway.Gateway` for product tools, or `defaults.Open` + `SetGuest`.

## Develop

```bash
go test ./...
```

Requires Go 1.25+ only.

## License

[MIT](LICENSE)

---

**GitHub About**

> Pure Go agent host: desk.db, Gateway/MCP core tools, skill/lesson; optional Eino Guest. No UI.
